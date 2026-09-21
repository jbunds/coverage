package main

import (
	"context"
	"io"
	"io/fs"
	"math"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/jbunds/progress"
	"golang.org/x/sync/errgroup"
)

// treeBuilder manages the global configuration, coverage data, and
// atomic counters used to create the directory tree HTML fragment.
type treeBuilder struct {
	fsys     writeFS
	modName  string
	outRoot  rootHandle
	cov      map[string]coverage
	counter  atomic.Uint64
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
	covered uint64
	total   uint64
}

// htmlBuilder stores the state used to render the navigable source tree HTML.
type htmlBuilder struct {
	indent int
	itemID string
	subDir string
}

// buildTree traverses the module-qualified output directory and returns
// the complete <ul> HTML fragment representing the source tree, with
// per-file and aggregate per-subdirectory coverage percentages.
func (tb *treeBuilder) buildTree(ctx context.Context, progressOutput io.Writer) (string, error) {
	if err := ctx.Err(); err != nil { return "", err }

	modDomain, _, _ := strings.Cut(tb.modName, "/")
	scanRoot        := filepath.Join(tb.outRoot.Name(), modDomain)

	entries, err := tb.fsys.ReadDir(scanRoot)
	if err != nil {
		return "", err
	}

	prog := progress.New(ctx, 0, progressOutput)
	defer prog.Close()

	// TODO(jbunds): calculate aggregate coverage percentage purely from results
	results, totStatements, totCovered, err := tb.scanEntries(ctx, prog, modDomain, entries)
	if err != nil { return "", err }

	return buildTreeHTML(modDomain, results, totStatements, totCovered), nil
}

// scanEntries processes each entry in a directory, collecting the resulting
// tree nodes and aggregating their statement and coverage counts.
func (tb *treeBuilder) scanEntries(ctx context.Context, prog *progress.Progress, modDomain string, entries []fs.DirEntry) ([]*entryResult, uint64, uint64, error) {
	if err := ctx.Err(); err != nil { return nil, 0, 0, err }

	if len(entries) < 1 { return nil, 0, 0, nil }

	results := make([]*entryResult, len(entries))

	var totalStatements, totalCovered atomic.Uint64
	budgets := splitBudget(prog.InitialBudget(), len(entries))

	group, gCtx := errgroup.WithContext(ctx)
	group.SetLimit(runtime.NumCPU())

	for i, entry := range entries {
		if err := gCtx.Err(); err != nil { break }
		group.Go(func() error {
			st := &scanState{
				parentPath: modDomain,
				entry:      entry,
				indent:     3,
				prog:       prog,
				budget:     budgets[i],
			}
			res, err := tb.processEntry(gCtx, st)
			if err != nil { return err }
			results[i] = res
			totalStatements.Add(res.total)
			totalCovered.Add(res.covered)
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, 0, 0, err
	}

	return results, totalStatements.Load(), totalCovered.Load(), nil
}

// processEntry recursively builds ordered HTML tree nodes and aggregates
// coverage metrics for individual files and subdirectories.
func (tb *treeBuilder) processEntry(ctx context.Context, st *scanState) (*entryResult, error) {
	if err := ctx.Err(); err != nil { return nil, err }

	isDir        := st.entry.IsDir()
	isTargetFile := !isDir && strings.HasSuffix(st.entry.Name(), ".go.html")

	if !isDir && !isTargetFile {
		st.prog.Report(st.budget, "")
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
		budgets := splitBudget(st.budget, len(subDirEntries))
		for i, subDirEntry := range subDirEntries {
			childState := &scanState{
				parentPath: pkgPath,
				entry:      subDirEntry,
				indent:     st.indent + 2,
				prog:       st.prog,
				budget:     budgets[i],
			}
			res, err := tb.processEntry(ctx, childState)
			if err != nil {
				return nil, err
			}
			subDirSB.WriteString(res.html)
			dirCovered.Add(res.covered)
			dirStatements.Add(res.total)
		}
	} else {
		st.prog.Report(st.budget, pkgPath)
	}

	hb   := &htmlBuilder{indent: st.indent, itemID: itemID, subDir: srcBasename}
	html := hb.buildSubDirHTML(subDirSB.String(), dirCovered.Load(), dirStatements.Load())

	return &entryResult{html: html, covered: dirCovered.Load(), total: dirStatements.Load()}, nil
}

// processFile renders a single <li> tree-node for a Go source file, including its coverage percentage.
func (tb *treeBuilder) processFile(st *scanState, pkgPath, srcBasename string) (*entryResult, error) {
	st.prog.Report(st.budget, pkgPath)

	cov     := tb.cov[pkgPath]
	percent := 0.0
	if cov.total > 0 {
		percent = float64(cov.covered) / float64(cov.total) * 100
	}

	var sb strings.Builder
	sb.Grow(st.indent * 2 + 128) // rough pre-allocation to avoid reallocations; should cover most cases
	sb.WriteString(strings.Repeat("  ", st.indent))
	sb.WriteString(`<li><div class="tree-node"><span class="src"><a href="`)
	sb.WriteString(filepath.Join(st.parentPath, st.entry.Name()))
	sb.WriteString(`">`)
	sb.WriteString(srcBasename)
	sb.WriteString(`</a></span> <span class="cov">`)
	sb.WriteString(strconv.FormatFloat(percent, 'f', 1, 64))
	sb.WriteString("%</span></div></li>\n")

	return &entryResult{html: sb.String(), covered: cov.covered, total: cov.total}, nil
}

// buildTreeHTML wraps the top-level entry results in the outermost <ul>, with
// the module name as the root label and the aggregate coverage percentage.
func buildTreeHTML(modDomain string, results []*entryResult, totalStatements, totalCovered uint64) string {
	// TODO(jbunds): calculate aggregate coverage percentage purely via the results argument
	aggregatePercent := "0.0"
	if totalStatements > 0 {
		aggregatePercent = strconv.FormatFloat(
			float64(totalCovered) / float64(totalStatements) * 100, 'f', 1, 64)
	}

	var sb strings.Builder
	sb.WriteString("<ul class=\"tree\">\n")
	sb.WriteString("  <li>\n")
	sb.WriteString("    <input type=\"checkbox\" id=\"tree-item-0\"/>\n")
	sb.WriteString("    <div class=\"tree-node\">\n")
	sb.WriteString(`      <label for="tree-item-0">`)
	sb.WriteString(modDomain)
	sb.WriteString("</label>\n")
	sb.WriteString(`      <span class="cov">`)
	sb.WriteString(aggregatePercent)
	sb.WriteString("%</span>\n")
	sb.WriteString("    </div>\n")
	sb.WriteString("    <ul>\n")

	for _, res := range results {
		sb.WriteString(res.html)
	}

	sb.WriteString("    </ul>\n")
	sb.WriteString("  </li>\n")
	sb.WriteString("</ul>")

	return sb.String()
}

// buildSubDirHTML wraps pre-rendered child nodes in a <li> tree-node for
// a subdirectory, with its name and aggregated coverage percentage.
func (hb *htmlBuilder) buildSubDirHTML(subDirHTML string, dirCovered, dirStatements uint64) string {
	if subDirHTML == "" { return "" }

	percent := 0.0
	if dirStatements > 0 {
		percent = float64(dirCovered) / float64(dirStatements) * 100
	}

	indent := strings.Repeat("  ", hb.indent)
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
	sb.WriteString(strconv.FormatFloat(percent, 'f', 1, 64))
	sb.WriteString("%</span>\n")
	sb.WriteString(indent)
	sb.WriteString("  </div>\n")
	sb.WriteString(indent)
	sb.WriteString("  <ul>\n")
	sb.WriteString(subDirHTML)
	sb.WriteString(indent)
	sb.WriteString("  </ul>\n")
	sb.WriteString(indent)
	sb.WriteString("</li>\n")
	return sb.String()
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
