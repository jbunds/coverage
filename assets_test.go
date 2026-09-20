package main

import (
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
			err := repGen.writeStaticFiles()
			if (err != nil) != tt.wantErr {
				t.Errorf("writeStaticFiles(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, string(mfs.data)); diff != "" {
				t.Errorf("writeStaticFiles(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}
