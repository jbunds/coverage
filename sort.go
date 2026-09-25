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
func (cs *coverageState) sortLex() []string {
	return slices.SortedFunc(maps.Keys(cs.cov), strings.Compare)
}

// sortByShallowPath first sorts file paths by path depth (shallowest first) and then lexicographically.
func (cs *coverageState) sortByShallowPath() []string {
	return slices.SortedFunc(maps.Keys(cs.cov), func(a, b string) int {
		depthA, depthB := strings.Count(a, "/"), strings.Count(b, "/")
		if depthA != depthB { return cmp.Compare(depthA, depthB) }
		return strings.Compare(a, b)
	})
}

// sortByDeepPath first sorts file paths by path depth (deepest first) and then lexicographically.
func (cs *coverageState) sortByDeepPath() []string {
	return slices.SortedFunc(maps.Keys(cs.cov), func(a, b string) int {
		depthA, depthB := strings.Count(a, "/"), strings.Count(b, "/")
		if depthA != depthB { return cmp.Compare(depthB, depthA) }
		return strings.Compare(a, b)
	})
}

// sortByLowCov first sorts file paths by coverage (lowest first) and then lexicographically.
func (cs *coverageState) sortByLowCov() []string {
	return slices.SortedFunc(maps.Keys(cs.cov), func(a, b string) int {
		aCovered, bCovered := cs.cov[a].covered, cs.cov[b].covered
		aTotal, bTotal     := cs.cov[a].total,   cs.cov[b].total
		aPct, bPct         := 0.0, 0.0
		if aTotal > 0 { aPct = float64(aCovered) / float64(aTotal) }
		if bTotal > 0 { bPct = float64(bCovered) / float64(bTotal) }
		if aPct != bPct { return cmp.Compare(aPct, bPct) }
		return strings.Compare(a, b)
	})
}

// sortByHighCov first sorts file paths by coverage (highest first) and then lexicographically.
func (cs *coverageState) sortByHighCov() []string {
	return slices.SortedFunc(maps.Keys(cs.cov), func(a, b string) int {
		aCovered, bCovered := cs.cov[a].covered, cs.cov[b].covered
		aTotal, bTotal     := cs.cov[a].total,   cs.cov[b].total
		aPct, bPct         := 0.0, 0.0
		if aTotal > 0 { aPct = float64(aCovered) / float64(aTotal) }
		if bTotal > 0 { bPct = float64(bCovered) / float64(bTotal) }
		if aPct != bPct { return cmp.Compare(bPct, aPct) }
		return strings.Compare(a, b)
	})
}

// sortByShortPath first sorts file paths by path length (shortest first) and then lexicographically.
func (cs *coverageState) sortByShortPath() []string {
	return slices.SortedFunc(maps.Keys(cs.cov), func(a, b string) int {
		aLen, bLen := len(a), len(b)
		if aLen != bLen { return cmp.Compare(aLen, bLen) }
		return strings.Compare(a, b)
	})
}

// sortByLongPath first sorts file paths by path length (longest first) and then lexicographically.
func (cs *coverageState) sortByLongPath() []string {
	return slices.SortedFunc(maps.Keys(cs.cov), func(a, b string) int {
		aLen, bLen := len(a), len(b)
		if aLen != bLen { return cmp.Compare(bLen, aLen) }
		return strings.Compare(a, b)
	})
}

// bind returns the sort function for the given order; unrecognized orders default to lex.
func (o sortOrder) bind(cs *coverageState) func() []string {
	switch o {
	case shallowest: return cs.sortByShallowPath
	case deepest:    return cs.sortByDeepPath
	case lowest:     return cs.sortByLowCov
	case highest:    return cs.sortByHighCov
	case shortest:   return cs.sortByShortPath
	case longest:    return cs.sortByLongPath
	default:         return cs.sortLex
	}
}
