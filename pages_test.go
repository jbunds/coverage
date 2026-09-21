package main

import (
	"fmt"
	"io"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/tools/cover"
)

func TestWriteCovHTMLFiles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		fsys           fs.FS
		modName        string
		pkgDirCache    map[string]string
		profiles       []*cover.Profile
		mkdirAllFails  bool
		writeFileFails bool
		want           string
		wantErr        bool
	}{{
		name: "succeeds",
		fsys: fstest.MapFS{
			"foo/bar/baz.go": &fstest.MapFile{
				Data: []byte(strings.Join([]string{
					`package hello`,
					``,
					`import "fmt"`,
					``,
					`// line comment`,
					``,
					`/* block`,
					`	comment */`,
					``,
					`func hello() {`,
					``,
					`	// another line comment`,
					``,
					`	/* another`,
					`		block`,
					`		comment */`,
					``,
					`	fmt.Println("hello world")`,
					``,
					`	// yet another line comment`,
					`}`,
					``,
				}, "\n")),
			},
		},
		modName:     "foo",
		pkgDirCache: map[string]string{ "foo/bar": "foo/bar" },
		profiles:    []*cover.Profile{{
			FileName:  "foo/bar/baz.go",
			Blocks: []cover.ProfileBlock{{
				StartLine: 18, StartCol: 2,
				EndLine:   19, EndCol:   1,
				NumStmt:    1, Count:    1,
			}},
		}},
		want: strings.Join([]string{
			`<!DOCTYPE html>`,
			`<html lang="en">`,
			`<head>`,
			`<meta charset="utf-8">`,
			`<link rel="stylesheet" href="../../style.css">`,
			`<title>foo/bar/baz.go</title>`,
			`</head>`,
			`<body id="code" class="line-numbers">`,
			`<div class="line">package hello</div>`,
			`<div class="line"></div>`,
			`<div class="line">import &#34;fmt&#34;</div>`,
			`<div class="line"></div>`,
			`<div class="line">// line comment</div>`,
			`<div class="line"></div>`,
			`<div class="line">/* block</div>`,
			`<div class="line">	comment */</div>`,
			`<div class="line"></div>`,
			`<div class="line">func hello() {</div>`,
			`<div class="line"></div>`,
			`<div class="line">	// another line comment</div>`,
			`<div class="line"></div>`,
			`<div class="line">	/* another</div>`,
			`<div class="line">		block</div>`,
			`<div class="line">		comment */</div>`,
			`<div class="line"></div>`,
			`<div class="line"><span class="hit">	fmt.Println(&#34;hello world&#34;)</span></div>`,
			`<div class="line"></div>`,
			`<div class="line">	// yet another line comment</div>`,
			`<div class="line">}</div>`,
			`<script src="../../child.js"></script>`,
			`</body>`,
			`</html>`}, "\n"),
	}, {
		name:     "source does not exist",
		fsys:     fstest.MapFS{},
		profiles: []*cover.Profile{{ FileName: "foo.go" }},
		wantErr:  true,
	}, {
		name:          "MkdirAll fails",
		fsys:          fstest.MapFS{ "foo.go": &fstest.MapFile{} },
		profiles:      []*cover.Profile{{ FileName: "foo.go" }},
		mkdirAllFails: true,
		wantErr:       true,
	}, {
		name:           "WriteFile fails",
		fsys:           fstest.MapFS{ "foo.go": &fstest.MapFile{} },
		profiles:       []*cover.Profile{{ FileName: "foo.go" }},
		writeFileFails: true,
		wantErr:        true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mfs := &mockFS{
				FS:             tt.fsys,
				mkdirAllFails:  tt.mkdirAllFails,
				writeFileFails: tt.writeFileFails,
			}
			repGen := &reportGenerator{
				fsys:             mfs,
				outRoot:          &mockRoot{name: "some/path"},
				profiles:         tt.profiles,
				pkgDirCache:      tt.pkgDirCache,
				styleCSSFilename: "style.css",
				childJSFilename:  "child.js",
			}
			err := repGen.writeCovHTMLFiles(t.Context(), io.Discard)
			if (err != nil) != tt.wantErr {
				t.Errorf("writeCovHTMLFiles(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			wantLines := strings.Split(strings.TrimSuffix(string(tt.want ), "\n"), "\n")
			gotLines  := strings.Split(strings.TrimSuffix(string(mfs.data), "\n"), "\n")
			var rep reporter
			if !cmp.Equal(wantLines, gotLines, cmp.Reporter(&rep)) {
				t.Errorf("writeCovHTMLFiles(%q) mismatch (-want +got):\n%s", tt.name, strings.Join(rep.diffs, "\n"))
			}
		})
	}
}

func TestWriteIndexHTMLFile(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name          string
		embeddedFiles fs.FS
		modName       string
		repoURL       string
		openRootFails bool
		rootCloseFunc func() error
		createFails   bool
		want          string
		wantErr       error
	}{{
		name:          "succeeds",
		embeddedFiles: fstest.MapFS{ "index.html": &fstest.MapFile{
			Data: []byte("ModName: {{ .ModName }}, ModURL: {{ .ModURL }}, TreeHTML: {{ .TreeHTML }}"),
		}},
		modName:       "github.com/foo/bar",
		repoURL:       "https://github.com/foo/bar",
		want:          "ModName: github.com/foo/bar, ModURL: https://github.com/foo/bar, TreeHTML: foo",
	}, {
		name:          "template.ParseFS fails because index file does not exist",
		embeddedFiles: fstest.MapFS{},
		wantErr:       fmt.Errorf("cannot parse %q: template: pattern matches no files: `index.html`", "index.html"),
	}, {
		name:          "Create fails",
		embeddedFiles: fstest.MapFS{ "index.html": &fstest.MapFile{} },
		createFails:   true,
		wantErr:       fmt.Errorf("cannot create %q: Create failed", "some/path/index.html"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mfs    := &mockFS{
				openRootFails: tt.openRootFails,
				createFails:   tt.createFails,
			}
			repGen := &reportGenerator{
				fsys:          mfs,
				outRoot:       &mockRoot{name: "some/path"},
				modName:       tt.modName,
				repoURL:       tt.repoURL,
				embeddedFiles: tt.embeddedFiles,
			}
			gotErr := repGen.writeIndexHTMLFile("index.html", "foo")
			if tt.wantErr == nil && gotErr != nil {
				t.Fatalf("unexpected error: %v", gotErr)
			}
			if tt.wantErr != nil {
				if gotErr == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if gotErr.Error() != tt.wantErr.Error() {
					t.Errorf("writeIndexHTMLFile(%q) returned unexpected error: got %q, want %q", tt.name, gotErr.Error(), tt.wantErr.Error())
				}
			}
			if diff := cmp.Diff(tt.want, string(mfs.data)); diff != "" {
				t.Errorf("writeIndexHTMLFile(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestWriteTemplateFile(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name          string
		embeddedFiles fs.FS
		fileName      string
		tmplData      struct{ VarExists string }
		want          string
		wantErr       bool
	}{
		{
			name:          "succeeds",
			fileName:      "foo",
			embeddedFiles: fstest.MapFS{ "foo": &fstest.MapFile{ Data: []byte("VarExists: {{ .VarExists }}") }},
			tmplData:      struct{ VarExists string }{ VarExists: "this var exists" },
			want:          "VarExists: this var exists",
		},
		{
			name:          "tmpl.Execute fails",
			fileName:      "bar",
			embeddedFiles: fstest.MapFS{ "bar": &fstest.MapFile{ Data: []byte("NoSuchData: {{ .NoSuchData }}") }},
			want:          "NoSuchData: ",
			wantErr:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mfs    := &mockFS{}
			repGen := &reportGenerator{
				fsys:          mfs,
				outRoot:       &mockRoot{name: "some/path"},
				embeddedFiles: tt.embeddedFiles,
			}
			err := repGen.writeTemplateFile(tt.fileName, tt.tmplData)
			if (err != nil) != tt.wantErr {
				t.Errorf("writeTemplateFile(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, string(mfs.data)); diff != "" {
				t.Errorf("writeTemplateFile(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}
