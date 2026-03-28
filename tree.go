package main

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
)

// treeBuilder stores state during processEntry recursion
type treeBuilder struct {
	fsys     writeFS
	outRoot  string
	cov      map[string]coverage
	counter  int
	maxWidth int
}

// entryResult stores the results of processing directory entries containing *.go.html files generated from coverge profiles
type entryResult struct {
	html    string
	covered int
	total   int
}

// htmlBuilder stores data used to render subdirectories in the tree
type htmlBuilder struct {
	indent int
	itemID string
	subDir string
}

// writeTreeHTML writes HTML to the specified treeHTML file
func (tb *treeBuilder) writeTreeHTML() (int, error) {
	html, err := tb.genHTML()
	if err != nil { return 0, err }

	treeFile, err := tb.fsys.Create(filepath.Clean(filepath.Join(tb.outRoot, treeHTML)))
	if                                       err != nil { return 0, err }
	if    err := preamble(  treeFile);       err != nil { return 0, err }
	if _, err := fmt.Fprint(treeFile, html); err != nil { return 0, err }
	if    err := postamble( treeFile);       err != nil { return 0, err }

	// +10 accounts for the coverage percentage width (8ch) plus a 2ch gap,
	// to cohere with "margin-right: 10ch;" in tree.css
	return tb.maxWidth + 10, treeFile.Close()
}

// genHTML reads a given directory to process its contents and generate HTML content
func (tb *treeBuilder) genHTML() (string, error) {
	entries, err := fs.ReadDir(tb.fsys, tb.outRoot)
	if err != nil { return "", err }

	var sb strings.Builder
	sb.WriteString("<ul class=\"tree\">\n")

	for _, entry := range entries {
		res, err := tb.processEntry(".", entry, 1)
		if err != nil { return "", err }
		sb.WriteString(res.html)
	}

	sb.WriteString("</ul>\n")

	return sb.String(), nil
}

// processEntry recursively processes a directory's contents to produce an entryResult for each relevant directory entry encountered
func (tb *treeBuilder) processEntry(relParentPath string, entry fs.DirEntry, indent int) (entryResult, error) {
	isDir        := entry.IsDir()
	isTargetFile := !isDir && strings.HasSuffix(entry.Name(), ".go.html")

	if !isDir && !isTargetFile { return entryResult{}, nil }

	src      := strings.TrimSuffix(entry.Name(), ".html")
	srcPath  := filepath.Clean(filepath.Join(relParentPath, src))
	htmlPath := filepath.Clean(filepath.Join(relParentPath, entry.Name()))

	width    := indent + len(src)

	if isDir { width += 2 } // account for the folder icon emoji
	if width > tb.maxWidth { tb.maxWidth = width }

	if isDir {
		tb.counter++
		itemID := fmt.Sprintf("tree-item-%d", tb.counter)

		fullPath           := filepath.Join(tb.outRoot, htmlPath)
		subDirEntries, err := fs.ReadDir(tb.fsys, fullPath)
		if err != nil { return entryResult{}, err }

		var subDirSB strings.Builder
		var dirCovered, dirStatements int

		for _, subEntry := range subDirEntries {
			res, err := tb.processEntry(htmlPath, subEntry, indent + 2) // indent by an additional two spaces each time we recurse into a subdirectory
			if err != nil { return entryResult{}, err }
			subDirSB.WriteString(res.html)
			dirCovered    += res.covered
			dirStatements += res.total
		}

		hb := &htmlBuilder{
			indent: indent,
			itemID: itemID,
			subDir: src,
		}

		return entryResult{
			html:    hb.buildHTML(subDirSB.String(), dirCovered, dirStatements),
			covered: dirCovered,
			total:   dirStatements}, nil
	}

	cov     := tb.cov[srcPath]
	percent := 0.0
	if cov.total > 0 {
		percent = float64(cov.covered) / float64(cov.total) * 100
	}

	srcSpan := fmt.Sprintf("<span class=\"src\"><a href=\"%s\">%s</a></span>", htmlPath, src)
	covSpan := fmt.Sprintf("<span class=\"cov\">%.1f%%</span>", percent)
	html    := strings.Repeat("  ", indent) + fmt.Sprintf("<li><div class=\"tree-node\">%s %s</div></li>\n", srcSpan, covSpan)

	return entryResult{
		html:    html,
		covered: cov.covered,
		total:   cov.total}, nil
}

// buildHTML builds an HTML string used to render a subdirectory in the tree
func (hb *htmlBuilder) buildHTML(subDirHTML string, dirCovered, dirStatements int) string {
	percent := 0.0
	if dirStatements > 0 {
		percent = float64(dirCovered) / float64(dirStatements) * 100
	}

	indentStr := strings.Repeat("  ", hb.indent)
	var sb strings.Builder

	sb.WriteString(  indentStr + "<li>\n")
	fmt.Fprintf(&sb, indentStr + "  <input type=\"checkbox\" id=\"%s\"/>\n", hb.itemID)
	sb.WriteString(  indentStr + "  <div class=\"tree-node\">\n")
	fmt.Fprintf(&sb, indentStr + "    <label for=\"%s\">%s</label>\n",       hb.itemID, hb.subDir)
	fmt.Fprintf(&sb, indentStr + "    <span class=\"cov\">%.1f%%</span>\n",  percent)
	sb.WriteString(  indentStr + "  </div>\n")
	sb.WriteString(  indentStr + "  <ul>\n")
	sb.WriteString(  subDirHTML )
	sb.WriteString(  indentStr + "  </ul>\n")
	sb.WriteString(  indentStr + "</li>\n")

	return sb.String()
}

// preamble writes the preliminary portion of the tree HTML document
func preamble(w io.Writer) error {
	if _, err := fmt.Fprintln(w, `<!DOCTYPE html>
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
func postamble(w io.Writer) error {
	if _, err := fmt.Fprint(w, `</body>
<script>
try {
  const parentTheme = window.parent.document.documentElement.getAttribute('theme');
  if (parentTheme) document.documentElement.setAttribute('theme', parentTheme);
} catch (e) {
  console.warn('direct parent access blocked by browser; waiting for postMessage');
}

window.addEventListener('message', (event) => {
  if (!event.data) return;
  if (event.data.type === 'SET_THEME') document.documentElement.setAttribute('theme', event.data.theme);
  if (event.data.type === 'EXPAND_OR_COLLAPSE') document.querySelectorAll('.tree input[type="checkbox"]').forEach(cb => cb.checked = event.data.expanded);
});
</script>
</html>`); err != nil { return err }
	return nil
}
