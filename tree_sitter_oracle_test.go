package main

import (
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	"golang.org/x/tools/go/packages"
)

// differential smoke test
//
// the oracle (tree-sitter-go) is purely syntactic: all identifiers
// are captured as @variable, so it cannot validate which sub-role
// the classifier assigns (call vs. field vs. arg vs. type).
//
// position and non-identifier classification are fully asserted.
//
// TODO(jbunds): add targeted table-driven cases for identifier
//               classification of constructs where the oracle is
//               structurally blind (e.g. generics, method values)

type span struct {
	start, end int
	label      string // oracle: raw capture name; subject: CSS class name
}

func sortSpans(s []span) {
	sort.Slice(s, func(a, b int) bool {
		if s[a].start != s[b].start { return s[a].start < s[b].start }
		if s[a].end   != s[b].end   { return s[a].end   < s[b].end   }
		return s[a].label < s[b].label
	})
}

// permittedClasses maps a tree-sitter-go capture to the set of CSS classes
// the classifier may emit for that span.
//
// The grammar is syntactic and coarse (e.g. all identifiers are @variable),
// while the classifier uses type info to distinguish sub-roles, so it may
// emit a more specific class than the capture implies.
func permittedClasses(capture string) []string {
	switch capture {
	case "variable":
		// classify() distinguishes these identifier sub-roles; tree-sitter-go does not
		return []string{"base", "call", "arg", "type", "field"}
	case "keyword":
		return []string{"kw"}
	case "property":
		return []string{"base", "field", "call"}
	case "type":
		return []string{"type"}
	case "operator":
		return []string{"oper"}
	case "string":
		return []string{"string", "number"} // @string also covers rune_literal in gotreesitter's query version
	case "number":
		return []string{"number"}
	case "constant.builtin":
		return []string{"base"}
	case "comment":
		return []string{"comment"}
	default:
		return nil // unmapped -> skip
	}
}

func TestClassifierMatchesTreeSitter(t *testing.T) {
	t.Parallel()

	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedSyntax   |
		      packages.NeedTypesInfo,
	}, ".")
	if err != nil { t.Fatal(err) }
	if len(pkgs) != 1 { t.Fatalf("expected 1 package, got %d", len(pkgs)) }

	pkg := pkgs[0]
	if len(pkg.Errors) > 0 { t.Fatalf("load errors: %v", pkg.Errors) }

	entry  := grammars.DetectLanguageByName("go")

	for _, file := range pkg.Syntax {
		name := pkg.Fset.File(file.Pos()).Name()
		t.Run(filepath.Base(name), func(t *testing.T) {
			t.Parallel()

			src, err := os.ReadFile(name); if err != nil { t.Fatal(err) } // #nosec G304

			h, err := gotreesitter.NewHighlighter(entry.Language(), entry.HighlightQuery)
			if err != nil { t.Fatal(err) }

			toFile     := pkg.Fset.File(file.Pos())
			classifier := buildSyntaxClassifier(file, pkg.TypesInfo)

			var (
				spans []span
				sc    scanner.Scanner
			)

			sc.Init(toFile, src, nil, scanner.ScanComments)

			for {
				pos, tok, lit := sc.Scan()
				if tok == token.EOF { break }
				cls := classifier.classify(tok, pos, lit)
				if cls == "base" && tok != token.IDENT { continue }
				if tok == token.IDENT && cls == "base" && lit == file.Name.Name { continue }
				text   := lit
				if text == "" { text = tok.String() }
				spans = append(spans, span{int(pos) - toFile.Base(), int(pos) - toFile.Base() + len(text), cls})
			}

			oracleRanges := h.Highlight(src)
			oracle       := make([]span, 0, len(oracleRanges))
			for _, r := range oracleRanges {
				if permittedClasses(r.Capture) == nil { continue }
				oracle = append(oracle, span{int(r.StartByte), int(r.EndByte), r.Capture})
			}

			sortSpans(oracle)
			sortSpans(spans)

			// tree-sitter-go splits string literals containing escape sequences into multiple nodes
			//
			// merge adjacent string spans to compensate
			merged := make([]span, 0, len(oracle))
			for _, s := range oracle {
				if s.label == "string" && len(merged) > 0 && merged[len(merged) - 1].label == "string" {
					merged[len(merged) - 1].end = s.end
					continue
				}
				merged = append(merged, s)
			}
			oracle = merged

			i, j := 0, 0
			for i < len(oracle) && j < len(spans) {
				if oracle[i].start == spans[j].start &&
				   oracle[i].end   == spans[j].end   {
					allowed := permittedClasses(oracle[i].label)
					if !slices.Contains(allowed, spans[j].label) {
						t.Errorf("span at [%d %d] class mismatch (allowed %v):\n  oracle:  %v\n  subject: %v",
							oracle[i].start, oracle[i].end, allowed, oracle[i], spans[j])
						return
					}
					i++; j++
					continue
				}

				// subject has a span the oracle doesn't (e.g., package qualifier in a type expr
				// or a similar unclassified identifier that tree-sitter silently skips)
				//
				// skip it if it's "base"; otherwise it's a genuine divergence
				if spans[j].start < oracle[i].start {
					if spans[j].label == "base" {
						j++
						continue
					}
				}

				// oracle string span covers multiple subject string spans
				// (tree-sitter-go merges multi-line import blocks into one node)
				if oracle[i].start == spans[j].start &&
				   oracle[i].end   >  spans[j].end   &&
				   oracle[i].label == "string" {
					for j < len(spans) && spans[j].start < oracle[i].end { j++ }
					i++
					continue
				}

				// position mismatch: print a small window surrounding the divergence (split / merge)
				loO := max(0,           i - 2)
				hiO := min(len(oracle), i + 3)
				loS := max(0,           j - 2)
				hiS := min(len(spans),  j + 3)
				t.Errorf("divergence at oracle[%d]=%v vs subject[%d]=%v\noracle:  %v\nsubject: %v",
					i, oracle[i], j, spans[j], oracle[loO:hiO], spans[loS:hiS])
				return
			}

			// one exhausted before the other
			if i < len(oracle) {
				t.Errorf("oracle has %d extra span(s) starting at: %v", len(oracle) - i, oracle[i:min(len(oracle), i + 3)])
			} else if j < len(spans) {
				t.Errorf("subject has %d extra span(s) starting at: %v", len(spans) - j, spans[ j:min(len(spans ), j + 3)])
			}
		})
	}
}
