package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// treeBuilder stores the state during recursion
type treeBuilder struct {
	fs       fs.FS
	cov      map[string]coverage
	counter  int
	maxWidth int
}

// entryResult stores the results of processing a directory tree containing *.go.html files generated from a Go test coverage profile
type entryResult struct {
	html    string
	covered int
	total   int
}

// writeTreeHTML writes HTML to the specified treeHTML file
func writeTreeHTML(root, treeHTML string, cov map[string]coverage) (int, error) {
	builder := &treeBuilder{
		fs:  os.DirFS(root),
		cov: cov,
	}

	html, err := builder.generateHTML(".")
	if err != nil { return 0, err }

	treeFile, err := os.Create(filepath.Clean(filepath.Join(root, treeHTML)))
	if                                       err != nil { return 0, err }
	if    err := preamble(treeFile);         err != nil { return 0, err }
	if _, err := fmt.Fprint(treeFile, html); err != nil { return 0, err }
	if    err := postamble(treeFile);        err != nil { return 0, err }
	return builder.maxWidth + 10, treeFile.Close() // TODO(jeff): eliminate (or at least document) magic numbers
}

// generateHTML reads a given directory to process its contents
func (tb *treeBuilder) generateHTML(root string) (string, error) {
	entries, err := fs.ReadDir(tb.fs, root)
	if err != nil { return "", err }

	var sb strings.Builder

	sb.WriteString("<ul class=\"tree\">\n")

	for _, entry := range entries {
		res, err := tb.processEntry(root, entry, 1)
		if err != nil { return "", err }
		sb.WriteString(res.html)
	}

	sb.WriteString("</ul>\n")

	return sb.String(), nil
}

// processEntry processes a directory's contents to produce an entryResult for each relevant directory entry encountered
func (tb *treeBuilder) processEntry(parentPath string, entry fs.DirEntry, indent int) (entryResult, error) {
	isDir        := entry.IsDir()
	isTargetFile := !isDir && strings.HasSuffix(entry.Name(), ".go.html")

	if !isDir && !isTargetFile { return entryResult{}, nil }

	src      := strings.TrimSuffix(entry.Name(), ".html")
	srcPath  := filepath.ToSlash(filepath.Join(parentPath, src))
	htmlPath := filepath.ToSlash(filepath.Join(parentPath, entry.Name()))

	width    := indent + len(src)

	if isDir {
		width += 2 // account for the folder icon emoji
	}
	if width > tb.maxWidth {
		tb.maxWidth = width
	}

	if isDir {
		tb.counter++
		itemID := fmt.Sprintf("tree-item-%d", tb.counter)

		subEntries, err := fs.ReadDir(tb.fs, htmlPath)
		if err != nil { return entryResult{}, err }

		var subSB strings.Builder
		var dirCovered, dirStatements int

		for _, subEntry := range subEntries {
			res, err := tb.processEntry(htmlPath, subEntry, indent + 2)
			if err != nil { return entryResult{}, err }
			subSB.WriteString(res.html)
			dirCovered += res.covered
			dirStatements   += res.total
		}

		percent := 0.0
		if dirStatements > 0 {
			percent = float64(dirCovered) / float64(dirStatements) * 100
		}

		indentStr := strings.Repeat("  ", indent)
		var sb strings.Builder

		sb.WriteString(  indentStr + "<li>\n")
		fmt.Fprintf(&sb, indentStr + "  <input type=\"checkbox\" id=\"%s\"/>\n", itemID)
		sb.WriteString(  indentStr + "  <div class=\"tree-node\">\n")
		fmt.Fprintf(&sb, indentStr + "    <label for=\"%s\">%s</label>\n",  itemID, src)
		fmt.Fprintf(&sb, indentStr + "    <span class=\"cov\">%.1f%%</span>\n", percent)
		sb.WriteString(  indentStr + "  </div>\n")
		sb.WriteString(  indentStr + "  <ul>\n")
		sb.WriteString(  subSB.String())
		sb.WriteString(  indentStr + "  </ul>\n")
		sb.WriteString(  indentStr + "</li>\n")

		return entryResult{
			html:    sb.String(),
			covered: dirCovered,
			total:   dirStatements}, nil
	}

	cov     := tb.cov[srcPath]
	percent := 0.0
	if cov.total > 0 {
		percent = float64(cov.covered) / float64(cov.total) * 100
	}

	srcSpan  := fmt.Sprintf("<span class=\"src\"><a href=\"%s\">%s</a></span>", htmlPath, src)
	covSpan  := fmt.Sprintf("<span class=\"cov\">%.1f%%</span>", percent)
	html     := strings.Repeat("  ", indent) + fmt.Sprintf("<li><div class=\"tree-node\">%s %s</div></li>\n", srcSpan, covSpan)
	return entryResult{
		html:    html,
		covered: cov.covered,
		total:   cov.total}, nil
}

// preamble writes the preliminary portion of the tree HTML document
func preamble(f *os.File) error {
	if _, err := fmt.Fprintln(f, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<link rel="stylesheet" href="style.css" type="text/css">
<link rel="stylesheet" href="tree.css"  type="text/css">
<title>Go source tree</title>
<base target="code"/>
</head>
<body id="tree-body">`); err != nil { return err }
	return nil
}

// postamble writes the final portion of the tree HTML document
func postamble(f *os.File) error {
	if _, err := fmt.Fprint(f, `</body>
<script>
try {
  const parentTheme = window.parent.document.documentElement.getAttribute('theme');
  if (parentTheme) document.documentElement.setAttribute('theme', parentTheme);
} catch (e) {
  console.warn('direct parent access blocked by browser; waiting for postMessage');
}

window.addEventListener('message', (event) => {
  if (!event.data) return;
  if (event.data.type === 'SET_THEME'         ) document.documentElement.setAttribute('theme', event.data.theme);
  if (event.data.type === 'EXPAND_OR_COLLAPSE') document.querySelectorAll('.tree input[type="checkbox"]').forEach(cb => cb.checked = event.data.expanded);
});
</script>
</html>`); err != nil { return err }
	return nil
}
