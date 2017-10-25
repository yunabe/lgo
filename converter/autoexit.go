// This file defines injectAutoExitToFile, which injects core.ExitIfCtxDone to interrupt lgo code
//
// Basic rules:
// - Injects ExitIfCtxDone between two heavy statements (== function calls).
// - Does not inject ExitIfCtxDone in functions under defer statements.
// - Injects ExitIfCtxDone at the top of a function.
// - Injects ExitIfCtxDone at the top of a for-loop body.

package converter

import (
	"go/ast"

	"github.com/yunabe/lgo/core"
)

func containsCall(expr ast.Expr) bool {
	var v containsCallVisitor
	ast.Walk(&v, expr)
	return v.contains
}

type containsCallVisitor struct {
	contains bool
}

func (v *containsCallVisitor) Visit(node ast.Node) ast.Visitor {
	if _, ok := node.(*ast.CallExpr); ok {
		v.contains = true
	}
	if _, ok := node.(*ast.FuncLit); ok {
		return nil
	}
	if v.contains {
		return nil
	}
	return v
}

func isHeavyStmt(stm ast.Stmt) bool {
	switch stm := stm.(type) {
	case *ast.AssignStmt:
		for _, e := range stm.Lhs {
			if containsCall(e) {
				return true
			}
		}
		for _, e := range stm.Rhs {
			if containsCall(e) {
				return true
			}
		}
	case *ast.ExprStmt:
		return containsCall(stm.X)
	case *ast.ForStmt:
		return isHeavyStmt(stm.Init)
	case *ast.GoStmt:
		if containsCall(stm.Call.Fun) {
			return true
		}
		for _, arg := range stm.Call.Args {
			if containsCall(arg) {
				return true
			}
		}
	case *ast.IfStmt:
		if isHeavyStmt(stm.Init) {
			return true
		}
		if containsCall(stm.Cond) {
			return true
		}
	case *ast.ReturnStmt:
		for _, r := range stm.Results {
			if containsCall(r) {
				return true
			}
		}
	case *ast.SwitchStmt:
		if isHeavyStmt(stm.Init) {
			return true
		}
		if containsCall(stm.Tag) {
			return true
		}
		// Return true if one of case clause contains a function call.
		for _, l := range stm.Body.List {
			cas := l.(*ast.CaseClause)
			for _, e := range cas.List {
				if containsCall(e) {
					return true
				}
			}
		}
	}
	return false
}

func injectAutoExitBlock(block *ast.BlockStmt, injectHead bool, defaultFlag bool, corePkg string) {
	injectAutoExitToBlockStmtList(&block.List, injectHead, defaultFlag, corePkg)
}

func injectAutoExitToBlockStmtList(lst *[]ast.Stmt, injectHead bool, defaultFlag bool, corePkg string) {
	newList := make([]ast.Stmt, 0, 2*len(*lst)+1)
	flag := defaultFlag
	appendAutoExpt := func() {
		newList = append(newList, &ast.ExprStmt{
			X: &ast.CallExpr{
				Fun: ast.NewIdent(corePkg + ".ExitIfCtxDone"),
			},
		})
	}
	if injectHead {
		appendAutoExpt()
		flag = false
	}
	for i := 0; i < len(*lst); i++ {
		stmt := (*lst)[i]
		heavy := injectAutoExitToStmt(stmt, corePkg, flag)
		if heavy {
			if flag {
				appendAutoExpt()
			}
			flag = true
		}
		newList = append(newList, stmt)
	}
	*lst = newList
}

func injectAutoExitToStmt(stm ast.Stmt, corePkg string, prevHeavy bool) bool {
	heavy := isHeavyStmt(stm)
	switch stm := stm.(type) {
	case *ast.ForStmt:
		injectAutoExit(stm.Init, corePkg)
		injectAutoExit(stm.Cond, corePkg)
		injectAutoExit(stm.Post, corePkg)
		injectAutoExitBlock(stm.Body, true, heavy, corePkg)
	case *ast.IfStmt:
		injectAutoExit(stm.Init, corePkg)
		injectAutoExit(stm.Cond, corePkg)
		injectAutoExitBlock(stm.Body, false, heavy, corePkg)
	case *ast.SwitchStmt:
		injectAutoExit(stm.Init, corePkg)
		injectAutoExit(stm.Tag, corePkg)
		for _, l := range stm.Body.List {
			cas := l.(*ast.CaseClause)
			injectAutoExitToBlockStmtList(&cas.Body, false, heavy, corePkg)
		}
	case *ast.BlockStmt:
		injectAutoExitBlock(stm, true, prevHeavy, corePkg)
	case *ast.DeferStmt:
		// Do not inject autoexit code inside a function defined in a defer statement.
	default:
		injectAutoExit(stm, corePkg)
	}
	return heavy
}

func injectAutoExit(node ast.Node, corePkg string) {
	if node == nil {
		return
	}
	v := autoExitInjector{corePkg: corePkg}
	ast.Walk(&v, node)
}

type autoExitInjector struct {
	corePkg string
}

func (v *autoExitInjector) Visit(node ast.Node) ast.Visitor {
	if decl, ok := node.(*ast.FuncDecl); ok {
		injectAutoExitBlock(decl.Body, true, false, v.corePkg)
		return nil
	}
	if fn, ok := node.(*ast.FuncLit); ok {
		injectAutoExitBlock(fn.Body, true, false, v.corePkg)
		return nil
	}
	return v
}

func injectAutoExitToFile(file *ast.File, immg *importManager) {
	corePkg, _ := defaultImporter.Import(core.SelfPkgPath)
	coreName := immg.shortName(corePkg)
	injectAutoExit(file, coreName)
}
