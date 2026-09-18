package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/tools/cover"
	"golang.org/x/tools/go/packages"
)

// fakes and mocks

type badWriter struct{}

func (w *badWriter) Write(_ []byte) (int, error) { return 0, errors.New("i refuse to write") }

type sliceWriter struct { data *[]byte }

func (w *sliceWriter) Write(p []byte) (int, error) {
	*w.data = append(*w.data, p...)
	return len(p), nil
}

type mockFS struct {
	fs.FS
	root           *mockRoot
	openRootFails  bool
	createFails    bool
	readDirFails   bool
	closeFails     bool
	mkdirAllFails  bool
	writeFileFails bool
	badWriter      bool
	data           []byte
}

func (m *mockFS) Create(_ string) (io.WriteCloser, error) {
	if m.createFails { return nil, errors.New("Create failed") }
	var w io.Writer
	if m.badWriter {
		w = &badWriter{}
	} else {
		w = &sliceWriter{data: &m.data}
	}
	return &mockFile{
		writer:     w,
		closeFails: m.closeFails,
	}, nil
}

func (m *mockFS) OpenRoot(name string) (rootHandle, error) {
	if m.openRootFails { return nil, errors.New("OpenRoot failed") }
	m.root = &mockRoot{name: name}
	return m.root, nil
}

func (m *mockFS) Open(name string) (fs.File, error) {
	return m.FS.Open(name)
}

func (m *mockFS) OpenWithContext(name string) (fs.File, error) {
	return m.Open(name)
}

func (m *mockFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if m.readDirFails { return nil, errors.New("ReadDir failed") }
	return fs.ReadDir(m.FS, name)
}

func (m *mockFS) MkdirAll(_ string, _ fs.FileMode) error {
	if m.mkdirAllFails { return errors.New("MkdirAll failed") }
	return nil
}

func (m *mockFS) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(m.FS, name)
}

func (m *mockFS) WriteFile(_ string, data []byte, _ fs.FileMode) error {
	if m.writeFileFails { return errors.New("WriteFile failed") }
	m.data = data
	return nil
}

func (m *mockFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(m.FS, name)
}

type mockFileInfo struct {
	mode fs.FileMode
	name string
}

func (m *mockFileInfo) Name()    string      { return m.name                   }
func (m *mockFileInfo) Size()    int64       { return 0                        }
func (m *mockFileInfo) Mode()    fs.FileMode { return m.mode                   }
func (m *mockFileInfo) ModTime() time.Time   { return time.Time{}              }
func (m *mockFileInfo) IsDir()   bool        { return m.mode & fs.ModeDir != 0 }
func (m *mockFileInfo) Sys()     any         { return nil                      }

type mockRoot struct {
	name      string
	closeFunc func() error // nil -> success; non-nil -> delegate
}

func (m *mockRoot) Close() error {
	if m.closeFunc != nil { return m.closeFunc() }
	return nil
}

func (m *mockRoot) Name() string {
	return m.name
}

type mockFile struct {
	writer     io.Writer
	closeFails bool
}

func (m *mockFile) Close() error {
	if m.closeFails { return errors.New("Close failed") }
	return nil
}

func (m *mockFile) Write(p []byte) (n int, err error) {
	return m.writer.Write(p)
}

// tests

func TestGetModName(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name    string
		fsys    fs.FS
		want    string
		wantErr bool
	}{
		{
			name: "succeeds",
			fsys: fstest.MapFS{ "go.mod": &fstest.MapFile{
				Data: []byte("module github.com/foo/bar"),
			}},
			want: "github.com/foo/bar",
		},
		{
			name:    "cannot read go.mod",
			fsys:    fstest.MapFS{},
			wantErr: true,
		},
		{
			name:    "cannot parse go.mod",
			fsys:    fstest.MapFS{ "go.mod": &fstest.MapFile{} },
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repGen := &reportGenerator{
				fsys: &mockFS{ FS: tt.fsys },
			}
			err := repGen.getModName(t.Context(), "go.mod")
			if (err != nil) != tt.wantErr {
				t.Errorf("getModName(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, repGen.modName); diff != "" {
				t.Errorf("getModName(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

type fakeRunner struct {
	stdout,
	stderr  string
	err     error
}

func (f *fakeRunner) Run(cmd *exec.Cmd) error {
	_, _ = io.WriteString(cmd.Stdout, f.stdout)
	_, _ = io.WriteString(cmd.Stderr, f.stderr)
	return f.err
}

func TestGetRemoteURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		runner  runner
		want    string
	}{
		{
			name:   "local SSH standard (SCP style)",
			runner: &fakeRunner{ stdout: "git@github.com:foo/bar.git" },
			want:   "https://github.com/foo/bar",
		},
		{
			name:   "local SSH standard (no extension)",
			runner: &fakeRunner{ stdout: "git@github.com:foo/bar" },
			want:   "https://github.com/foo/bar",
		},
		{
			name:   "local SSH explicit protocol",
			runner: &fakeRunner{ stdout: "ssh://git@github.com:foo/bar.git" },
			want:   "https://github.com/foo/bar",
		},
		{
			name:   "local HTTPS standard",
			runner: &fakeRunner{ stdout: "https://github.com/foo/bar.git" },
			want:   "https://github.com/foo/bar",
		},
		{
			name:   "GitHub CI runner (token authentication)",
			runner: &fakeRunner{ stdout: "https://x-access-token:ghp_1234567890@github.com/foo/bar.git" }, // #nosec G101 - false positive (hardcoded creds)
			want:   "https://github.com/foo/bar",
		},
		{
			name:   "GitHub CI runner (standard checkout)",
			runner: &fakeRunner{ stdout: "https://github.com/foo/bar.git" },
			want:   "https://github.com/foo/bar",
		},
		{
			name:    "git config fails",
			runner:  &fakeRunner{ err: errors.New("git config failed") },
			want:    "https://github.com/foo/bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repGen := &reportGenerator{modName: "github.com/foo/bar"}
			err    := repGen.getRemoteURL(t.Context(), tt.runner, "go.mod")
			if err != nil {
				t.Errorf("getRemoteURL(%q) returned unexpected error: %v", tt.name, err)
			}
			if diff := cmp.Diff(tt.want, repGen.repoURL); diff != "" {
				t.Errorf("getRemoteURL(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestGetAllPkgPaths(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		profilePath string
		fsys        fs.FS
		want        []string
		wantErr     bool
	}{
		{
			name:        "succeeds",
			profilePath: "cov.out",
			fsys:        fstest.MapFS{
				"cov.out": &fstest.MapFile{
					Data: []byte(strings.Join([]string{
						"mode: set",
						"github.com/foo/bar/baz.go:0",
						"invalid line",
						"github.com/foo/bar/boo/bug.go:0",
					}, "\n")),
				},
			},
			want: []string{
				"github.com/foo/bar",
				"github.com/foo/bar/boo",
			},
		},
		{
			name:        "fails",
			profilePath: "nope",
			fsys:        fstest.MapFS{},
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repGen := &reportGenerator{
				profilePath: tt.profilePath,
				fsys:        &mockFS{ FS: tt.fsys },
			}
			got, err := repGen.getAllPkgPaths(t.Context())
			if (err != nil) != tt.wantErr {
				t.Errorf("getAllPkgPaths(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got, cmpopts.SortSlices(strings.Compare)); diff != "" {
				t.Errorf("getAllPkgPaths(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestPrimePkgDirCache(t *testing.T) {
	t.Parallel()
	mockPkgLoader := func(_ *packages.Config, patterns ...string) ([]*packages.Package, error) {
		pkgs := make([]*packages.Package, len(patterns))
		for i, p := range patterns {
			if strings.Contains(p, "this/will/fail") {
				return nil, errors.New("packages.Load failed")
			}
			pkgs[i] = &packages.Package{
				PkgPath: p,
				GoFiles: []string{p + ".go"},
			}
		}
		return pkgs, nil
	}
	tests := []struct {
		name        string
		profilePath string
		fsys        fs.FS
		want        map[string]string
		wantErr     bool
	}{
		{
			name:        "succeeds",
			profilePath: "cov.out",
			fsys:        fstest.MapFS{
				"cov.out": &fstest.MapFile{
					Data: []byte(strings.Join([]string{
						"mode: set",
						"github.com/foo/bar/baz.go:0",
						"invalid line",
						"github.com/foo/bar/boo/bug.go:0",
					}, "\n")),
				},
			},
			want: map[string]string{
				"github.com/foo/bar":     "github.com/foo",
				"github.com/foo/bar/boo": "github.com/foo/bar",
			},
		},
		{
			name:        "cannot read coverage profile file",
			profilePath: "nope",
			fsys:        fstest.MapFS{},
			wantErr:     true,
		},
		{
			name:        "packages.Load fails",
			profilePath: "cov.out",
			fsys:        fstest.MapFS{
				"cov.out": &fstest.MapFile{
					Data: []byte(strings.Join([]string{
						"mode: set",
						"this/will/fail/fosho:0",
					}, "\n")),
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repGen := &reportGenerator{
				profilePath: tt.profilePath,
				fsys:        &mockFS{ FS: tt.fsys },
			}
			err := repGen.primePkgDirCache(t.Context(), mockPkgLoader)
			if (err != nil) != tt.wantErr {
				t.Errorf("primePkgDirCache(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			if tt.wantErr { return }
			if diff := cmp.Diff(tt.want, repGen.pkgDirCache); diff != "" {
				t.Errorf("primePkgDirCache(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

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
				fsys:         mfs,
				outRoot:      &mockRoot{name: "some/path"},
				profiles:     tt.profiles,
				pkgDirCache:  tt.pkgDirCache,
				styleCSSFile: "style.css",
				childJSFile:  "child.js",
			}
			err := repGen.writeCovHTMLFiles(t.Context(), io.Discard)
			if (err != nil) != tt.wantErr {
				t.Errorf("writeCovHTMLFiles(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			wantLines := strings.Split(strings.TrimSuffix(string(tt.want ), "\n"), "\n")
			gotLines  := strings.Split(strings.TrimSuffix(string(mfs.data), "\n"), "\n")
			if !cmp.Equal(wantLines, gotLines) {
				var rep reporter
				cmp.Equal(wantLines, gotLines, cmp.Reporter(&rep))
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
			gotErr := repGen.writeIndexHTMLFile(t.Context(), "index.html", "foo")
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
			err := repGen.writeTemplateFile(t.Context(), tt.fileName, tt.tmplData)
			if (err != nil) != tt.wantErr {
				t.Errorf("writeTemplateFile(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, string(mfs.data)); diff != "" {
				t.Errorf("writeTemplateFile(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestPrintCoverage(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name            string
		cov             map[string]coverage
		totalCovered    int64
		totalStatements int64
		want            string
		wantErr         bool
	}{
		{
			name: "succeeds",
			cov:  map[string]coverage{
				"foo":     { covered:  10, total: 100 },
				"bar/baz": { covered: 180, total: 200 },
				"boo":     { covered:  40, total:  40 },
			},
			totalCovered:     10 + 180 + 40,
			totalStatements: 100 + 200 + 40,
			want:            strings.Join([]string{
				"File    Coverage",
				"————————————————",
				"boo      100.00%",
				"foo       10.00%",
				"bar/baz   90.00%",
				"————————————————",
				"Total     67.65%" + "\n"}, "\n"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repGen := &reportGenerator{ cov: tt.cov }
			repGen.totalCovered.Store(tt.totalCovered)
			repGen.totalStatements.Store(tt.totalStatements)
			got := new(bytes.Buffer)
			err := repGen.printCoverage(t.Context(), got)
			if (err != nil) != tt.wantErr {
				t.Errorf("printCoverage(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got.String()); diff != "" {
				t.Errorf("printCoverage(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestWriteStaticFiles(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name          string
		embeddedFiles fs.FS
		staticFiles   []string
		createFails   bool
		closeFails    bool
		badWriter     bool
		wantErr       bool
		want          string
	}{
		{
			name:          "succeeds",
			embeddedFiles: fstest.MapFS{ "foo": &fstest.MapFile{ Data: []byte("bar") }},
			staticFiles:   []string{"foo"},
			want:          "bar",
		},
		{
			name:          "Create fails",
			embeddedFiles: fstest.MapFS{},
			staticFiles:   []string{"foo"},
			createFails:   true,
			wantErr:       true,
		},
		{
			name:          "ReadFile fails",
			embeddedFiles: fstest.MapFS{},
			staticFiles:   []string{"foo"},
			wantErr:       true,
		},
		{
			name:          "Close fails",
			embeddedFiles: fstest.MapFS{ "foo": &fstest.MapFile{}},
			staticFiles:   []string{"foo"},
			closeFails:    true,
			wantErr:       true,
		},
		{
			name:          "fmt.Fprint fails",
			embeddedFiles: fstest.MapFS{ "foo": &fstest.MapFile{ Data: []byte("bar") }},
			staticFiles:   []string{"foo"},
			badWriter:     true,
			wantErr:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mfs := &mockFS{
				createFails: tt.createFails,
				closeFails:  tt.closeFails,
				badWriter:   tt.badWriter,
			}
			repGen := &reportGenerator{
				fsys:          mfs,
				outRoot:       &mockRoot{name: "some/path"},
				embeddedFiles: tt.embeddedFiles,
				staticFiles:   tt.staticFiles,
			}
			err := repGen.writeStaticFiles(t.Context())
			if (err != nil) != tt.wantErr {
				t.Errorf("writeStaticFiles(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, string(mfs.data)); diff != "" {
				t.Errorf("writeStaticFiles(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestFilterArgs(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name string
		args []string
		want []string
	}{
		{
			name: "no extra args",
			args: []string{"-gomod", "foo", "-coverfile", "bar", "-path", "baz"},
			want: []string{"-gomod", "foo", "-coverfile", "bar", "-path", "baz"},
		},
		{
			name: "extra args",
			args: []string{"-gomod", "foo", "-coverfile", "bar", "-path", "baz", "--", "boo", "hoo"},
			want: []string{"boo", "hoo"},
		},
		{
			name: "invalid args",
			args: []string{"foo", "bar", "--", "baz", "boo"},
			want: []string{"baz", "boo"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := filterArgs(tt.args)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("filterArgs(%v) mismatch (-want +got):\n%s", tt.args, diff)
			}
		})
	}
}

// custom cmp reporter which renders []string (line) diffs without truncation

type reporter struct{
	path  cmp.Path
	diffs []string
}

func (r *reporter) PushStep(ps cmp.PathStep) {
	r.path = append(r.path, ps)
}

func (r *reporter) PopStep() {
	r.path = r.path[:len(r.path) - 1]
}

func (r *reporter) Report(rs cmp.Result) {
	if !rs.Equal() {
		want,    got    := r.path.Last().Values() 
		wantStr, gotStr := "<missing>", "<missing>"
		if want.IsValid() {
			wantStr = fmt.Sprintf("%v", want.Interface())
		}
		if got.IsValid() {
			gotStr = fmt.Sprintf("%v", got.Interface())
		}
		r.diffs = append(r.diffs, fmt.Sprintf("- %s\n+ %s", wantStr, gotStr))
	}
}
