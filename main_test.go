package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/tools/cover"
	"golang.org/x/tools/go/packages"
)

// fakes and mocks

type badWriter struct{}

func (w *badWriter) Write(_ []byte) (int, error) {
	return 0, fmt.Errorf("i refuse to write")
}

type mockFS struct {
	fs.FS
	createFails    bool
	readDirFails   bool
	closeFails     bool
	mkdirAllFails  bool
	writeFileFails bool
	badWriter      bool
	data           []byte
}

func (m *mockFS) Create(ctx context.Context, _ string) (io.WriteCloser, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	if m.createFails { return nil, fmt.Errorf("Create failed") }
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

func (m *mockFS) Open(name string) (fs.File, error) {
	return m.FS.Open(name)
}

func (m *mockFS) OpenWithContext(ctx context.Context, name string) (fs.File, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	return m.Open(name)
}

func (m *mockFS) ReadDir(ctx context.Context, dir string) ([]fs.DirEntry, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	if m.readDirFails { return nil, fmt.Errorf("ReadDir failed") }
	return fs.ReadDir(m.FS, dir)
}

func (m *mockFS) MkdirAll(ctx context.Context, _ string, _ fs.FileMode) error {
	if err := ctx.Err(); err != nil { return err }
	if m.mkdirAllFails { return fmt.Errorf("MkdirAll failed") }
	return nil
}

func (m *mockFS) ReadFile(ctx context.Context, name string) ([]byte, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	return fs.ReadFile(m, name)
}

func (m *mockFS) WriteFile(ctx context.Context, _ string, data []byte, _ fs.FileMode) error {
	if err := ctx.Err(); err != nil { return err }
	if m.writeFileFails { return fmt.Errorf("WriteFile failed") }
	m.data = data
	return nil
}

type sliceWriter struct {
	data *[]byte
}

func (w *sliceWriter) Write(p []byte) (int, error) {
	*w.data = append(*w.data, p...)
	return len(p), nil
}

type mockFile struct {
	writer     io.Writer
	closeFails bool
}

func (m *mockFile) Close() error {
	if m.closeFails { return fmt.Errorf("Close failed") }
	return nil
}

func (m *mockFile) Write(p []byte) (n int, err error) {
	return m.writer.Write(p)
}

type mockIniFileConfig struct {
	returnValue string
	err         error
}

func (m *mockIniFileConfig) Value(_, _ string) (string, error) {
	return m.returnValue, m.err
}

// tests

func TestGetModName(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name     string
		fsys     fs.FS
		want     string
		wantErr  bool
	}{
		{
			name: "succeeds",
			fsys: fstest.MapFS{ "go.mod": &fstest.MapFile{ Data: []byte("module github.com/foo/bar") }},
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
				t.Errorf("getModeName(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, repGen.modName); diff != "" {
				t.Errorf("getModName(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestGetRepoURL(t *testing.T) {
	tests := []struct {
		name    string
		gitCfg  *mockIniFileConfig
		fsys    fs.FS
		want    string
		wantErr bool
	}{
		{
			name: "GitHub CI environment variable set",
			want: "https://github.com/foo/bar",
		},
		{
			name:   "local SSH standard (SCP style)",
			gitCfg: &mockIniFileConfig{ returnValue: "git@github.com:foo/bar.git" },
			want:   "https://github.com/foo/bar",
		},
		{
			name:   "local SSH standard (no extension)",
			gitCfg: &mockIniFileConfig{ returnValue: "git@github.com:foo/bar" },
			want:   "https://github.com/foo/bar",
		},
		{
			name:   "local SSH explicit protocol",
			gitCfg: &mockIniFileConfig{ returnValue: "ssh://git@github.com:foo/bar.git" },
			want:   "https://github.com/foo/bar",
		},
		{
			name:   "local HTTPS standard",
			gitCfg: &mockIniFileConfig{ returnValue: "https://github.com/foo/bar.git" },
			want:   "https://github.com/foo/bar",
		},
		{
			name:   "GitHub CI runner (token authentication)",
			gitCfg: &mockIniFileConfig{ returnValue: "https://x-access-token:ghp_1234567890@github.com/foo/bar.git" }, // #nosec G101 - false positive
			want:   "https://github.com/foo/bar",
		},
		{
			name:   "GitHub CI runner (standard checkout)",
			gitCfg: &mockIniFileConfig{ returnValue: "https://github.com/foo/bar.git" },
			want:   "https://github.com/foo/bar",
		},
		{
			name:    "fails",
			gitCfg:  &mockIniFileConfig{ err: fmt.Errorf("inifile.IniConfig.Value failed") },
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "GitHub CI environment variable set" {
				t.Setenv("GITHUB_REPOSITORY", "foo/bar")
			}
			if tt.name == "GitHub CI environment variable set with trailing slash removal check" {
				t.Setenv("GITHUB_REPOSITORY", "foo/bar/")
			}
			repGen := &reportGenerator{
				fsys: &mockFS{ FS: tt.fsys },
			}
			err := repGen.getRepoURL(t.Context(), tt.gitCfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("getRepoURL(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, repGen.repoURL); diff != "" {
				t.Errorf("getRepoURL(%q) mismatch (-want +got):\n%s", tt.name, diff)
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
				return nil, fmt.Errorf("packages.Load failed")
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
	}{
		{
			name: "succeeds",
			fsys: fstest.MapFS{
				"foo/bar/baz.go": &fstest.MapFile{
					Data: []byte(strings.Join([]string{
						"package main",
						"",
						"import \"fmt\"",
						"",
						"func main() {",
						"  fmt.Println(\"hello world\")",
						"}\n",
					}, "\n"))}},
			modName:     "foo",
			pkgDirCache: map[string]string{ "foo/bar": "foo/bar" },
			profiles:    []*cover.Profile{{
				FileName:  "foo/bar/baz.go",
				Blocks: []cover.ProfileBlock{{
					StartLine: 5,
					StartCol:  13,
					EndLine:   7,
					EndCol:    1,
					NumStmt:   3,
					Count:     3,
				}},
			}},
			want: strings.Join([]string{
				"<!DOCTYPE html>",
				"<html lang=\"en\">",
				"<head>",
				"<meta charset=\"utf-8\">",
				"<link rel=\"stylesheet\" href=\"../../style.css\" type=\"text/css\">",
				"<title>foo/bar/baz.go</title>",
				"</head>",
				"<body id=\"code\">",
				"<pre>",
				"package main",
				"",
				"import &#34;fmt&#34;",
				"",
				"<span class=\"hit\">func main() {",
				"  fmt.Println(&#34;hello world&#34;)",
				"</span>}",
				"</pre>",
				"<script>",
				"try {",
				"  const parentTheme = window.parent.document.documentElement.getAttribute('theme');",
				"  if (parentTheme) document.documentElement.setAttribute('theme', parentTheme);",
				"} catch (e) {",
				"  console.warn('direct parent access blocked by browser; waiting for postMessage');",
				"}",
				"",
				"window.addEventListener('message', (event) => {",
				"  if (event.data && event.data.type === 'SET_THEME') document.documentElement.setAttribute('theme', event.data.theme);",
				"});",
				"</script>",
				"</body>",
				"</html>"}, "\n"),
		},
		{
			name:     "source does not exist",
			fsys:     fstest.MapFS{},
			profiles: []*cover.Profile{{ FileName: "foo.go" }},
			wantErr:  true,
		},
		{
			name:          "MkdirAll fails",
			fsys:          fstest.MapFS{ "foo.go": &fstest.MapFile{} },
			profiles:      []*cover.Profile{{ FileName: "foo.go" }},
			mkdirAllFails: true,
			wantErr:       true,
		},
		{
			name:           "WriteFile fails",
			fsys:           fstest.MapFS{ "foo.go": &fstest.MapFile{} },
			profiles:       []*cover.Profile{{ FileName: "foo.go" }},
			writeFileFails: true,
			wantErr:        true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mfs := &mockFS{
				FS:             tt.fsys,
				mkdirAllFails:  tt.mkdirAllFails,
				writeFileFails: tt.writeFileFails,
			}
			repGen := &reportGenerator{
				fsys:        mfs,
				profiles:    tt.profiles,
				pkgDirCache: tt.pkgDirCache,
			}
			err := repGen.writeCovHTMLFiles(t.Context(), io.Discard)
			if (err != nil) != tt.wantErr {
				t.Errorf("writeCovHTMLFiles(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, string(mfs.data)); diff != "" {
				t.Errorf("writeCovHTMLFiles(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestWriteIndexHTML(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name          string
		embeddedFiles fs.FS
		modName       string
		repoURL       string
		createFails   bool
		want          string
		wantErr       bool
	}{
		{
			name:          "succeeds",
			embeddedFiles: fstest.MapFS{ "index.html": &fstest.MapFile{ Data: []byte("ModName: {{ .ModName }}, ModURL: {{ .ModURL }}") }},
			modName:       "github.com/foo/bar",
			repoURL:       "https://github.com/foo/bar",
			want:          "ModName: github.com/foo/bar, ModURL: https://github.com/foo/bar",
		},
		{
			name:          "template.ParseFS fails because index file does not exist",
			embeddedFiles: fstest.MapFS{},
			wantErr:       true,
		},
		{
			name:          "Create fails",
			embeddedFiles: fstest.MapFS{ "index.html": &fstest.MapFile{} },
			createFails:   true,
			wantErr:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mfs := &mockFS{createFails: tt.createFails}
			repGen := &reportGenerator{
				fsys:          mfs,
				modName:       tt.modName,
				repoURL:       tt.repoURL,
				embeddedFiles: tt.embeddedFiles,
			}
			err := repGen.writeIndexHTML(t.Context(), "index.html")
			if (err != nil) != tt.wantErr {
				t.Errorf("writeIndexHTML(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, string(mfs.data)); diff != "" {
				t.Errorf("writeIndexHTML(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestWriteStyleCSS(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name          string
		embeddedFiles fs.FS
		createFails   bool
		maxWidth      int
		want          string
		wantErr       bool
	}{
		{
			name:          "succeeds",
			embeddedFiles: fstest.MapFS{ "style.css": &fstest.MapFile{ Data: []byte("MaxWidth: {{ .MaxWidth }}") }},
			maxWidth:      13,
			want:          "MaxWidth: 13",
		},
		{
			name:          "template.ParseFS fails because CSS file does not exist",
			embeddedFiles: fstest.MapFS{},
			wantErr:       true,
		},
		{
			name:          "Create fails",
			embeddedFiles: fstest.MapFS{ "style.css": &fstest.MapFile{} },
			createFails:   true,
			wantErr:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mfs    := &mockFS{ createFails: tt.createFails }
			repGen := &reportGenerator{
				fsys:          mfs,
				embeddedFiles: tt.embeddedFiles,
				maxWidth:      tt.maxWidth,
			}
			err := repGen.writeStyleCSS(t.Context(), "style.css")
			if (err != nil) != tt.wantErr {
				t.Errorf("writeStyleCSS(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, string(mfs.data)); diff != "" {
				t.Errorf("writeStyleCSS(%q) mismatch (-want +got):\n%s", tt.name, diff)
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

func TestWriteAncillaryFiles(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name           string
		embeddedFiles  fs.FS
		ancillaryFiles []string
		createFails    bool
		closeFails     bool
		badWriter      bool
		wantErr        bool
		want           string
	}{
		{
			name:           "succeeds",
			embeddedFiles:  fstest.MapFS{ "foo": &fstest.MapFile{ Data: []byte("bar") }},
			ancillaryFiles: []string{"foo"},
			want:           "bar",
		},
		{
			name:           "Create fails",
			embeddedFiles:  fstest.MapFS{},
			ancillaryFiles: []string{"foo"},
			createFails:    true,
			wantErr:        true,
		},
		{
			name:           "ReadFile fails",
			embeddedFiles:  fstest.MapFS{},
			ancillaryFiles: []string{"foo"},
			wantErr:        true,
		},
		{
			name:           "Close fails",
			embeddedFiles:  fstest.MapFS{ "foo": &fstest.MapFile{}},
			ancillaryFiles: []string{"foo"},
			closeFails:     true,
			wantErr:        true,
		},
		{
			name:           "fmt.Fprint fails",
			embeddedFiles:  fstest.MapFS{ "foo": &fstest.MapFile{ Data: []byte("bar") }},
			ancillaryFiles: []string{"foo"},
			badWriter:      true,
			wantErr:        true,
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
				fsys:           mfs,
				embeddedFiles:  tt.embeddedFiles,
				ancillaryFiles: tt.ancillaryFiles,
			}
			err := repGen.writeAncillaryFiles(t.Context())
			if (err != nil) != tt.wantErr {
				t.Errorf("writeAncillaryFiles(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, string(mfs.data)); diff != "" {
				t.Errorf("writeAncillaryFiles(%q) mismatch (-want +got):\n%s", tt.name, diff)
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
