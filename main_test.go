package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPrintCoverage(t *testing.T) {
	repGen := &reportGenerator{
		cov: map[string]coverage{
			"foo": { covered:  10, total: 100 },
			"bar": { covered: 180, total: 200 },
			"baz": { covered:  40, total:  40 },
		},
	}
	want := strings.Join([]string{
		"File  Coverage",
    "——————————————",
    "bar     90.00%",
		"baz    100.00%",
    "foo     10.00%",
    "——————————————",
    "Total    0.00%" + "\n"}, "\n")
	got  := captureStdout(t, func() { repGen.printCoverage() })
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("printCoverage() mismatch (-want +got):\n%s", diff)
	}
}

func captureStdout(tb testing.TB, fn func()) string {
	tb.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		tb.Fatalf("failed to create pipe to capture stdout: %v", err)
	}
	orig := os.Stdout
	tb.Cleanup(func() { os.Stdout = orig })
	os.Stdout = w
	type result struct {
		out string
		err error
	}
	resChan := make(chan result)
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
		tb.Errorf("failed to capture stdout: %v", err)
	}
	return res.out
}
