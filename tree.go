package main

import (
	"context"
	"io"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/jbunds/progress"
	"golang.org/x/sync/errgroup"
)

// treeBuilder stores state during processEntry recursion
type treeBuilder struct {
	fsys     writeFS
	outRoot  string
	cov      map[string]coverage
	counter  atomic.Int64
	maxWidth atomic.Int64
}

// entryResult stores the results of processing directory entries containing *.go.html files generated from coverge profiles
type entryResult struct {
	html    string
	covered int64
	total   int64
}

// htmlBuilder stores the state used to render the navigable directory tree (tree.html)
type htmlBuilder struct {
	indent int
	itemID string
	subDir string
}

// writeTreeHTML writes the tree HTML file (canonically, tree.html)
func (tb *treeBuilder) writeTreeHTML(ctx context.Context, progressOutput io.Writer) (int, error) {
	if err := ctx.Err(); err != nil { return 0, err }

	html, err := tb.genHTML(ctx, progressOutput)
	if err != nil { return 0, err }

	treeFile, err := tb.fsys.Create(ctx, filepath.Clean(filepath.Join(tb.outRoot, treeHTML)))
	if                                           err != nil { return 0, err }
	if    err := preamble      (ctx,  treeFile); err != nil { return 0, err }
	if _, err := io.WriteString(treeFile, html); err != nil { return 0, err }
	if    err := postamble     (ctx,  treeFile); err != nil { return 0, err }

	return int(tb.maxWidth.Add(10)), treeFile.Close() // +10 == len("100.0%") + 2ch (gap) to cohere with "margin-right: 10ch;" in tree.css
}

// genHTML recursively traverses the output directory to generate the nested <ul> and <li> HTML string representing the file coverage tree
func (tb *treeBuilder) genHTML(ctx context.Context, progressOutput io.Writer) (string, error) {
	if err := ctx.Err(); err != nil { return "", err }

	entries, err := fs.ReadDir(tb.fsys, tb.outRoot)
	if err != nil { return "", err }

	prog := progress.New(ctx, 0, progressOutput)
	defer prog.Close(ctx)

	group, gCtx    := errgroup.WithContext(ctx)
	results        := make([]string, len(entries))
	budgetPerEntry := prog.InitialBudget() / float64(len(entries))

	for i, entry := range entries {
		group.Go(func() error {
			res, err := tb.processEntry(gCtx, ".", entry, 1, prog, budgetPerEntry)
			if err != nil { return err }
			results[i] = res.html
			return nil
		})
	}

	if err := group.Wait(); err != nil { return "", err }

	var sb strings.Builder
	sb.WriteString("<ul class=\"tree\">\n")
	for _, html := range results {
		sb.WriteString(html)
	}
	sb.WriteString("</ul>\n")

	return sb.String(), nil
}


// processEntry recursively builds ordered HTML tree nodes and aggregates coverage metrics for individual files and directories
func (tb *treeBuilder) processEntry(ctx context.Context, relParentPath string, entry fs.DirEntry, indent int, prog *progress.Progress, budget float64) (entryResult, error) {
	if err := ctx.Err(); err != nil { return entryResult{}, err }

	isDir        := entry.IsDir()
	isTargetFile := !isDir && strings.HasSuffix(entry.Name(), ".go.html")

	if !isDir && !isTargetFile {
		prog.Report(budget, "") // ensure progress ultimately adds up to 100% by consuming budget even if the file is not processed
		return entryResult{}, nil
	}

	src      := strings.TrimSuffix(entry.Name(), ".html")         // normalized filename
	srcPath  := filepath.Clean(filepath.Join(relParentPath, src)) // package-normalized path
	htmlPath := filepath.Clean(filepath.Join(relParentPath, entry.Name()))

	width := int64(indent + len(src))
	if isDir { width += 2 } // account for the folder icon emoji
	for {
		current := tb.maxWidth.Load()
		if width <= current {
			break
		}
		if tb.maxWidth.CompareAndSwap(current, width) {
			break
		}
	}

	if isDir {
		itemID             := "tree-item-" + strconv.FormatInt(tb.counter.Add(1), 10)
		fullPath           := filepath.Join(tb.outRoot, htmlPath)
		subDirEntries, err := fs.ReadDir(tb.fsys, fullPath)
		if err != nil { return entryResult{}, err }

		var subDirSB strings.Builder
		var dirCovered, dirStatements int64

		if len(subDirEntries) > 0 { // split this subdir's budget up among its children
			childBudget := budget / float64(len(subDirEntries))
			remaining   := budget

			for i, subDirEntry := range subDirEntries {
				var subDirBudget float64
				if i == len(subDirEntries) - 1 {
					subDirBudget += remaining // the last child takes on the remainder
				} else {
					subDirBudget = childBudget
					remaining   -= subDirBudget
				}

				res, err := tb.processEntry(ctx, srcPath, subDirEntry, indent + 2, prog, subDirBudget)
				if err != nil { return entryResult{}, err }

				subDirSB.WriteString(res.html)
				dirCovered    += res.covered
				dirStatements += res.total
			}
		} else {
			prog.Report(budget, srcPath) // inform the progress tracker that srcPath has been processed
		}

		hb := &htmlBuilder{
			indent: indent,
			itemID: itemID,
			subDir: src,
		}

		html, err := hb.buildHTML(ctx, subDirSB.String(), dirCovered, dirStatements)
		if err != nil { return entryResult{}, err }

		return entryResult{
			html:    html,
			covered: dirCovered,
			total:   dirStatements}, nil
	}

	prog.Report(budget, srcPath) // inform the progress tracker that srcPath has been processed

	cov     := tb.cov[srcPath]
	percent := 0.0
	if cov.total > 0 {
		percent = float64(cov.covered) / float64(cov.total) * 100
	}

	pct     := strconv.FormatFloat(percent, 'f', 1, 64)
	srcSpan := "<span class=\"src\"><a href=\"" + htmlPath + "\">" + src + "</a></span>"
	covSpan := "<span class=\"cov\">" + pct + "%</span>"

	return entryResult{
		html:    strings.Repeat("  ", indent) + "<li><div class=\"tree-node\">" + srcSpan + " " + covSpan + "</div></li>\n",
		covered: cov.covered,
		total:   cov.total}, nil
}

// buildHTML builds an HTML string used to render a subdirectory in the tree
func (hb *htmlBuilder) buildHTML(ctx context.Context, subDirHTML string, dirCovered, dirStatements int64) (string, error) {
	if err := ctx.Err(); err != nil { return "", err }

	percent := 0.0
	if dirStatements > 0 {
		percent = float64(dirCovered) / float64(dirStatements) * 100
	}

	indent := strings.Repeat("  ", hb.indent)
	id     := hb.itemID
	pct    := strconv.FormatFloat(percent, 'f', 1, 64)

	return indent     + "<li>\n"                                                    +
	       indent     + "  <input type=\"checkbox\" id=\"" + id + "\"/>\n"          +
	       indent     + "  <div class=\"tree-node\">\n"                             +
	       indent     + "    <label for=\"" + id + "\">" + hb.subDir + "</label>\n" +
	       indent     + "    <span class=\"cov\">" + pct + "%</span>\n"             +
	       indent     + "  </div>\n"                                                +
	       indent     + "  <ul>\n"                                                  +
	       subDirHTML                                                               +
	       indent     + "  </ul>\n"                                                 +
	       indent     + "</li>\n", nil
}

// preamble writes the preliminary portion of the tree HTML document
func preamble(ctx context.Context, w io.Writer) error {
	if err := ctx.Err(); err != nil { return err }
	const content = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<link rel="stylesheet" href="style.css" type="text/css">
<link rel="stylesheet" href="tree.css"  type="text/css">
<title>Go source tree</title>
<base target="code"/>
</head>
<body id="tree-body">
`
	_, err := io.WriteString(w, content)
	return err
}

// postamble writes the final portion of the tree HTML document
func postamble(ctx context.Context, w io.Writer) error {
	if err := ctx.Err(); err != nil { return err }
	const content = `</body>
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
</html>`
	_, err := io.WriteString(w, content)
	return err
}
