package main

import (
	"bytes"
	"flag"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestUsage(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name             string
		args             []string
		wantGoMod        string
		wantCoverProfile string
		wantPath         string
		wantOut          string
		wantNoBrowser    bool
		err              string // zero value means no error expected (err113)
	}{{
		name: "valid",
		args: []string{
			"-gomod",        "foo",
			"-coverprofile", "bar",
			"-path",         "baz",
		},
		wantGoMod:         "foo",
		wantCoverProfile:  "bar",
		wantPath:          "baz",
	}, {
		name:    "missing -gomod",
		err:     "no value specified for -gomod",
		wantOut: strings.Join([]string{
			"missing -gomod usage:",
			"",
			"  -coverprofile string",
			"    	path to the Go test coverage profile file",
			"  -gomod string",
			"    	path to the root go.mod file",
			"  -n	suppress opening the browser",
			"  -path string",
			"    	path where HTML files will be written",
			"\n"}, "\n"),
	}, {
		name:    "missing -coverprofile",
		args:    []string{"-gomod", "foo"},
		err:     "no value specified for -coverprofile",
		wantOut: strings.Join([]string{
			"missing -coverprofile usage:",
			"",
			"  -coverprofile string",
			"    	path to the Go test coverage profile file",
			"  -gomod string",
			"    	path to the root go.mod file",
			"  -n	suppress opening the browser",
			"  -path string",
			"    	path where HTML files will be written",
			"\n"}, "\n"),
	}, {
		name:    "missing -path",
		args:    []string{
			"-gomod",        "foo",
			"-coverprofile", "bar",
		},
		err:     "no value specified for -path",
		wantOut: strings.Join([]string{
			"missing -path usage:",
			"",
			"  -coverprofile string",
			"    	path to the Go test coverage profile file",
			"  -gomod string",
			"    	path to the root go.mod file",
			"  -n	suppress opening the browser",
			"  -path string",
			"    	path where HTML files will be written",
			"\n"}, "\n"),
	}, {
		name: "ignored args",
		args: []string{
			"-gomod",        "foo",
			"-coverprofile", "bar",
			"-path",         "baz",
			"bug",
			"boo",
		},
		wantGoMod:        "foo",
		wantCoverProfile: "bar",
		wantPath:         "baz",
		wantOut:          "ignored arguments: bug, boo\n",
	}, {
		name: "-n (supress opening browser) specified",
		args: []string{
			"-gomod",        "foo",
			"-coverprofile", "bar",
			"-path",         "baz",
			"-n",
		},
		wantGoMod:        "foo",
		wantCoverProfile: "bar",
		wantPath:         "baz",
		wantNoBrowser:    true,
	}, {
		name:    "invalid",
		args:    []string{"-invalid"},
		wantOut: strings.Join([]string{
			"flag provided but not defined: -invalid",
			"invalid usage:",
			"",
			"  -coverprofile string",
			"    	path to the Go test coverage profile file",
			"  -gomod string",
			"    	path to the root go.mod file",
			"  -n	suppress opening the browser",
			"  -path string",
			"    	path where HTML files will be written",
			"\n"}, "\n"),
		err: "flag provided but not defined: -invalid",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotOut := new(bytes.Buffer)
			fs     := flag.NewFlagSet(tt.name, flag.ContinueOnError)
			fs.SetOutput(gotOut)
			gotGoMod, gotCoverProfile, gotPath, gotNoBrowser, err := flags(fs, tt.args)
			if tt.err != "" {
				if err == nil {
					t.Errorf("flags(%q) did not fail", tt.name)
				}
				if tt.err != err.Error() {
					t.Errorf("flags(%q) returned %q; expected %q\n", tt.name, err, tt.err)
				}
			}
			if diff := cmp.Diff(tt.wantOut, gotOut.String()); diff != "" {
				t.Errorf("flags(%q) usage message mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantGoMod, gotGoMod); diff != "" {
				t.Errorf("flags(%q) goMod mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantCoverProfile, gotCoverProfile); diff != "" {
				t.Errorf("flags(%q) coverProfile mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantPath, gotPath); diff != "" {
				t.Errorf("flags(%q) path mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantNoBrowser, gotNoBrowser); diff != "" {
				t.Errorf("flags(%q) path mismatch (-want +got):\n%s", tt.name, diff)
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
		args: []string{"-gomod", "foo", "-coverfile", "bar", "-path", "baz"},
		want: []string{"-gomod", "foo", "-coverfile", "bar", "-path", "baz"},
	}, {
		name: "extra args",
		args: []string{"-gomod", "foo", "-coverfile", "bar", "-path", "baz", "--", "boo", "hoo"},
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
