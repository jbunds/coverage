package main

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/tools/go/packages"
)

func TestRegisterModPaths(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name       string
		fsys       fs.FS
		goModFiles string
		want       []string
		wantErr    error
	}{{
		name:       "single go.mod file",
		fsys:       fstest.MapFS{"go.mod": &fstest.MapFile{Data: []byte("module github.com/foo/bar")}},
		goModFiles: "go.mod",
		want:       []string{"github.com/foo/bar"},
	}, {
		name:       "cannot read go.mod",
		fsys:       fstest.MapFS{},
		goModFiles: "go.mod",
		wantErr:    errors.New(`cannot read "go.mod": open go.mod: file does not exist`),
	}, {
		name:       "cannot parse go.mod",
		fsys:       fstest.MapFS{"go.mod": &fstest.MapFile{Data: []byte("bad_directive")}},
		goModFiles: "go.mod",
		wantErr:    errors.New(`cannot parse "go.mod": go.mod:1: unknown directive: bad_directive`),
	}, {
		name:       "go.work file",
		fsys:       fstest.MapFS{
			"go.work":    &fstest.MapFile{Data: []byte("use ./foo")},
			"foo/go.mod": &fstest.MapFile{Data: []byte("module foo")},
		},
		goModFiles: "go.work",
		want:       []string{"foo"},
	}, {
		name:       "cannot read go.work",
		fsys:       fstest.MapFS{},
		goModFiles: "go.work",
		wantErr:    errors.New(`cannot read "go.work": open go.work: file does not exist`),
	}, {
		name:       "cannot parse go.work",
		fsys:       fstest.MapFS{"go.work": &fstest.MapFile{Data: []byte("bad_directive")}},
		goModFiles: "go.work",
		wantErr:    errors.New(`cannot parse "go.work": go.work:1: unknown directive: bad_directive`),
	}, {
		name:       "cannot read go.mod file of used module",
		fsys:       fstest.MapFS{
			"go.work": &fstest.MapFile{Data: []byte("use ./foo")},
		},
		goModFiles: "go.work",
		wantErr:    errors.New(`cannot read "foo/go.mod": open foo/go.mod: file does not exist`),
	}, {
		name:       "cannot parse go.mod file of used module",
		fsys:       fstest.MapFS{
			"go.work":    &fstest.MapFile{Data: []byte("use ./foo")},
			"foo/go.mod": &fstest.MapFile{Data: []byte("bad_directive")},
		},
		goModFiles: "go.work",
		wantErr:    errors.New(`cannot parse "foo/go.mod": foo/go.mod:1: unknown directive: bad_directive`),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rg := &reportGenerator{
				fsys: &mockFS{FS: tt.fsys},
			}
			err := rg.registerModPaths(&flagVals{goModFiles: tt.goModFiles})
			if got, want := errStr(err), errStr(tt.wantErr); got != want {
				t.Errorf("registerModPaths(%q) returned unexpected error:\ngot:  %v\nwant: %v", tt.name, got, want)
			}
			if diff := cmp.Diff(tt.want, rg.modPaths); diff != "" {
				t.Errorf("registerModPaths(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestResolveRepoURLs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		runner  runner
		want    []string
	}{{
		name:   "local SSH standard (SCP style)",
		runner: &mockRunner{outputs: []mockOutput{{stdout: "git@github.com:foo/bar.git"}}},
		want:   []string{"https://github.com/foo/bar"},
	}, {
		name:   "local SSH standard (no extension)",
		runner: &mockRunner{outputs: []mockOutput{{stdout: "git@github.com:foo/bar"}}},
		want:   []string{"https://github.com/foo/bar"},
	}, {
		name:   "local SSH explicit protocol",
		runner: &mockRunner{outputs: []mockOutput{{stdout: "ssh://git@github.com:foo/bar.git"}}},
		want:   []string{"https://github.com/foo/bar"},
	}, {
		name:   "local HTTPS standard",
		runner: &mockRunner{outputs: []mockOutput{{stdout: "https://github.com/foo/bar.git"}}},
		want:   []string{"https://github.com/foo/bar"},
	}, {
		name:   "GitHub CI runner (token authentication)",
		runner: &mockRunner{outputs: []mockOutput{{
			stdout: "https://x-access-token:ghp_0123456789@github.com/foo/bar.git", // #nosec G101 - false positive (hardcoded creds)
		}}},
		want:   []string{"https://github.com/foo/bar"},
	}, {
		name:   "GitHub CI runner (standard checkout)",
		runner: &mockRunner{outputs: []mockOutput{{stdout: "https://github.com/foo/bar.git"}}},
		want:   []string{"https://github.com/foo/bar"},
	}, {
		name:    "git config fails",
		runner:  &mockRunner{outputs: []mockOutput{{err: errors.New("git config failed")}}},
		want:    []string{"https://github.com/foo/bar"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rg  := &reportGenerator{modPaths: []string{"github.com/foo/bar"}}
			err := rg.resolveRepoURLs(tt.runner, &flagVals{goModFiles: "go.mod"})
			if err != nil {
				t.Errorf("resolveRepoURL(%q) returned unexpected error: %v", tt.name, err)
			}
			if diff := cmp.Diff(tt.want, rg.repoURLs); diff != "" {
				t.Errorf("resolveRepoURL(%q) mismatch (-want +got):\n%s", tt.name, diff)
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
		wantErr     error
	}{{
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
	}, {
		name:        "fails",
		profilePath: "nope",
		fsys:        fstest.MapFS{},
		wantErr:     errors.New("open nope: file does not exist"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rg := &reportGenerator{
				profilePath: tt.profilePath,
				fsys:        &mockFS{FS: tt.fsys},
			}
			got, err := rg.getAllPkgPaths()
			if gotErr, wantErr := errStr(err), errStr(tt.wantErr); gotErr != wantErr {
				t.Errorf("getAllPkgPaths(%q) returned unexpected error:\ngot:  %v\nwant: %v", tt.name, gotErr, wantErr)
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
		wantErr     error
	}{{
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
	}, {
		name:        "cannot read coverage profile file",
		profilePath: "nope",
		fsys:        fstest.MapFS{},
		wantErr:     errors.New("open nope: file does not exist"),
	}, {
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
		wantErr: errors.New("packages.Load failed"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rg := &reportGenerator{
				fsys:        &mockFS{FS: tt.fsys},
				profilePath: tt.profilePath,
				goModFiles:  []string{"go.mod"},
			}
			err := rg.primePkgDirCache(mockPkgLoader)
			if gotErr, wantErr := errStr(err), errStr(tt.wantErr); gotErr != wantErr {
				t.Errorf("primePkgDirCache(%q) returned unexpected error:\ngot:  %v\nwant: %v", tt.name, gotErr, wantErr)
			}
			if tt.wantErr != nil { return }
			if diff := cmp.Diff(tt.want, rg.pkgDirCache); diff != "" {
				t.Errorf("primePkgDirCache(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}
