package main

import (
	"bytes"
	"flag"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFlags(t *testing.T) {
	t.Parallel()
	usage := getUsage(t)
	tests := []struct{
		name         string
		args         []string
		wantFlagVals *flagVals
		wantOut      string
		err          string // zero value means no error expected (err113)
	}{{
		name: "valid",
		args: []string{
			"-gomod",        "foo",
			"-coverprofile", "bar",
			"-outdir",       "baz",
		},
		wantFlagVals: &flagVals{
			goModFile:        "foo",
			coverProfileFile: "bar",
			outDir:           "baz",
			sortOrder:        lex,
		},
	}, {
		name: "ignored args",
		args: []string{
			"-gomod",        "foo",
			"-coverprofile", "bar",
			"-outdir",       "baz",
			"bug",
			"boo",
		},
		wantFlagVals: &flagVals{
			goModFile:        "foo",
			coverProfileFile: "bar",
			outDir:           "baz",
			sortOrder:        lex,
		},
		wantOut: "ignored arguments: bug, boo\n",
	}, {
		name:    "missing -gomod",
		err:     "no value specified for -gomod",
		wantOut: "missing -gomod usage:\n" + usage,
	}, {
		name:    "missing -coverprofile",
		args:    []string{"-gomod", "foo"},
		err:     "no value specified for -coverprofile",
		wantOut: "missing -coverprofile usage:\n" + usage,
	}, {
		name: "missing -outdir",
		args: []string{
			"-gomod",        "foo",
			"-coverprofile", "bar",
		},
		err:     "no value specified for -outdir",
		wantOut: "missing -outdir usage:\n" + usage,
	}, {
		name: "-n set",
		args: []string{
			"-gomod",        "foo",
			"-coverprofile", "bar",
			"-outdir",       "baz",
			"-n",
		},
		wantFlagVals: &flagVals{
			goModFile:        "foo",
			coverProfileFile: "bar",
			outDir:           "baz",
			sortOrder:        lex,
			noBrowser:        true,
		},
	}, {
		name: "-s set",
		args: []string{
			"-gomod",        "foo",
			"-coverprofile", "bar",
			"-outdir",       "baz",
			"-s",
		},
		wantFlagVals: &flagVals{
			goModFile:        "foo",
			coverProfileFile: "bar",
			outDir:           "baz",
			sortOrder:        lex,
			httpServer:       true,
		},
	}, {
		name: "-order lex",
		args: []string{
			"-gomod",        "foo",
			"-coverprofile", "bar",
			"-outdir",       "baz",
			"-order",        "lex",
		},
		wantFlagVals: &flagVals{
			goModFile:        "foo",
			coverProfileFile: "bar",
			outDir:           "baz",
			sortOrder:        lex,
		},
	}, {
		name: "-order shallowest",
		args: []string{
			"-gomod",        "foo",
			"-coverprofile", "bar",
			"-outdir",       "baz",
			"-order",        "shallowest",
		},
		wantFlagVals: &flagVals{
			goModFile:        "foo",
			coverProfileFile: "bar",
			outDir:           "baz",
			sortOrder:        shallowest,
		},
	}, {
		name: "-order deepest",
		args: []string{
			"-gomod",        "foo",
			"-coverprofile", "bar",
			"-outdir",       "baz",
			"-order",        "deepest",
		},
		wantFlagVals: &flagVals{
			goModFile:        "foo",
			coverProfileFile: "bar",
			outDir:           "baz",
			sortOrder:        deepest,
		},
	}, {
		name: "-order lowest",
		args: []string{
			"-gomod",        "foo",
			"-coverprofile", "bar",
			"-outdir",       "baz",
			"-order",        "lowest",
		},
		wantFlagVals: &flagVals{
			goModFile:        "foo",
			coverProfileFile: "bar",
			outDir:           "baz",
			sortOrder:        lowest,
		},
	}, {
		name: "-order highest",
		args: []string{
			"-gomod",        "foo",
			"-coverprofile", "bar",
			"-outdir",       "baz",
			"-order",        "highest",
		},
		wantFlagVals: &flagVals{
			goModFile:        "foo",
			coverProfileFile: "bar",
			outDir:           "baz",
			sortOrder:        highest,
		},
	}, {
		name: "-order shortest",
		args: []string{
			"-gomod",        "foo",
			"-coverprofile", "bar",
			"-outdir",       "baz",
			"-order",        "shortest",
		},
		wantFlagVals: &flagVals{
			goModFile:        "foo",
			coverProfileFile: "bar",
			outDir:           "baz",
			sortOrder:        shortest,
		},
	}, {
		name: "-order longest",
		args: []string{
			"-gomod",        "foo",
			"-coverprofile", "bar",
			"-outdir",       "baz",
			"-order",        "longest",
		},
		wantFlagVals: &flagVals{
			goModFile:        "foo",
			coverProfileFile: "bar",
			outDir:           "baz",
			sortOrder:        longest,
		},
	}, {
		name:    "-order invalid",
		args:    []string{"-order", "invalid"},
		wantOut: strings.Join([]string{
			`invalid value "invalid" for flag -order: invalid sort order specified`,
			"must be one of (lex, shallowest, deepest, lowest, highest, shortest, longest)",
			"-order invalid usage:",
			usage,
		}, "\n"),
		err: strings.Join([]string{
			`invalid value "invalid" for flag -order: invalid sort order specified`,
			"must be one of (lex, shallowest, deepest, lowest, highest, shortest, longest)",
		}, "\n"),
	}, {
		name:    "invalid",
		args:    []string{"-invalid"},
		wantOut: strings.Join([]string{
			"flag provided but not defined: -invalid",
			"invalid usage:",
			usage,
		}, "\n"),
		err: "flag provided but not defined: -invalid",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotOut := new(bytes.Buffer)
			fs     := flag.NewFlagSet(tt.name, flag.ContinueOnError)
			fs.SetOutput(gotOut)
			gotFlagVals, err := flags(fs, tt.args)
			if tt.err != "" {
				if err == nil {
					t.Errorf("flags(%q) did not fail", tt.name)
				}
				if tt.err != err.Error() {
					t.Errorf("flags(%q) returned %q; expected %q\n", tt.name, err, tt.err)
				}
				return
			}
			if diff := cmp.Diff(tt.wantOut, gotOut.String()); diff != "" {
				t.Errorf("flags(%q) usage message mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantFlagVals.goModFile, gotFlagVals.goModFile); diff != "" {
				t.Errorf("flags(%q) goMod mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantFlagVals.coverProfileFile, gotFlagVals.coverProfileFile); diff != "" {
				t.Errorf("flags(%q) coverProfile mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantFlagVals.outDir, gotFlagVals.outDir); diff != "" {
				t.Errorf("flags(%q) path mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantFlagVals.noBrowser, gotFlagVals.noBrowser); diff != "" {
				t.Errorf("flags(%q) browser mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantFlagVals.httpServer, gotFlagVals.httpServer); diff != "" {
				t.Errorf("flags(%q) httpServer mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantFlagVals.sortOrder, gotFlagVals.sortOrder); diff != "" {
				t.Errorf("flags(%q) sortOrder mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestFilterArgs(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name string
		args []string
		want []string
	}{{
		name: "no extra args",
		args: []string{"-gomod", "foo", "-coverfile", "bar", "-outdir", "baz"},
		want: []string{"-gomod", "foo", "-coverfile", "bar", "-outdir", "baz"},
	}, {
		name: "extra args",
		args: []string{"-gomod", "foo", "-coverfile", "bar", "-outdir", "baz", "--", "boo", "hoo"},
		want: []string{"boo", "hoo"},
	}, {
		name: "invalid args",
		args: []string{"foo", "bar", "--", "baz", "boo"},
		want: []string{"baz", "boo"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := filterArgs(tt.args)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("filterArgs(%v) mismatch (-want +got):\n%s", tt.args, diff)
			}
		})
	}
}

func getUsage(t *testing.T) string {
	t.Helper()
	buf := new(bytes.Buffer)
	fs  := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(buf)
	if _, err := flags(fs, []string{"-invalid"}); err == nil {
		t.Fatal("flags() unexpectedly succeeded")
	}
	return strings.SplitN(buf.String(), "\n", 3)[2]
}
