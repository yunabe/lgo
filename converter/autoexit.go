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
	"go/token"

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

func injectAutoExitBlock(block *ast.BlockStmt, injectHead bool, defaultFlag bool, importCore func() string) {
	injectAutoExitToBlockStmtList(&block.List, injectHead, defaultFlag, importCore)
}

func makeExitIfDoneCommClause(importCore func() string) *ast.CommClause {
	ctx := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: importCore()},
			Sel: &ast.Ident{Name: "GetExecContext"},
		},
	}
	done := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ctx,
			Sel: &ast.Ident{Name: "Done"},
		},
	}
	return &ast.CommClause{
		Comm: &ast.ExprStmt{X: &ast.UnaryExpr{Op: token.ARROW, X: done}},
		Body: []ast.Stmt{&ast.ExprStmt{
			X: &ast.CallExpr{
				Fun: ast.NewIdent("panic"),
				Args: []ast.Expr{&ast.SelectorExpr{
					X:   &ast.Ident{Name: importCore()},
					Sel: &ast.Ident{Name: "Bailout"},
				}},
			},
		}},
	}
}

func injectAutoExitToBlockStmtList(lst *[]ast.Stmt, injectHead bool, defaultFlag bool, importCore func() string) {
	newList := make([]ast.Stmt, 0, 2*len(*lst)+1)
	flag := defaultFlag
	appendAutoExpt := func() {
		newList = append(newList, &ast.ExprStmt{
			X: &ast.CallExpr{
				Fun: ast.NewIdent(importCore() + ".ExitIfCtxDone"),
			},
		})
	}
	if injectHead {
		appendAutoExpt()
		flag = false
	}
	for i := 0; i < len(*lst); i++ {
		stmt := (*lst)[i]
		if stmt, ok := stmt.(*ast.SendStmt); ok {
			newList = append(newList, &ast.SelectStmt{
				Body: &ast.BlockStmt{
					List: []ast.Stmt{
						&ast.CommClause{Comm: stmt},
						makeExitIfDoneCommClause(importCore),
					},
				},
			})
			flag = false
			continue
		}

		heavy := injectAutoExitToStmt(stmt, importCore, flag)
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

func injectAutoExitToStmt(stm ast.Stmt, importCore func() string, prevHeavy bool) bool {
	heavy := isHeavyStmt(stm)
	switch stm := stm.(type) {
	case *ast.ForStmt:
		injectAutoExit(stm.Init, importCore)
		injectAutoExit(stm.Cond, importCore)
		injectAutoExit(stm.Post, importCore)
		injectAutoExitBlock(stm.Body, true, heavy, importCore)
	case *ast.IfStmt:
		injectAutoExit(stm.Init, importCore)
		injectAutoExit(stm.Cond, importCore)
		injectAutoExitBlock(stm.Body, false, heavy, importCore)
	case *ast.SwitchStmt:
		injectAutoExit(stm.Init, importCore)
		injectAutoExit(stm.Tag, importCore)
		for _, l := range stm.Body.List {
			cas := l.(*ast.CaseClause)
			injectAutoExitToBlockStmtList(&cas.Body, false, heavy, importCore)
		}
	case *ast.BlockStmt:
		injectAutoExitBlock(stm, true, prevHeavy, importCore)
	case *ast.DeferStmt:
		// Do not inject autoexit code inside a function defined in a defer statement.
	default:
		injectAutoExit(stm, importCore)
	}
	return heavy
}

func injectAutoExitToSelectStmt(sel *ast.SelectStmt, importCore func() string) {
	for _, stmt := range sel.Body.List {
		stmt := stmt.(*ast.CommClause)
		if stmt.Comm == nil {
			// stmt is default case. Do nothing if there is default-case.
			return
		}
	}
	sel.Body.List = append(sel.Body.List, makeExitIfDoneCommClause(importCore))
}

func injectAutoExit(node ast.Node, importCore func() string) {
	if node == nil {
		return
	}
	v := autoExitInjector{importCore: importCore}
	ast.Walk(&v, node)
}

type autoExitInjector struct {
	importCore func() string
}

func (v *autoExitInjector) Visit(node ast.Node) ast.Visitor {
	if decl, ok := node.(*ast.FuncDecl); ok {
		injectAutoExitBlock(decl.Body, true, false, v.importCore)
		return nil
	}
	if fn, ok := node.(*ast.FuncLit); ok {
		injectAutoExitBlock(fn.Body, true, false, v.importCore)
		return nil
	}
	if node, ok := node.(*ast.SelectStmt); ok {
		injectAutoExitToSelectStmt(node, v.importCore)
	}
	return v
}

func injectAutoExitToFile(file *ast.File, immg *importManager) {
	importCore := func() string {
		corePkg, _ := defaultImporter.Import(core.SelfPkgPath)
		return immg.shortName(corePkg)
	}
	injectAutoExit(file, importCore)
}
