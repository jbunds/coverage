package main

import (
	"bytes"
	"go/ast"
	"go/scanner"
	"go/token"
	"sort"
	"strings"
	"text/template"

	"golang.org/x/tools/cover"
)

// profileBlock maps a coverage profile range to its absolute byte offsets in the source file.
type profileBlock struct {
	startOffset, endOffset, count int
}

// funcSpan identifies a top-level function in a source file by name and byte offsets.
type funcSpan struct {
	name                   string
	startOffset, endOffset int
}

// annotatedLine is a coverage-annotated single line of source code with func metadata.
type annotatedLine struct {
	text      string // annotated HTML for this line (spans, etc)
	funcIdx   int    // index into funcs, or -1 if not in a function
	isFuncEnd bool   // true on the last line of a function
}

// annotateSource scans src and returns one annotatedLine per source line.
func annotateSource(file *token.File, src []byte, blocks []*profileBlock, funcs []funcSpan) []annotatedLine {
	var (
		pendingBuf        bytes.Buffer
		pendingClass      string
		pendingBaseOffset int
		lastOffset        int
	)

	lineOffsets := make([]int, 0, 256)
	lineOffsets  = append(lineOffsets, 0)
	for i, ch := range src {
		if ch == '\n' {
			lineOffsets = append(lineOffsets, i + 1)
		}
	}

	buf   := new(bytes.Buffer)
	flush := func() {
		if pendingBuf.Len() > 0 {
			writeTokenFragment(buf, pendingBuf.Bytes(), pendingBaseOffset, pendingClass, blocks)
			pendingBuf.Reset()
		}
	}

	scnr := new(scanner.Scanner)
	scnr.Init(file, src, nil, scanner.ScanComments)

	for {
		pos, tok, lit := scnr.Scan()
		if tok == token.EOF {
			flush()
			if lastOffset < len(src) {
				writeTokenFragment(buf, src[lastOffset:], lastOffset, "", blocks)
			}
			break
		}

		startOffset := file.Offset(pos)
		endOffset   := startOffset + len(lit) // stable behavior since Go 1.0

		if lit       == ""          &&
		   endOffset == startOffset {
			continue
		}

		coverClass := coverClass(blocks, startOffset, endOffset)

		if coverClass != pendingClass {
			flush()
			pendingClass = coverClass
			if pendingClass != "" {
				pendingBaseOffset = min(startOffset, lastOffset)
			}
		}

		if pendingClass != "" {
			if startOffset > lastOffset {
				pendingBuf.Write(src[lastOffset:startOffset])
			}
			pendingBuf.Write(src[startOffset:endOffset])
		} else {
			if startOffset > lastOffset {
				writeTokenFragment(buf, src[lastOffset:startOffset], lastOffset, "", blocks)
			}
			writeTokenFragment(buf, src[startOffset:endOffset], startOffset, "", blocks)
		}

		lastOffset = endOffset
	}

	annotated := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	lines     := make([]annotatedLine, len(annotated))

	for i, text := range annotated {
		lines[i].text      = text
		lines[i].funcIdx   = funcIndexForOffset(funcs, lineOffsets[i])
		lines[i].isFuncEnd = lines[i].funcIdx   >= 0 &&
		                     lineOffsets[i + 1] >= funcs[lines[i].funcIdx].endOffset
	}

	return lines
}

// writeTokenFragment splits fragments across newlines and resolves absolute sub-line boundaries.
func writeTokenFragment(buf *bytes.Buffer, fragment []byte, baseFileOffset int, baseClass string, blocks []*profileBlock) {
	lines          := bytes.SplitAfter(fragment, []byte("\n"))
	relativeOffset := 0

	for _, line := range lines {
		if len(line) == 0 { continue }

		hasNewline := bytes.HasSuffix(line, []byte("\n"))
		content    := line
		if hasNewline {
			content = line[:len(line) - 1]
		}

		if len(content) > 0 {
			class := baseClass
			if class == "" { // calculate the absolute position in the file for this whitespace line chunk
				absStart := baseFileOffset + relativeOffset
				absEnd   := absStart       + len(content)
				class     = coverClass(blocks, absStart, absEnd)
			}

			if class != "" {
				buf.WriteString(`<span class="`)
				buf.WriteString(class)
				buf.WriteString(`">`)
				template.HTMLEscape(buf, content)
				buf.WriteString("</span>")
			} else {
				template.HTMLEscape(buf, content)
			}
		}

		if hasNewline {
			buf.WriteByte('\n')
		}

		relativeOffset += len(line)
	}
}

// coverClass performs an O(log n) lookup over the sorted blocks slice to determine the
// coverage state ("hit", "miss", or "") of a specified byte offset range [start, end].
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

// computeBlockOffsets converts cover.ProfileBlock line/column positions to byte offsets.
//
// The first block on a given line starts at the line's beginning rather than at its
// StartCol, since the `go tool cover` reports the column of the first token on the line.
func computeBlockOffsets(file *token.File, blocks []cover.ProfileBlock) (out []*profileBlock) {
	lineStarted := make(map[int]bool)
	out          = make([]*profileBlock, 0, len(blocks))

	for _, b := range blocks {
		startOffset := 0
		if !lineStarted[b.StartLine] {
			startOffset = file.Offset(file.LineStart(b.StartLine))
			lineStarted[b.StartLine] = true
		} else {
			startOffset = file.Offset(file.LineStart(b.StartLine)) + (b.StartCol - 1)
		}

		endOffset := file.Offset(file.LineStart(b.EndLine)) + (b.EndCol - 1)
		out = append(out, &profileBlock{
			startOffset: startOffset,
			endOffset:   endOffset,
			count:       b.Count,
		})
	}
	return
}

// computeFuncSpans returns one funcSpan per top-level function in the file.
func computeFuncSpans(f *ast.File, file *token.File) []funcSpan {
	var out []funcSpan
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
	return out
}

// funcIndexForOffset returns the index of the funcSpan containing offset, or -1 if none.
func funcIndexForOffset(funcs []funcSpan, offset int) int {
	for i, f := range funcs {
		if offset >= f.startOffset &&
		   offset <  f.endOffset   {
			return i
		}
	}
	return -1
}

// O(log n) implementation of funcIndexForOffset; above is O(n)
// func funcIndexForOffset(funcs []funcSpan, offset int) int {
//	i := sort.Search(len(funcs), func(j int) bool { // find the last func with startOffset <= offset
//		return funcs[j].startOffset > offset
//	}) - 1                                          // subtract 1 to get that last func
//	if i >= 0 && offset < funcs[i].endOffset {      // verify offset actually falls within that function's span
//		return i
//	}
//	return -1
//}
