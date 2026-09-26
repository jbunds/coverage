package main

import (
	"bytes"
	"context"
	"fmt"
	"go/token"
	"io"
	"path/filepath"
	"runtime"
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
			return rg.processUnit(gCtx, prog, i, unit, perFileCov)
		})
	}
	return perFileCov, group.Wait()
}

// processUnit builds and writes the coverage HTML for a single work unit,
// reports progress, and updates the aggregate coverage atomics.
func (rg *reportGenerator) processUnit(ctx context.Context, prog *progress.Progress, i int, unit workUnit, perFileCov []coverage) error {
	fileStatements, fileCovered := countStatements(unit.profile.Blocks)
	prog.AddTotal(uint64(fileStatements))

	var buf bytes.Buffer
	ew := newErrorWriter(&buf)
	if err := rg.buildCovHTML(ctx, ew, unit.profile, unit.profile.FileName); err != nil {
		return fmt.Errorf("cannot build HTML for %q: %w", unit.profile.FileName, err)
	}
	if rg.write {
		if err := rg.fsys.WriteFile(unit.outPath, buf.Bytes(), 0600); err != nil {
			return fmt.Errorf("cannot write HTML file for %q: %w", unit.outPath, err)
		}
	}

	prog.Report(float64(fileStatements), unit.profile.FileName)
	rg.covState.totalCovered.Add(fileCovered)
	rg.covState.totalStatements.Add(fileStatements)
	perFileCov[i] = coverage{covered: fileCovered, total: fileStatements}
	return nil
}

// buildCovHTML builds the HTML content for a single *.go.html file, with
// green (covered) and red (uncovered) lines to indicate test coverage.
func (rg *reportGenerator) buildCovHTML(ctx context.Context, ew stickyWriter, profile *cover.Profile, srcPath string) error {
	if err := ctx.Err(); err != nil { return err }

	pkgPath  := filepath.Dir( profile.FileName)
	fileName := filepath.Base(profile.FileName)

	src, err := rg.fsys.ReadFile(filepath.Join(rg.pkgDirCache[pkgPath], fileName))
	if err != nil {
		return err
	}

	fset := token.NewFileSet()
	file := fset.AddFile(fileName, fset.Base(), len(src))
	file.SetLinesForContent(src)

	blocks := computeBlockOffsets(file, profile.Blocks)
	buf    := scanAndAnnotate(file, src, blocks)

	relPath := strings.Repeat("../", strings.Count(srcPath, "/"))
	writePreamble(ew, relPath + rg.iconFilename, srcPath, relPath + rg.styleCSSFilename)
	for line := range bytes.SplitSeq(bytes.TrimRight(buf.Bytes(), "\n"), []byte{'\n'}) {
		ew.write(`<div class="line">`)
		ew.write(string(line))
		ew.write("</div>\n")
	}
	writePostamble(ew, relPath + rg.childJSFilename)
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
// template parameters (ModName, ModURL, and TreeHTML), and hosts one
// iframe within which the generated source code HTML files are rendered.
func (rg *reportGenerator) writeIndexHTMLFile(treeHTML string) error {
	data := struct{
		ModName,
		ModURL,
		TreeHTML  string
	}{
		ModName:  rg.modName,
		ModURL:   rg.repoURL,
		TreeHTML: treeHTML,
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
