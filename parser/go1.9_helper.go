// +build go1.9

package parser

import (
	"go/ast"
	"go/token"
)

func (p *parser) mayAcceptTypeAliasAssign(spec *ast.TypeSpec) {
	if p.tok == token.ASSIGN {
		spec.Assign = p.pos
		p.next()
	}
}
