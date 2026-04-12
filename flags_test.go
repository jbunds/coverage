package main

import (
	"bytes"
	"flag"
	"io"
	"os"
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
		stdErr           string
		err              string // zero value means no error expected (err113)
	}{
		{
			name: "valid",
			args: []string{
				"-gomod",        "foo",
				"-coverprofile", "bar",
				"-path",         "baz",
			},
			wantGoMod:         "foo",
			wantCoverProfile:  "bar",
			wantPath:          "baz",
		},
		{
			name:    "missing -gomod",
			err:     "no value specified for -gomod",
			wantOut: strings.Join([]string{
				"missing -gomod usage:",
				"",
				"  -coverprofile string",
				"    	path to Go test coverage profile file",
				"  -gomod string",
				"    	path to the root go.mod file",
				"  -path string",
				"    	path where HTML files will be written",
				"\n"}, "\n"),
		},
		{
			name:    "missing -coverprofile",
			args:    []string{"-gomod", "foo"},
			err:     "no value specified for -coverprofile",
			wantOut: strings.Join([]string{
				"missing -coverprofile usage:",
				"",
				"  -coverprofile string",
				"    	path to Go test coverage profile file",
				"  -gomod string",
				"    	path to the root go.mod file",
				"  -path string",
				"    	path where HTML files will be written",
				"\n"}, "\n"),
		},
		{
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
				"    	path to Go test coverage profile file",
				"  -gomod string",
				"    	path to the root go.mod file",
				"  -path string",
				"    	path where HTML files will be written",
				"\n"}, "\n"),
		},
		{
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
			stdErr:           "ignored arguments: bug, boo\n",
		},
		{
			name:    "invalid",
			args:    []string{"-invalid"},
			wantOut: strings.Join([]string{
				"flag provided but not defined: -invalid",
				"invalid usage:",
				"",
				"  -coverprofile string",
				"    	path to Go test coverage profile file",
				"  -gomod string",
				"    	path to the root go.mod file",
				"  -path string",
				"    	path where HTML files will be written",
				"\n"}, "\n"),
			err: "flag provided but not defined: -invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOut := new(bytes.Buffer)
			fs     := flag.NewFlagSet(tt.name, flag.ContinueOnError)
			fs.SetOutput(gotOut)
			var err error
			var gotGoMod, gotCoverProfile, gotPath string
			gotErr := captureStderr(t, func() {
				gotGoMod, gotCoverProfile, gotPath, err = flags(fs, tt.args)
			})
			if tt.err != "" {
				if err == nil {
					t.Errorf("flags(%q) did not fail", tt.name)
				}
				if tt.err != err.Error() {
					t.Errorf("flags(%q) returned %q; expected %q\n", tt.name, err, tt.err)
				}
			}
			if diff := cmp.Diff(tt.stdErr, gotErr); diff != "" {
				t.Errorf("flags(%q) stderr mismatch (-want +got):\n%s", tt.name, diff)
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
		})
	}
}

func captureStderr(tb testing.TB, fn func()) string {
	tb.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		tb.Fatalf("failed to create pipe to capture stderr: %v", err)
	}
	orig := os.Stderr
	tb.Cleanup(func() { os.Stderr = orig })
	os.Stderr = w
	type result struct {
		out string
		err error
	}
	resChan  := make(chan result)
	go func() {
		buf        := new(bytes.Buffer)
		_, copyErr := io.Copy(buf, r)
		resChan <- result{out: buf.String(), err: copyErr}
	}()
	fn()
	if err := w.Close(); err != nil {
		tb.Errorf("w.Close() failed: %v", err)
	}
	res := <-resChan
	if res.err != nil {
		tb.Errorf("failed to capture stderr: %v", err)
	}
	return res.out
}
