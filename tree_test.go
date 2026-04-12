package main

import (
	"io"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
)

func TestWriteTreeHTML(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name         string
		fsys         fs.FS
		createFails  bool
		readDirFails bool
		want         int
		wantErr      bool
	}{
		{
			name: "succeeds",
			fsys: fstest.MapFS{ "foo.go.html": &fstest.MapFile{}},
			want: 17, // len("foo.go") == 6 + indent == 1 + 10 == 17
		},
		{
			name:         "genHTML fails",
			fsys:         fstest.MapFS{},
			readDirFails: true,
			wantErr:      true,
		},
		{
			name:        "Create fails",
			fsys:        fstest.MapFS{},
			createFails: true,
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		mfs := &mockFS{
			FS:           tt.fsys,
			readDirFails: tt.readDirFails,
			createFails:  tt.createFails,
		}
		tb := &treeBuilder{
			fsys:    mfs,
			outRoot: ".",
		}
		got, err := tb.writeTreeHTML()
		if (err != nil) != tt.wantErr {
			t.Errorf("writeTreeHTML(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
		}
		if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("writeTreeHTML(%q) mismatch (-want +got):\n%s", tt.name, diff)
		}
	}
}

func TestGenHTML(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name         string
		fsys         fs.FS
		cov          map[string]coverage
		readDirFails bool
		want         string
		wantErr      bool
	}{
		{
			name: "succeeds",
			fsys: fstest.MapFS{
				"a.go.html":            &fstest.MapFile{},
				"dir/b.go.html":        &fstest.MapFile{},
				"dir/subdir/c.go.html": &fstest.MapFile{},
			},
			cov: map[string]coverage{
				"a.go":            {covered: 10, total: 10},
				"dir/b.go":        {covered:  5, total: 10},
				"dir/subdir/c.go": {covered:  0, total: 10},
			},
			want: strings.Join([]string{
				"<ul class=\"tree\">",
				"  <li><div class=\"tree-node\"><span class=\"src\"><a href=\"a.go.html\">a.go</a></span> <span class=\"cov\">100.0%</span></div></li>",
				"  <li>",
				"    <input type=\"checkbox\" id=\"tree-item-1\"/>",
				"    <div class=\"tree-node\">",
				"      <label for=\"tree-item-1\">dir</label>",
				"      <span class=\"cov\">25.0%</span>",
				"    </div>",
				"    <ul>",
				"      <li><div class=\"tree-node\"><span class=\"src\"><a href=\"dir/b.go.html\">b.go</a></span> <span class=\"cov\">50.0%</span></div></li>",
				"      <li>",
				"        <input type=\"checkbox\" id=\"tree-item-2\"/>",
				"        <div class=\"tree-node\">",
				"          <label for=\"tree-item-2\">subdir</label>",
				"          <span class=\"cov\">0.0%</span>",
				"        </div>",
				"        <ul>",
				"          <li><div class=\"tree-node\"><span class=\"src\"><a href=\"dir/subdir/c.go.html\">c.go</a></span> <span class=\"cov\">0.0%</span></div></li>",
				"        </ul>",
				"      </li>",
				"    </ul>",
				"  </li>",
				"</ul>\n"}, "\n"),
		},
		{
			name:         "ReadDir fails",
			fsys:         fstest.MapFS{},
			readDirFails: true,
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		mfs := &mockFS{
			FS:           tt.fsys,
			readDirFails: tt.readDirFails,
		}
		tb := &treeBuilder{
			fsys:    mfs,
			cov:     tt.cov,
			outRoot: ".",
		}
		got, err := tb.genHTML()
		if (err != nil) != tt.wantErr {
			t.Errorf("genHTML(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
		}
		if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("genHTML(%q) mismatch (-want +got):\n%s", tt.name, diff)
		}
	}
}

func TestProcessEntry(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name         string
		fileName     string
		file         *fstest.MapFile
		fsys         fs.FS
		entry        fs.DirEntry
		readDirFails bool
		want         entryResult
		wantErr      bool
	}{
		{
			name:     "neither DirEntry nor *.go.teml file",
			fileName: "file",
			file:     &fstest.MapFile{},
			fsys:     fstest.MapFS{ "file": &fstest.MapFile{} },
			want:     entryResult{},
		},
		{
			name:         "DirEntry but ReadDir fails",
			fileName:     "dir",
			file:         &fstest.MapFile{ Mode: fs.ModeDir },
			fsys:         fstest.MapFS{ "dir": &fstest.MapFile{ Mode: fs.ModeDir } },
			readDirFails: true,
			want:         entryResult{},
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		mfs := &mockFS{
			FS:           tt.fsys,
			readDirFails: tt.readDirFails,
		}
		tb        := &treeBuilder{ fsys: mfs }
		info, err := fs.Stat(mfs, tt.fileName)
		if err != nil {
			t.Errorf("fs.Stat failed unexpectedly: %v", err)
		}
		got, err := tb.processEntry(".", fs.FileInfoToDirEntry(info), 1)
		if (err != nil) != tt.wantErr {
			t.Errorf("processEntry(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
		}
		if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(entryResult{})); diff != "" {
			t.Errorf("processEntry(%q) mismatch (-want +got):\n%s", tt.name, diff)
		}
	}
}

func TestPreamble(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name    string
		writer  io.Writer
		wantErr bool
	}{
		{
			name:  "succeeds",
			writer: io.Discard,
		},
		{
			name:    "fails",
			writer:  &badWriter{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		err := preamble(tt.writer)
		if (err != nil) != tt.wantErr {
			t.Errorf("preamble(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestPostamble(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name    string
		writer  io.Writer
		wantErr bool
	}{
		{
			name:  "succeeds",
			writer: io.Discard,
		},
		{
			name:    "fails",
			writer:  &badWriter{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		err := postamble(tt.writer)
		if (err != nil) != tt.wantErr {
			t.Errorf("postamble(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
		}
	}
}
