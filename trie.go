package main

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
)

// trieNode represents a single directory or source file in the source tree.
type trieNode struct {
	segment   string               // package path segment
	modPath   string               // module path (https://go.dev/ref/mod#module-path) used to generate IDs for the "modules" menu
	html      string               // pre-rendered HTML fragment
	isModRoot bool                 // identifies this node as the module root
	children  map[string]*trieNode // child nodes
	covered   uint64               // covered statements
	total     uint64               // total statements
}

// renderHTML renders the root <ul> containing all direct children of
// the root node, sorted lexicographically. counter is used to assign
// unique IDs to non-module-root nodes.
func (tn *trieNode) renderHTML(counter *atomic.Uint64) string {
	if tn == nil { return "" }

	var sb strings.Builder

	sb.WriteString("<ul class=\"tree\">\n")

	for _, k := range slices.Sorted(maps.Keys(tn.children)) {
		tn.children[k].assemble(&sb, 2, counter)
	}

	sb.WriteString("</ul>")

	return sb.String()
}

// assemble recursively renders this node's <li> element (checkbox, label,
// and child <ul>) at the given indentation depth. Module-root node IDs
// are normalized to a valid URI anchor string; all other nodes receive
// a sequential ID from counter.
func (tn *trieNode) assemble(sb *strings.Builder, depth int, counter *atomic.Uint64) {
	if tn == nil { return }

	indent := strings.Repeat(" ", depth)

	var itemID string
	if tn.isModRoot {
		itemID = normalizeModID(tn.modPath)
	} else {
		itemID = "tree-item-" + strconv.FormatUint(counter.Add(1), 10)
	}

	sb.WriteString(indent)
	sb.WriteString(`<li id="`)
	sb.WriteString(itemID)
	sb.WriteString("\">\n")
	sb.WriteString(indent)
	sb.WriteString(`  <input type="checkbox" id="module-`)
	sb.WriteString(itemID)
	sb.WriteString("\"/>\n")
	sb.WriteString(indent)
	sb.WriteString("  <div class=\"tree-node\">\n")
	sb.WriteString(indent)
	sb.WriteString(`    <label for="module-`)
	sb.WriteString(itemID)
	sb.WriteString(`"><span class="icon"></span>`)
	sb.WriteString(tn.segment)
	sb.WriteString("</label>\n")

	if tn.html != "" { // ignore namespace segments preceeding the module root
		sb.WriteString(indent)
		sb.WriteString(`    <span class="cov">`)
		sb.WriteString(formatCov(tn.covered, tn.total))
		sb.WriteString("</span>\n")
	}

	sb.WriteString(indent)
	sb.WriteString("  </div>\n")
	sb.WriteString(indent)
	sb.WriteString("  <ul>\n")

	for _, k := range slices.Sorted(maps.Keys(tn.children)) {
		tn.children[k].assemble(sb, depth + 4, counter)
	}

	if tn.html != "" {
		sb.WriteString(tn.html)
	}

	sb.WriteString(indent)
	sb.WriteString("  </ul>\n")
	sb.WriteString(indent)
	sb.WriteString("</li>\n")
}

// insert walks modPath segment by segment, creating intermediate nodes as
// needed, and aggregates coverage stats at every ancestor node, up to the
// module root node.
func (tn *trieNode) insert(modPath string, dirHTML string, covered, total uint64) {
	node := tn

	for segment := range strings.SplitSeq(modPath, "/") {
		child, ok := node.children[segment]
		if !ok {
			child = &trieNode{segment: segment, children: make(map[string]*trieNode)}
			node.children[segment] = child
		}

		node.covered += covered // aggregate coverage stats for higher-level shared parent paths
		node.total   += total
		node          = child
	}

	node.covered  += covered
	node.total    += total
	node.html      = dirHTML
	node.isModRoot = true
	node.modPath   = modPath
}

// normalizeModID converts a module path (https://go.dev/ref/mod#module-path)
// to a valid URI anchor string by replacing "/" and "." with "-".
func normalizeModID(modPath string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '.' {
			return  '-'
		}
		return r
	}, modPath)
}
