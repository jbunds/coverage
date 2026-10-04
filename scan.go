package main

import (
	"bytes"
	"go/ast"
	"go/scanner"
	"go/token"
	"sort"
	"strconv"
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

// scanAndAnnotate returns an HTML-annotated version of src, grouping consecutive
// tokens that share the same coverage class into a single span fragment.
func scanAndAnnotate(file *token.File, src []byte, blocks []*profileBlock, funcs []funcSpan) *bytes.Buffer {
	var (
		pendingBuf        bytes.Buffer
		pendingClass      string
		pendingBaseOffset int
		lastOffset        int
		funcIdx           int
		inFunc            bool
	)

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

		if !inFunc && funcIdx < len(funcs) && startOffset >= funcs[funcIdx].startOffset {
			lastOffset = writeFuncStart(buf, flush, src, blocks, startOffset, lastOffset, funcIdx)
			inFunc     = true
		}

		if inFunc && funcIdx < len(funcs) && startOffset >= funcs[funcIdx].endOffset {
			startOffset,
			lastOffset,
			funcIdx = writeFuncEnd(buf, flush, src, blocks, startOffset, endOffset, lastOffset, funcIdx)
			inFunc  = false
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
	return buf
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

// writeFuncStart emits the opening <div class="func"> wrapper
// (with hidden checkbox) for the function at funcs[funcIdx].
//
// It first truncates any gap between lastOffset and startOffset at
// the last newline, ensures a leading newline, then writes the
// source from lastOffset to startOffset, and returns lastOffset.
func writeFuncStart(buf *bytes.Buffer, flush func(), src []byte, blocks []*profileBlock, startOffset, lastOffset, funcIdx int) int {
	flush()
	if startOffset > lastOffset {
		gap := src[lastOffset:startOffset]
		if lastNL := bytes.LastIndexByte(gap, '\n'); lastNL >= 0 {
			writeTokenFragment(buf, gap[:lastNL + 1], lastOffset, "", blocks)
			lastOffset += lastNL + 1
		}
	}
	if buf.Len() > 0 && buf.Bytes()[buf.Len() - 1] != '\n' {
		buf.WriteRune('\n')
	}
	buf.WriteString(`<div class="func"><input type="checkbox" id="func-`)
	buf.WriteString(strconv.Itoa(funcIdx))
	buf.WriteString("\" checked/>\n")
	if startOffset > lastOffset {
		writeTokenFragment(buf, src[lastOffset:startOffset], lastOffset, "", blocks)
		lastOffset = startOffset
	}
	return lastOffset
}

// writeFuncEnd emits the closing </div></div> for the function at funcs[funcIdx].
//
// It consumes source up to and including the first newline at or before
// endOffset (or up to endOffset if none is found), writes the closing
// div tags for the nested div.func > div pair, and returns startOffset
// (clamped to at least lastOffset), lastOffset, and incremented funcIdx.
func writeFuncEnd(buf *bytes.Buffer, flush func(), src []byte, blocks []*profileBlock, startOffset, endOffset, lastOffset, funcIdx int) (int, int, int) {
	flush()
	nlIdx := bytes.IndexByte(src[lastOffset:], '\n')
	if nlIdx >= 0 && lastOffset + nlIdx <= endOffset {
		target := lastOffset + nlIdx
		if target > lastOffset {
			writeTokenFragment(buf, src[lastOffset:target], lastOffset, "", blocks)
			lastOffset = target
		}
		if buf.Len() > 0 && buf.Bytes()[buf.Len() - 1] != '\n' {
			buf.WriteRune('\n')
		}
		buf.WriteString("</div></div>\n")
		lastOffset++
		if startOffset < lastOffset {
			startOffset = lastOffset
		}
	} else {
		if startOffset > lastOffset {
			writeTokenFragment(buf, src[lastOffset:startOffset], lastOffset, "", blocks)
			lastOffset = startOffset
		}
		if buf.Len() > 0 && buf.Bytes()[buf.Len() - 1] != '\n' {
			buf.WriteRune('\n')
		}
		buf.WriteString("</div></div>\n")
	}
	return startOffset, lastOffset, funcIdx + 1
}
