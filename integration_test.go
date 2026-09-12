package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/tools/cover"
)

func TestIntegrationTest(t *testing.T) {
	t.Parallel()

	tmpDir        := t.TempDir()
	profiles, err := cover.ParseProfiles("testdata/cov.out"); if err != nil { t.Fatal(err) }
	rg            := &reportGenerator{
		fsys:     &localFS{},
		outRoot:  tmpDir,
		profiles: profiles,
	}

	if err := rg.getModName(t.Context(), "go.mod"); err != nil { t.Fatal(err) }

	if err := rg.writeCovHTMLFiles(t.Context(), io.Discard, "style.css");  err != nil { t.Fatal(err) }

	tests := []struct{
		name string
	}{
		{name: "flags.go"},
		{name: "main.go"},
		{name: "tree.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			goldenFile := tt.name + ".html"

			wantPath   := filepath.Join("testdata",         goldenFile)
			gotPath    := filepath.Join(tmpDir, rg.modName, goldenFile)

			want, err  := os.ReadFile(wantPath); if err != nil { t.Fatal(err) } // #nosec G304
			got,  err  := os.ReadFile( gotPath); if err != nil { t.Fatal(err) } // #nosec G304

			wantLines  := strings.Split(strings.TrimSuffix(string(want), "\n"), "\n")
			gotLines   := strings.Split(strings.TrimSuffix(string( got), "\n"), "\n")

			if !cmp.Equal(wantLines, gotLines) {
				if os.Getenv("GITHUB_ACTIONS") != "true" && // https://docs.github.com/actions/reference/workflows-and-actions/variables
				   os.Getenv("CI"            ) != "true" {
					cmd := exec.Command("meld", wantPath, gotPath) // #nosec G204
					_ = cmd.Run()
				}
				var rep reporter
				cmp.Equal(wantLines, gotLines, cmp.Reporter(&rep))
				t.Errorf("mismatch (-want +got):\n%s", strings.Join(rep.diffs, "\n"))
			}
		})
	}
}

type reporter struct{
	path  cmp.Path
	diffs []string
}

func (r *reporter) PushStep(ps cmp.PathStep) {
	r.path = append(r.path, ps)
}

func (r *reporter) PopStep() {
	r.path = r.path[:len(r.path) - 1]
}

func (r *reporter) Report(rs cmp.Result) {
	if !rs.Equal() {
		want, got := r.path.Last().Values() 
			r.diffs = append(r.diffs, fmt.Sprintf("- %s\n+ %s", want, got))
	}
}
