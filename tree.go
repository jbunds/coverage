package main

import (
	"context"
	"io"
	"io/fs"
	"math"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/jbunds/progress"
	"golang.org/x/sync/errgroup"
)

// treeBuilder renders the source tree HTML fragment.
type treeBuilder struct {
	fsys     writeFS        // directory listing (thin wrapper around os.ReadDir)
	modPaths []string       // module paths; https://go.dev/ref/mod#glos-module-path
	outRoot  rootHandle     // output root for the generated HTML files, per -outdir
	covState *coverageState // accumulated coverage data
	counter  atomic.Uint64  // ID for each subdirectory node (<input type="checkbox"> and its <label>)
}

// scanState holds per-iteration state during recursive
// directory traversal and incremental progress tracking.
type scanState struct {
	parentPath string             // package prefix for the current branch
	entry      fs.DirEntry        // directory entry currently being processed
	indent     int                // <ul> nesting depth (2 spaces per level)
	prog       *progress.Progress // incremental progress tracker
	budget     float64            // progress budget allocated for this branch
}

// entryResult stores the results of processing a directory
// entry containing the generated *.go.html files.
type entryResult struct {
	pkgPath     string // package path (https://go.dev/ref/mod#glos-package-path)
	srcBasename string // subdirectory or file basename of a directory entry in the source tree
	html        string // HTML fragment for the entry
	covered     uint64 // weighted covered statement count
	total       uint64 // total statement count
}

// htmlBuilder renders source tree HTML fragments.
type htmlBuilder struct {
	indent int    // <ul> nesting depth (2 spaces per level)
	subDir string // subdirectory name
	itemID string // subdirectory node ID ("tree-item-%d")
}

// buildTree traverses the module-qualified output directory and returns
// the complete <ul> HTML fragment representing the source tree, with
// per-file and aggregate per-subdirectory coverage percentages.
func (tb *treeBuilder) buildTree(ctx context.Context, progressOutput io.Writer) (string, error) {
	if err := ctx.Err(); err != nil { return "", err }

	prog := progress.New(ctx, 0, progressOutput)
	defer prog.Close()

	rootNode := &trieNode{children: make(map[string]*trieNode)}

	for _, modPath := range tb.modPaths {
		scanRoot     := filepath.Join(tb.outRoot.Name(), modPath)
		entries, err := tb.fsys.ReadDir(scanRoot)
		if err != nil { return "", err }

		baseIndent := 2 * strings.Count(modPath, "/") + 3 // initial nesting depth: root <ul>, current subdir <li>, and inner <ul> wrapping source file <li> elements

		results, err := tb.scanEntries(ctx, prog, modPath, baseIndent, entries) // TODO(jbunds): maybe parallelize the call to tb.scanEntries
		if err != nil { return "", err }

		var modCovered, modTotal uint64
		var sb strings.Builder
		for _, res := range results {
			sb.WriteString(res.html)
			modCovered += res.covered
			modTotal   += res.total
		}

		rootNode.insert(modPath, sb.String(), modCovered, modTotal)
	}

	return rootNode.renderHTML(&tb.counter), nil
}

// scanEntries processes each entry in a directory, collecting the resulting
// tree nodes and aggregating their statement and coverage counts.
func (tb *treeBuilder) scanEntries(ctx context.Context, prog *progress.Progress, modPath string, baseIndent int, entries []fs.DirEntry) ([]*entryResult, error ) {
	if err := ctx.Err(); err != nil { return nil, err }

	if len(entries) < 1 { return nil, nil }

	results := make([]*entryResult, len(entries))
	budgets := splitBudget(prog.InitialBudget(), len(entries))

	group, gCtx := errgroup.WithContext(ctx)
	group.SetLimit(runtime.NumCPU())

	for i, entry := range entries {
		if err := gCtx.Err(); err != nil { break }
		group.Go(func() error {
			st := &scanState{
				parentPath: modPath,
				entry:      entry,
				indent:     baseIndent,
				prog:       prog,
				budget:     budgets[i],
			}
			res, err := tb.processEntry(gCtx, st)
			if err != nil { return err }

			results[i] = res
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

// processEntry recursively builds ordered HTML tree nodes and aggregates
// coverage metrics for individual files and subdirectories.
func (tb *treeBuilder) processEntry(ctx context.Context, st *scanState) (*entryResult, error) {
	if err := ctx.Err(); err != nil { return nil, err }

	isDir        := st.entry.IsDir()
	isTargetFile := !isDir && strings.HasSuffix(st.entry.Name(), ".go.html")

	if !isDir && !isTargetFile {
		st.prog.Report(st.budget, st.entry.Name())
		return nil, nil
	}

	srcBasename := strings.TrimSuffix(st.entry.Name(), ".html")
	pkgPath     := filepath.Join(st.parentPath, srcBasename)

	if isDir {
		return tb.processDir(ctx, st, pkgPath, srcBasename)
	}
	return tb.processFile(st, pkgPath, srcBasename)
}

// processDir renders a <li> tree-node for a directory, with nested <li> nodes for
// its subdirectories and source files, including aggregated coverage percentage.
func (tb *treeBuilder) processDir(ctx context.Context, st *scanState, pkgPath, srcBasename string) (*entryResult, error) {
	if err := ctx.Err(); err != nil { return nil, err }

	itemID   := "tree-item-" + strconv.FormatUint(tb.counter.Add(1), 10)
	fullPath := filepath.Join(tb.outRoot.Name(), st.parentPath, st.entry.Name())

	subDirEntries, err := tb.fsys.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	var subDirSB strings.Builder
	var dirCovered, dirStatements atomic.Uint64

	if len(subDirEntries) > 0 {
		budgets      := splitBudget(st.budget, len(subDirEntries))
		childResults := make([]*entryResult, len(subDirEntries))

		group, gCtx := errgroup.WithContext(ctx)
		group.SetLimit(runtime.NumCPU())

		for i, subDirEntry := range subDirEntries {
			group.Go(func() error {
				childState := &scanState{
					parentPath: pkgPath,
					entry:      subDirEntry,
					indent:     st.indent + 2,
					prog:       st.prog,
					budget:     budgets[i],
				}
				res, err := tb.processEntry(gCtx, childState)
				if err != nil { return err }

				childResults[i] = res
				return nil
			})
		}

    if err := group.Wait(); err != nil {
      return nil, err
    }

		slices.SortFunc(childResults, func(a, b *entryResult) int {
			return strings.Compare(a.srcBasename, b.srcBasename)
		})
    
		for _, res := range childResults {
			dirCovered.Add(res.covered)
			dirStatements.Add(res.total)
			subDirSB.WriteString(res.html)
		}
	}

  res := &entryResult{
    pkgPath:     pkgPath,
    srcBasename: srcBasename,
    html:        subDirSB.String(),
    covered:     dirCovered.Load(),
    total:       dirStatements.Load(),
  }
  
  hb := &htmlBuilder{indent: st.indent, itemID: itemID, subDir: srcBasename}
  hb.buildSubDirHTML(res)

  return res, nil
}

// processFile renders a single <li> tree-node for a Go source file, including its coverge percentage.
func (tb *treeBuilder) processFile(st *scanState, pkgPath, srcBasename string) (*entryResult, error) {
	cov     := tb.covState.cov[pkgPath]
	percent := formatCov(cov.covered, cov.total)

	var sb strings.Builder
	sb.Grow(st.indent * 2 + 128) // rough pre-allocation to avoid reallocations; should cover most cases
	sb.WriteString(strings.Repeat("  ", st.indent))
	sb.WriteString(`<li><div class="tree-node"><span class="src"><a href="`)
	sb.WriteString(filepath.Join(st.parentPath, st.entry.Name()))
	sb.WriteString(`">`)
	sb.WriteString(srcBasename)
	sb.WriteString(`</a></span> <span class="cov">`)
	sb.WriteString(percent)
	sb.WriteString("</span></div></li>\n")

	st.prog.Report(st.budget, pkgPath)

	return &entryResult{
		pkgPath:     pkgPath,
		srcBasename: srcBasename,
		html:        sb.String(),
		covered:     cov.covered,
		total:       cov.total,
	}, nil
}

// buildSubDirHTML wraps pre-rendered child nodes in a <li> subdirectory
// tree-node, with its name and aggregated coverge percentage.
func (hb *htmlBuilder) buildSubDirHTML(res *entryResult) {
	indent  := strings.Repeat("  ", hb.indent)
	percent := formatCov(res.covered, res.total)

	var sb strings.Builder

	sb.WriteString(indent)
	sb.WriteString("<li>\n")
	sb.WriteString(indent)
	sb.WriteString(`  <input type="checkbox" id="`)
	sb.WriteString(hb.itemID)
	sb.WriteString("\"/>\n")
	sb.WriteString(indent)
	sb.WriteString("  <div class=\"tree-node\">\n")
	sb.WriteString(indent)
	sb.WriteString(`    <label for="`)
	sb.WriteString(hb.itemID)
	sb.WriteString(`">`)
	sb.WriteString(hb.subDir)
	sb.WriteString("</label>\n")
	sb.WriteString(indent)
	sb.WriteString(`    <span class="cov">`)
	sb.WriteString(percent)
	sb.WriteString("</span>\n")
	sb.WriteString(indent)
	sb.WriteString("  </div>\n")
	sb.WriteString(indent)
	sb.WriteString("  <ul>\n")
	sb.WriteString(res.html)
	sb.WriteString(indent)
	sb.WriteString("  </ul>\n")
	sb.WriteString(indent)
	sb.WriteString("</li>\n")

	res.html = sb.String()
}

// formatCov returns covered/total as a one-decimal percentage
// string (e.g. "12.3%", or "0.0%" if total is zero).
func formatCov(covered, total uint64) string {
	if total == 0 {
		return "0.0%"
	}
	return strconv.FormatFloat(float64(covered) / float64(total) * 100, 'f', 1, 64) + "%"
}

// splitBudget divides a progress budget into n child allocations,
// with the last child absorbing any remainder.
func splitBudget(total float64, n int) []float64 {
	if n <= 0 { return []float64{} } // defensive; callers are expected to pass n ≥ 1
	budgets := make([]float64, n)
	per     := total / float64(n)
	for i := range budgets {
		budgets[i] = per
	}
	budgets[n - 1] = math.FMA(per, -float64(n - 1), total) // last child absorbs any remainder
	return budgets
}
