package main

import (
	"cmp"
	"maps"
	"slices"
	"strings"
)

// sortOrder selects the order by which per-file coverage stats are printed to the terminal.
type sortOrder int

const ( // must cohere with `allowedSortOrders` in flags.go
	// shortest sorts rows by path depth (lowest to highest; default).
	shortest sortOrder = iota
	// longest sorts rows by path depth (highest to lowest).
	longest
	// lowest sorts rows by coverage (lowest to highest).
	lowest
	// highest sorts rows by coverage (highest to lowest).
	highest
	// alphanumerically sorts rows alphanumerically.
	alpha
)

func (rg *reportGenerator) sortAlpha() []string {
	return slices.SortedFunc(maps.Keys(rg.cov), cmp.Compare) // sort alphanumerically
}

func (rg *reportGenerator) sortByLowCov() []string {
	return slices.SortedFunc(maps.Keys(rg.cov), func(a, b string) int {
		aCovered, bCovered := rg.cov[a].covered, rg.cov[b].covered
		aTotal, bTotal     := rg.cov[a].total,   rg.cov[b].total
		aPct, bPct         := 0.0, 0.0
		if aTotal > 0 { aPct = float64(aCovered) / float64(aTotal) }
		if bTotal > 0 { bPct = float64(bCovered) / float64(bTotal) }
		if aPct != bPct { return cmp.Compare(aPct, bPct) } // sort by coverage (lowest first)
		return cmp.Compare(a, b)                           // sort alphanumerically
	})
}

func (rg *reportGenerator) sortByHighCov() []string {
	return slices.SortedFunc(maps.Keys(rg.cov), func(a, b string) int {
		aCovered, bCovered := rg.cov[a].covered, rg.cov[b].covered
		aTotal, bTotal     := rg.cov[a].total,   rg.cov[b].total
		aPct, bPct         := 0.0, 0.0
		if aTotal > 0 { aPct = float64(aCovered) / float64(aTotal) }
		if bTotal > 0 { bPct = float64(bCovered) / float64(bTotal) }
		if aPct != bPct { return cmp.Compare(bPct, aPct) } // sort by coverage (highest first)
		return cmp.Compare(a, b)                           // sort alphanumerically
	})
}

func (rg *reportGenerator) sortByShortPath() []string {
	return slices.SortedFunc(maps.Keys(rg.cov), func(a, b string) int {
		depthA, depthB := strings.Count(a, "/"), strings.Count(b, "/")
		if depthA != depthB { return cmp.Compare(depthA, depthB) } // sort by path depth (shortest first)
		return cmp.Compare(a, b)                                   // sort alphanumerically
	})
}

func (rg *reportGenerator) sortByLongPath() []string {
	return slices.SortedFunc(maps.Keys(rg.cov), func(a, b string) int {
		depthA, depthB := strings.Count(a, "/"), strings.Count(b, "/")
		if depthA != depthB { return cmp.Compare(depthB, depthA) } // sort by path depth (longest first)
		return cmp.Compare(a, b)                                   // sort alphanumerically
	})
}

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
