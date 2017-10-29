// This file defines rewriteExpr

package converter

import (
	"go/ast"
)

// rewriteExpr rewrites expressions under node.
func rewriteExpr(node ast.Node, rewrite func(ast.Expr) ast.Expr) {
	ast.Walk(rewiteVisitor{rewrite}, node)
}

type rewiteVisitor struct {
	rewrite func(ast.Expr) ast.Expr
}

func rewriteExprList(rewrite func(ast.Expr) ast.Expr, list []ast.Expr) {
	for i, x := range list {
		list[i] = rewrite(x)
	}
}

func (v rewiteVisitor) Visit(node ast.Node) (w ast.Visitor) {
	switch n := node.(type) {
	case *ast.Field:
		n.Type = v.rewrite(n.Type)

	case *ast.Ellipsis:
		if n.Elt != nil {
			n.Elt = v.rewrite(n.Elt)
		}

	case *ast.CompositeLit:
		if n.Type != nil {
			n.Type = v.rewrite(n.Type)
		}
		rewriteExprList(v.rewrite, n.Elts)

	case *ast.ParenExpr:
		n.X = v.rewrite(n.X)

	case *ast.SelectorExpr:
		n.X = v.rewrite(n.X)

	case *ast.IndexExpr:
		n.X = v.rewrite(n.X)
		n.Index = v.rewrite(n.Index)

	case *ast.SliceExpr:
		n.X = v.rewrite(n.X)
		if n.Low != nil {
			n.Low = v.rewrite(n.Low)
		}
		if n.High != nil {
			n.High = v.rewrite(n.High)
		}
		if n.Max != nil {
			n.Max = v.rewrite(n.Max)
		}

	case *ast.TypeAssertExpr:
		n.X = v.rewrite(n.X)
		if n.Type != nil {
			n.Type = v.rewrite(n.Type)
		}

	case *ast.CallExpr:
		n.Fun = v.rewrite(n.Fun)
		rewriteExprList(v.rewrite, n.Args)

	case *ast.StarExpr:
		n.X = v.rewrite(n.X)

	case *ast.UnaryExpr:
		n.X = v.rewrite(n.X)

	case *ast.BinaryExpr:
		n.X = v.rewrite(n.X)
		n.Y = v.rewrite(n.Y)

	case *ast.KeyValueExpr:
		n.Key = v.rewrite(n.Key)
		n.Value = v.rewrite(n.Value)

	// Types
	case *ast.ArrayType:
		if n.Len != nil {
			n.Len = v.rewrite(n.Len)
		}
		n.Elt = v.rewrite(n.Elt)

	case *ast.MapType:
		n.Key = v.rewrite(n.Key)
		n.Value = v.rewrite(n.Value)

	case *ast.ChanType:
		n.Value = v.rewrite(n.Value)

	case *ast.ExprStmt:
		n.X = v.rewrite(n.X)

	case *ast.SendStmt:
		n.Chan = v.rewrite(n.Chan)
		n.Value = v.rewrite(n.Value)

	case *ast.IncDecStmt:
		n.X = v.rewrite(n.X)

	case *ast.AssignStmt:
		rewriteExprList(v.rewrite, n.Lhs)
		rewriteExprList(v.rewrite, n.Rhs)

	case *ast.ReturnStmt:
		rewriteExprList(v.rewrite, n.Results)

	case *ast.IfStmt:
		n.Cond = v.rewrite(n.Cond)

	case *ast.CaseClause:
		rewriteExprList(v.rewrite, n.List)

	case *ast.SwitchStmt:
		if n.Tag != nil {
			n.Tag = v.rewrite(n.Tag)
		}

	case *ast.ForStmt:
		if n.Cond != nil {
			n.Cond = v.rewrite(n.Cond)
		}

	case *ast.RangeStmt:
		if n.Key != nil {
			n.Key = v.rewrite(n.Key)
		}
		if n.Value != nil {
			n.Value = v.rewrite(n.Value)
		}
		n.X = v.rewrite(n.X)

	case *ast.ValueSpec:
		if n.Type != nil {
			n.Type = v.rewrite(n.Type)
		}
		rewriteExprList(v.rewrite, n.Values)

	case *ast.TypeSpec:
		n.Type = v.rewrite(n.Type)
	}
	return v
}
