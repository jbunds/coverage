package main

import (
	"io"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
	"github.com/jbunds/progress"
)

func TestBuildTreeHTML(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name         string
		fsys         fs.FS
		cov          map[string]coverage
		readDirFails bool
		want         string
		wantErr      bool
	}{{
		name: "succeeds",
		fsys: fstest.MapFS{
			"some/path/bar/a.go.html":            &fstest.MapFile{},
			"some/path/bar/dir/b.go.html":        &fstest.MapFile{},
			"some/path/bar/dir/subdir/c.go.html": &fstest.MapFile{},
		},
		cov: map[string]coverage{
			"bar/a.go":            {covered: 10, total: 10},
			"bar/dir/b.go":        {covered:  5, total: 10},
			"bar/dir/subdir/c.go": {covered:  0, total: 10},
		},
		want: strings.Join([]string{
			`<ul class="tree">`,
			`  <li>`,
			`    <input type="checkbox" id="tree-item-0"/>`,
			`    <div class="tree-node">`,
			`      <label for="tree-item-0">bar</label>`,
			`      <span class="cov">50.0%</span>`, // covered: 10 + 5 + 0 == 15; total: 10 + 10 + 10 == 30; 15 / 30 == 50
			`    </div>`,
			`    <ul>`,
			`      <li><div class="tree-node"><span class="src"><a href="bar/a.go.html">a.go</a></span> <span class="cov">100.0%</span></div></li>`,
			`      <li>`,
			`        <input type="checkbox" id="tree-item-1"/>`,
			`        <div class="tree-node">`,
			`          <label for="tree-item-1">dir</label>`,
			`          <span class="cov">25.0%</span>`,
			`        </div>`,
			`        <ul>`,
			`          <li><div class="tree-node"><span class="src"><a href="bar/dir/b.go.html">b.go</a></span> <span class="cov">50.0%</span></div></li>`,
			`          <li>`,
			`            <input type="checkbox" id="tree-item-2"/>`,
			`            <div class="tree-node">`,
			`              <label for="tree-item-2">subdir</label>`,
			`              <span class="cov">0.0%</span>`,
			`            </div>`,
			`            <ul>`,
			`              <li><div class="tree-node"><span class="src"><a href="bar/dir/subdir/c.go.html">c.go</a></span> <span class="cov">0.0%</span></div></li>`,
			`            </ul>`,
			`          </li>`,
			`        </ul>`,
			`      </li>`,
			`    </ul>`,
			`  </li>`,
			`</ul>`}, "\n"),
	}, {
		name:         "ReadDir fails",
		fsys:         fstest.MapFS{},
		readDirFails: true,
		wantErr:      true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mfs := &mockFS{
				FS:           tt.fsys,
				readDirFails: tt.readDirFails,
			}
			tb := &treeBuilder{
				fsys:    mfs,
				cov:     tt.cov,
				outRoot: &mockRoot{name: "some/path"},
				modName: "bar/baz",
			}
			got, err := tb.buildTreeHTML(t.Context(), io.Discard)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildTreeHTML(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			wantLines := strings.Split(tt.want, "\n")
			gotLines  := strings.Split(got, "\n")
			if !cmp.Equal(wantLines, gotLines) {
				var rep reporter
				cmp.Equal(wantLines, gotLines, cmp.Reporter(&rep))
				t.Errorf("mismatch (-want +got):\n%s", strings.Join(rep.diffs, "\n"))
			}
		})
	}
}

func TestProcessEntry(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name         string
		src          string
		entryPoint   string
		currentEntry fs.FileInfo
		cov          map[string]coverage
		fsys         fs.FS
		entry        fs.DirEntry
		readDirFails bool
		want         *entryResult
		wantErr      bool
	}{{
		name:        "succeeds",
		src:         "foo/bar/baz/boo.go",
		entryPoint:  "foo",
		fsys:        fstest.MapFS{"some/path/foo/bar/baz/boo.go.html": &fstest.MapFile{}}, // apparently behaves like mkdir -p
		currentEntry: &mockFileInfo{mode: fs.ModeDir, name: "bar"},
		cov:         map[string]coverage{"foo/bar/baz/boo.go": {covered: 17, total: 53}},
		want:        &entryResult{
			covered: 17,
			total:   53,
			html:    strings.Join([]string{
				`  <li>`,
				`    <input type="checkbox" id="tree-item-1"/>`,
				`    <div class="tree-node">`,
				`      <label for="tree-item-1">bar</label>`,
				`      <span class="cov">32.1%</span>`,
				`    </div>`,
				`    <ul>`,
				`      <li>`,
				`        <input type="checkbox" id="tree-item-2"/>`,
				`        <div class="tree-node">`,
				`          <label for="tree-item-2">baz</label>`,
				`          <span class="cov">32.1%</span>`,
				`        </div>`,
				`        <ul>`,
				`          <li><div class="tree-node"><span class="src"><a href="foo/bar/baz/boo.go.html">boo.go</a></span> <span class="cov">32.1%</span></div></li>`,
				`        </ul>`,
				`      </li>`,
				`    </ul>`,
				`  </li>`,
				``}, "\n"),
		},
	}, {
		name:         "file is neither DirEntry nor *.go.html file",
		entryPoint:   "some/path/file",
		fsys:         fstest.MapFS{"some/path/file": &fstest.MapFile{}},
		currentEntry: &mockFileInfo{},
		want:         &entryResult{},
	}, {
		name:         "file is DirEntry but ReadDir fails",
		entryPoint:   "some/path",
		fsys:         fstest.MapFS{"some/path": &fstest.MapFile{Mode: fs.ModeDir}},
		currentEntry: &mockFileInfo{mode: fs.ModeDir},
		readDirFails: true,
		want:         &entryResult{},
		wantErr:      true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mfs := &mockFS{
				FS:           tt.fsys,
				readDirFails: tt.readDirFails,
			}
			tb := &treeBuilder{
				fsys:    mfs,
				cov:     tt.cov,
				outRoot: &mockRoot{name: "some/path"},
			}
			ctx  := t.Context()
			prog := progress.New(ctx, 0, io.Discard); t.Cleanup(func() { prog.Close() })
			st   := scanState{
				parentPath: tt.entryPoint,
				entry:      fs.FileInfoToDirEntry(tt.currentEntry),
				indent:     1,
				prog:       prog,
			}
			got, err := tb.processEntry(t.Context(), st)
			if (err != nil) != tt.wantErr {
				t.Errorf("processEntry(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(entryResult{})); diff != "" {
				t.Errorf("processEntry(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}
