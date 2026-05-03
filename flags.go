package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"
)

// flags parses command line flags
func flags(fs *flag.FlagSet, args []string) (goMod, coverProfile, path string, err error) {
	// tests may call fs.SetOutput(); it is not called here
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "%s usage:\n\n", filepath.Base(fs.Name()))
		fs.PrintDefaults()
		fmt.Fprintln(fs.Output())
	}
	fs.StringVar(&goMod,        "gomod",        "", "path to the root go.mod file")
	fs.StringVar(&coverProfile, "coverprofile", "", "path to Go test coverage profile file")
	fs.StringVar(&path,         "path",         "", "path where HTML files will be written")
	if err := fs.Parse(args); err != nil {
		return "", "", "", err
	}
	if goMod == "" {
		fs.Usage()
		return "", "", "", fmt.Errorf("no value specified for -gomod")
	}
	if coverProfile == "" {
		fs.Usage()
		return "", "", "", fmt.Errorf("no value specified for -coverprofile")
	}
	if path == "" {
		fs.Usage()
		return "", "", "", fmt.Errorf("no value specified for -path")
	}
	if len(fs.Args()) > 0 {
		fmt.Fprintf(fs.Output(), "ignored arguments: %s\n", strings.Join(fs.Args(), ", "))
	}
	return goMod, coverProfile, path, nil
}
