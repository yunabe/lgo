// +build go1.9

package types

import (
	"go/ast"
	"go/token"
)

func getTypeSpecAssign(spec *ast.TypeSpec) token.Pos {
	return spec.Assign
}
