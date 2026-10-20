package main

import (
	"go/ast"
	"go/token"
	"go/types"
)

// syntaxClassifier resolves the syntactic role of each identifier in a Go source file.
//
// It holds role-specific maps populated during a single walk of the AST, allowing the
// classify method to perform a single O(1) lookup during the sequential token scan.
type syntaxClassifier struct {
	typeInfo     *types.Info           // maps expressions to their types
	builtinTypes map[string      ]bool // primitives (int, string, bool, etc) + any, error
	builtinFuncs map[string      ]bool // builtin funcs: len, make, append
	importPos    map[token.Pos   ]bool // import path string literal positions
	typePos      map[token.Pos   ]bool // type definition name positions
	funcPos      map[token.Pos   ]bool // func decl, method signature, and call positions
	fieldPos     map[token.Pos   ]bool // struct field decl and key positions
	argPos       map[token.Pos   ]bool // param and recv decl positions
	typeExprPos  map[token.Pos   ]bool // type expr positions: params, field types, type assertions
	activeParams map[types.Object]bool // maps parameter / receiver variable objects
}

// buildSyntaxClassifier walks fileAST once, populating the role-specific position
// maps used by the classify method to resolve each identifier's syntactic role.
func buildSyntaxClassifier(fileAST *ast.File, typeInfo *types.Info) (c *syntaxClassifier) {
	c = &syntaxClassifier{
		typeInfo:     typeInfo,
		importPos:    make(map[token.Pos   ]bool),
		typePos:      make(map[token.Pos   ]bool),
		funcPos:      make(map[token.Pos   ]bool),
		fieldPos:     make(map[token.Pos   ]bool),
		argPos:       make(map[token.Pos   ]bool),
		typeExprPos:  make(map[token.Pos   ]bool),
		activeParams: make(map[types.Object]bool),
		builtinTypes: map[string]bool{ // primitives + any, error
			"int":     true, "int8":      true, "int16":      true,
			"int32":   true, "int64":     true, "uint":       true,
			"uint8":   true, "uint16":    true, "uint32":     true,
			"uint64":  true, "uintptr":   true, "float32":    true,
			"float64": true, "complex64": true, "complex128": true,
			"rune":    true, "string":    true, "byte":       true,
			"bool":    true, "any":       true, "error":      true,
		},
		builtinFuncs: map[string]bool{
			"len":     true, "cap":   true, "make":    true,
			"new":     true, "panic": true, "recover": true,
			"append":  true, "copy":  true, "delete":  true,
			"close":   true, "print": true, "println": true,
			"complex": true, "real":  true, "imag":    true,
			"min":     true, "max":   true, "clear":   true,
		},
	}

	ast.Inspect(fileAST, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ImportSpec:     c.visitImportSpec(x)
		case *ast.TypeSpec:       c.visitTypeSpec(x)
		case *ast.InterfaceType:  c.visitInterfaceType(x)
		case *ast.StructType:     c.visitStructType(x)
		case *ast.ValueSpec:      c.registerTypeExpr(x.Type)
		case *ast.FuncDecl:       c.visitFuncDecl(x)
		case *ast.Ident:          c.visitIdent(x)
		case *ast.SelectorExpr:   c.visitSelectorExpr(x)
		case *ast.CallExpr:       c.visitCallExpr(x)
		case *ast.CompositeLit:   c.visitCompositeLit(x)
		case *ast.FuncLit:        c.visitFuncLit(x)
		case *ast.IndexExpr:      c.visitIndexExpr(x)
		case *ast.IndexListExpr:  c.visitIndexListExpr(x)
		case *ast.TypeAssertExpr: c.registerTypeExpr(x.Type)
		case *ast.ChanType:       c.registerTypeExpr(x.Value)
		}
		return true
	})

	return
}

// classify returns the syntax CSS class for the token at the given position.
//
// It performs a pure lookup: no AST navigation or state mutation occurs.
func (c *syntaxClassifier) classify(tok token.Token, pos token.Pos, lit string) string {
	if tok.IsOperator() {
		switch tok {
		case token.LPAREN, token.RPAREN,
		     token.LBRACK, token.RBRACK,
		     token.LBRACE, token.RBRACE,
		     token.PERIOD, token.COMMA,
		     token.COLON,  token.SEMICOLON:
			return "base"
		default:
			return "oper"
		}
	}

	switch tok {

	case token.PACKAGE, token.IMPORT, token.STRUCT,   token.INTERFACE,
	     token.TYPE,    token.FUNC,   token.CONST,    token.VAR,         token.MAP,
	     token.IF,      token.ELSE,   token.FOR,      token.RANGE,
	     token.RETURN,  token.BREAK,  token.CONTINUE, token.GOTO,
	     token.SWITCH,  token.CASE,   token.DEFAULT,  token.FALLTHROUGH,
	     token.GO,      token.SELECT, token.CHAN,     token.DEFER:
		return "kw"

	case token.STRING:
		return "string"

	// tree-sitter-go captures token.CHAR (rune_literal) as @character
	// gruvbox.nvim maps @character to the same purple as @number
	case token.CHAR, token.INT, token.FLOAT, token.IMAG:
		return "number"

	case token.COMMENT:
		return "comment"

	case token.IDENT:
		switch {
		case c.typePos[pos]:      return "type"  // type declaration names
		case c.typeExprPos[pos]:  return "type"  // type references
		case c.fieldPos[pos]:     return "field" // struct field names
		case c.funcPos[pos]:      return "call"  // function & method names & calls
		case c.argPos[pos]:       return "arg"   // function & method arguments
		case c.builtinTypes[lit]: return "type"  // primitives + error, any
		case c.builtinFuncs[lit]: return "call"  // builtin function calls
		}

	}

	return "base"
}

func (c *syntaxClassifier) visitImportSpec(x *ast.ImportSpec) {
	c.importPos[x.Path.Pos()] = true
}

func (c *syntaxClassifier) visitFuncDecl(x *ast.FuncDecl) {
	if x.Name != nil {
		c.funcPos[x.Name.NamePos] = true
	}
	c.registerFieldList(x.Recv, true)
	if x.Type != nil {
		c.registerFieldList(x.Type.Params, true)
		c.registerFieldList(x.Type.Results, false)
	}
}

func (c *syntaxClassifier) visitFuncLit(x *ast.FuncLit) {
	if x.Type != nil {
		c.registerFieldList(x.Type.Params, true)
		c.registerFieldList(x.Type.Results, false)
	}
}

func (c *syntaxClassifier) visitCompositeLit(x *ast.CompositeLit) {
	c.registerTypeExpr(x.Type)

	isStruct := false
	if tv, ok := c.typeInfo.Types[x]; ok {
		if u := tv.Type.Underlying(); u != nil {
			_, isStruct = u.(*types.Struct)
		}
	}

	if isStruct {
		for _, el := range x.Elts {
			if kve, ok := el.(*ast.KeyValueExpr); ok {
				if id, ok := kve.Key.(*ast.Ident); ok {
					c.fieldPos[id.NamePos] = true
				}
			}
		}
	}
}

func (c *syntaxClassifier) visitIndexExpr(x *ast.IndexExpr) {
	if c.isTypeExpr(x.X) {
		c.registerTypeExpr(x.X)
		c.registerTypeExpr(x.Index)
	}
}

func (c *syntaxClassifier) visitIndexListExpr(x *ast.IndexListExpr) {
	if c.isTypeExpr(x.X) {
		c.registerTypeExpr(x.X)
		for _, idx := range x.Indices {
			c.registerTypeExpr(idx)
		}
	}
}

func (c *syntaxClassifier) visitCallExpr(x *ast.CallExpr) {
	if fun, ok := x.Fun.(*ast.Ident); ok {
		if obj := c.typeInfo.ObjectOf(fun); obj != nil {
			switch obj.(type) {
			case *types.TypeName:
				c.typeExprPos[fun.NamePos] = true // type casts
				return
			case *types.Var:
				return
			case *types.Func:
				c.funcPos[fun.NamePos] = true
				return
			}
		}
		if !c.builtinFuncs[fun.Name] {
			c.funcPos[fun.NamePos] = true
		}
	}
	for _, arg := range x.Args {
		if tv, ok := c.typeInfo.Types[arg]; ok {
			if tv.IsType() {
				c.registerTypeExpr(arg)
			}
		}
	}
}

func (c *syntaxClassifier) visitIdent(x *ast.Ident) {
	if obj := c.typeInfo.ObjectOf(x); obj != nil {
		if c.activeParams[obj] {
			c.argPos[x.NamePos] = true
		}
	}
}

func (c *syntaxClassifier) visitSelectorExpr(x *ast.SelectorExpr) {
	if obj := c.typeInfo.ObjectOf(x.Sel); obj != nil {
		switch v := obj.(type) {
		case *types.Func:
			c.funcPos[x.Sel.NamePos] = true
		case *types.TypeName:
			c.typeExprPos[x.Sel.NamePos] = true
		case *types.Var:
			if v.IsField() {
				c.fieldPos[x.Sel.NamePos] = true
			} else if c.activeParams[obj] {
				c.argPos[x.Sel.NamePos] = true
			}
		}
	}
}

func (c *syntaxClassifier) visitTypeSpec(x *ast.TypeSpec) {
	if x.Name != nil {
		c.typePos[x.Name.NamePos] = true
	}
	c.registerTypeExpr(x.Type)
}

func (c *syntaxClassifier) visitStructType(x *ast.StructType) {
	if x.Fields != nil {
		for _, field := range x.Fields.List {
			for _, name := range field.Names {
				if name != nil {
					c.fieldPos[name.NamePos] = true
				}
			}
			c.registerTypeExpr(field.Type)
		}
	}
}

func (c *syntaxClassifier) visitInterfaceType(x *ast.InterfaceType) {
	if x.Methods != nil {
		for _, method := range x.Methods.List {
			for _, name := range method.Names {
				if name != nil {
					c.funcPos[name.NamePos] = true
				}
			}
			if funcType, ok := method.Type.(*ast.FuncType); ok && funcType.Params != nil {
				for _, field := range funcType.Params.List {
					for _, name := range field.Names {
						if name != nil {
							c.argPos[name.NamePos] = true
						}
					}
					c.registerTypeExpr(field.Type)
				}
				if funcType.Results != nil {
					for _, field := range funcType.Results.List {
						c.registerTypeExpr(field.Type)
					}
				}
			}
		}
	}
}

// registerTypeExpr recursively indexes all type-reference
// positions within a type expression into typeExprPos.
func (c *syntaxClassifier) registerTypeExpr(expr ast.Expr) {
	switch t := expr.(type) {
	case *ast.Ellipsis:
		c.registerTypeExpr(t.Elt)
	case *ast.Ident:
		c.typeExprPos[t.NamePos] = true
	case *ast.SelectorExpr:
		if t.Sel != nil {
			c.typeExprPos[t.Sel.NamePos] = true
		}
	case *ast.StarExpr:
		c.registerTypeExpr(t.X)
	case *ast.ParenExpr:
		c.registerTypeExpr(t.X)
	case *ast.ArrayType:
		c.registerTypeExpr(t.Elt)
	case *ast.MapType:
		c.registerTypeExpr(t.Key)
		c.registerTypeExpr(t.Value)
	case *ast.ChanType:
		c.registerTypeExpr(t.Value)
	case *ast.IndexExpr:
		c.registerTypeExpr(t.X)
		c.registerTypeExpr(t.Index)
	case *ast.IndexListExpr:
		c.registerTypeExpr(t.X)
		for _, idx := range t.Indices {
			c.registerTypeExpr(idx)
		}
	case *ast.FuncType:
		if t.Params != nil {
			for _, field := range t.Params.List {
				c.registerTypeExpr(field.Type)
			}
		}
		if t.Results != nil {
			for _, field := range t.Results.List {
				c.registerTypeExpr(field.Type)
			}
		}
	}
}

// registerFieldList indexes parameter/receiver names
// and type expressions from a FieldList.
//
// If trackActive is true, declared variables are also recorded
// in activeParams for classifying variable references.
func (c *syntaxClassifier) registerFieldList(fieldList *ast.FieldList, trackActive bool) {
	if fieldList == nil { return }
	for _, field := range fieldList.List {
		for _, name := range field.Names {
			c.argPos[name.NamePos] = true
			if trackActive {
				if obj := c.typeInfo.Defs[name]; obj != nil {
					c.activeParams[obj] = true
				}
			}
		}
		c.registerTypeExpr(field.Type)
	}
}

// isTypeExpr determines if an expression resolves to a TypeName.
func (c *syntaxClassifier) isTypeExpr(expr ast.Expr) bool {
	for { // unwrap any parentheses
		p, ok := expr.(*ast.ParenExpr)
		if !ok { break }
		expr = p.X
	}

	switch t := expr.(type) {
	case *ast.Ident:
		if obj := c.typeInfo.Uses[t]; obj != nil {
			_, isType := obj.(*types.TypeName)
			return isType
		}
	case *ast.SelectorExpr:
		if obj := c.typeInfo.Uses[t.Sel]; obj != nil {
			_, isType := obj.(*types.TypeName)
			return isType
		}
	}

	return false
}
