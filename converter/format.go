package converter

import (
	"bytes"
	"github.com/yunabe/lgo/go/go/printer"
	"go/ast"
	"go/token"
	"strings"
)

// Format formats lgo src code like go fmt command.
func Format(src string) (string, error) {
	fset, blk, err := parseLesserGoString(src)
	if err != nil {
		return "", err
	}
	var imports []ast.Decl
	for _, s := range blk.Stmts {
		decl, ok := s.(*ast.DeclStmt)
		if !ok {
			continue
		}
		if gen, _ := decl.Decl.(*ast.GenDecl); gen != nil && gen.Tok == token.IMPORT {
			imports = append(imports, decl.Decl)
		}
	}
	ast.SortImports(fset, &ast.File{
		Decls: imports,
	})

	cn := &printer.CommentedNode{
		Comments: blk.Comments,
		Node:     printer.LGOStmtList(blk.Stmts),
	}
	config := printer.Config{
		Mode:     printer.UseSpaces,
		Tabwidth: 4,
	}
	var buf bytes.Buffer
	if err := config.Fprint(&buf, fset, cn); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}
