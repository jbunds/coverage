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
	"golang.org/x/tools/go/packages"
)

// mocks

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
			err := repGen.getModName("go.mod")
			if (err != nil) != tt.wantErr {
				t.Errorf("getModName(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, repGen.modName); diff != "" {
				t.Errorf("getModName(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

type mockRunner struct {
	stdout,
	stderr  string
	err     error
}

func (f *mockRunner) Run(cmd *exec.Cmd) error {
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
			runner: &mockRunner{ stdout: "git@github.com:foo/bar.git" },
			want:   "https://github.com/foo/bar",
		},
		{
			name:   "local SSH standard (no extension)",
			runner: &mockRunner{ stdout: "git@github.com:foo/bar" },
			want:   "https://github.com/foo/bar",
		},
		{
			name:   "local SSH explicit protocol",
			runner: &mockRunner{ stdout: "ssh://git@github.com:foo/bar.git" },
			want:   "https://github.com/foo/bar",
		},
		{
			name:   "local HTTPS standard",
			runner: &mockRunner{ stdout: "https://github.com/foo/bar.git" },
			want:   "https://github.com/foo/bar",
		},
		{
			name:   "GitHub CI runner (token authentication)",
			runner: &mockRunner{ stdout: "https://x-access-token:ghp_1234567890@github.com/foo/bar.git" }, // #nosec G101 - false positive (hardcoded creds)
			want:   "https://github.com/foo/bar",
		},
		{
			name:   "GitHub CI runner (standard checkout)",
			runner: &mockRunner{ stdout: "https://github.com/foo/bar.git" },
			want:   "https://github.com/foo/bar",
		},
		{
			name:    "git config fails",
			runner:  &mockRunner{ err: errors.New("git config failed") },
			want:    "https://github.com/foo/bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repGen := &reportGenerator{modName: "github.com/foo/bar"}
			err    := repGen.getRemoteURL(tt.runner, "go.mod")
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
			got, err := repGen.getAllPkgPaths()
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
			err := repGen.primePkgDirCache(mockPkgLoader)
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

func TestPrintCoverage(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name            string
		cov             map[string]coverage
		totalCovered    uint64
		totalStatements uint64
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
			err := repGen.printCoverage(got)
			if (err != nil) != tt.wantErr {
				t.Errorf("printCoverage(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got.String()); diff != "" {
				t.Errorf("printCoverage(%q) mismatch (-want +got):\n%s", tt.name, diff)
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
