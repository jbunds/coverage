package main

import (
	"cmp"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/term"
)

// printCoverage prints per-file coverage percentages to stdout.
func (cs *coverageState) printCoverage(w io.Writer) error {
	keys       := slices.Collect(maps.Keys(cs.cov))
	maxPathLen := len(slices.MaxFunc(keys, func(a, b string) int {
		return cmp.Compare(len(a), len(b))
	}))

	maxPathLen = max(maxPathLen, 5) // 5 == len("Total")

	divider := strings.Repeat("—", maxPathLen + 9) + "\n" // 9 == 2 spaces + len("100.00%")

	ew := newErrorWriter(w)
	ew.write("File")
	ew.write(strings.Repeat(" ", maxPathLen - 4 + 1))
	ew.write("Coverage\n")
	ew.write(divider)

	for _, path := range cs.sort() {
		cov     := cs.cov[path]
		percent := 0.0
		if cov.total > 0 {
			percent = float64(cov.covered) / float64(cov.total) * 100
		}
		writeRow(ew, path, percent, maxPathLen)
	}

	totalPercent    := 0.0
	totalCovered    := cs.totalCovered.Load()
	totalStatements := cs.totalStatements.Load()
	if totalStatements > 0 {
		totalPercent = float64(totalCovered) / float64(totalStatements) * 100
	}

	ew.write(divider)
	writeRow(ew, "Total", totalPercent, maxPathLen)

	return ew.err()
}

// writeRow writes a single padded, color-coded (green ≥ 50%, red < 50%) coverage row.
func writeRow(ew *errorWriter, path string, percent float64, maxPathLen int) {
	const (
		green = "\033[32m"
		red   = "\033[31m"
	)
	ew.write(path)
	ew.write(strings.Repeat(" ", maxPathLen - len(path) + 2))
	pct       := strconv.FormatFloat(percent, 'f', 2, 64) + "%"
	colorCode := green
	if percent < 50 { colorCode = red }
	ew.write(strings.Repeat(" ", 7 - len(pct))) // 7 == len("100.00%")
	ew.writeColor(pct, colorCode)
	ew.write("\n")
}

// maybeOpenBrowser opens the generated index.html file in the
// default browser when stdout is a TTY and -n is not set.
func (rg *reportGenerator) maybeOpenBrowser(runner runner, fv *flagVals) error {
	if fv.noBrowser      ||
	   !rg.write         ||
	   !isTerm(os.Stdin) {
		return nil
	}

	if fv.httpServer {
		return launchHTTPServer(runner, rg.outRoot.Name())
	}

	absPath, err := filepath.Abs(filepath.Join(rg.outRoot.Name(), "index.html"))
	if err != nil {
		return err
	}
	if runtime.GOOS == "linux" {
		absPath = "file://" + absPath // Linux requires explicit file:// scheme prefix
	}
	return openBrowser(runner, absPath)
}

// launchHTTPServer launches a Python HTTP server when -s is set and -n is not set.
func launchHTTPServer(runner runner, outDir string) error {
	python := "python3"
	if runtime.GOOS == "windows" {
		python = "python"
	}
	pyPath, err := exec.LookPath(python)
	if err != nil {
		return err
	}

	const port = "8000" // https://docs.python.org/3/library/http.server.html#cmdoption-http.server-arg-port

	url := "http://localhost:" + port

	if runtime.GOOS != "windows" {
		if pid, err := findPortPID(runner, port); err == nil { // macOS / Linux: check for existing process listening on port 8000
			if isTerm(os.Stderr) {
				fmt.Fprintf(os.Stderr, "process already listening on port %s (PID: %d)\n", port, pid)
			}
			return openBrowser(runner, url) // assume the process listening on port 8000 is an HTTP server rather than launch a new one
		}
	}

	cmd := exec.Command(pyPath, "-m", "http.server", "-d", outDir) // #nosec G204 G702 - no shell (no injection); outDir confined to outRoot (no path escape)

	if err := runner.Start(cmd); err != nil {
		return err
	}

	if runtime.GOOS != "windows" && isTerm(os.Stderr) {
		fmt.Fprintf(os.Stderr, "HTTP server listening on port %s (PID: %d)\n", port, cmd.Process.Pid)
	}

	return openBrowser(runner, url)
}

// openBrowser opens the default browser with the specified URL.
func openBrowser(runner runner, url string) error {
	var cmd *exec.Cmd
	switch os := runtime.GOOS; os {
	case "darwin":
		cmd = exec.Command("open",         url) // #nosec G204 G702 - no shell (no injection); url confined to outRoot (no path escape)
	case "windows":
		cmd = exec.Command("explorer.exe", url) // #nosec G204 G702 - no shell (no injection); url confined to outRoot (no path escape)
	case "linux":
		cmd = exec.Command("xdg-open",     url) // #nosec G204 G702 - no shell (no injection); url confined to outRoot (no path escape)
	default:
		return fmt.Errorf("unrecognized OS: %s", os)
	}
	return runner.Run(cmd)
}

// findPortPID searches for a process listening on the specified port and returns its PID if found.
func findPortPID(runner runner, port string) (int, error) {
	out, err := runner.Output(exec.Command("lsof", "-ti", ":" + port)) // #nosec G204 - port is a const defined in launchHTTPServer()
	if err != nil { return -1, err }
	var pid int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &pid); err != nil {
		return -1, err
	}
	return pid, nil
}

// isTerm determines if the specified writer is connected to a terminal.
func isTerm(v any) bool {
	if testing.Testing()                     ||
	   os.Getenv("GITHUB_ACTIONS") == "true" || // https://docs.github.com/actions/reference/workflows-and-actions/variables
	   os.Getenv("CI"            ) == "true" { return false }
	fd := getFD(v)
	if fd < 0 { return false }
	return term.IsTerminal(fd)
}

// getFD returns the file descriptor of the provided argument.
func getFD(v any) int {
	if f, ok := v.(interface{ Fd() uintptr }); ok { return int(f.Fd()) }
	return -1
}
