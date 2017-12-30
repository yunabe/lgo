package converter

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yunabe/lgo/parser"
)

type completeTarget interface {
	completeTarget()
}
type selectExprTarget struct {
	base       ast.Expr
	src        string
	start, end int
}
type idExprTarget struct {
	src        string
	start, end int
}

func (*selectExprTarget) completeTarget() {}
func (*idExprTarget) completeTarget()     {}

// isIdentRune returns whether a rune can be a part of an identifier.
func isIdentRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func identifierAt(src string, idx int) (start, end int) {
	if idx > len(src) || idx < 0 {
		return -1, -1
	}
	end = idx
	for {
		r, size := utf8.DecodeRuneInString(src[end:])
		if !isIdentRune(r) {
			break
		}
		end += size
	}
	start = idx
	for {
		r, size := utf8.DecodeLastRuneInString(src[:start])
		if !isIdentRune(r) {
			break
		}
		start -= size
	}
	if start == end {
		return -1, -1
	}
	if r, _ := utf8.DecodeRuneInString(src[start:]); unicode.IsDigit(r) {
		// Starts with a digit, which is not an identifier.
		return -1, -1
	}
	return
}

func findLastDot(src string, idx int) (dot, idStart, idEnd int) {
	idStart, idEnd = identifierAt(src, idx)
	var s int
	if idStart < 0 {
		s = idx
	} else {
		s = idStart
	}
	for {
		r, size := utf8.DecodeLastRuneInString(src[:s])
		if unicode.IsSpace(r) {
			s -= size
			continue
		}
		if r == '.' {
			s -= size
		}
		break
	}
	if s < len(src) && src[s] == '.' {
		if idStart < 0 {
			return s, idx, idx
		}
		return s, idStart, idEnd
	}
	return -1, -1, -1
}

// findDotBaseVisitor finds 'x' of 'x.y' or 'x.(type)' expression and stores it to base.
type findDotBaseVisitor struct {
	dotPos token.Pos
	base   ast.Expr
}

func (v *findDotBaseVisitor) Visit(n ast.Node) ast.Visitor {
	if v.base != nil || n == nil {
		return nil
	}
	if v.dotPos < n.Pos() || n.End() <= v.dotPos {
		return nil
	}
	if n, _ := n.(*ast.SelectorExpr); n != nil && n.X.End() <= v.dotPos && v.dotPos < n.Sel.Pos() {
		v.base = n.X
		return nil
	}
	if n, _ := n.(*ast.TypeAssertExpr); n != nil && n.X.End() <= v.dotPos && v.dotPos < n.Lparen {
		v.base = n.X
		return nil
	}
	return v
}

type isPosInFuncBodyVisitor struct {
	pos    token.Pos
	inBody bool
}

func (v *isPosInFuncBodyVisitor) Visit(n ast.Node) ast.Visitor {
	if n == nil || v.inBody {
		return nil
	}
	pos := v.pos
	if pos < n.Pos() || n.End() <= pos {
		// pos is out side of n.
		return nil
	}
	var body *ast.BlockStmt
	switch n := n.(type) {
	case *ast.FuncDecl:
		body = n.Body
	case *ast.FuncLit:
		body = n.Body
	}
	if body != nil && body.Pos() < pos && pos < body.End() {
		// Note: pos == n.Pos() means the cursor is right before '{'. Return false in that case.
		v.inBody = true
	}
	return v
}

// isPosInFuncBody returns whether pos is inside a function body in lgo source.
// Please call this method before any convertion on blk.
func isPosInFuncBody(blk *parser.LGOBlock, pos token.Pos) bool {
	v := isPosInFuncBodyVisitor{pos: pos}
	for _, stmt := range blk.Stmts {
		ast.Walk(&v, stmt)
		if v.inBody {
			return true
		}
	}
	return false
}

type findCompleteTargetVisitor struct {
	skip ast.Node
	pos  token.Pos
	src  string

	target completeTarget
	found  bool
}

func (v *findCompleteTargetVisitor) Visit(n ast.Node) ast.Visitor {
	if v.found || n == nil {
		return nil
	}
	if n == v.skip {
		return v
	}
	if v.pos < n.Pos() || n.End() <= v.pos {
		return nil
	}
	v.found = true
	cv := findCompleteTargetVisitor{skip: n, pos: v.pos, src: v.src}
	cv.Visit(n)
	if cv.found {
		v.target = cv.target
		return nil
	}
	if _, ok := n.(*ast.Comment); ok {
		return nil
	}
	if start, end := identifierAt(v.src, int(v.pos-1)); start != -1 {
		v.target = &idExprTarget{src: v.src, start: start, end: end}
		return nil
	}
	v.target = &idExprTarget{src: v.src, start: int(v.pos - 1), end: int(v.pos - 1)}
	return nil
}

func findNearestScope(s *types.Scope, pos token.Pos) *types.Scope {
	valid := s.Pos() != token.NoPos && s.End() != token.NoPos
	if valid && (pos < s.Pos() || s.End() <= pos) {
		return nil
	}
	for i := 0; i < s.NumChildren(); i++ {
		if c := findNearestScope(s.Child(i), pos); c != nil {
			return c
		}
	}
	if valid {
		return s
	}
	return nil
}

func completeTargetFromAST(src string, pos token.Pos, blk *parser.LGOBlock) completeTarget {
	if dot, start, end := findLastDot(src, int(pos-1)); dot >= 0 {
		var base ast.Expr
		for _, stmt := range blk.Stmts {
			v := &findDotBaseVisitor{dotPos: token.Pos(dot + 1)}
			ast.Walk(v, stmt)
			if v.base != nil {
				base = v.base
				break
			}
		}
		if base == nil {
			return nil
		}
		return &selectExprTarget{
			base:  base,
			src:   src,
			start: start,
			end:   end,
		}
	}
	for _, stmt := range blk.Stmts {
		v := findCompleteTargetVisitor{pos: pos, src: src}
		v.Visit(stmt)
		if v.found {
			return v.target
		}
	}
	if start, end := identifierAt(src, int(pos-1)); start != -1 {
		return &idExprTarget{src: src, start: start, end: end}
	}
	return &idExprTarget{src: src, start: int(pos - 1), end: int(pos - 1)}
}

func listCandidatesFromScope(s *types.Scope, pos token.Pos, prefix string, candidates map[string]bool) {
	if s == nil {
		return
	}
	for _, name := range s.Names() {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if _, obj := s.LookupParent(name, pos); obj != nil {
			candidates[name] = true
		}
	}
	listCandidatesFromScope(s.Parent(), pos, prefix, candidates)
}

func completeWithChecker(target completeTarget, checker *types.Checker, pkg *types.Package, initFunc *ast.FuncDecl) ([]string, int, int) {
	if target, _ := target.(*selectExprTarget); target != nil {
		match := completeFieldAndMethods(target.base, target.src[target.start:target.end], checker)
		return match, target.start, target.end
	}
	if target, _ := target.(*idExprTarget); target != nil {
		pos := token.Pos(target.start + 1)
		n := findNearestScope(pkg.Scope(), pos)
		if n == nil && initFunc != nil {
			n = checker.Scopes[initFunc.Type]
		}
		prefix := strings.ToLower(target.src[target.start:target.end])

		candidates := make(map[string]bool)
		listCandidatesFromScope(n, pos, prefix, candidates)
		if len(candidates) == 0 {
			return nil, 0, 0
		}
		l := make([]string, 0, len(candidates))
		for key := range candidates {
			l = append(l, key)
		}
		return l, target.start, target.end
	}
	return nil, 0, 0
}

func Complete(src string, pos token.Pos, conf *Config) ([]string, int, int) {
	match, start, end := complete(src, pos, conf)
	// case-insensitive sort
	sort.Slice(match, func(i, j int) bool {
		c := strings.Compare(strings.ToLower(match[i]), strings.ToLower(match[j]))
		if c < 0 {
			return true
		}
		if c > 0 {
			return false
		}
		c = strings.Compare(match[i], match[j])
		if c < 0 {
			return true
		}
		return false
	})
	return match, start, end
}

func complete(src string, pos token.Pos, conf *Config) ([]string, int, int) {
	fset, blk, _ := parseLesserGoString(src)

	target := completeTargetFromAST(src, pos, blk)
	if target == nil {
		return nil, 0, 0
	}

	// Whether pos is inside a function body.
	inFuncBody := isPosInFuncBody(blk, pos)

	phase1 := convertToPhase1(blk)
	makePkg := func() *types.Package {
		// TODO: Add a proper name to the package though it's not used at this moment.
		pkg, vscope := types.NewPackageWithOldValues("cmd/hello", "", conf.Olds)
		pkg.IsLgo = true
		// TODO: Come up with better implementation to resolve pkg <--> vscope circular deps.
		for _, im := range conf.OldImports {
			pname := types.NewPkgName(token.NoPos, pkg, im.Name(), im.Imported())
			vscope.Insert(pname)
		}
		injectLgoContext(pkg, vscope)
		return pkg
	}

	chConf := &types.Config{
		Importer:          defaultImporter,
		Error:             func(err error) {},
		IgnoreFuncBodies:  true,
		DontIgnoreLgoInit: true,
	}
	var info = types.Info{
		Defs:   make(map[*ast.Ident]types.Object),
		Uses:   make(map[*ast.Ident]types.Object),
		Scopes: make(map[ast.Node]*types.Scope),
		Types:  make(map[ast.Expr]types.TypeAndValue),
	}
	pkg := makePkg()
	checker := types.NewChecker(chConf, fset, pkg, &info)
	checker.Files([]*ast.File{phase1.file})

	if !inFuncBody {
		return completeWithChecker(target, checker, pkg, phase1.initFunc)
	}

	convertToPhase2(phase1, pkg, checker, conf)
	{
		chConf := &types.Config{
			Importer: newImporterWithOlds(conf.Olds),
			Error: func(err error) {
				// Ignore errors.
				// It is necessary to set this noop func because checker stops analyzing code
				// when the first error is found if Error is nil.
			},
			IgnoreFuncBodies:  false,
			DontIgnoreLgoInit: true,
		}
		var info = types.Info{
			Defs:   make(map[*ast.Ident]types.Object),
			Uses:   make(map[*ast.Ident]types.Object),
			Scopes: make(map[ast.Node]*types.Scope),
			Types:  make(map[ast.Expr]types.TypeAndValue),
		}
		// Note: Do not reuse pkg above here because variables are already defined in the scope of pkg above.
		pkg := makePkg()
		checker := types.NewChecker(chConf, fset, pkg, &info)
		checker.Files([]*ast.File{phase1.file})
		return completeWithChecker(target, checker, pkg, nil)
	}
}

// scanFieldOrMethod scans all possible fields and methods of typ.
// c.f. LookupFieldOrMethod of go/types.
func scanFieldOrMethod(typ types.Type, add func(string)) {
	// deref dereferences typ if it is a *Pointer and returns its base and true.
	// Otherwise it returns (typ, false).
	deref := func(typ types.Type) (types.Type, bool) {
		if p, _ := typ.(*types.Pointer); p != nil {
			return p.Elem(), true
		}
		return typ, false
	}

	var ignoreMethod bool
	if named, _ := typ.(*types.Named); named != nil {
		// https://golang.org/ref/spec#Selectors
		// As an exception, if the type of x is a named pointer type and (*x).f is a valid selector expression
		// denoting a field (but not a method), x.f is shorthand for (*x).f.
		if p, _ := named.Underlying().(*types.Pointer); p != nil {
			typ = p
			ignoreMethod = true
		}
	}
	typ, isPtr := deref(typ)
	if isPtr && types.IsInterface(typ) {
		return
	}
	type embeddedType struct {
		typ types.Type
	}
	current := []embeddedType{{typ}}
	var seen map[*types.Named]bool

	// search current depth
	for len(current) > 0 {
		var next []embeddedType // embedded types found at current depth

		for _, e := range current {
			typ := e.typ

			// If we have a named type, we may have associated methods.
			// Look for those first.
			if named, _ := typ.(*types.Named); named != nil {
				if seen[named] {
					// We have seen this type before.
					continue
				}
				if seen == nil {
					seen = make(map[*types.Named]bool)
				}
				seen[named] = true

				if !ignoreMethod {
					// scan methods
					for i := 0; i < named.NumMethods(); i++ {
						f := named.Method(i)
						if f.Exported() {
							add(f.Name())
						}
					}
				}
				// continue with underlying type
				typ = named.Underlying()
			}

			switch t := typ.(type) {
			case *types.Struct:
				for i := 0; i < t.NumFields(); i++ {
					f := t.Field(i)
					if f.Exported() {
						add(f.Name())
					}
					if f.Anonymous() {
						typ, _ := deref(f.Type())
						next = append(next, embeddedType{typ})
					}
				}

			case *types.Interface:
				// scan methods
				for i := 0; i < t.NumMethods(); i++ {
					if m := t.Method(i); m.Exported() {
						add(m.Name())
					}
				}
			}
		}
		current = next
	}
}

func completeFieldAndMethods(expr ast.Expr, orig string, checker *types.Checker) []string {
	orig = strings.ToLower(orig)
	suggests := make(map[string]bool)
	add := func(s string) {
		if strings.HasPrefix(strings.ToLower(s), orig) {
			suggests[s] = true
		}
	}
	func() {
		// Complete package fields selector (e.g. bytes.buf[cur] --> bytes.Buffer)
		id, _ := expr.(*ast.Ident)
		if id == nil {
			return
		}
		obj := checker.Uses[id]
		if obj == nil {
			return
		}
		pkg, _ := obj.(*types.PkgName)
		if pkg == nil {
			return
		}
		im := pkg.Imported()
		for _, name := range im.Scope().Names() {
			if o := im.Scope().Lookup(name); o.Exported() {
				add(name)
			}
		}
	}()
	if tv, ok := checker.Types[expr]; ok && tv.IsValue() {
		scanFieldOrMethod(tv.Type, add)
	}
	if len(suggests) == 0 {
		return nil
	}
	var results []string
	for key := range suggests {
		results = append(results, key)
	}
	return results
}
