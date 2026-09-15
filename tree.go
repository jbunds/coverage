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

// buildTreeHTML recursively traverses the output directory to build
// the nested <ul> and <li> HTML string representing the source tree.
func (tb *treeBuilder) buildTreeHTML(ctx context.Context, progressOutput io.Writer) (string, error) {
	if err := ctx.Err(); err != nil { return "", err }

	modDomain, _, _ := strings.Cut(tb.modName, "/")         // module's top-level namespace
	scanRoot        := filepath.Join(tb.outRoot, modDomain) // physical directory entry point for recursive scan

	entries, err := tb.fsys.ReadDir(ctx, scanRoot)
	if err != nil { return "", err }

	prog := progress.New(ctx, 0, progressOutput)
	defer prog.Close()

	var totalStatements, totalCovered atomic.Int64

	results         := make([]entryResult, len(entries) + 2) // +2 pre-allocates slots for the rootTreeNode and its corresponding closing </li> tag
	initialBudget   := prog.InitialBudget()
	budgetPerEntry  := initialBudget / float64(len(entries)) // assumes len(entries) > 0
	remainingBudget := initialBudget

	group, gCtx := errgroup.WithContext(ctx)
	group.SetLimit(runtime.NumCPU()) // saturate available CPU threads for maximum throughput while bounding memory used by concurrent HTML buffers

	for i, entry := range entries {
		if err := gCtx.Err(); err != nil { break }
		currentBudget := budgetPerEntry
		if i == len(entries) - 1 {
			currentBudget = remainingBudget
		}
		remainingBudget -= currentBudget
		group.Go(func() error {
			st := scanState{
				parentPath: modDomain,
				entry:      entry,
				indent:     3, // 2 levels of indentation are added by prepending rootTreeNode to results below
				prog:       prog,
				budget:     currentBudget,
			}
			res, err := tb.processEntry(gCtx, st)
			if err != nil { return err }
			results[i + 1]  = res
			totalStatements.Add(res.total)
			totalCovered.Add(res.covered)
			return nil
		})
	}

	if err := group.Wait(); err != nil { return "", err }

	totStatements := totalStatements.Load()
	totCovered    := totalCovered.Load()
	aggregatePercent := "0.0"
	if totStatements > 0 {
		aggregatePercent = strconv.FormatFloat(float64(totCovered) / float64(totStatements) * 100, 'f', 1, 64)
	}

	rootTreeNode := entryResult{html: `  <li>
    <input type="checkbox" id="tree-item-0"/>
    <div class="tree-node">
      <label for="tree-item-0">` + modDomain        + `</label>
      <span class="cov">`        + aggregatePercent + `%</span>
    </div>
    <ul>
`}

	// the first and last slots were pre-allocated when results was initialized
	results[0               ] = rootTreeNode
	results[len(entries) + 1] = entryResult{html: "    </ul>\n  </li>\n"}

	var sb strings.Builder
	sb.WriteString(`<ul class="tree">` + "\n")
	for _, res := range results {
		sb.WriteString(res.html)
	}
	sb.WriteString("</ul>")

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

	if isDir {
		itemID             := "tree-item-" + strconv.FormatInt(tb.counter.Add(1), 10)
		fullPath           := filepath.Join(tb.outRoot, relHTMLPath)
		subDirEntries, err := tb.fsys.ReadDir(ctx, fullPath)
		if err != nil { return entryResult{}, err }

		var subDirSB strings.Builder
		var dirCovered, dirStatements atomic.Int64

		if len(subDirEntries) > 0 { // split this subdir's budget up among its children
			childBudget     := st.budget / float64(len(subDirEntries))
			remainingBudget := st.budget

			for i, subDirEntry := range subDirEntries {
				subDirBudget := childBudget
				if i == len(subDirEntries) - 1 {
					subDirBudget = remainingBudget // the last child takes on the remainder
				}
				remainingBudget -= subDirBudget

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
				dirCovered.Add(res.covered)
				dirStatements.Add(res.total)
			}
		} else {
			st.prog.Report(st.budget, pkgPath) // inform the progress tracker that pkgPath has been processed
		}

		hb := &htmlBuilder{
			indent: st.indent,
			itemID: itemID,
			subDir: srcBasename,
		}

		html, err := hb.buildHTML(ctx, subDirSB.String(), dirCovered.Load(), dirStatements.Load())
		if err != nil { return entryResult{}, err }

		return entryResult{
			html:    html,
			covered: dirCovered.Load(),
			total:   dirStatements.Load()}, nil
	}

	st.prog.Report(st.budget, pkgPath) // inform the progress tracker that pkgPath has been processed

	cov     := tb.cov[pkgPath]
	percent := 0.0
	if cov.total > 0 {
		percent = float64(cov.covered) / float64(cov.total) * 100
	}

	srcSpan := `<span class="src"><a href="` + relHTMLPath + `">` + srcBasename + "</a></span>"
	covSpan := `<span class="cov">` + strconv.FormatFloat(percent, 'f', 1, 64) + "%</span>"

	return entryResult{
		html:    strings.Repeat("  ", st.indent) + `<li><div class="tree-node">` + srcSpan + " " + covSpan + "</div></li>\n",
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

	return indent     + "<li>\n"                                                  +
	       indent     + `  <input type="checkbox" id="` + id + `"/>` + "\n"       +
	       indent     + `  <div class="tree-node">`  + "\n"                       +
	       indent     + `    <label for="` + id + `">` + hb.subDir + "</label>\n" +
	       indent     + `    <span class="cov">` + pct + "%</span>\n"             +
	       indent     + "  </div>\n"                                              +
	       indent     + "  <ul>\n"                                                +
	       subDirHTML                                                             +
	       indent     + "  </ul>\n"                                               +
	       indent     + "</li>\n", nil
}
