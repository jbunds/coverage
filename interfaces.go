package main

import (
	"context"
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
	Create         (context.Context, string)                      (io.WriteCloser, error)
	MkdirAll       (context.Context, string,         fs.FileMode)                  error
	ReadDir        (context.Context, string)                      ([]fs.DirEntry,  error)
	ReadFile       (context.Context, string)                      ([]byte,         error)
	WriteFile      (context.Context, string, []byte, fs.FileMode)                  error
	OpenWithContext(context.Context, string)                      (fs.File,        error)
}

// localFS provides a context-aware, concrete implementation of the writeFS
// interface by wrapping the standard "os" package. This allows the program
// to perform actual system operations in production while remaining easily
// testable via alternative interface implementations.
type localFS struct{}

func (lfs *localFS) OpenWithContext(ctx context.Context, name string) (fs.File, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	return lfs.Open(name)
}

func (lfs *localFS) Open(name string) (fs.File, error) {
	return os.Open(filepath.Clean(name))
}

func (lfs *localFS) Create(ctx context.Context, name string) (io.WriteCloser, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	return os.Create(filepath.Clean(name))
}

func (lfs *localFS) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	if err := ctx.Err(); err != nil { return err }
	return os.MkdirAll(path, perm)
}

func (lfs *localFS) ReadDir(ctx context.Context, name string) ([]fs.DirEntry, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	return os.ReadDir(name)
}

func (lfs *localFS) ReadFile(ctx context.Context, name string) ([]byte, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	return fs.ReadFile(lfs, name)
}

func (lfs *localFS) WriteFile(ctx context.Context, name string, data []byte, perm fs.FileMode) error {
	if err := ctx.Err(); err != nil { return err }
	return os.WriteFile(name, data, perm)
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
