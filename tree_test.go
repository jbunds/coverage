package main

import (
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
			`      <span class="cov">50.0%</span>`, // covered: 10 + 5 + 0 == 15; total: 10 + 10 + 10 == 30; 15 / 30 == 50.0%
			`    </div>`,
			`    <ul>`,
			`      <li><div class="tree-node"><span class="src"><a href="bar/a.go.html">a.go</a></span> <span class="cov">100.0%</span></div></li>`,
			`      <li>`,
			`        <input type="checkbox" id="tree-item-1"/>`,
			`        <div class="tree-node">`,
			`          <label for="tree-item-1">dir</label>`,
			`          <span class="cov">25.0%</span>`, // covered: 5 + 0 == 5; total: 10 + 10 == 20; 5 / 20 == 25.0%
			`        </div>`,
			`        <ul>`,
			`          <li><div class="tree-node"><span class="src"><a href="bar/dir/b.go.html">b.go</a></span> <span class="cov">50.0%</span></div></li>`,
			`          <li>`,
			`            <input type="checkbox" id="tree-item-2"/>`,
			`            <div class="tree-node">`,
			`              <label for="tree-item-2">subdir</label>`,
			`              <span class="cov">0.0%</span>`, // covered: 0; total: 10; 0 / 10 == 0.0%
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
			got, err := tb.buildTree(t.Context(), io.Discard)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildTree(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			wantLines := strings.Split(tt.want, "\n")
			gotLines  := strings.Split(got, "\n")
			var rep reporter
			if !cmp.Equal(wantLines, gotLines, cmp.Reporter(&rep)) {
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
				`<li>`,
				`  <input type="checkbox" id="tree-item-1"/>`,
				`  <div class="tree-node">`,
				`    <label for="tree-item-1">bar</label>`,
				`    <span class="cov">32.1%</span>`,
				`  </div>`,
				`  <ul>`,
				`    <li>`,
				`      <input type="checkbox" id="tree-item-2"/>`,
				`      <div class="tree-node">`,
				`        <label for="tree-item-2">baz</label>`,
				`        <span class="cov">32.1%</span>`,
				`      </div>`,
				`      <ul>`,
				`        <li><div class="tree-node"><span class="src"><a href="foo/bar/baz/boo.go.html">boo.go</a></span> <span class="cov">32.1%</span></div></li>`,
				`      </ul>`,
				`    </li>`,
				`  </ul>`,
				`</li>`,
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
				`<li>`,
				`  <input type="checkbox" id="tree-item-1"/>`,
				`  <div class="tree-node">`,
				`    <label for="tree-item-1"></label>`, // empty label
				`    <span class="cov">53.8%</span>`,
				`  </div>`,
				`  <ul>`,
				`    <li>`,
				`      <input type="checkbox" id="tree-item-2"/>`,
				`      <div class="tree-node">`,
				`        <label for="tree-item-2">dir</label>`,
				`        <span class="cov">53.8%</span>`,
				`      </div>`,
				`      <ul>`,
				`        <li><div class="tree-node"><span class="src"><a href="dir/foo.go.html">foo.go</a></span> <span class="cov">53.8%</span></div></li>`,
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
			st   := &scanState{
				parentPath: tt.initialDir,
				entry:      fs.FileInfoToDirEntry(tt.initialDirEntry),
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

func TestProcessFile(t *testing.T) {
	t.Parallel()
	tb   := &treeBuilder{}
	prog := progress.New(t.Context(), 0, io.Discard)
	st   := &scanState{
		prog:       prog,
		parentPath: "foo",
		entry:      fs.FileInfoToDirEntry(&mockFileInfo{name: "bar.go"}),
	}
	want := &entryResult{
		html: `<li><div class="tree-node"><span class="src"><a href="foo/bar.go">bar.go</a></span> <span class="cov">0.0%</span></div></li>` + "\n",
	}
	got, err := tb.processFile(st, "packagePath", "bar.go")
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

func TestBuildTreeHTML(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name         string
		entryResults []*entryResult
		wantLines    []string
	}{{
		name:         "foo",
		entryResults: []*entryResult{{
			html: "bar\n",
			covered: 3,
			total:   5,
		}},
		wantLines: []string{
			`<ul class="tree">`,
			`  <li>`,
			`    <input type="checkbox" id="tree-item-0"/>`,
			`    <div class="tree-node">`,
			`      <label for="tree-item-0">foo</label>`,
			`      <span class="cov">60.0%</span>`,
			`    </div>`,
			`    <ul>`,
			`bar`,
			`    </ul>`,
			`  </li>`,
			`</ul>`,
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got      := buildTreeHTML("foo", tt.entryResults)
			gotLines := strings.Split(got, "\n")
			var rep reporter
			if !cmp.Equal(tt.wantLines, gotLines, cmp.Reporter(&rep)) {
				t.Errorf("buildTreeHTML() mismatch (-want +got):\n%s", strings.Join(rep.diffs, "\n"))
			}
		})
	}
}

func TestBuildSubDirHTML(t *testing.T) {
	t.Parallel()
	subDirHTML := "subdirectory HTML\n"
	res        := &entryResult{html: subDirHTML, covered: 1, total: 3}
	wantLines  := []string{
		`<li>`,
		`  <input type="checkbox" id="3"/>`,
		`  <div class="tree-node">`,
		`    <label for="3">foo</label>`,
		`    <span class="cov">33.3%</span>`,
		`  </div>`,
		`  <ul>`,
		strings.TrimRight(subDirHTML, "\n"),
		`  </ul>`,
		`</li>`,
		``,
	}
	hb       := &htmlBuilder{itemID: "3", subDir: "foo"}
	hb.buildSubDirHTML(res)
	got      := res.html
	gotLines := strings.Split(got, "\n")
	var rep reporter
	if !cmp.Equal(wantLines, gotLines, cmp.Reporter(&rep)) {
		t.Errorf("buildSubDirHTML() mismatch (-want +got):\n%s", strings.Join(rep.diffs, "\n"))
	}
}
