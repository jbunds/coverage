package main

import (
	"bytes"
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"

	"github.com/jbunds/progress"
	"golang.org/x/sync/errgroup"
	"golang.org/x/tools/cover"
)

// stickyWriter is a wrapper interface used to enable sequential
// string writing with deferred error handling.
type stickyWriter interface {
	io.Writer
	write(string)
	err() error
}

// errorWriter tracks the first encountered I/O failure to allow multiple
// sequential writes without repetitive inline error checking.
type errorWriter struct {
	w        io.Writer
	e        error
	useColor bool
}

// write performs a sticky-error write.
func (w *errorWriter) write(s string) {
	if w.e != nil { return }
	_, w.e = io.WriteString(w.w, s)
}

// write performs a sticky-error write that optionally
// wraps the provided string in ANSI color codes.
func (w *errorWriter) writeColor(s, color string) {
	if w.e != nil { return }
	if !w.useColor {
		w.write(s)
		return
	}
	w.write(color)
	w.write(s)
	w.write("\033[0m") // reset attributes, styles, and colors to defaults
}

// err returns the first I/O failure encountered
// during multiple sequential write operations.
func (w *errorWriter) err() error { return w.e }

// Write satisfies the io.Writer interface, but is otherwise unused.
func (w *errorWriter) Write(p []byte) (int, error) {
	if w.e != nil { return 0, w.e }
	return w.w.Write(p)
}

func newErrorWriter(w io.Writer) *errorWriter {
	return &errorWriter{
		w:        w,
		useColor: isTerm(w),
	}
}

// workUnit represents a unit of work to be performed: the generation
// of an HTML file for a Go source file's coverage profile.
type workUnit struct {
	profile *cover.Profile
	outPath string
}

// writeCovHTMLFiles calculates per-file coverage percentages and writes a *.go.html
// file for each Go source file listed in the coverage profile file.
func (rg *reportGenerator) writeCovHTMLFiles(ctx context.Context, progressOutput io.Writer) error {
	if err := ctx.Err(); err != nil { return err }

	rg.covState.cov = make(map[string]coverage, len(rg.profiles))
	units, dirs := rg.buildWorkUnits()

	if err := rg.createDirs(ctx, dirs, progressOutput); err != nil {
		return err
	}

	perFileCov, err := rg.genHTMLFiles(ctx, units, progressOutput)
	if err != nil {
		return err
	}

	for i, cov := range perFileCov {
		rg.covState.cov[units[i].profile.FileName] = cov
	}
	return nil
}

// buildWorkUnits creates one work unit per coverage profile and collects
// the set of output directories that must exist before writing.
func (rg *reportGenerator) buildWorkUnits() (units []workUnit, dirs map[string]struct{}) {
	units = make([]workUnit, 0, len(rg.profiles))
	dirs  = make(map[string]struct{})
	for _, profile := range rg.profiles {
		outPath := filepath.Join(rg.outRoot.Name(), profile.FileName + ".html")
		units    = append(units, workUnit{profile: profile, outPath: outPath})
		dirs[filepath.Dir(outPath)] = struct{}{}
	}
	return
}

// createDirs creates all required output directories concurrently, bounded by NumCPU.
func (rg *reportGenerator) createDirs(ctx context.Context, dirs map[string]struct{}, progressOutput io.Writer) error {
	if !rg.write { return nil }

	prog := progress.New(ctx, uint64(len(dirs)), progressOutput)
	defer prog.Close()

	group, gCtx := errgroup.WithContext(ctx)
	group.SetLimit(runtime.NumCPU())

	for dir := range dirs {
		if err := gCtx.Err(); err != nil {
			break
		}
		group.Go(func() error {
			if err := rg.fsys.MkdirAll(dir, 0700); err != nil {
				return fmt.Errorf("cannot create directory %q: %w", dir, err)
			}
			prog.Report(1, "created " + dir)
			return nil
		})
	}
	return group.Wait()
}

// genHTMLFiles processes all work units concurrently (bounded by NumCPU),
// writing one HTML file per unit and returning per-file coverage counts.
func (rg *reportGenerator) genHTMLFiles(ctx context.Context, units []workUnit, progressOutput io.Writer) ([]coverage, error) {
	perFileCov := make([]coverage, len(units))
	prog       := progress.New(ctx, 0, progressOutput)
	defer prog.Close()

	group, gCtx := errgroup.WithContext(ctx)
	group.SetLimit(runtime.NumCPU())

	for i, unit := range units {
		if err := gCtx.Err(); err != nil {
			break
		}
		group.Go(func() error {
			return rg.processUnit(gCtx, prog, unit, &perFileCov[i])
		})
	}
	return perFileCov, group.Wait()
}

// processUnit renders and writes the coverage HTML for a single work unit,
// reports progress, and updates the aggregate coverage atomics.
func (rg *reportGenerator) processUnit(ctx context.Context, prog *progress.Progress, unit workUnit, cov *coverage) error {
	if err := ctx.Err(); err != nil { return err }

	fileStatements, fileCovered := countStatements(unit.profile.Blocks)
	prog.AddTotal(uint64(fileStatements))

	pkgPath  := filepath.Dir( unit.profile.FileName)
	fileName := filepath.Base(unit.profile.FileName)
	srcFile  := filepath.Join(rg.pkgDirCache[pkgPath], fileName)

	src, err := rg.fsys.ReadFile(srcFile)
	if err != nil { return fmt.Errorf("cannot read %q: %w", srcFile, err) }

	var buf bytes.Buffer
	ew := newErrorWriter(&buf)

	relPath := strings.Repeat("../", strings.Count(unit.profile.FileName, "/"))
	writePreamble(ew, relPath + rg.iconFilename, unit.profile.FileName, relPath + rg.styleCSSFilename)

	fset         := token.NewFileSet()
	fileAST, err := parser.ParseFile(fset, fileName, src, parser.ParseComments); if err != nil { return err }
	file         := fset.File(fileAST.Pos())

	if err := renderCovHTML(ctx, ew, annotateSource(file, src,
		computeBlockOffsets(file, unit.profile.Blocks),
		computeFuncSpans(fileAST, file))); err != nil {
		return err
	}

	writePostamble(ew, relPath + rg.childJSFilename)

	if rg.write {
		if err := rg.fsys.WriteFile(unit.outPath, buf.Bytes(), 0600); err != nil {
			return fmt.Errorf("cannot write HTML file for %q: %w", unit.outPath, err)
		}
	}

	prog.Report(float64(fileStatements), unit.profile.FileName)
	rg.covState.totalCovered.Add(fileCovered)
	rg.covState.totalStatements.Add(fileStatements)
	*cov = coverage{covered: fileCovered, total: fileStatements}
	return nil
}

// renderCovHTML renders the HTML content for a single *.go.html file, with
// green (covered) and red (uncovered) lines to indicate test coverage.
func renderCovHTML(ctx context.Context, ew stickyWriter, lines []annotatedLine) error {
	if err := ctx.Err(); err != nil { return err }

	prevFunc     := -1
	firstLineIdx := -1 // index of current function's signature

	for i, line := range lines {
		if line.funcIdx >= 0        &&
		   line.funcIdx != prevFunc { // new function starts
			prevFunc     = line.funcIdx
			firstLineIdx = i
			ew.write(`<div class="func"><input type="checkbox" id="func-`)
			ew.write(strconv.Itoa(line.funcIdx))
			ew.write("\" checked/>\n")
		}

		if line.funcIdx < 0 { // outside any function: plain line
			ew.write(`<div class="line" data-line="`)
			ew.write(strconv.Itoa(i + 1))
			ew.write(`">`)
			ew.write(line.text)
			ew.write("</div>\n")
			continue
		}

		if i == firstLineIdx {
			ew.write(`  <label for="func-`)
			ew.write(strconv.Itoa(line.funcIdx))
			ew.write(`" class="line" data-line="`)
			ew.write(strconv.Itoa(i + 1))
			ew.write(`">`)
			ew.write(line.text)
			ew.write("</label>\n")
			if i + 1 < len(lines) && lines[i + 1].funcIdx == line.funcIdx {
				ew.write("  <div class=\"func-body\">\n")
			}
		} else {
			ew.write(`    <div class="line" data-line="`)
			ew.write(strconv.Itoa(i + 1))
			ew.write(`">`)
			ew.write(line.text)
			ew.write("</div>\n")
		}

		if line.isFuncEnd {
			if i > firstLineIdx { // function signature and body span more than one line: close func-body
				ew.write("  </div>\n")
			}
			ew.write("</div>\n")
			prevFunc = -1
		}
	}

	return ew.err()
}

// writePreamble writes the preamble portion of the
// HTML content common to every Go source HTML file.
func writePreamble(ew stickyWriter, iconPath, srcPath, cssPath string) {
	ew.write("<!DOCTYPE html>\n")
	ew.write("<html lang=\"en\">\n")
	ew.write("<head>\n")
	ew.write("<meta charset=\"utf-8\">\n")
	ew.write(`<link rel="icon"       href="`)
	ew.write(iconPath)
	ew.write("\" type=\"image/vnd.microsoft.icon\">\n")
	ew.write(`<link rel="stylesheet" href="`)
	ew.write(cssPath)
	ew.write("\">\n")
	ew.write("<title>")
	ew.write(srcPath)
	ew.write("</title>\n")
	ew.write("</head>\n")
	ew.write("<body id=\"code\" class=\"line-numbers\">\n")
}

// writePostamble writes the postamble portion of the
// HTML content common to every Go source HTML file.
func writePostamble(ew stickyWriter, childJSPath string) {
	ew.write(`<script src="`)
	ew.write(childJSPath) // DOM-dependent
	ew.write("\"></script>\n")
	ew.write("</body>\n")
	ew.write("</html>")
}

// writeIndexHTMLFile writes the index HTML file, which contains three
// template parameters (Title, HeaderHTML, and TreeHTML), and hosts one
// iframe within which the generated source code HTML files are rendered.
func (rg *reportGenerator) writeIndexHTMLFile(treeHTML string) error {
	title := "Go test coverage"
	var headerSB strings.Builder
	if len(rg.modPaths) > 1 {
		headerSB.WriteString("  <div class=\"dropdown-wrapper\">\n")
		headerSB.WriteString("    <input type=\"checkbox\" id=\"modules-menu\"/>\n")
		headerSB.WriteString("    <label for=\"modules-menu\" class=\"dropdown-toggle\">modules</label>\n")
		headerSB.WriteString("    <div class=\"dropdown-menu\">\n")
		for _, mod := range rg.modPaths {
			headerSB.WriteString(`      <label for="module-`)
			headerSB.WriteString(normalizeModID(mod))
			headerSB.WriteString(`" class="dropdown-link">`)
			headerSB.WriteString(mod)
			headerSB.WriteString("</label>\n")
		}
		headerSB.WriteString("    </div>\n")
		headerSB.WriteString("  </div>")
	} else {
		title += " // " + rg.modPaths[0]
		headerSB.WriteString(`<code><a target="_blank" href="`)
		headerSB.WriteString(rg.repoURLs[0])
		headerSB.WriteString(`">`)
		headerSB.WriteString(rg.modPaths[0])
		headerSB.WriteString(`</a></code>`)
	}

	data := struct{
		Title      string
		HeaderHTML string
		TreeHTML   string
	}{
		Title:      title,
		HeaderHTML: headerSB.String(),
		TreeHTML:   treeHTML,
	}

	return rg.writeTemplateFile("html/index.html", data)
}

// writeTemplateFile writes the specified template file.
func (rg *reportGenerator) writeTemplateFile(file string, data any) error {
	outFile := filepath.Join(rg.outRoot.Name(), filepath.Base(file))
	t, err  := template.ParseFS(rg.embeddedFiles, file)
	if                            err != nil { return fmt.Errorf("cannot parse %q: %w",     file, err) }
	f, err := rg.fsys.Create(outFile)
	if                            err != nil { return fmt.Errorf("cannot create %q: %w", outFile, err) }
	if err := t.Execute(f, data); err != nil { return fmt.Errorf("cannot render template: %w",    err) }

	return f.Close()
}

// countStatements returns the total and covered statement counts.
// A block is covered if its Count is greater than zero.
func countStatements(blocks []cover.ProfileBlock) (total, covered uint64) {
	for _, b := range blocks {
		total += toUint64(b.NumStmt)
		if b.Count > 0 {
			covered += toUint64(b.NumStmt)
		}
	}
	return
}

// toUint64 safely casts an int to a uint64 by guarding
// against overflow when n is negative.
func toUint64(n int) uint64 {
	if n < 0 { return 0 } // satisfy the gosec linter's overflow conversion check (G115)
	return uint64(n)
}
