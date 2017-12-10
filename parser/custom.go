package parser

import (
	"go/ast"
	"go/token"
)

type LGOBlock struct {
	Scope      *ast.Scope          // package scope (this file only)
	Imports    []*ast.ImportSpec   // imports in this file
	Unresolved []*ast.Ident        // unresolved identifiers in this file
	Comments   []*ast.CommentGroup // list of all comments in the source file
	Stmts      []ast.Stmt
}

func (p *parser) parseLgoStmtList() (list []ast.Stmt) {
	if p.trace {
		defer un(trace(p, "LgoStatementList"))
	}

	for p.tok != token.CASE && p.tok != token.DEFAULT && p.tok != token.RBRACE && p.tok != token.EOF {
		list = append(list, p.parseStmtWithFuncDeclImport(true, true))
	}

	return
}

func (p *parser) parseLesserGoSrc() *LGOBlock {
	// Extended from parseFile
	if p.trace {
		defer un(trace(p, "File"))
	}

	// Don't bother parsing the rest if we had errors scanning the first token.
	// Likely not a Go source file at all.
	if p.errors.Len() != 0 {
		return nil
	}

	p.openScope()
	// We need to open/close a label-scope to support label in the top-level scope.
	p.openLabelScope()
	p.pkgScope = p.topScope
	// rest of package body
	stmts := p.parseLgoStmtList()
	p.closeLabelScope()
	p.closeScope()
	assert(p.topScope == nil, "unbalanced scopes")
	assert(p.labelScope == nil, "unbalanced label scopes")

	// resolve global identifiers within the same file
	i := 0
	for _, ident := range p.unresolved {
		// i <= index for current ident
		assert(ident.Obj == unresolved, "object already resolved")
		ident.Obj = p.pkgScope.Lookup(ident.Name) // also removes unresolved sentinel
		if ident.Obj == nil {
			p.unresolved[i] = ident
			i++
		}
	}

	return &LGOBlock{
		// Package:    pos,
		// Name:       ident,
		// Decls:      decls,
		Stmts:      stmts,
		Scope:      p.pkgScope,
		Imports:    p.imports,
		Unresolved: p.unresolved[0:i],
		Comments:   p.comments,
	}
}

func ParseLesserGoFile(fset *token.FileSet, filename string, src interface{}, mode Mode) (f *LGOBlock, err error) {
	if fset == nil {
		panic("parser.ParseFile: no token.FileSet provided (fset == nil)")
	}

	// get source
	text, err := readSource(filename, src)
	if err != nil {
		return nil, err
	}

	var p parser
	defer func() {
		if e := recover(); e != nil {
			// resume same panic if it's not a bailout
			if _, ok := e.(bailout); !ok {
				panic(e)
			}
		}

		// set result values
		if f == nil {
			// source is not a valid Go source file - satisfy
			// ParseFile API and return a valid (but) empty
			// *ast.File
			f = &LGOBlock{
				// Name:  new(ast.Ident),
				Scope: ast.NewScope(nil),
			}
		}

		p.errors.Sort()
		err = p.errors.Err()
	}()

	// parse source
	p.init(fset, filename, text, mode)
	f = p.parseLesserGoSrc()

	return
}
