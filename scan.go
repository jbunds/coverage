package main

import (
	"bytes"
	"go/scanner"
	"go/token"
	"sort"
	"text/template"

	"golang.org/x/tools/cover"
)

// profileBlock maps a coverage profile range to its absolute byte offsets in the source file.
type profileBlock struct {
	startOffset, endOffset, count int
}

// scanAndAnnotate returns an HTML-annotated version of src, grouping consecutive
// tokens that share the same coverage class into a single span fragment.
func scanAndAnnotate(file *token.File, src []byte, blocks []*profileBlock) *bytes.Buffer {
	buf  := new(bytes.Buffer)
	scnr := new(scanner.Scanner)
	scnr.Init(file, src, nil, scanner.ScanComments)

	var (
		pendingBaseOffset int
		pendingClass      string
		pendingBuf        bytes.Buffer
		lastOffset        int
	)

	flush := func() {
		if pendingBuf.Len() > 0 {
			writeTokenFragment(buf, pendingBuf.Bytes(), pendingBaseOffset, pendingClass, blocks)
			pendingBuf.Reset()
		}
	}

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
		endOffset := tokenEndOffset(src, startOffset, lit, tok)

		if lit == "" && endOffset == startOffset {
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
	return buf
}

// writeTokenFragment splits fragments across newlines and resolves absolute sub-line boundaries.
func writeTokenFragment(buf *bytes.Buffer, fragment []byte, baseFileOffset int, baseClass string, blocks []*profileBlock) {
	lines          := bytes.SplitAfter(fragment, []byte("\n"))
	relativeOffset := 0

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		hasNewline := bytes.HasSuffix(line, []byte("\n"))
		content    := line
		if hasNewline {
			content = line[:len(line) - 1]
		}

		if len(content) > 0 {
			class := baseClass
			if class == "" { // calculate the absolute position in the file for this whitespace line chunk
				absStart := baseFileOffset + relativeOffset
				absEnd   := absStart + len(content)
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

// tokenEndOffset returns the byte offset immediately past the end of the token.
//
// In practice, the scanner always returns a non-empty literal for all token types,
// so this is simply startOffset + len(lit).
//
// The manual comment-scanning branches below are defensive and should never execute.
func tokenEndOffset(src []byte, startOffset int, lit string, tok token.Token) int {
	// startOffset + len(lit) // stable behavior since Go 1.0
	if lit != "" || tok != token.COMMENT {
		return startOffset + len(lit)
	}
	// line comment: extend to (but not including) newline
	if startOffset + 2 <= len(src) &&
	   src[startOffset    ] == '/' &&
	   src[startOffset + 1] == '/' {
		end := startOffset
		for end < len(src) && src[end] != '\n' { end++ }
		return end
	}
	// block comment: extend to closing */
	end := startOffset
	for end < len(src) - 1 {
		if src[end  ] == '*' &&
		   src[end+1] == '/' { return end + 2 }
		end++
	}
	return len(src)
}
