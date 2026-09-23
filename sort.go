package main

import (
	"cmp"
	"maps"
	"slices"
	"strings"
)

// sortOrder selects the order in which rows of file paths
// and their coverage stats are printed to stdout.
type sortOrder int

const ( // must cohere with `allowedSortOrders` in flags.go
	// shortest sorts file paths by path depth (deepest to shallowest; default).
	shortest sortOrder = iota

	// longest sorts file paths by path depth (shallowest to deepest).
	longest

	// lowest sorts file paths by coverage (lowest to highest).
	lowest

	// highest sorts file paths by coverage (highest to lowest).
	highest

	// alpha sorts file paths lexicographically.
	alpha
)

// sortAlpha sorts file paths lexicographically.
func (rg *reportGenerator) sortAlpha() []string {
	return slices.SortedFunc(maps.Keys(rg.cov), cmp.Compare)
}

// sortByLowCov first sorts file paths by coverage (lowest first) and then lexicographically.
func (rg *reportGenerator) sortByLowCov() []string {
	return slices.SortedFunc(maps.Keys(rg.cov), func(a, b string) int {
		aCovered, bCovered := rg.cov[a].covered, rg.cov[b].covered
		aTotal, bTotal     := rg.cov[a].total,   rg.cov[b].total
		aPct, bPct         := 0.0, 0.0
		if aTotal > 0 { aPct = float64(aCovered) / float64(aTotal) }
		if bTotal > 0 { bPct = float64(bCovered) / float64(bTotal) }
		if aPct != bPct { return cmp.Compare(aPct, bPct) }
		return cmp.Compare(a, b)
	})
}

// sortByHighCov first sorts file paths by coverage (highest first) and then lexicographically.
func (rg *reportGenerator) sortByHighCov() []string {
	return slices.SortedFunc(maps.Keys(rg.cov), func(a, b string) int {
		aCovered, bCovered := rg.cov[a].covered, rg.cov[b].covered
		aTotal, bTotal     := rg.cov[a].total,   rg.cov[b].total
		aPct, bPct         := 0.0, 0.0
		if aTotal > 0 { aPct = float64(aCovered) / float64(aTotal) }
		if bTotal > 0 { bPct = float64(bCovered) / float64(bTotal) }
		if aPct != bPct { return cmp.Compare(bPct, aPct) }
		return cmp.Compare(a, b)
	})
}

// sortByShortPath first sorts file paths by path depth (deepest first) and then lexicographically.
func (rg *reportGenerator) sortByShortPath() []string {
	return slices.SortedFunc(maps.Keys(rg.cov), func(a, b string) int {
		depthA, depthB := strings.Count(a, "/"), strings.Count(b, "/")
		if depthA != depthB { return cmp.Compare(depthA, depthB) }
		return cmp.Compare(a, b)
	})
}

// sortByLongPath first sorts file paths by path depth (deepest first) and then lexicographically.
func (rg *reportGenerator) sortByLongPath() []string {
	return slices.SortedFunc(maps.Keys(rg.cov), func(a, b string) int {
		depthA, depthB := strings.Count(a, "/"), strings.Count(b, "/")
		if depthA != depthB { return cmp.Compare(depthB, depthA) }
		return cmp.Compare(a, b)
	})
}

// bind sets rg's sort method according to sortOrder o.
func (o sortOrder) bind(rg *reportGenerator) {
	switch o {
	case shortest:
		rg.sort = rg.sortByShortPath
	case longest:
		rg.sort = rg.sortByLongPath
	case lowest:
		rg.sort = rg.sortByLowCov
	case highest:
		rg.sort = rg.sortByHighCov
	case alpha:
		rg.sort = rg.sortAlpha
	}
}
