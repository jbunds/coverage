package main

import (
	"bytes"
	"go/ast"
	"go/scanner"
	"go/token"
	"go/types"
	"sort"
	"strings"
	"text/template"

	"golang.org/x/tools/cover"
)

// profileBlock maps a coverage profile range to
// its absolute byte offsets in the source file.
type profileBlock struct {
	startOffset, endOffset, count int
}

// funcSpan identifies a top-level function in
// a source file by name and byte offsets.
type funcSpan struct {
	name                   string
	startOffset, endOffset int
}

// srcSpan is a contiguous range of source bytes with its syntax class.
//
// It is emitted immediately or buffered in pendingSpans between coverage boundaries.
type srcSpan struct {
	fragment []byte
	absStart int
	synClass string
}

// spanWriter accumulates buffered spans between coverage-class boundaries.
type spanWriter struct {
	buf          *strings.Builder
	src          []byte
	blocks       []*profileBlock
	pendingClass string
	pendingSpans []srcSpan
	lastOffset   int
}

// annotatedLine is a coverage-annotated single
// line of source code with func metadata.
type annotatedLine struct {
	text      string // annotated HTML for this line (spans, etc)
	funcIdx   int    // index into funcs, or -1 if not in a function
	isFuncEnd bool   // true on the last line of a function
}

// annotateSource scans src and returns one annotatedLine per source line.
func annotateSource(fileAST *ast.File, file *token.File, src []byte, typeInfo *types.Info, profile *cover.Profile) []annotatedLine {
	lineOffsets := computeLineOffsets(src)
	blocks      := computeBlockOffsets(file, profile.Blocks)
	funcSpans   := computeFuncSpans(fileAST, file)
	classifier  := buildSyntaxClassifier(fileAST, typeInfo)

	w := &spanWriter{
		buf:    new(strings.Builder),
		src:    src,
		blocks: blocks,
	}

	scnr := new(scanner.Scanner)
	scnr.Init(file, src, nil, scanner.ScanComments)

	for {
		pos, tok, lit := scnr.Scan()
		if tok == token.EOF {
			w.flushSpans()
			if w.lastOffset < len(src) {
				writeSpan(w.buf, w.src[w.lastOffset:], w.lastOffset, "", "", w.blocks)
			}
			break
		}

		startOffset := file.Offset(pos)
		endOffset   := startOffset + len(lit)

		if lit == "" { // handle explicit and virtual punctuation tokens
			if tok == token.SEMICOLON && (startOffset >= len(src) || src[startOffset] != ';') {
				continue // virtual semicolon: zero source bytes
			}
			endOffset = startOffset + len(tok.String())
		}

		covClass := coverClass(blocks, startOffset, endOffset)
		synClass := classifier.classify(tok, pos, lit)
		spans    := []srcSpan{{src[startOffset : endOffset], startOffset, synClass}}

		if tok == token.STRING && classifier.importPos[pos] {
			if importSpans := splitImportSpans(src, startOffset, endOffset, lit); importSpans != nil {
				spans = importSpans
			}
		}

		w.writeGroup(spans, covClass, startOffset, endOffset)
	}

	return finalizeLines(w.buf.String(), lineOffsets, funcSpans)
}

// writeGroup appends spans to the writer, writing uncovered spans immediately,
// and buffering spans in covered regions (flushed at the next class boundary).
func (w *spanWriter) writeGroup(spans []srcSpan, covClass string, startOffset, endOffset int) {
	if covClass != w.pendingClass {
		w.flushSpans()
		w.pendingClass = covClass
	}
	if w.pendingClass != "" {
		if startOffset > w.lastOffset {
			w.pendingSpans = append(w.pendingSpans, srcSpan{w.src[w.lastOffset : startOffset], w.lastOffset, ""})
		}
		w.pendingSpans = append(w.pendingSpans, spans...)
	} else {
		if startOffset > w.lastOffset {
			writeSpan(w.buf, w.src[w.lastOffset : startOffset], w.lastOffset, "", "", w.blocks)
		}
		for _, s := range spans {
			writeSpan(w.buf, s.fragment, s.absStart, "", s.synClass, w.blocks)
		}
	}
	w.lastOffset = endOffset
}

// flushSpans writes all pending spans to buf and resets the pendingSpans slice.
func (w *spanWriter) flushSpans() {
	for _, s := range w.pendingSpans {
		writeSpan(w.buf, s.fragment, s.absStart, w.pendingClass, s.synClass, w.blocks)
	}
	w.pendingSpans = w.pendingSpans[:0]
}

// writeSpan writes a fragment to buf, wrapping it in a <span>
// element with coverage and syntax classes if non-empty.
//
// single-line fragments (the common case) are emitted directly;
// multi-line fragments are delegated to writeMultiLineSpan.
func writeSpan(buf *strings.Builder, fragment []byte, baseFileOffset int, baseCovClass, synClass string, blocks []*profileBlock) {
	if bytes.IndexByte(fragment, '\n') < 0 { // fast path for the overwhelming majority of cases
		emitSpan(buf, fragment, baseFileOffset, baseCovClass, synClass, blocks)
		return
	}
	writeMultiLineSpan(buf, fragment, baseFileOffset, baseCovClass, synClass, blocks) // rare since statements are usually separated by whitespace
}

// writeMultiLineSpan splits a multi-line fragment across newlines and
// resolves absolute sub-line boundaries for coverage classification.
func writeMultiLineSpan(buf *strings.Builder, fragment []byte, baseFileOffset int, baseCovClass, synClass string, blocks []*profileBlock) {
	newline        := []byte("\n")
	lines          := bytes.SplitAfter(fragment, newline)
	relativeOffset := 0

	for _, line := range lines {
		if len(line) == 0 { continue }
		hasNewline := bytes.HasSuffix(line, newline)
		content    := line
		if hasNewline {
			content = line[:len(line) - 1]
		}
		if len(content) > 0 {
			emitSpan(buf, content, baseFileOffset + relativeOffset, baseCovClass, synClass, blocks)
		}
		if hasNewline {
			buf.WriteByte('\n')
		}
		relativeOffset += len(line)
	}
}

// emitSpan writes fragment to buf, wrapping it in a <span> element with
// the resolved coverage and syntax classes if non-empty.
func emitSpan(buf *strings.Builder, fragment []byte, absStart int, baseCovClass, synClass string, blocks []*profileBlock) {
	covClass := baseCovClass
	if covClass == "" {
		covClass = coverClass(blocks, absStart, absStart + len(fragment))
	}

	classStr := ""
	switch {
	case covClass != "" && synClass != "":
		classStr = covClass + " " + synClass
	case covClass != "":
		classStr = covClass
	case synClass != "":
		classStr = synClass
	}

	if classStr != "" {
		buf.WriteString(`<span class="`)
		buf.WriteString(classStr)
		buf.WriteString(`">`)
	}

	writeEscaped(buf, fragment)

	if classStr != "" {
		buf.WriteString("</span>")
	}
}

// writeEscaped writes s to buf, HTML-escaping if s contains escapable chars.
func writeEscaped(buf *strings.Builder, s []byte) {
	for i := range len(s) {
		switch s[i] {
		case '<', '>', '&', '"', '\'':
			template.HTMLEscape(buf, s)
			return
		}
	}
	buf.Write(s)
}

// computeLineOffsets returns the byte offset of each line start in src.
func computeLineOffsets(src []byte) (offsets []int) {
	offsets = make([]int, 0, 256)
	offsets = append(offsets, 0)
	for i, ch := range src {
		if ch == '\n' {
			offsets = append(offsets, i + 1)
		}
	}
	return
}

// computeBlockOffsets converts cover.ProfileBlock line/column positions to byte offsets.
//
// The first block on a given line starts at the line's beginning rather than at its
// StartCol, since the `go tool cover` reports the column of the first token on the line.
func computeBlockOffsets(file *token.File, blocks []cover.ProfileBlock) (out []*profileBlock) {
	lineStarted := make(map[int]bool)
	out          = make([]*profileBlock, 0, len(blocks))

	for _, block := range blocks {
		startOffset := 0
		if !lineStarted[block.StartLine] {
			startOffset = file.Offset(file.LineStart(block.StartLine))
			lineStarted[block.StartLine] = true
		} else {
			startOffset = file.Offset(file.LineStart(block.StartLine)) + (block.StartCol - 1)
		}

		endOffset := file.Offset(file.LineStart(block.EndLine)) + (block.EndCol - 1)
		out        = append(out, &profileBlock{
			startOffset: startOffset,
			endOffset:   endOffset,
			count:       block.Count,
		})
	}

	return
}

// computeFuncSpans returns one funcSpan per top-level function in the file.
func computeFuncSpans(f *ast.File, file *token.File) (out []funcSpan) {
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok { return true }

		out = append(out, funcSpan{
			name:        fn.Name.Name,
			startOffset: file.Offset(fn.Pos()),
			endOffset:   file.Offset(fn.End()),
		})
		return true
	})

	return
}

// coverClass performs an O(log n) lookup over the sorted blocks
// slice to determine the coverage state ("hit", "miss", or "")
// of a specified byte offset range [start, end].
//
// precondition: blocks must be sorted in ascending order by endOffset.
func coverClass(blocks []*profileBlock, start, end int) string {
	idx := sort.Search(len(blocks), func(i int) bool {
		return blocks[i].endOffset >= end
	})
	if idx   <  len(blocks)             &&
	   start >= blocks[idx].startOffset &&
	   end   <= blocks[idx].endOffset   {
		if blocks[idx].count > 0 {
			return "hit"
		}
		return "miss"
	}
	return ""
}

// finalizeLines splits the rendered buffer into per-line entries
// and attaches function-span metadata.
func finalizeLines(raw string, lineOffsets []int, funcSpans []funcSpan) (lines []annotatedLine) {
	annotated := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	lines      = make([]annotatedLine, len(annotated))

	for i, text := range annotated {
		lines[i].text      = text
		lines[i].funcIdx   = funcIndexForOffset(funcSpans, lineOffsets[i])
		lines[i].isFuncEnd = lines[i].funcIdx   >= 0 &&
		                     lineOffsets[i + 1] >= funcSpans[lines[i].funcIdx].endOffset
	}
	return
}

// splitImportSpans splits an import path string literal into prefix,
// base (filename), and suffix sub-spans for syntax highlighting.
func splitImportSpans(src []byte, startOffset, endOffset int, lit string) []srcSpan {
	if idx := strings.LastIndex(lit, "/"); idx > 0 {
		lastSlash := startOffset + idx
		return []srcSpan{
			{src[startOffset   : lastSlash + 1], startOffset,   "string"},
			{src[lastSlash + 1 : endOffset - 1], lastSlash + 1, "base"  },
			{src[endOffset - 1 : endOffset    ], endOffset - 1, "string"},
		}
	}
	return nil
}

// funcIndexForOffset returns the index of the funcSpan containing offset, or -1 if none.
func funcIndexForOffset(funcSpans []funcSpan, offset int) int {
	for i, f := range funcSpans {
		if offset >= f.startOffset &&
		   offset <  f.endOffset   {
			return i
		}
	}
	return -1
}
