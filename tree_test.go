package main

import (
	"errors"
	"io"
	"io/fs"
	"math"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jbunds/progress"
)

func TestBuildTree(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name         string
		fsys         fs.FS
		cov          map[string]coverage
		readDirFails bool
		want         string
		wantErr      error
	}{{
		name: "succeeds",
		fsys: fstest.MapFS{
			"some/path/github.com/user/project/a.go.html":            &fstest.MapFile{},
			"some/path/github.com/user/project/dir/b.go.html":        &fstest.MapFile{},
			"some/path/github.com/user/project/dir/subdir/c.go.html": &fstest.MapFile{},
		},
		cov: map[string]coverage{
			"github.com/user/project/a.go":            {covered: 10, total: 10},
			"github.com/user/project/dir/b.go":        {covered:  5, total: 10},
			"github.com/user/project/dir/subdir/c.go": {covered:  0, total: 10},
		},
		want: strings.Join([]string{
			`<ul class="tree">`,
			`  <li id="tree-item-3">`,
			`    <input type="checkbox" id="module-tree-item-3"/>`,
			`    <div class="tree-node">`,
			`      <label for="module-tree-item-3"><span class="icon"></span>github.com</label>`,
			`    </div>`,
			`    <ul>`,
			`      <li id="tree-item-4">`,
			`        <input type="checkbox" id="module-tree-item-4"/>`,
			`        <div class="tree-node">`,
			`          <label for="module-tree-item-4"><span class="icon"></span>user</label>`,
			`        </div>`,
			`        <ul>`,
			`          <li id="github-com-user-project">`,
			`            <input type="checkbox" id="module-github-com-user-project"/>`,
			`            <div class="tree-node">`,
			`              <label for="module-github-com-user-project"><span class="icon"></span>project</label>`,
			`              <span class="cov">50.0%</span>`, // covered: 10 + 5 + 0 == 15; total: 10 + 10 + 10 == 30; 15 / 30 == 50.0%
			`            </div>`,
			`            <ul>`,
			`              <li><div class="tree-node"><span class="src"><a href="github.com/user/project/a.go.html">a.go</a></span> <span class="cov">100.0%</span></div></li>`, // covered: 10; total: 10; 10 / 10 == 100.0%
			`              <li>`,
			`                <input type="checkbox" id="tree-item-1"/>`,
			`                <div class="tree-node">`,
			`                  <label for="tree-item-1"><span class="icon"></span>dir</label>`,
			`                  <span class="cov">25.0%</span>`, // covered: 5 + 0 == 5; total: 10 + 10 == 20; 5 / 20 == 25.0%
			`                </div>`,
			`                <ul>`,
			`                  <li><div class="tree-node"><span class="src"><a href="github.com/user/project/dir/b.go.html">b.go</a></span> <span class="cov">50.0%</span></div></li>`, // covered: 5; total: 10; 5 / 10 == 50.0%
			`                  <li>`,
			`                    <input type="checkbox" id="tree-item-2"/>`,
			`                    <div class="tree-node">`,
			`                      <label for="tree-item-2"><span class="icon"></span>subdir</label>`,
			`                      <span class="cov">0.0%</span>`, // covered: 0; total: 10; 0 / 10 == 0.0%
			`                    </div>`,
			`                    <ul>`,
			`                      <li><div class="tree-node"><span class="src"><a href="github.com/user/project/dir/subdir/c.go.html">c.go</a></span> <span class="cov">0.0%</span></div></li>`, // covered: 0; total: 10; 0 / 10 == 0.0%
			`                    </ul>`,
			`                  </li>`,
			`                </ul>`,
			`              </li>`,
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
		wantErr:      errors.New("ReadDir failed"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mfs := &mockFS{
				FS:           tt.fsys,
				readDirFails: tt.readDirFails,
			}
			tb := &treeBuilder{
				fsys:     mfs,
				modPaths: []string{"github.com/user/project"},
				outRoot:  &mockRoot{name: "some/path"},
				covState: &coverageState{cov: tt.cov},
			}
			got, err := tb.buildTree(t.Context(), io.Discard)
			if gotErr, wantErr := errStr(err), errStr(tt.wantErr); gotErr != wantErr {
				t.Errorf("buildTree(%q) returned unexpected error:\ngot:  %v\nwant: %v", tt.name, gotErr, wantErr)
			}
			wantLines := strings.Split(tt.want, "\n")
			gotLines  := strings.Split(got, "\n")
			var rep reporter
			if !cmp.Equal(wantLines, gotLines, cmp.Reporter(&rep)) {
				t.Errorf("buildTree(%q) mismatch (-want +got):\n%s", tt.name, strings.Join(rep.diffs, "\n"))
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
		wantErr         error
	}{{
		name:            "succeeds",
		src:             "github.com/user/project/foo/bar/baz.go",
		initialDir:      "github.com/user/project",
		initialDirEntry: &mockFileInfo{mode: fs.ModeDir, name: "foo"},
		fsys:            fstest.MapFS{"some/path/github.com/user/project/foo/bar/baz.go.html": &fstest.MapFile{}},
		cov:             map[string]coverage{"github.com/user/project/foo/bar/baz.go": {covered: 17, total: 53}},
		want:            &entryResult{
			covered:     17,
			total:       53,
			pkgPath:     "github.com/user/project/foo",
			srcBasename: "foo",
			html:         strings.Join([]string{
				`<li>`,
				`  <input type="checkbox" id="tree-item-1"/>`,
				`  <div class="tree-node">`,
				`    <label for="tree-item-1"><span class="icon"></span>foo</label>`,
				`    <span class="cov">32.1%</span>`,
				`  </div>`,
				`  <ul>`,
				`    <li>`,
				`      <input type="checkbox" id="tree-item-2"/>`,
				`      <div class="tree-node">`,
				`        <label for="tree-item-2"><span class="icon"></span>bar</label>`,
				`        <span class="cov">32.1%</span>`,
				`      </div>`,
				`      <ul>`,
				`        <li><div class="tree-node"><span class="src"><a href="github.com/user/project/foo/bar/baz.go.html">baz.go</a></span> <span class="cov">32.1%</span></div></li>`,
				`      </ul>`,
				`    </li>`,
				`  </ul>`,
				`</li>`,
				``}, "\n"),
		},
	}, {
		name:            "entry is neither DirEntry nor *.go.html file",
		initialDir:      "some/path/file",
		initialDirEntry: &mockFileInfo{},
		fsys:            fstest.MapFS{"some/path/file": &fstest.MapFile{}},
	}, {
		name:            "entry is DirEntry but ReadDir fails",
		initialDir:      "some/path/dir",
		initialDirEntry: &mockFileInfo{mode: fs.ModeDir},
		fsys:            fstest.MapFS{"some/path/dir": &fstest.MapFile{Mode: fs.ModeDir}},
		readDirFails:    true,
		wantErr:         errors.New("ReadDir failed"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mfs := &mockFS{
				FS:           tt.fsys,
				readDirFails: tt.readDirFails,
			}
			tb := &treeBuilder{
				fsys:     mfs,
				outRoot:  &mockRoot{name: "some/path"},
				covState: &coverageState{cov: tt.cov},
			}
			prog := progress.New(t.Context(), 0, io.Discard); t.Cleanup(func() { prog.Close() })
			st   := &scanState{
				parentPath: tt.initialDir,
				entry:      fs.FileInfoToDirEntry(tt.initialDirEntry),
				prog:       prog,
			}
			got, err := tb.processEntry(t.Context(), st)
			if gotErr, wantErr := errStr(err), errStr(tt.wantErr); gotErr != wantErr {
        t.Errorf("processEntry(%q) returned unexpected error:\ngot:  %v\nwant: %v", tt.name, gotErr, wantErr)
			}
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(entryResult{})); diff != "" {
				t.Errorf("processEntry(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestProcessFile(t *testing.T) {
	t.Parallel()
	tb      := &treeBuilder{covState: &coverageState{}}
	prog    := progress.New(t.Context(), 0, io.Discard); t.Cleanup(func() { prog.Close() })
	pkgPath := "some/package/path/foo"
	st      := &scanState{
		prog:       prog,
		parentPath: "foo",
		entry:      fs.FileInfoToDirEntry(&mockFileInfo{name: "bar.go"}),
	}
	want := &entryResult{
		pkgPath:     pkgPath,
		srcBasename: "bar.go",
		html:        `<li><div class="tree-node"><span class="src"><a href="foo/bar.go">bar.go</a></span> <span class="cov">0.0%</span></div></li>` + "\n",
	}
	got, err := tb.processFile(st, pkgPath, "bar.go")
	if err != nil { t.Fatal(err) }
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(entryResult{})); diff != "" {
		t.Errorf("processFile() mismatch (-want +got):\n%s", diff)
	}
}

func TestSplitBudget(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name    string
		total   float64
		entries int
		want    []float64
	}{{
		name:    "exact split",
		total:   10,
		entries:  2,
		want:    []float64{5, 5},
	}, {
		name:    "single entry absorbs all",
		total:   10,
		entries:  1,
		want:    []float64{10},
	}, {
		name:    "zero entries returns empty slice",
		total:   10,
		entries:  0,
		want:    []float64{},
	}, {
		name:    "zero total",
		total:   0,
		entries: 3,
		want:    []float64{0, 0, 0},
	}, {
		name:    "remainder absorbed by last entry",
		total:   10,
		entries:  3,
		want:    []float64{10.0 / 3.0, 10.0 / 3.0, 10.0 - 2.0 * (10.0 / 3.0)},
	}, {
		name:    "large n",
		total:      1,
		entries: 1000,
		want: func() []float64 {
			per := 1.0 / 1000.0
			s   := make([]float64, 1000)
			for i := range s { s[i] = per }
			s[999] = 1.0 - 999.0 * per
			return s
		}(),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := splitBudget(tt.total, tt.entries)
			// relative tolerance of 1e-13 covers the ~100-ULP cancellation gap at small magnitudes
			if diff := cmp.Diff(tt.want, got, cmpopts.EquateApprox(1e-13, 0)); diff != "" {
				t.Errorf("splitBudget() mismatch (-want +got):\n%s", diff)
			}
			if tt.entries < 1 { return }
			// invariant: sum must equal total (when n > 0)
			tol := float64(tt.entries) * (math.Nextafter(tt.total, math.Inf(1)) - tt.total)
			var sum float64
			for _, v := range got { sum += v }
			if math.Abs(sum - tt.total) > tol {
				t.Errorf("sum = %v, want %v", sum, tt.total)
			}
		})
	}
}

func TestRenderSubDirHTML(t *testing.T) {
	t.Parallel()
	subDirHTML := "subdirectory HTML\n"
	res        := &entryResult{html: subDirHTML, covered: 1, total: 3}
	wantLines  := []string{
		`<li>`,
		`  <input type="checkbox" id="3"/>`,
		`  <div class="tree-node">`,
		`    <label for="3"><span class="icon"></span>foo</label>`,
		`    <span class="cov">33.3%</span>`,
		`  </div>`,
		`  <ul>`,
		strings.TrimRight(subDirHTML, "\n"),
		`  </ul>`,
		`</li>`,
		``,
	}
	hb       := &htmlBuilder{itemID: "3", subDir: "foo"}
	hb.renderSubDirHTML(res)
	got      := res.html
	gotLines := strings.Split(got, "\n")
	var rep reporter
	if !cmp.Equal(wantLines, gotLines, cmp.Reporter(&rep)) {
		t.Errorf("renderSubDirHTML() mismatch (-want +got):\n%s", strings.Join(rep.diffs, "\n"))
	}
}
