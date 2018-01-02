// This file defines injectAutoExitToFile, which injects core.ExitIfCtxDone to interrupt lgo code
//
// Basic rules:
// - Injects ExitIfCtxDone between two heavy statements (== function calls).
// - Does not inject ExitIfCtxDone in functions under defer statements.
// - Injects ExitIfCtxDone at the top of a function.
// - Injects ExitIfCtxDone at the top of a for-loop body.

package converter

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/yunabe/lgo/core"
	"github.com/yunabe/lgo/parser"
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
	if node == nil {
		return nil
	}
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

// selectCommExprRecorder collects `<-ch` expressions that are used inside select-case clauses.
// The result of this recorder is used from mayWrapRecvOp so that it does not modify `<-ch` operations in select-case clauses.
type selectCommExprRecorder struct {
	m map[ast.Expr]bool
}

func (v *selectCommExprRecorder) Visit(node ast.Node) ast.Visitor {
	sel, ok := node.(*ast.SelectStmt)
	if !ok {
		return v
	}
	for _, cas := range sel.Body.List {
		comm := cas.(*ast.CommClause).Comm
		if comm == nil {
			continue
		}
		if assign, ok := comm.(*ast.AssignStmt); ok {
			for _, expr := range assign.Rhs {
				v.m[expr] = true
			}
		}
		if expr, ok := comm.(*ast.ExprStmt); ok {
			v.m[expr.X] = true
		}
	}
	return v
}

func mayWrapRecvOp(conf *Config, file *ast.File, fset *token.FileSet, checker *types.Checker, pkg *types.Package, runctx types.Object, oldImports []*types.PkgName) (*types.Checker, *types.Package, types.Object, []*types.PkgName) {
	rec := selectCommExprRecorder{make(map[ast.Expr]bool)}
	ast.Walk(&rec, file)
	picker := newNamePicker(checker.Defs)
	immg := newImportManager(pkg, file, checker)
	importCore := func() string {
		corePkg, _ := defaultImporter.Import(core.SelfPkgPath)
		return immg.shortName(corePkg)
	}

	var rewritten bool
	rewriteExpr(file, func(expr ast.Expr) ast.Expr {
		if rec.m[expr] {
			return expr
		}
		ue, ok := expr.(*ast.UnaryExpr)
		if !ok || ue.Op != token.ARROW {
			return expr
		}
		rewritten = true
		wrapperName := picker.NewName("recvChan")
		decl := makeChanRecvWrapper(ue, wrapperName, immg, importCore, checker)
		file.Decls = append(file.Decls, decl)
		return &ast.CallExpr{
			Fun:  ast.NewIdent(wrapperName),
			Args: []ast.Expr{ue.X},
		}
	})
	if !rewritten {
		return checker, pkg, runctx, oldImports
	}
	if len(immg.injectedImports) > 0 {
		var newDecls []ast.Decl
		for _, im := range immg.injectedImports {
			newDecls = append(newDecls, im)
		}
		newDecls = append(newDecls, file.Decls...)
		file.Decls = newDecls
	}
	var err error
	checker, pkg, runctx, oldImports, err = checkFileInPhase2(conf, file, fset)
	if err != nil {
		panic(fmt.Errorf("No error expected but got: %v", err))
	}
	return checker, pkg, runctx, oldImports
}

// makeChanRecvWrapper makes ast.Node trees of a function declaration like
// func recvChan1(c chan int) (x int, ok bool) {
//    select {
//    case x, ok = <-c:
//        return
//    }
// }
// The select statement is equivalent to <-c in this phase. The code to interrupt the select statement
// is injected in the latter phase by autoExitInjector.
func makeChanRecvWrapper(expr *ast.UnaryExpr, funcName string, immg *importManager, importCore func() string, checker *types.Checker) *ast.FuncDecl {
	typeExpr := func(typ types.Type) ast.Expr {
		s := types.TypeString(typ, func(pkg *types.Package) string {
			return immg.shortName(pkg)
		})
		expr, err := parser.ParseExpr(s)
		if err != nil {
			panic(fmt.Sprintf("Failed to parse type expr %q: %v", s, err))
		}
		return expr
	}

	var results []*ast.Field
	typ := checker.Types[expr].Type
	if tup, ok := typ.(*types.Tuple); ok {
		results = []*ast.Field{
			{
				Names: []*ast.Ident{ast.NewIdent("x")},
				Type:  typeExpr(tup.At(0).Type()),
			}, {
				Names: []*ast.Ident{ast.NewIdent("ok")},
				Type:  typeExpr(tup.At(1).Type()),
			},
		}
	} else {
		results = []*ast.Field{
			{
				Names: []*ast.Ident{ast.NewIdent("x")},
				Type:  typeExpr(typ),
			},
		}
	}
	chType := checker.Types[expr.X].Type.(*types.Chan)
	var lhs []ast.Expr
	if len(results) == 1 {
		lhs = []ast.Expr{ast.NewIdent("x")}
	} else {
		lhs = []ast.Expr{ast.NewIdent("x"), ast.NewIdent("ok")}
	}
	return &ast.FuncDecl{
		Name: ast.NewIdent(funcName),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("c")},
						Type:  typeExpr(chType),
					},
				},
			},
			Results: &ast.FieldList{List: results},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.SelectStmt{
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.CommClause{
								Comm: &ast.AssignStmt{
									Tok: token.ASSIGN,
									Lhs: lhs,
									Rhs: []ast.Expr{&ast.UnaryExpr{
										Op: token.ARROW,
										X:  ast.NewIdent("c"),
									}},
								},
								Body: []ast.Stmt{&ast.ReturnStmt{}},
							},
						},
					},
				},
			},
		},
	}
}
