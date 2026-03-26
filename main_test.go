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
			fsys: os.DirFS("."),
			want: "github.com/jbunds/coverage",
		},
		{
			name:    "cannot parse go.mod",
			fsys:    fstest.MapFS{ "go.mod": &fstest.MapFile{} },
			wantErr: true,
		},
		{
			name:     "cannot determine common root",
			fsys:     fstest.MapFS{},
			profiles: []*cover.Profile{
				{ FileName: "foo" },
				{ FileName: "bar" },
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		repGen := &reportGenerator{
			fsys:     &localFS{tt.fsys},
			profiles: tt.profiles,
		}
		err    := repGen.getModName()
		fmt.Printf("%q\n", tt.name)
		fmt.Printf("repGen.modName = %q\n", repGen.modName)
		if (err != nil) != tt.wantErr {
			t.Errorf("getModeName(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
		}
		if diff := cmp.Diff(tt.want, repGen.modName); diff != "" {
			t.Errorf("getModName(%q) mismatch (-want +got):\n%s", tt.name, diff)
		}
	}
}

func TestPrintCoverage(t *testing.T) {
	repGen := &reportGenerator{
		cov: map[string]coverage{
			"foo":     { covered:  10, total: 100 },
			"bar/baz": { covered: 180, total: 200 },
			"boo":     { covered:  40, total:  40 },
		},
	}
	want := strings.Join([]string{
		"File    Coverage",
		"————————————————",
		"boo      100.00%",
		"foo       10.00%",
		"bar/baz   90.00%",
		"————————————————",
		"Total      0.00%" + "\n"}, "\n")
	got := captureStdout(t, func() { repGen.printCoverage() })
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("printCoverage() mismatch (-want +got):\n%s", diff)
	}
}

func TestFindCommonRoot(t *testing.T) {
	tests := []struct{
		name     string
		profiles []*cover.Profile
		want     string
	}{
		{
			name:     "empty",
			profiles: []*cover.Profile{},
		},
		{
			name:     "shallow",
			profiles: []*cover.Profile{{ FileName: "one" }},
			want:     "one",
		},
		{
			name:     "deep",
			profiles: []*cover.Profile{
				{ FileName: "one/two/three" },
				{ FileName: "one/two/three/four" },
				{ FileName: "one/two" },
				{ FileName: "one/two/three/four/five" },
			},
			want: "one/two",
		},
	}
	for _, tt := range tests {
		repGen := &reportGenerator{ profiles: tt.profiles }
		err := repGen.findCommonRoot()
		if err != nil {
			t.Errorf("findCommonRoot(%q) failed: %v", tt.name, err)
		}
		got := repGen.modName
		if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("findCommonRoot(%q) mismatch (-want +got):\n%s", tt.name, diff)
		}
	}
}

func TestGetSrcRoot(t *testing.T) {
	tests := []struct{
		name     string
		fsys     fs.FS
		profiles []*cover.Profile
		want     string
		wantErr  bool
	}{
		{
			name:    "does exist",
			fsys:     os.DirFS("."),
			profiles: []*cover.Profile{{ FileName: "main.go" }},
			want:     ".",
		},
		{
			name:     "does not exist",
			fsys:     os.DirFS("."),
			profiles: []*cover.Profile{{ FileName: "foo/bar.go" }},
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		repGen := &reportGenerator{
			fsys:     &localFS{tt.fsys},
			profiles: tt.profiles,
		}
		err := repGen.getSrcRoot()
		if (err != nil) != tt.wantErr {
			t.Errorf("getSrcRoot(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
		}
		if diff := cmp.Diff(tt.want, repGen.srcRoot); diff != "" {
			t.Errorf("getSrcRoot(%q) mismatch (-want +got):\n%s", tt.name, diff)
		}
	}
}

type mockFS struct {
	fs.FS
	fail bool
	data []byte
}

type mockFile struct {
	fail bool
	data *[]byte
}

func (m *mockFile) Write(p []byte) (n int, err error) {
	*m.data = append(*m.data, p...)
	return len(p), nil
}

func (m *mockFile) Close() error {
	if m.fail { return fmt.Errorf("Close failed") }
	return nil
}

func (m *mockFS) Create(_ string) (io.WriteCloser, error) {
	if m.fail { return nil, fmt.Errorf("Create failed") }
	return &mockFile{data: &m.data}, nil
}

func (m *mockFS) MkdirAll(_ string, _ fs.FileMode) error {
	if m.fail { return fmt.Errorf("MkdirAll failed") }
	return nil
}

func (m *mockFS) WriteFile(_ string, data []byte, _ fs.FileMode) error {
	m.data = data
	if m.fail { return fmt.Errorf("WriteFile failed") }
	return nil
}

func TestWriteCovHTMLFiles(t *testing.T) {
	tests := []struct{
		name      string
		fsys      fs.FS
		modName   string
		srcRoot   string
		profiles  []*cover.Profile
		want      string
		wantErr   bool
	}{
		{
			name:      "succeeds",
			fsys:      fstest.MapFS{ "foo/bar/baz.go": &fstest.MapFile{ Data: []byte(strings.Join([]string{
				"package main",
				"",
				"import \"fmt\"",
				"",
				"func main() {",
				"  fmt.Println(\"hello world\")",
				"}\n",
			}, "\n"))}},
			modName:   "foo",
			srcRoot:   ".",
			profiles:   []*cover.Profile{{FileName: "foo/bar/baz.go"}},
			want:      strings.Join([]string{
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
				"func main() {",
				"  fmt.Println(&#34;hello world&#34;)",
				"}",
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
	}
	for _, tt := range tests {
		mfs    := &mockFS{ FS: tt.fsys }
		repGen := &reportGenerator{
			fsys:     mfs,
			profiles: tt.profiles,
			srcRoot:  tt.srcRoot,
		}
		err := repGen.writeCovHTMLFiles()
		if (err != nil) != tt.wantErr {
			t.Errorf("writeCovHTMLFile(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
		}
		if diff := cmp.Diff(tt.want, string(mfs.data)); diff != "" {
			t.Errorf("writeCovHTMLFile(%q) mismatch (-want +got):\n%s", tt.name, diff)
		}
	}
}

func TestWriteIndexHTML(t *testing.T) {
	tests := []struct{
		name    string
		modName string
		outRoot string
		want    string
		wantErr bool
	}{
		{
			name:    "succeeds",
			modName: "foo/bar/baz",
			outRoot: "out",
			want:    "ModName: foo/bar/baz, ModURL: https://foo/bar/baz",
		},
	}
	for _, tt := range tests {
		mfs := &mockFS{}
		efs := fstest.MapFS{
			"index.html": &fstest.MapFile{
				Data: []byte("ModName: {{ .ModName }}, ModURL: {{ .ModURL }}"),
			},
		}
		repGen := &reportGenerator{
			fsys:          mfs,
			embeddedFiles: efs,
			modName:       tt.modName,
			outRoot:       tt.outRoot,
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
		name     string
		maxWidth int
		outRoot  string
		want     string
		wantErr  bool
	}{
		{
			name:     "succeeds",
			maxWidth: 13,
			outRoot:  "out",
			want:     "MaxWidth: 13",
		},
	}
	for _, tt := range tests {
		mfs := &mockFS{}
		efs := fstest.MapFS{
			"style.css": &fstest.MapFile{
				Data: []byte("MaxWidth: {{ .MaxWidth }}"),
			},
		}
		repGen := &reportGenerator{
			fsys:          mfs,
			embeddedFiles: efs,
			maxWidth:      tt.maxWidth,
			outRoot:       tt.outRoot,
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
