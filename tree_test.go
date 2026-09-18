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
		name            string
		src             string
		initialDir      string
		initialDirEntry fs.FileInfo
		fsys            fs.FS
		cov             map[string]coverage
		readDirFails    bool
		want            *entryResult
		wantErr         bool
	}{{
		name:            "succeeds",
		src:             "foo/bar/baz/boo.go",
		initialDir:      "foo",
		initialDirEntry: &mockFileInfo{mode: fs.ModeDir, name: "bar"},
		fsys:            fstest.MapFS{"some/path/foo/bar/baz/boo.go.html": &fstest.MapFile{}},
		cov:             map[string]coverage{"foo/bar/baz/boo.go": {covered: 17, total: 53}},
		want:            &entryResult{
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
		// this scenario should be programmatically precluded by writeCovHTMLFiles's
		// logic, since its output is based purely on per-source file coverage
		// profiles, so empty subdirs should never be created in the output tree
		//
		// regardless, this test verifies processEntry produces no empty tree nodes
		name:            "entry is DirEntry with empty subdir",
		initialDir:      ".",
		initialDirEntry: &mockFileInfo{mode: fs.ModeDir},
		fsys:            fstest.MapFS{
			"some/path/dir":             &fstest.MapFile{Mode: fs.ModeDir},
			"some/path/dir/subdir":      &fstest.MapFile{Mode: fs.ModeDir}, // artificial empty subdir
			"some/path/dir/foo.go.html": &fstest.MapFile{},
		},
		cov:             map[string]coverage{"dir/foo.go": {covered: 7, total: 13}},
		want:            &entryResult{
			covered:  7,
			total:   13,
			html:    strings.Join([]string{
				`  <li>`,
				`    <input type="checkbox" id="tree-item-1"/>`,
				`    <div class="tree-node">`,
				`      <label for="tree-item-1"></label>`, // empty label
				`      <span class="cov">53.8%</span>`,
				`    </div>`,
				`    <ul>`,
				`      <li>`,
				`        <input type="checkbox" id="tree-item-2"/>`,
				`        <div class="tree-node">`,
				`          <label for="tree-item-2">dir</label>`,
				`          <span class="cov">53.8%</span>`,
				`        </div>`,
				`        <ul>`,
				`          <li><div class="tree-node"><span class="src"><a href="dir/foo.go.html">foo.go</a></span> <span class="cov">53.8%</span></div></li>`,
				`        </ul>`,
				`      </li>`,
				`    </ul>`,
				`  </li>`,
				``}, "\n"),
			},
	}, {
		name:            "entry is neither DirEntry nor *.go.html file",
		initialDir:      "some/path/file",
		initialDirEntry: &mockFileInfo{},
		fsys:            fstest.MapFS{"some/path/file": &fstest.MapFile{}},
		want:            &entryResult{},
	}, {
		name:            "entry is DirEntry but ReadDir fails",
		initialDir:      "some/path/dir",
		initialDirEntry: &mockFileInfo{mode: fs.ModeDir},
		fsys:            fstest.MapFS{"some/path/dir": &fstest.MapFile{Mode: fs.ModeDir}},
		readDirFails:    true,
		want:            &entryResult{},
		wantErr:         true,
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
				parentPath: tt.initialDir,
				entry:      fs.FileInfoToDirEntry(tt.initialDirEntry),
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
