package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestBind(t *testing.T) {
	t.Parallel()
	if lex != 0 {
		t.Fatal("sortOrder zero value is not lex")
	}
	cov := map[string]coverage{ // fully discriminates between all sort orders
		"a.go":      { covered: 80, total: 100 },
		"z.go":      { covered: 70, total: 100 },
		"a/b.go":    { covered: 10, total: 100 },
		"b/c/d.go":  { covered: 50, total: 100 },
		"abcdef.go": { covered: 60, total: 100 },
	}
	tests := []struct{
		name  string
		order sortOrder
		want  []string
	}{
		{"lex",        lex,        []string{"a.go",      "a/b.go",    "abcdef.go", "b/c/d.go",  "z.go"     }},
		{"shallowest", shallowest, []string{"a.go",      "abcdef.go", "z.go",      "a/b.go",    "b/c/d.go" }},
		{"deepest",    deepest,    []string{"b/c/d.go",  "a/b.go",    "a.go",      "abcdef.go", "z.go"     }},
		{"lowest",     lowest,     []string{"a/b.go",    "b/c/d.go",  "abcdef.go", "z.go",      "a.go"     }},
		{"highest",    highest,    []string{"a.go",      "z.go",      "abcdef.go", "b/c/d.go",  "a/b.go"   }},
		{"shortest",   shortest,   []string{"a.go",      "z.go",      "a/b.go",    "b/c/d.go",  "abcdef.go"}},
		{"longest",    longest,    []string{"abcdef.go", "b/c/d.go",  "a/b.go",    "a.go",      "z.go"     }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cs  := &coverageState{cov: cov}
			got := tt.order.bind(cs)()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("sort() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

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
			cs  := &coverageState{cov: cov}
			got := tt.order.bind(cs)()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("sort() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSortByPathDepth(t *testing.T) {
	t.Parallel()
	cov := map[string]coverage{
		"a":     {},
		"a/b":   {},
		"a/b/c": {},
		"d/e":   {},
		"f/g":   {},
	}
	tests := []struct {
		name  string
		order sortOrder
		want  []string
	}{{
		name: "sortByShallowPath",
		order: shallowest,
		want:  []string{"a", "a/b", "d/e", "f/g", "a/b/c"},
	}, {
		name: "sortByDeepPath",
		order: deepest,
		want:  []string{"a/b/c", "a/b", "d/e", "f/g", "a"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cs  := &coverageState{cov: cov}
			got := tt.order.bind(cs)()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("sort() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
