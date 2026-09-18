package main

import (
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/tools/go/packages"
)

// wrappers to facilitate test injection

// writeFS defines an interface that extends fs.FS with writing capabilities.
// This abstraction is necessary to allow for mocking the file system within
// unit tests, as the standard "os" package cannot be directly mocked.
type writeFS interface {
	fs.FS
	Create   (string)                      (io.WriteCloser, error)
	MkdirAll (string,         fs.FileMode)                  error
	ReadDir  (string)                      ([]fs.DirEntry,  error)
	ReadFile (string)                      ([]byte,         error)
	WriteFile(string, []byte, fs.FileMode)                  error
	OpenRoot (string)                      (rootHandle,     error)
	Open     (string)                      (fs.File,        error)
	Stat     (string)                      (fs.FileInfo,    error)
}

// rootHandle is the subset of *os.Root methods the SUT calls.
type rootHandle interface {
	io.Closer
	Name() string
}

// localFS provides a concrete implementation of the writeFS interface
// by wrapping the standard "os" package. This allows the program to
// perform actual system operations in production while remaining
// easily testable via alternative interface implementations.
type localFS struct{}

func (lfs *localFS) OpenRoot(name string) (rootHandle, error ) {
	return os.OpenRoot(name)
}

func (lfs *localFS) Open(name string) (fs.File, error) {
	return os.Open(filepath.Clean(name))
}

func (lfs *localFS) Create(name string) (io.WriteCloser, error) {
	return os.Create(filepath.Clean(name))
}

func (lfs *localFS) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (lfs *localFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(name)
}

func (lfs *localFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name) // #nosec G304 -- all input is either operator-specified or generated herein
}

func (lfs *localFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (lfs *localFS) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

// wraps packages.Load for test injection.
type pkgLoader func(cfg *packages.Config, patterns ...string) ([]*packages.Package, error)

// wraps exec.Cmd and exec.Run for test injection.
type runner interface {
	Run(*exec.Cmd) error
}

type realRunner struct{}

func (*realRunner) Run(cmd *exec.Cmd) error {
	return cmd.Run()
}
