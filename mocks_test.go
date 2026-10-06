package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"time"

	"github.com/google/go-cmp/cmp"
)

type badWriter struct{}

func (w *badWriter) Write(_ []byte) (int, error) { return 0, errors.New("i refuse to write") }

type sliceWriter struct { data *[]byte }

func (w *sliceWriter) Write(p []byte) (int, error) {
	*w.data = append(*w.data, p...)
	return len(p), nil
}

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

type mockOutput struct {
	stdout, stderr string
	err            error
}

type mockRunner struct {
	pid,
	idx     int
	calls   []string
	outputs []mockOutput
}

func (m *mockRunner) Run(cmd *exec.Cmd) error {
	m.calls = append(m.calls, cmd.String())
	out    := m.outputs[m.idx]
	m.idx++
	if cmd.Stdout != nil { _, _ = io.WriteString(cmd.Stdout, out.stdout) }
	if cmd.Stderr != nil { _, _ = io.WriteString(cmd.Stderr, out.stderr) }
	return out.err
}

func (m *mockRunner) Start(cmd *exec.Cmd) error {
	m.calls     = append(m.calls, cmd.String())
	cmd.Process = &os.Process{Pid: m.pid}
	out        := m.outputs[m.idx]
	m.idx++
	return out.err
}

func (m *mockRunner) Output(cmd *exec.Cmd) ([]byte, error) {
	m.calls = append(m.calls, cmd.String())
	out    := m.outputs[m.idx]
	m.idx++
	return []byte(out.stdout), out.err
}

type mockFD struct{ fd uintptr }

func (m mockFD) Fd() uintptr { return m.fd }

// errStr returns the error message, or "" if err is nil.
//
// errStr is intended for comparing error messages in tests where the SUT
// produces formatted errors with no exported sentinel, which is the norm.
func errStr(err error) string {
	if err == nil { return "" }
	return err.Error()
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
