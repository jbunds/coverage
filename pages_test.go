package main

import (
	"errors"
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
		modPath        string
		pkgDirCache    map[string]string
		profiles       []*cover.Profile
		mkdirAllFails  bool
		writeFileFails bool
		want           string
		wantErr        error
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
		modPath:     "foo",
		pkgDirCache: map[string]string{"foo/bar": "foo/bar"},
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
			`<link rel="icon"       href="../../favicon.ico" type="image/vnd.microsoft.icon">`,
			`<link rel="stylesheet" href="../../style.css">`,
			`<title>foo/bar/baz.go</title>`,
			`</head>`,
			`<body id="code" class="line-numbers">`,
			`<div class="line" data-line="1">package hello</div>`,
			`<div class="line" data-line="2"></div>`,
			`<div class="line" data-line="3">import &#34;fmt&#34;</div>`,
			`<div class="line" data-line="4"></div>`,
			`<div class="line" data-line="5">// line comment</div>`,
			`<div class="line" data-line="6"></div>`,
			`<div class="line" data-line="7">/* block</div>`,
			`<div class="line" data-line="8">	comment */</div>`,
			`<div class="line" data-line="9"></div>`,
			`<div class="func"><input type="checkbox" id="func-0" checked/>`,
			`  <label for="func-0" class="line" data-line="10">func hello() {</label>`,
			`  <div class="func-body">`,
			`    <div class="line" data-line="11"></div>`,
			`    <div class="line" data-line="12">	// another line comment</div>`,
			`    <div class="line" data-line="13"></div>`,
			`    <div class="line" data-line="14">	/* another</div>`,
			`    <div class="line" data-line="15">		block</div>`,
			`    <div class="line" data-line="16">		comment */</div>`,
			`    <div class="line" data-line="17"></div>`,
			`    <div class="line" data-line="18"><span class="hit">	fmt.Println(&#34;hello world&#34;)</span></div>`,
			`    <div class="line" data-line="19"></div>`,
			`    <div class="line" data-line="20">	// yet another line comment</div>`,
			`    <div class="line" data-line="21">}</div>`,
			`  </div>`,
			`</div>`,
			`<script src="../../child.js"></script>`,
			`</body>`,
			`</html>`}, "\n"),
	}, {
		name:     "source does not exist",
		fsys:     fstest.MapFS{},
		profiles: []*cover.Profile{{FileName: "foo.go"}},
		wantErr:  errors.New(`cannot read "foo.go": open foo.go: file does not exist`),
	}, {
		name:          "MkdirAll fails",
		fsys:          fstest.MapFS{"foo.go": &fstest.MapFile{}},
		profiles:      []*cover.Profile{{FileName: "foo.go"}},
		mkdirAllFails: true,
		wantErr:       errors.New(`cannot create directory "some/path": MkdirAll failed`),
	}, {
		name:           "WriteFile fails",
		fsys:           fstest.MapFS{"foo.go": &fstest.MapFile{Data: []byte("package foo")}},
		profiles:       []*cover.Profile{{FileName: "foo.go"}},
		writeFileFails: true,
		wantErr:        errors.New(`cannot write HTML file for "some/path/foo.go.html": WriteFile failed`),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mfs := &mockFS{
				FS:             tt.fsys,
				mkdirAllFails:  tt.mkdirAllFails,
				writeFileFails: tt.writeFileFails,
			}
			rg := &reportGenerator{
				write:            true,
				fsys:             mfs,
				outRoot:          &mockRoot{name: "some/path"},
				covState:         &coverageState{},
				profiles:         tt.profiles,
				pkgDirCache:      tt.pkgDirCache,
				iconFilename:     "favicon.ico",
				styleCSSFilename: "style.css",
				childJSFilename:  "child.js",
			}
			err := rg.writeCovHTMLFiles(t.Context(), io.Discard)
			if got, want := errStr(err), errStr(tt.wantErr); got != want {
        t.Errorf("writeCovHTMLFiles(%q) returned unexpected error:\ngot:  %v\nwant: %v", tt.name, got, want)
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
		modPaths      []string
		repoURLs      []string
		treeHTML      string
		openRootFails bool
		rootCloseFunc func() error
		createFails   bool
		want          string
		wantErr       error
	}{{
		name:          "single go.mod file",
		embeddedFiles: fstest.MapFS{"html/index.html": &fstest.MapFile{
			Data: []byte(strings.Join([]string{
				"title:      {{ .Title      }}",
				"headerHTML: {{ .HeaderHTML }}",
				"treeHTML:   {{ .TreeHTML   }}"}, "\n"),
			),
		}},
		modPaths:      []string{"github.com/foo/bar"},
		repoURLs:      []string{"https://github.com/foo/bar"},
		treeHTML:      "foo",
		want:          strings.Join([]string{
			"title:      Go test coverage // github.com/foo/bar",
			`headerHTML: <code><a target="_blank" href="https://github.com/foo/bar">github.com/foo/bar</a></code>`,
			`treeHTML:   foo`}, "\n",
		),
	}, {
		name:          "multiple go.mod files",
		embeddedFiles: fstest.MapFS{"html/index.html": &fstest.MapFile{
			Data: []byte(strings.Join([]string{
				`<title>{{ .Title }}</title>`,
				"<div class=\"centered\">\n{{ .HeaderHTML }}\n</div>",
				"<div id=\"tree-body\">\n{{ .TreeHTML }}\n</div>",
			}, "\n")),
		}},
		modPaths: []string{"github.com/foo/bar", "github.com/baz/boo"},
		treeHTML: "tree HTML",
		want:     strings.Join([]string{
			`<title>Go test coverage</title>`,
			`<div class="centered">`,
			`  <div class="dropdown-wrapper">`,
			`    <input type="checkbox" id="modules-menu"/>`,
			`    <label for="modules-menu" class="dropdown-toggle">modules</label>`,
			`    <div class="dropdown-menu">`,
			`      <label for="module-github-com-foo-bar" class="dropdown-link">github.com/foo/bar</label>`,
			`      <label for="module-github-com-baz-boo" class="dropdown-link">github.com/baz/boo</label>`,
			`    </div>`,
			`  </div>`,
			`</div>`,
			`<div id="tree-body">`,
			`tree HTML`,
			`</div>`}, "\n"),
	}, {
		name:          "template.ParseFS fails because index file does not exist",
		modPaths:      []string{"github.com/foo/bar", "github.com/baz/boo"},
		embeddedFiles: fstest.MapFS{},
		wantErr:       fmt.Errorf("cannot parse %q: template: pattern matches no files: `html/index.html`", "html/index.html"),
	}, {
		name:          "Create fails",
		modPaths:      []string{"foo"},
		repoURLs:      []string{"bar"},
		embeddedFiles: fstest.MapFS{"html/index.html": &fstest.MapFile{}},
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
			rg := &reportGenerator{
				fsys:          mfs,
				outRoot:       &mockRoot{name: "some/path"},
				modPaths:      tt.modPaths,
				repoURLs:      tt.repoURLs,
				embeddedFiles: tt.embeddedFiles,
			}
			gotErr := rg.writeIndexHTMLFile(tt.treeHTML)
			if got, want := errStr(gotErr), errStr(tt.wantErr); got != want {
				t.Errorf("writeIndexHTMLFile(%q) returned unexpected error:\ngot:  %v\nwant: %v", tt.name, got, want)
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
		tmplData      struct{ SomeVar string }
		want          string
		wantErr       error
	}{
		{
			name:          "succeeds",
			fileName:      "foo",
			embeddedFiles: fstest.MapFS{"foo": &fstest.MapFile{Data: []byte("someVar: {{ .SomeVar }}") }},
			tmplData:      struct{ SomeVar string }{SomeVar: "some var value"},
			want:          "someVar: some var value",
		},
		{
			name:          "template.Execute fails",
			fileName:      "bar",
			embeddedFiles: fstest.MapFS{"bar": &fstest.MapFile{Data: []byte("NoSuchData: {{ .NoSuchData }}") }},
			want:          "NoSuchData: ",
			wantErr:       errors.New(
				`cannot render template: ` +
				`template: bar:1:15: ` +
				`executing "bar" at <.NoSuchData>: ` +
				`can't evaluate field NoSuchData in type struct { SomeVar string }`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mfs := &mockFS{}
			rg  := &reportGenerator{
				fsys:          mfs,
				outRoot:       &mockRoot{name: "some/path"},
				embeddedFiles: tt.embeddedFiles,
			}
			err := rg.writeTemplateFile(tt.fileName, tt.tmplData)
			if got, want := errStr(err), errStr(tt.wantErr); got != want {
				t.Errorf("writeTemplateFile(%q) returned unexpected error:\ngot:  %v\nwant: %v", tt.name, got, want)
			}
			if diff := cmp.Diff(tt.want, string(mfs.data)); diff != "" {
				t.Errorf("writeTemplateFile(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}
