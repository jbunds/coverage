package main

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"strings"
)

// flags parses command line flags.
func flags(fs *flag.FlagSet, args []string) (goMod, coverProfile, path string, noBrowser, httpServer bool, sortOrder sortOrder, err error) {
	allowedSortOrders := []string{ // must cohere with the `sortOrder` enum in sort.go
		"lex",
		"shallowest", "deepest",
		"lowest",     "highest",
		"shortest",   "longest",
	}
	// tests may call fs.SetOutput(); it is not called here
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "%s usage:\n\n", filepath.Base(fs.Name()))
		fs.PrintDefaults()
		fmt.Fprintln(fs.Output())
	}
	fs.StringVar(&goMod,        "gomod",        "",    "path to the root go.mod file")
	fs.StringVar(&coverProfile, "coverprofile", "",    "path to the Go test coverage profile file")
	fs.StringVar(&path,         "path",         "",    "path where HTML files will be written")
	fs.BoolVar  (&noBrowser,    "n",            false, "suppress opening the browser (overrides -s)")
	fs.BoolVar  (&httpServer,   "s",            false, "serve the generated HTML via a Python HTTP server")
	fs.Func     (               "order",               "per-file coverage stdout rows sort order", func(val string) error {
		switch val {
		case "lex":
			sortOrder = lex
			return nil
		case "shallowest":
			sortOrder = shallowest
			return nil
		case "deepest":
			sortOrder = deepest
			return nil
		case "lowest":
			sortOrder = lowest
			return nil
		case "highest":
			sortOrder = highest
			return nil
		case "shortest":
			sortOrder = shortest
			return nil
		case "longest":
			sortOrder = longest
			return nil
		default:
			return fmt.Errorf("invalid sort order specified\nmust be one of (%s)", strings.Join(allowedSortOrders, ", "))
		}
	})
	if err := fs.Parse(args); err != nil {
		return "", "", "", false, false, lex, err
	}
	if goMod == "" {
		fs.Usage()
		return "", "", "", false, false, lex, errors.New("no value specified for -gomod")
	}
	if coverProfile == "" {
		fs.Usage()
		return "", "", "", false, false, lex, errors.New("no value specified for -coverprofile")
	}
	if path == "" {
		fs.Usage()
		return "", "", "", false, false, lex, errors.New("no value specified for -path")
	}
	if len(fs.Args()) > 0 {
		fmt.Fprintf(fs.Output(), "ignored arguments: %s\n", strings.Join(fs.Args(), ", "))
	}
	return
}

// filterArgs discards any arguments up to and including "--".
func filterArgs(args []string) []string {
	for i, arg := range args {
		if arg == "--" {
			args = args[i + 1:]
			break
		}
	}
	return args
}
