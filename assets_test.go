package main

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
)

func TestWriteStaticFiles(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name          string
		embeddedFiles fs.FS
		staticFiles   []string
		createFails   bool
		closeFails    bool
		badWriter     bool
		wantErr       error
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
			wantErr:       errors.New(`cannot create "some/path/foo": Create failed`),
		},
		{
			name:          "ReadFile fails",
			embeddedFiles: fstest.MapFS{},
			staticFiles:   []string{"foo"},
			wantErr:       errors.New(`cannot read "foo": open foo: file does not exist`),
		},
		{
			name:          "Close fails",
			embeddedFiles: fstest.MapFS{ "foo": &fstest.MapFile{}},
			staticFiles:   []string{"foo"},
			closeFails:    true,
			wantErr:       errors.New(`cannot close file "some/path/foo": Close failed`),
		},
		{
			name:          "fmt.Fprint fails",
			embeddedFiles: fstest.MapFS{ "foo": &fstest.MapFile{ Data: []byte("bar") }},
			staticFiles:   []string{"foo"},
			badWriter:     true,
			wantErr:       errors.New(`cannot write file "some/path/foo": i refuse to write`),
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
			rg := &reportGenerator{
				fsys:          mfs,
				outRoot:       &mockRoot{name: "some/path"},
				embeddedFiles: tt.embeddedFiles,
				staticFiles:   tt.staticFiles,
			}
			err := rg.writeStaticFiles()
      if got, want := errStr(err), errStr(tt.wantErr); got != want {
        t.Errorf("writeStaticFiles(%q) returned unexpected error:\ngot:  %v\nwant: %v", tt.name, got, want)
			}
			if diff := cmp.Diff(tt.want, string(mfs.data)); diff != "" {
				t.Errorf("writeStaticFiles(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}
