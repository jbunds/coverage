package main

import (
	"io"
	"os"
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

	// hack to avoid calling writeStyleCSS which requires rg.embeddedFiles
	// the style.css file is copied purely as a convenience measure to aid in debugging CSS rendering errors
	if err := copyFile("css/style.css", filepath.Join(tmpDir, "style.css")); err != nil { t.Fatal(err) }

	if err := rg.writeCovHTMLFiles(t.Context(), io.Discard,   "style.css");  err != nil { t.Fatal(err) }

	tests := []struct{
		name string
	}{
		{name: "flags.go"},
//		{name: "main.go"},
		{name: "tree.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			goldenFile := tt.name + ".html"
			want, err  := os.ReadFile(filepath.Join("testdata",         goldenFile)); if err != nil { t.Fatal(err) } // #nosec G304
			got,  err  := os.ReadFile(filepath.Join(tmpDir, rg.modName, goldenFile)); if err != nil { t.Fatal(err) } // #nosec G304
			wantLines  := strings.Split(strings.TrimSuffix(string(want), "\n"), "\n")
			gotLines   := strings.Split(strings.TrimSuffix(string( got), "\n"), "\n")
			if diff := cmp.Diff(wantLines, gotLines); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func copyFile(src, dst string) error {
	in,  err := os.Open(src);     if err != nil { return err } // #nosec G304
	defer in.Close()
	out, err := os.Create(dst);   if err != nil { return err } // #nosec G304
	defer out.Close()
	_, err    = io.Copy(out, in); if err != nil { return err }
	return out.Sync()
}
