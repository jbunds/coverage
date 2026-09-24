package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// integration_test.sh provides a convenience wrapper for this test

func TestIntegrationTest(t *testing.T) {
	t.Parallel()

	rg, err := newReportGenerator("go.mod", "testdata/cov.out", t.TempDir(), lex)
	if err != nil { t.Fatal(err) }

	if err := rg.getModName("go.mod");                        err != nil { t.Fatal(err) }
	if err := rg.writeCovHTMLFiles(t.Context(), io.Discard);  err != nil { t.Fatal(err) }

	tests := []struct{
		name string
	}{
		{name:     "assets.go"},
		{name:      "flags.go"},
		{name: "interfaces.go"},
		{name:       "main.go"},
		{name:      "pages.go"},
		{name:       "scan.go"},
		{name:       "tree.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			goldenFile := tt.name + ".html"

			wantPath   := filepath.Join("testdata",                    goldenFile)
			gotPath    := filepath.Join(rg.outRoot.Name(), rg.modName, goldenFile)

			want, err  := os.ReadFile(wantPath); if err != nil { t.Fatal(err) } // #nosec G304
			got,  err  := os.ReadFile( gotPath); if err != nil { t.Fatal(err) } // #nosec G304

			wantLines  := strings.Split(strings.TrimSuffix(string(want), "\n"), "\n")
			gotLines   := strings.Split(strings.TrimSuffix(string( got), "\n"), "\n")

			var rep reporter
			if !cmp.Equal(wantLines, gotLines, cmp.Reporter(&rep)) {
				t.Errorf("mismatch (-want +got):\n%s", strings.Join(rep.diffs, "\n"))
				if isTerm(os.Stdout) {
					_ = exec.Command("meld", wantPath, gotPath).Run() // #nosec G204
				}
			}
		})
	}
}
