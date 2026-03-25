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
		name   string
		args   []string
		want   string
		stdErr string
		err    string // zero value means no error expected (err113)
	}{
		{
			name: "valid",
			args: []string{
				"-coverprofile", "foo",
				"-path",         "bar",
			},
		},
		{
			name: "missing -coverprofile",
			err:  "no value specified for -coverprofile",
			want: strings.Join([]string{
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
			err:  "no value specified for -path",
			want: strings.Join([]string{
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
				"-coverprofile", "foo",
				"-path",         "bar",
				"baz",
				"boo",
			},
			stdErr: "ignored arguments: baz, boo\n",
		},
		{
			name: "invalid",
			args: []string{"-invalid"},
			want: strings.Join([]string{
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
			gotErr := captureStderr(t, func() {
				_, _, err = flags(fs, tt.args)
			})
			if tt.err != "" {
				if err == nil {
					t.Errorf("flags(%v) did not fail", tt.args)
				}
				if tt.err != err.Error() {
					t.Errorf("flags(%v) returned %q; expected %q\n", tt.args, err, tt.err)
				}
			}
			if diff := cmp.Diff(tt.stdErr, gotErr); diff != "" {
				t.Errorf("flags(%v) stderr mismatch (-want +got):\n%s", tt.args, diff)
			}
			if diff := cmp.Diff(tt.want, gotOut.String()); diff != "" {
				t.Errorf("flags(%v) usage message mismatch (-want +got):\n%s", tt.args, diff)
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
