package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPrintCoverage(t *testing.T) {
	t.Parallel()
	tests := []struct{
		name            string
		cov             map[string]coverage
		totalCovered    uint64
		totalStatements uint64
		want            string
		wantErr         bool
	}{
		{
			name: "succeeds",
			cov:  map[string]coverage{
				"foo":     { covered:  10, total: 100 },
				"bar/baz": { covered: 180, total: 200 },
				"boo":     { covered:  40, total:  40 },
			},
			totalCovered:     10 + 180 + 40,
			totalStatements: 100 + 200 + 40,
			want:            strings.Join([]string{
				"File    Coverage",
				"————————————————",
				"boo      100.00%",
				"foo       10.00%",
				"bar/baz   90.00%",
				"————————————————",
				"Total     67.65%" + "\n"}, "\n"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repGen := &reportGenerator{ cov: tt.cov }
			repGen.totalCovered.Store(tt.totalCovered)
			repGen.totalStatements.Store(tt.totalStatements)
			got := new(bytes.Buffer)
			err := repGen.printCoverage(got)
			if (err != nil) != tt.wantErr {
				t.Errorf("printCoverage(%q) returned unexpected error: %v; wantErr = %v", tt.name, err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got.String()); diff != "" {
				t.Errorf("printCoverage(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}
