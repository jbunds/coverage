package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSortByCov(t *testing.T) {
	t.Parallel()
	cov := map[string]coverage{
		"lowest":  { covered:  10, total: 100 },
		"one":     { covered:  90, total: 100 },
		"two":     { covered: 180, total: 200 },
		"highest": { covered:  40, total:  40 },
	}
	tests := []struct {
		name  string
		order sortOrder
		want  []string
	}{{
		name: "sortByLowCov",
		order: lowest,
		want:  []string{"lowest", "one", "two", "highest"},
	}, {
		name: "sortByHighCov",
		order: highest,
		want:  []string{"highest", "one", "two", "lowest"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rg   := &reportGenerator{cov: cov}
			tt.order.bind(rg)
			got  := rg.sort()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("sort() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
