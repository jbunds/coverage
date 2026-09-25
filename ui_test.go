package main

import (
	"bytes"
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
		wantErr         bool
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
			if (err != nil) != tt.wantErr {
				t.Errorf("printCoverage(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got.String()); diff != "" {
				t.Errorf("printCoverage(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}
