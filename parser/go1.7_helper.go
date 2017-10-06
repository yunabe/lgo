// +build go1.7,!go1.9

package parser

import (
	"go/ast"
)

func (p *parser) mayAcceptTypeAliasAssign(spec *ast.TypeSpec) {
	// Do nothing in go1.7 and go1.8, which do not support type alias.
}
