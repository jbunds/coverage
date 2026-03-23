// Package main writes HTML files for each Go source file listed in the user-specified Go coverage profile file.
//
// This module also generates a directory tree HTML file rendered within an iframe of the index HTML file.
//
// The header portion of the index HTML file will also render two buttons if the browser's CORS policies allow it. These buttons are:
//
//   "theme"                  - toggles between two hardcoded "light" and "dark" themes
//   "expand" (or "collapse") - toggles the opening (or closing) of all subdirectories rendered within the tree HTML document
//
// Note that the "theme" and "expand" / "collapse" buttons will not be rendered when the index page is loaded via the file:// scheme.
//
// A simple workaround is to instantiate an HTTP server to serve the HTML files, e.g.:
//
//   $ python3 -m http.server 8000
//
// and then load http://localhost:8000/ in a browser.

package main

import (
	"cmp"
	"embed"
	"flag"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/cover"
)

//go:embed favicon.ico go-blue-gradient.svg index.html tree.css
var content embed.FS

var (
	indexHTML      = "index.html"
	treeHTML       = "tree.html"
	ancillaryFiles = []string{
		"favicon.ico",
		"go-blue-gradient.svg",
		"tree.css"}
)

type coverage struct {
	covered int
	total   int
}

func main() {
	profilePath, outRoot, err := flags(flag.CommandLine, filterArgs(os.Args[1:]))
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot parse flags: %v\n", err)
		os.Exit(1)
	}

	modName, err := getModName()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot determine module name: %v\n", err)
		os.Exit(2)
	}

	profiles, err := cover.ParseProfiles(profilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot parse coverage profile file: %v\n", err)
		os.Exit(3)
	}

	cov, totalStatements, totalCovered, err := writeCovHTMLFiles(modName, outRoot, profiles)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot write HTML coverage files: %v\n", err)
		os.Exit(4)
	}

	printCoverage(cov, totalStatements, totalCovered)

	maxWidth := 0

	if maxWidth, err = writeTreeHTML(outRoot, treeHTML, cov); err != nil {
		fmt.Fprintf(os.Stderr, "cannot write tree.html: %v\n", err)
		os.Exit(5)
	}

	if err := writeAncillaryFiles(outRoot, content, ancillaryFiles); err != nil {
		fmt.Fprintf(os.Stderr, "cannot write image files: %v\n", err)
		os.Exit(6)
	}

	if err := writeIndexHTML(outRoot, content, maxWidth); err != nil {
		fmt.Fprintf(os.Stderr, "cannot write %s: %v\n", indexHTML, err)
		os.Exit(7)
	}
}

// getModName reads the go.mod file to determine the name of the local Go module
func getModName() (string, error) {
	goMod, err := os.ReadFile("go.mod")
	if err != nil { return "", fmt.Errorf("cannot read go.mod: %w", err) }
	f, err := modfile.Parse("go.mod", goMod, nil)
	if err != nil { return "", fmt.Errorf("cannot parse go.mod: %w", err) }
	return f.Module.Mod.Path, nil
}

// writeCovHTMLFiles calculates per-file coverage percentages and writes a *.go.html file for each Go source file listed in the user-specified coverage profile file
func writeCovHTMLFiles(modName, outRoot string, profiles []*cover.Profile) (map[string]coverage, int, int, error) {
	var totalStatements, totalCovered int

	cov := make(map[string]coverage, len(profiles))

	for _, profile := range profiles { // calculate per-file coverage
		var fileStatements, fileCovered int
		for _, block := range profile.Blocks {
			fileStatements += int(block.NumStmt)
			if block.Count > 0 {
				fileCovered += int(block.NumStmt)
			}
		}

		totalStatements += fileStatements
		totalCovered    += fileCovered

		localPath     := strings.TrimPrefix(profile.FileName, modName + "/")
		cov[localPath] = coverage{
			covered: fileCovered,
			total:   fileStatements,
		}

		if err := writeCovHTMLFile(profile, outRoot, localPath); err != nil {
			return cov, totalStatements, totalCovered, fmt.Errorf("error generating HTML for %s: %w", localPath, err)
		}
	}
	return cov, totalStatements, totalCovered, nil
}

// writeCovHTMLFile writes a single *.go.html file with green (covered) and red (uncovered) lines to illustrate test coverage
func writeCovHTMLFile(profile *cover.Profile, outRoot, localPath string) error {
	src, err := os.ReadFile(filepath.Clean(localPath))
	if err != nil { return fmt.Errorf("cannot read source: %w", err) }

	// TODO(jeff): refactor, consolidate, and better encapsulate all CSS that's currently strewn about among different files
	// TODO(jeff): clean up these very long multi-line strings
	var builder strings.Builder
	builder.WriteString(`<html>
<head>
<style>
:root {
  --bg-color:   beige;
  --text-color: darkslategrey;
  --cov-hit:    seagreen;
  --cov-miss:   red;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg-color:   black;
    --text-color: lightyellow;
    --cov-hit:    seagreen;
    --cov-miss:   red;
  }
}
[theme="light"] {
  --bg-color:   beige;
  --text-color: darkslategrey;
  --cov-hit:    seagreen;
  --cov-miss:   red;
}
[theme="dark"] {
  --bg-color:   black;
  --text-color: lightyellow;
  --cov-hit:    seagreen;
  --cov-miss:   red;
}
body {
  tab-size:         4;
  background-color: var(--bg-color);
  color:            var(--text-color);
  font-family:      monospace;
  font-weight:      bold;
}
.hit  { color: var(--cov-hit);  }
.miss { color: var(--cov-miss); }
</style>
</head>
<body>
<pre>`)

	pos := 0
	for _, b := range profile.Boundaries(src) {
		builder.WriteString(html.EscapeString(string(src[pos:b.Offset])))
		if b.Start {
			class := "miss"
			if b.Count > 0 {
				class = "hit"
			}
			fmt.Fprintf(&builder, "<span class='%s'>", class)
		} else {
			builder.WriteString("</span>")
		}
		pos = b.Offset
	}

	builder.WriteString(html.EscapeString(string(src[pos:])))
	builder.WriteString(`</pre>
<script>
try {
  const parentTheme = window.parent.document.documentElement.getAttribute('theme');
  if (parentTheme) document.documentElement.setAttribute('theme', parentTheme);
} catch (e) {
	console.warn('direct parent access blocked by browser; waiting for postMessage');
}

window.addEventListener('message', (event) => {
  if (event.data && event.data.type === 'SET_THEME') document.documentElement.setAttribute('theme', event.data.theme);
});
</script>
</body>
</html>`)

	outPath := filepath.Join(outRoot, localPath + ".html")
	if err := os.MkdirAll(filepath.Dir(filepath.Clean(outPath)), 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return os.WriteFile(filepath.Clean(outPath), []byte(builder.String()), 0600)
}

// printCoverage prints per-file coverage percentages to stdout
func printCoverage(cov map[string]coverage, totalStatements, totalCovered int) {
	keys       := slices.Collect(maps.Keys(cov))
	maxPathLen := len(slices.MaxFunc(keys, func(a, b string) int {
		return cmp.Compare(len(a), len(b))
	}))

	fmtString := fmt.Sprintf("%%-%ds  %%6.2f%%%%\n", maxPathLen)
	fmtHeader := fmt.Sprintf("%%-%ds %%s\n",         maxPathLen)

	fmt.Printf(fmtHeader, "File", "Coverage")
	fmt.Println(strings.Repeat("—", maxPathLen + 9)) // 9 == 3 + len("100.0%")

	// TODO(jeff): allow users to chose how the rows rendered in the tree should be sorted;
	//             default should probably path-depth, then alphanumerically, just like here
	slices.SortFunc(keys, func(a, b string) int {
		depthA := strings.Count(a, "/")
		depthB := strings.Count(b, "/")
		if depthA != depthB {
			return cmp.Compare(depthA, depthB) // sort by path depth
		}
		return cmp.Compare(a, b) // sort alphanumerically
	})

	for _, path := range keys {
		c       := cov[path]
		percent := 0.0
		if c.total > 0 {
			percent = float64(c.covered) / float64(c.total) * 100
		}
		fmt.Printf(fmtString, path, percent)
	}

	totalPercent := 0.0
	if totalStatements > 0 {
		totalPercent = float64(totalCovered) / float64(totalStatements) * 100
	}

	fmt.Println(strings.Repeat("—", maxPathLen + 9))
	fmt.Printf(fmtString, "Total", totalPercent)
}

// writeAncillaryFiles writes the supporting files defined per the ancillaryFiles global variable to the user-specified path
func writeAncillaryFiles(outRoot string, fsys fs.FS, files []string) error {
	for _, file := range files {
		f, err    := os.Create(filepath.Clean(filepath.Join(outRoot, file)))
		if                                        err != nil { return err }
		data, err := fs.ReadFile(fsys, file)
		if                                        err != nil { return err }
		if _, err := fmt.Fprint(f, string(data)); err != nil { return err }
		if    err := f.Close();                   err != nil { return err }
	}
	return nil
}

// writeIndexHTML writes the indexHTML file, which contains a single "MaxWidth" HTML template parameter within its inline CSS
func writeIndexHTML(outRoot string, fsys fs.FS, maxWidth int) error {
	tmpl, err := template.ParseFS(fsys, indexHTML)
	if err != nil { return err }
	f, err := os.Create(filepath.Clean(filepath.Join(outRoot, indexHTML)))
	if err != nil { return err }
	tmpl.Execute(f, struct { MaxWidth int }{ MaxWidth: maxWidth })
	if err := f.Close(); err != nil { return err }
	return nil
}

// filterArgs discards any flags up to and including "--", particularly useful for testing.
func filterArgs(args []string) []string {
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}
	return args
}
