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
	// lex sorts file paths lexicographically (default).
	lex sortOrder = iota

	// shallowest sorts file paths by path depth (shallowest to deepest).
	shallowest

	// deepest sorts file paths by path depth (deepest to shallowest).
	deepest

	// lowest sorts file paths by coverage (lowest to highest).
	lowest

	// highest sorts file paths by coverage (highest to lowest).
	highest

	// shortest sorts file paths by path length (shortest to longest).
	shortest

	// longest sorts file paths by path length (longest to shortest).
	longest
)

// sortLex sorts file paths lexicographically.
func (rg *reportGenerator) sortLex() []string {
	return slices.SortedFunc(maps.Keys(rg.cov), strings.Compare)
}

// sortByShallowPath first sorts file paths by path depth (shallowest first) and then lexicographically.
func (rg *reportGenerator) sortByShallowPath() []string {
	return slices.SortedFunc(maps.Keys(rg.cov), func(a, b string) int {
		depthA, depthB := strings.Count(a, "/"), strings.Count(b, "/")
		if depthA != depthB { return cmp.Compare(depthA, depthB) }
		return strings.Compare(a, b)
	})
}

// sortByDeepPath first sorts file paths by path depth (deepest first) and then lexicographically.
func (rg *reportGenerator) sortByDeepPath() []string {
	return slices.SortedFunc(maps.Keys(rg.cov), func(a, b string) int {
		depthA, depthB := strings.Count(a, "/"), strings.Count(b, "/")
		if depthA != depthB { return cmp.Compare(depthB, depthA) }
		return strings.Compare(a, b)
	})
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
		return strings.Compare(a, b)
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
		return strings.Compare(a, b)
	})
}

// sortByShortPath first sorts file paths by path length (shortest first) and then lexicographically.
func (rg *reportGenerator) sortByShortPath() []string {
	return slices.SortedFunc(maps.Keys(rg.cov), func(a, b string) int {
		aLen, bLen := len(a), len(b)
		if aLen != bLen { return cmp.Compare(aLen, bLen) }
		return strings.Compare(a, b)
	})
}

// sortByLongPath first sorts file paths by path length (longest first) and then lexicographically.
func (rg *reportGenerator) sortByLongPath() []string {
	return slices.SortedFunc(maps.Keys(rg.cov), func(a, b string) int {
		aLen, bLen := len(a), len(b)
		if aLen != bLen { return cmp.Compare(bLen, aLen) }
		return strings.Compare(a, b)
	})
}

// bind sets rg's sort method according to sortOrder o.
func (o sortOrder) bind(rg *reportGenerator) {
	switch o {
	case lex:
		rg.sort = rg.sortLex
	case shallowest:
		rg.sort = rg.sortByShallowPath
	case deepest:
		rg.sort = rg.sortByDeepPath
	case lowest:
		rg.sort = rg.sortByLowCov
	case highest:
		rg.sort = rg.sortByHighCov
	case shortest:
		rg.sort = rg.sortByShortPath
	case longest:
		rg.sort = rg.sortByLongPath
	}
}
