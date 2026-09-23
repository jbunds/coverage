// go run loctrace.go ./your/project
package main

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type function struct {
	name,
	kind,
	filename   string
	startLine,
	loc        int
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	var funcs []*function

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
			return nil
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			kind := "func"
			name := fn.Name.Name
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				kind = "method"
				// extract receiver type
				switch t := fn.Recv.List[0].Type.(type) {
				case *ast.Ident:
					name = t.Name + "." + name
				case *ast.StarExpr:
					if id, ok := t.X.(*ast.Ident); ok {
						name = id.Name + "." + name
					}
				}
			}
			startLine := fset.Position(fn.Pos()).Line
			endLine   := fset.Position(fn.End()).Line
			loc       := endLine - startLine + 1
			function  := &function{
				name:      name,
				kind:      kind,
				loc:       loc,
				filename:  filepath.Base(path),
				startLine: startLine,
			}
			funcs = append(funcs, function)
		}
		return nil
	})

	slices.SortFunc(funcs, func(a, b *function) int { return cmp.Compare(b.loc, a.loc)})

	for _, fn := range funcs {
		fmt.Printf("%-40s %-6s %4d lines  %s:%d\n", fn.name, fn.kind, fn.loc, fn.filename, fn.startLine)
	}
}
