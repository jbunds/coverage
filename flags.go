package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// flags parses command line flags
func flags(fs *flag.FlagSet, args []string) (coverProfile, path string, err error) {
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "%s usage:\n\n", filepath.Base(fs.Name())) //nolint:errcheck // such usage is documented in the standard library: https://pkg.go.dev/flag#pkg-variables
		fs.PrintDefaults()
		fmt.Fprintln(fs.Output())
	}
	fs.StringVar(&coverProfile, "coverprofile", "", "path to Go test coverage profile file")
	fs.StringVar(&path,         "path",         "", "path where HTML files will be written")
	if err := fs.Parse(args); err != nil {
		return "", "", err
	}
	if coverProfile == "" {
		fs.Usage()
		return "", "", fmt.Errorf("no value specified for -coverprofile")
	}
	if path == "" {
		fs.Usage()
		return "", "", fmt.Errorf("no value specified for -path")
	}
	if len(fs.Args()) > 0 {
		fmt.Fprintf(os.Stderr, "ignored arguments: %s\n", strings.Join(fs.Args(), ", "))
	}
	return coverProfile, path, nil
}
