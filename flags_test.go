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
		name             string
		args             []string
		wantGoMod        string
		wantCoverProfile string
		wantPath         string
		wantOut          string
		wantNoBrowser    bool
		wantHTTPServer   bool
		wantSortOrder    sortOrder
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
		wantOut: "missing -gomod usage:\n" + usage,
	}, {
		name:    "missing -coverprofile",
		args:    []string{"-gomod", "foo"},
		err:     "no value specified for -coverprofile",
		wantOut: "missing -coverprofile usage:\n" + usage,
	}, {
		name:    "missing -path",
		args:    []string{
			"-gomod",        "foo",
			"-coverprofile", "bar",
		},
		err:     "no value specified for -path",
		wantOut: "missing -path usage:\n" + usage,
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
		name: "-n set",
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
		name: "-s set",
		args: []string{
			"-gomod",        "foo",
			"-coverprofile", "bar",
			"-path",         "baz",
			"-s",
		},
		wantGoMod:        "foo",
		wantCoverProfile: "bar",
		wantPath:         "baz",
		wantHTTPServer:   true,
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
			gotGoMod,
			gotCoverProfile,
			gotPath,
			gotNoBrowser,
			gotHTTPServer,
			gotSortOrder, err := flags(fs, tt.args)
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
				t.Errorf("flags(%q) noBrowser mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantHTTPServer, gotHTTPServer); diff != "" {
				t.Errorf("flags(%q) httpServer mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantSortOrder, gotSortOrder); diff != "" {
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

func getUsage(t *testing.T) string {
	t.Helper()
	buf := new(bytes.Buffer)
	fs  := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(buf)
	if _, _, _, _, _, _, err := flags(fs, []string{"-invalid"}); err == nil {
		t.Fatal("flags() unexpectedly succeeded")
	}
	return strings.SplitN(buf.String(), "\n", 3)[2]
}
