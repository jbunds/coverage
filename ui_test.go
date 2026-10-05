package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPrintCoverage(t *testing.T) {
	t.Parallel()
	cov := map[string]coverage{
		"foo":     { covered:  10, total: 100 },
		"bar/baz": { covered: 180, total: 200 },
		"boo":     { covered:  40, total:  40 },
	}
	var totalCovered    uint64 =  10 + 180 + 40
	var totalStatements uint64 = 100 + 200 + 40
	tests := []struct{
		name            string
		order           sortOrder
		cov             map[string]coverage
		totalCovered    uint64
		totalStatements uint64
		want            string
	}{{
		name:            "sort by shortest path",
		order:           shortest,
		cov:             cov,
		totalCovered:    totalCovered,
		totalStatements: totalStatements,
		want:            strings.Join([]string{
			"File    Coverage",
			"————————————————",
			"boo      100.00%",
			"foo       10.00%",
			"bar/baz   90.00%",
			"————————————————",
			"Total     67.65%" + "\n"}, "\n"),
	}, {
		name:            "sort by longest path",
		order:           longest,
		cov:             cov,
		totalCovered:    totalCovered,
		totalStatements: totalStatements,
		want:            strings.Join([]string{
			"File    Coverage",
			"————————————————",
			"bar/baz   90.00%",
			"boo      100.00%",
			"foo       10.00%",
			"————————————————",
			"Total     67.65%" + "\n"}, "\n"),
	}, {
		name:            "sort by lowest coverage",
		order:           lowest,
		cov:             cov,
		totalCovered:    totalCovered,
		totalStatements: totalStatements,
		want:            strings.Join([]string{
			"File    Coverage",
			"————————————————",
			"foo       10.00%",
			"bar/baz   90.00%",
			"boo      100.00%",
			"————————————————",
			"Total     67.65%" + "\n"}, "\n"),
	}, {
		name:            "sort by highest coverage",
		order:           highest,
		cov:             cov,
		totalCovered:    totalCovered,
		totalStatements: totalStatements,
		want:            strings.Join([]string{
			"File    Coverage",
			"————————————————",
			"boo      100.00%",
			"bar/baz   90.00%",
			"foo       10.00%",
			"————————————————",
			"Total     67.65%" + "\n"}, "\n"),
	}, {
		name:            "sort lexicographically",
		order:           lex,
		cov:             cov,
		totalCovered:    totalCovered,
		totalStatements: totalStatements,
		want:            strings.Join([]string{
			"File    Coverage",
			"————————————————",
			"bar/baz   90.00%",
			"boo      100.00%",
			"foo       10.00%",
			"————————————————",
			"Total     67.65%" + "\n"}, "\n"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cs     := &coverageState{cov: tt.cov}
			cs.sort = tt.order.bind(cs)
			cs.totalCovered.Store(tt.totalCovered)
			cs.totalStatements.Store(tt.totalStatements)
			got := new(bytes.Buffer)
			err := cs.printCoverage(got)
			if err != nil {
				t.Errorf("printCoverage(%q) returned unexpected error: %v", tt.name, err)
			}
			if diff := cmp.Diff(tt.want, got.String()); diff != "" {
				t.Errorf("printCoverage(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestLaunchHTTPServer(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name    string
		runner  runner
		wantErr error
	}{{
		name:   "succeeds",
		runner:  &mockRunner{outputs: []mockOutput{
			{}, // Output: findPortPID -> no listener
			{}, // Start:  python -m http.server
			{}, // Run:    openBrowser
		}},
	}, {
		name:   "port in use",
		runner:  &mockRunner{outputs: []mockOutput{
			{stdout: "8000"}, // Output: findPortPID -> listener identified
			{},               // Run:    openBrowser
		}},
		wantErr: nil,
	}, {
		name:   "findPortPID fails",
		runner:  &mockRunner{outputs: []mockOutput{
			{err: errors.New("findPortPID failed")}, // Output: findPortPID -> error, ignored by SUT
			{},                                      // Start:  python -m http.server
			{},                                      // Run:    openBrowser
		}},
		wantErr: nil,
	}, {
		name:   "Start fails",
		runner:  &mockRunner{outputs: []mockOutput{
			{},                                // Output: findPortPID -> no listener
			{err: errors.New("Start failed")}, // Start:  python -m http.server
		}},
		wantErr: errors.New("Start failed"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := launchHTTPServer(tt.runner, "foo")
			if gotErr, wantErr := errStr(err), errStr(tt.wantErr); gotErr != wantErr {
				t.Errorf("launchHTTPServer(%q) returned unexpected error:\ngot:  %v\nwant: %v", tt.name, gotErr, wantErr)
			}
		})
	}
}

func TestOpenBrowser(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name    string
		runner  runner
		url     string
		wantErr error
	}{{
		name:   "succeeds",
		runner: &mockRunner{outputs: []mockOutput{{}}},
		url:    "bar",
	}, {
		name:    "fails",
		runner:  &mockRunner{outputs: []mockOutput{{err: errors.New("Run failed")}}},
		url:    "baz",
		wantErr: errors.New("Run failed"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := openBrowser(tt.runner, tt.url)
			if gotErr, wantErr := errStr(err), errStr(tt.wantErr); gotErr != wantErr {
				t.Errorf("openBrowser(%q) returned unexpected error:\ngot:  %v\nwant: %v", tt.name, gotErr, wantErr)
			}
		})
	}
}

func TestFindPortPID(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name    string
		runner  runner
		want    int
		wantErr error
	}{{
		name:   "succeeds",
		runner: &mockRunner{outputs: []mockOutput{{stdout: "8000"}}},
		want:   8000,
	}, {
		name:    "pid not found",
		runner:  &mockRunner{outputs: []mockOutput{{stdout: "not an integer"}}},
		want:    -1,
		wantErr: errors.New("expected integer"),
	}, {
		name:    "lsof fails",
		runner:  &mockRunner{outputs: []mockOutput{{err: errors.New("some lsof error")}}},
		want:    -1,
		wantErr: errors.New("some lsof error"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := findPortPID(tt.runner, "8000")
			if gotErr, wantErr := errStr(err), errStr(tt.wantErr); gotErr != wantErr {
				t.Errorf("findPortPID(%q) returned unexpected error:\ngot:  %v\nwant: %v", tt.name, gotErr, wantErr)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("findPortPID(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestGetFD(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name  string
		fd    any
		want  int
	}{{
		name:  "succeeds",
		fd:    mockFD{fd: 3},
		want:  3,
	}, {
		name:  "fails",
		fd:    "not a file",
		want:  -1,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got  := getFD(tt.fd)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("getFD(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}
