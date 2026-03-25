package main

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestGenerateHTML(t *testing.T) {
	fsys := fstest.MapFS{
		"a.go.html":            &fstest.MapFile{Data: []byte("content")},
		"dir/b.go.html":        &fstest.MapFile{Data: []byte("content")},
		"dir/subdir/c.go.html": &fstest.MapFile{Data: []byte("content")},
	}

	cov := map[string]coverage{
		"a.go":            {covered: 10, total: 10},
		"dir/b.go":        {covered:  5, total: 10},
		"dir/subdir/c.go": {covered:  0, total: 10},
	}

	tb        := &treeBuilder{fs: fsys, cov: cov}
	html, err := tb.generateHTML(".")
	if err != nil {
		t.Fatalf("generateHTML failed: %v", err)
	}

	//t.Logf("generated HTML:\n%s", html)

	// check top-level indentation for file
	if !strings.Contains(html, "\n  <li><div class=\"tree-node\"><span class=\"src\"><a href=\"a.go.html\">a.go</a>") {
		t.Errorf("top-level file <li> should be indented by 2 spaces")
	}

	// check top-level directory indentation
	if !strings.Contains(html, "\n  <li>\n    <input type=\"checkbox\" id=\"tree-item-1\"/>") {
		t.Errorf("top-level directory <li> should be indented by 2 spaces")
	}

	// check nested file indentation (b.go)
	if !strings.Contains(html, "\n      <li><div class=\"tree-node\"><span class=\"src\"><a href=\"dir/b.go.html\">b.go</a>") {
		t.Errorf("nested file b.go <li> should be indented by 6 spaces")
	}

	// check nested directory indentation (subdir)
	if !strings.Contains(html, "\n      <li>\n        <input type=\"checkbox\" id=\"tree-item-2\"/>") {
		t.Errorf("nested directory subdir <li> should be indented by 6 spaces")
	}
}
