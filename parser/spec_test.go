package parser

import (
	"go/ast"
	"testing"
)

func TestFuncSpec(t *testing.T) {
	{
		expr, err := ParseExpr(`func(int, int){}`)
		if err != nil {
			t.Error(err)
		}
		f := expr.(*ast.FuncLit)
		if len(f.Type.Params.List) != 2 {
			t.Errorf("Unexpected len(params): %d", len(f.Type.Params.List))
		}
	}
	{
		expr, err := ParseExpr(`func(int, x int){}`)
		if err != nil {
			t.Error(err)
		}
		f := expr.(*ast.FuncLit)
		if len(f.Type.Params.List) != 1 {
			t.Errorf("Unexpected len(params): %d", len(f.Type.Params.List))
		}
	}
}
