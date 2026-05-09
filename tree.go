package main

import (
	"context"
	"io"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/jbunds/progress"
	"golang.org/x/sync/errgroup"
)

// treeBuilder manages the global configuration, coverage data, and
// and atomic counters used to create the directory tree HTML.
type treeBuilder struct {
	fsys     writeFS
	modName  string
	outRoot  string
	cov      map[string]coverage
	counter  atomic.Int64
	maxWidth atomic.Int64
}

// scanState captures the ephemeral, per-iteration state required for 
// recursive directory traversal and incremental progress tracking.
type scanState struct {
	parentPath string             // logical Go package prefix for the current branch
	entry      fs.DirEntry        // specific file or directory currently being processed
	indent     int                // current indentation level of nested UL elements
	prog       *progress.Progress // progress tracker
	budget     float64            // progress budget allocated for this branch
}

// entryResult stores the results of processing directory entries
// containing *.go.html files generated from coverge profiles.
type entryResult struct {
	html    string
	covered int64
	total   int64
}

// htmlBuilder stores the state used to render the navigable directory tree (tree.html).
type htmlBuilder struct {
	indent int
	itemID string
	subDir string
}

// writeTreeHTML writes the tree HTML (tree.html) file.
func (tb *treeBuilder) writeTreeHTML(ctx context.Context, progressOutput io.Writer) (int, error) {
	if err := ctx.Err(); err != nil { return 0, err }

	html, err := tb.genHTML(ctx, progressOutput)
	if err != nil { return 0, err }

	treeFile, err := tb.fsys.Create(ctx, filepath.Join(tb.outRoot, treeHTML))
	if                                           err != nil { return 0, err }
	if    err := preamble      (ctx,  treeFile); err != nil { return 0, err }
	if _, err := io.WriteString(treeFile, html); err != nil { return 0, err }
	if    err := postamble     (ctx,  treeFile); err != nil { return 0, err }

	return int(tb.maxWidth.Add(10)), treeFile.Close() // +10 == len("100.0%") + 2ch (gap) to cohere with "margin-right: 10ch;" in tree.css
}

// genHTML recursively traverses the output directory to generate the nested <ul> and <li> HTML string representing the file coverage tree
func (tb *treeBuilder) genHTML(ctx context.Context, progressOutput io.Writer) (string, error) {
	if err := ctx.Err(); err != nil { return "", err }

	pkgRelRoot := strings.Split(tb.modName, "/")[0]     // module's top-level namspace
	scanRoot   := filepath.Join(tb.outRoot, pkgRelRoot) // physical directory entry point for recursive scan

	entries, err := tb.fsys.ReadDir(ctx, scanRoot)
	if err != nil { return "", err }

	prog := progress.New(ctx, 0, progressOutput)
	defer prog.Close()

	results         := make([]string, len(entries))
	initialBudget   := prog.InitialBudget()
	budgetPerEntry  := initialBudget / float64(len(entries))
	remainingBudget := initialBudget

	group, gCtx := errgroup.WithContext(ctx)
	group.SetLimit(runtime.NumCPU()) // saturate available CPU threads for maximum throughput while bounding memory used by concurrent HTML buffers

	for i, entry := range entries {
		if err := gCtx.Err(); err != nil { break }
		var currentBudget float64
		if i == len(entries) - 1 {
			currentBudget    = remainingBudget
		} else {
			currentBudget    = budgetPerEntry
			remainingBudget -= currentBudget
		}
		group.Go(func() error {
			st := scanState{
				parentPath: pkgRelRoot,
				entry:      entry,
				indent:     1,
				prog:       prog,
				budget:     currentBudget,
			}
			res, err := tb.processEntry(gCtx, st)
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

// processEntry recursively builds ordered HTML tree nodes and aggregates coverage metrics for individual files and directories.
func (tb *treeBuilder) processEntry(ctx context.Context, st scanState) (entryResult, error) {
	if err := ctx.Err(); err != nil { return entryResult{}, err }

	isDir        := st.entry.IsDir()
	isTargetFile := !isDir && strings.HasSuffix(st.entry.Name(), ".go.html")

	if !isDir && !isTargetFile {
		st.prog.Report(st.budget, "") // ensure progress ultimately adds up to 100% by consuming budget even if a file is not processed
		return entryResult{}, nil
	}

	srcBasename := strings.TrimSuffix(st.entry.Name(), ".html")  // basename of the subdirectory or source file
	pkgPath     := filepath.Join(st.parentPath, srcBasename)     // package-normalized path used as the key for coverage map lookup
	relHTMLPath := filepath.Join(st.parentPath, st.entry.Name()) // physical path relative to tb.outRoot

	width := int64(st.indent + len(srcBasename))
	if isDir { width += 2 } // account for the folder icon emoji
	for {
		current := tb.maxWidth.Load()
		if width <= current                           { break }
		if tb.maxWidth.CompareAndSwap(current, width) { break }
	}

	if isDir {
		itemID             := "tree-item-" + strconv.FormatInt(tb.counter.Add(1), 10)
		fullPath           := filepath.Join(tb.outRoot, relHTMLPath)
		subDirEntries, err := tb.fsys.ReadDir(ctx, fullPath)
		if err != nil { return entryResult{}, err }

		var subDirSB strings.Builder
		var dirCovered, dirStatements int64

		if len(subDirEntries) > 0 { // split this subdir's budget up among its children
			childBudget := st.budget / float64(len(subDirEntries))
			remaining   := st.budget

			for i, subDirEntry := range subDirEntries {
				var subDirBudget float64
				if i == len(subDirEntries) - 1 {
					subDirBudget = remaining // the last child takes on the remainder
				} else {
					subDirBudget = childBudget
					remaining   -= subDirBudget
				}

				childState := scanState{
					parentPath: pkgPath,
					entry:      subDirEntry,
					indent:     st.indent + 2,
					prog:       st.prog,
					budget:     subDirBudget,
				}

				res, err := tb.processEntry(ctx, childState)
				if err != nil { return entryResult{}, err }

				subDirSB.WriteString(res.html)
				dirCovered    += res.covered
				dirStatements += res.total
			}
		} else {
			st.prog.Report(st.budget, pkgPath) // inform the progress tracker that pkgPath has been processed
		}

		hb := &htmlBuilder{
			indent: st.indent,
			itemID: itemID,
			subDir: srcBasename,
		}

		html, err := hb.buildHTML(ctx, subDirSB.String(), dirCovered, dirStatements)
		if err != nil { return entryResult{}, err }

		return entryResult{
			html:    html,
			covered: dirCovered,
			total:   dirStatements}, nil
	}

	st.prog.Report(st.budget, pkgPath) // inform the progress tracker that pkgPath has been processed

	cov     := tb.cov[pkgPath]
	percent := 0.0
	if cov.total > 0 {
		percent = float64(cov.covered) / float64(cov.total) * 100
	}

	pct     := strconv.FormatFloat(percent, 'f', 1, 64)
	srcSpan := "<span class=\"src\"><a href=\"" + relHTMLPath + "\">" + srcBasename + "</a></span>"
	covSpan := "<span class=\"cov\">" + pct + "%</span>"

	return entryResult{
		html:    strings.Repeat("  ", st.indent) + "<li><div class=\"tree-node\">" + srcSpan + " " + covSpan + "</div></li>\n",
		covered: cov.covered,
		total:   cov.total}, nil
}

// buildHTML builds an HTML string used to render a subdirectory in the tree.
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

// preamble writes the preliminary portion of the tree HTML document.
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

// postamble writes the final portion of the tree HTML document.
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
