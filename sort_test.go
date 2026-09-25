package main

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestBindIdentity(t *testing.T) {
	t.Parallel()
	// the compiler appends "-fm" (mnemonic for "function from method")
	//
	// this is an internal implementation detail that leaked into the
	// symbol table, so hardcoding it here is decidedly brittle
	suffix := "-fm"
	tests  := []struct{
		name  string
		order sortOrder
		want  string
	}{
		{"default (lex)", 0,          "sortLex"          }, // validate that the sortOrder's zero value (i.e., the default value) is lex
		{"shallowest",    shallowest, "sortByShallowPath"},
		{"deepest",       deepest,    "sortByDeepPath"   },
		{"lowest",        lowest,     "sortByLowCov"     },
		{"highest",       highest,    "sortByHighCov"    },
		{"shortest",      shortest,   "sortByShortPath"  },
		{"longest",       longest,    "sortByLongPath"   },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rg := &reportGenerator{fsys: &localFS{}}
			if err := rg.getModName(&flagVals{goModFile: "go.mod"}); err != nil {
				t.Fatal(err)
			}
			prefix := rg.modName + ".(*reportGenerator)." // "github.com/jbunds/coverage.(*reportGenerator)."
			tt.order.bind(rg)
			want := prefix + tt.want + suffix
			got  := runtime.FuncForPC(reflect.ValueOf(rg.sort).Pointer()).Name()
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("bind(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestBind(t *testing.T) {
	t.Parallel()
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
			rg := &reportGenerator{cov: cov}
			tt.order.bind(rg)
			got := rg.sort()
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
			rg   := &reportGenerator{cov: cov}
			tt.order.bind(rg)
			got  := rg.sort()
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
			rg   := &reportGenerator{cov: cov}
			tt.order.bind(rg)
			got  := rg.sort()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("sort() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
