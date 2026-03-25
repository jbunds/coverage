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
	tests := []struct{
		name             string
		args             []string
		wantOut          string
		wantCoverProfile string
		wantPath         string
		stdErr           string
		err              string // zero value means no error expected (err113)
	}{
		{
			name: "valid",
			args: []string{
				"-coverprofile", "foo",
				"-path",         "bar",
			},
			wantCoverProfile:  "foo",
			wantPath:          "bar",
		},
		{
			name:    "missing -coverprofile",
			err:     "no value specified for -coverprofile",
			wantOut: strings.Join([]string{
				"missing -coverprofile usage:",
				"",
				"  -coverprofile string",
				"    	path to Go test coverage profile file",
				"  -path string",
				"    	path where HTML files will be written",
				""}, "\n"),
		},
		{
			name: "missing -path",
			args: []string{
				"-coverprofile", "baz",
			},
			err:     "no value specified for -path",
			wantOut: strings.Join([]string{
				"missing -path usage:",
				"",
				"  -coverprofile string",
				"    	path to Go test coverage profile file",
				"  -path string",
				"    	path where HTML files will be written",
				""}, "\n"),
		},
		{
			name: "ignored args",
			args: []string{
				"-coverprofile", "boo",
				"-path",         "bug",
				"qux",
				"bip",
			},
			wantCoverProfile: "boo",
			wantPath:         "bug",
			stdErr:           "ignored arguments: qux, bip\n",
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
				"  -path string",
				"    	path where HTML files will be written",
				""}, "\n"),
			err: "flag provided but not defined: -invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOut := new(bytes.Buffer)
			fs     := flag.NewFlagSet(tt.name, flag.ContinueOnError)
			fs.SetOutput(gotOut)
			var err error
			var gotCoverProfile, gotPath string
			gotErr := captureStderr(t, func() {
				gotCoverProfile, gotPath, err = flags(fs, tt.args)
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
