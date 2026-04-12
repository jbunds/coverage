package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/tools/cover"
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

func (m *mockFS) Create(_ string) (io.WriteCloser, error) {
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

func (m *mockFS) ReadDir(dir string) ([]fs.DirEntry, error) {
	if m.readDirFails { return nil, fmt.Errorf("ReadDir failed") }
	return fs.ReadDir(m.FS, dir)
}

func (m *mockFS) MkdirAll(_ string, _ fs.FileMode) error {
	if m.mkdirAllFails { return fmt.Errorf("MkdirAll failed") }
	return nil
}

func (m *mockFS) WriteFile(_ string, data []byte, _ fs.FileMode) error {
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
	closeFails bool
	writer     io.Writer
}

func (m *mockFile) Close() error {
	if m.closeFails { return fmt.Errorf("Close failed") }
	return nil
}

func (m *mockFile) Write(p []byte) (n int, err error) {
	return m.writer.Write(p)
}

// tests

func TestGetModName(t *testing.T) {
	tests := []struct{
		name     string
		fsys     fs.FS
		profiles []*cover.Profile
		want     string
		wantErr  bool
	}{
		{
			name: "succeeds",
			fsys: fstest.MapFS{ "go.mod": &fstest.MapFile{ Data: []byte("module github.com/foo/bar") } },
			want: "github.com/foo/bar",
		},
		{
			name:    "cannot parse go.mod",
			fsys:    fstest.MapFS{ "go.mod": &fstest.MapFile{} },
			wantErr: true,
		},
	}
	for _, tt := range tests {
		repGen := &reportGenerator{
			fsys:     &mockFS{ FS: tt.fsys },
			profiles: tt.profiles,
		}
		err := repGen.getModName("go.mod")
		if (err != nil) != tt.wantErr {
			t.Errorf("getModeName(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
		}
		if diff := cmp.Diff(tt.want, repGen.modName); diff != "" {
			t.Errorf("getModName(%q) mismatch (-want +got):\n%s", tt.name, diff)
		}
	}
}

func TestWriteCovHTMLFiles(t *testing.T) {
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
				"<pre>package main",
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
			name:        "source does not exist",
			fsys:        fstest.MapFS{},
			pkgDirCache: make(map[string]string),
			profiles:    []*cover.Profile{{ FileName: "foo.go" }},
			wantErr:     true,
		},
		{
			name:          "MkdirAll fails",
			fsys:          fstest.MapFS{ "foo.go": &fstest.MapFile{} },
			pkgDirCache:   make(map[string]string),
			profiles:      []*cover.Profile{{ FileName: "foo.go" }},
			mkdirAllFails: true,
			wantErr:       true,
		},
		{
			name:           "WriteFile fails",
			fsys:           fstest.MapFS{ "foo.go": &fstest.MapFile{} },
			pkgDirCache:    make(map[string]string),
			profiles:       []*cover.Profile{{ FileName: "foo.go" }},
			writeFileFails: true,
			wantErr:        true,
		},
	}
	for _, tt := range tests {
		mfs := &mockFS{
			FS:             tt.fsys,
			mkdirAllFails:  tt.mkdirAllFails,
			writeFileFails: tt.writeFileFails,
		}
		repGen := &reportGenerator{
			fsys:        mfs,
			profiles:    tt.profiles,
			pkgDirCache: &pkgDirCache{ cache: tt.pkgDirCache },
		}
		err := repGen.writeCovHTMLFiles(&strings.Builder{})
		if (err != nil) != tt.wantErr {
			t.Errorf("writeCovHTMLFiles(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
		}
		if diff := cmp.Diff(tt.want, string(mfs.data)); diff != "" {
			t.Errorf("writeCovHTMLFiles(%q) mismatch (-want +got):\n%s", tt.name, diff)
		}
	}
}

func TestWriteIndexHTML(t *testing.T) {
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
		mfs := &mockFS{createFails: tt.createFails}
		repGen := &reportGenerator{
			fsys:          mfs,
			modName:       tt.modName,
			repoURL:       tt.repoURL,
			embeddedFiles: tt.embeddedFiles,
		}
		err := repGen.writeIndexHTML("index.html")
		if (err != nil) != tt.wantErr {
			t.Errorf("writeIndexHTML(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
		}
		if diff := cmp.Diff(tt.want, string(mfs.data)); diff != "" {
			t.Errorf("writeIndexHTML(%q) mismatch (-want +got):\n%s", tt.name, diff)
		}
	}
}

func TestWriteStyleCSS(t *testing.T) {
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
		mfs    := &mockFS{ createFails: tt.createFails }
		repGen := &reportGenerator{
			fsys:          mfs,
			embeddedFiles: tt.embeddedFiles,
			maxWidth:      tt.maxWidth,
		}
		err := repGen.writeStyleCSS("style.css")
		if (err != nil) != tt.wantErr {
			t.Errorf("writeStyleCSS(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
		}
		if diff := cmp.Diff(tt.want, string(mfs.data)); diff != "" {
			t.Errorf("writeStyleCSS(%q) mismatch (-want +got):\n%s", tt.name, diff)
		}
	}
}

func TestWriteTemplateFile(t *testing.T) {
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
		mfs    := &mockFS{}
		repGen := &reportGenerator{
			fsys:          mfs,
			embeddedFiles: tt.embeddedFiles,
		}
		err := repGen.writeTemplateFile(tt.fileName, tt.tmplData)
		if (err != nil) != tt.wantErr {
			t.Errorf("writeTemplateFile(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
		}
		if diff := cmp.Diff(tt.want, string(mfs.data)); diff != "" {
			t.Errorf("writeTemplateFile(%q) mismatch (-want +got):\n%s", tt.name, diff)
		}
	}
}

func TestPrintCoverage(t *testing.T) {
	tests := []struct{
		name            string
		cov             map[string]coverage
		totalCovered    int
		totalStatements int
		want            string
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
			want: strings.Join([]string{
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
		repGen := &reportGenerator{
			cov:             tt.cov,
			totalCovered:    tt.totalCovered,
			totalStatements: tt.totalStatements,
		}
		got := captureStdout(t, func() { repGen.printCoverage() })
		if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("printCoverage() mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestWriteAncillaryFiles(t *testing.T) {
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
		err := repGen.writeAncillaryFiles()
		if (err != nil) != tt.wantErr {
			t.Errorf("writeAncillaryFiles(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
		}
		if diff := cmp.Diff(tt.want, string(mfs.data)); diff != "" {
			t.Errorf("writeAncillaryFiles(%q) mismatch (-want +got):\n%s", tt.name, diff)
		}
	}
}

func TestFilterArgs(t *testing.T) {
	tests := []struct{
		args []string
		want []string
	}{
		{
			args: []string{"-coverfile", "foo", "-path", "bar"},
			want: []string{"-coverfile", "foo", "-path", "bar"},
		},
		{
			args: []string{"-coverfile", "foo", "-path", "bar", "--", "boo", "hoo"},
			want: []string{"boo", "hoo"},
		},
		{
			args: []string{"foo", "bar", "--", "baz", "boo"},
			want: []string{"baz", "boo"},
		},
	}
	for _, tt := range tests {
		got := filterArgs(tt.args)
		if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("filterArgs(%v) mismatch (-want +got):\n%s", tt.args, diff)
		}
	}
}

func captureStdout(tb testing.TB, fn func()) string {
	tb.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		tb.Fatalf("failed to create pipe to capture stdout: %v", err)
	}
	orig := os.Stdout
	tb.Cleanup(func() { os.Stdout = orig })
	os.Stdout = w
	type result struct {
		out string
		err error
	}
	resChan := make(chan result)
	go func() {
		buf        := new(bytes.Buffer)
		_, copyErr := io.Copy(buf, r)
		resChan <- result{out: buf.String(), err: copyErr}
	}()
	fn()
	if err := w.Close(); err != nil {
		tb.Errorf("w.Close() failed: %v", err)
	}
	res := <-resChan
	if res.err != nil {
		tb.Errorf("failed to capture stdout: %v", err)
	}
	return res.out
}
