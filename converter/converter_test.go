package converter

import (
	"bytes"
	"flag"
	"go/ast"
	"go/importer"
	"go/types"
	"io/ioutil"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"strings"
	"testing"

	// Rebuild core library before this test if it's modified.
	_ "github.com/yunabe/lgo/core"
)

func calcDiff(b1, b2 []byte) (data []byte, err error) {
	f1, err := ioutil.TempFile("", "converter_test")
	if err != nil {
		return
	}
	defer os.Remove(f1.Name())
	defer f1.Close()

	f2, err := ioutil.TempFile("", "converter_test")
	if err != nil {
		return
	}
	defer os.Remove(f2.Name())
	defer f2.Close()

	f1.Write(b1)
	f2.Write(b2)

	data, err = exec.Command("diff", "-u", f1.Name(), f2.Name()).CombinedOutput()
	if len(data) > 0 {
		// diff exits with a non-zero status when the files don't match.
		// Ignore that failure as long as we get output.
		err = nil
	}
	return
}

var update = flag.Bool("update", false, "update .golden files")

func checkGolden(t *testing.T, got []byte, golden string) {
	expected, err := ioutil.ReadFile(golden)
	if err != nil && !*update {
		t.Error(err)
		return
	}
	if err == nil && bytes.Equal(got, expected) {
		return
	}
	if *update {
		if err := ioutil.WriteFile(golden, got, 0666); err != nil {
			t.Error(err)
		}
		return
	}
	d, err := calcDiff(expected, got)
	if err != nil {
		t.Errorf("Failed to calculate diff: %v", err)
		return
	}
	t.Errorf("%s", d)
}

func TestUniqueSortedNames(t *testing.T) {
	names := uniqueSortedNames([]*ast.Ident{
		{Name: "c"}, {Name: "a"}, {Name: "c"}, {Name: "z"}, {Name: "b"},
	})
	exp := []string{"a", "b", "c", "z"}
	if !reflect.DeepEqual(exp, names) {
		t.Errorf("Expected %v but got %v", exp, names)
	}
}

func TestConvert_simple(t *testing.T) {
	result := Convert(`
	import (
		"fmt"
	)
	import renamedio "io"

	func fact(n int64) int64 {
		if n > 0 {
			return n * fact(n - 1)
		}
		return 1
	}

	type myStruct struct {
		value int
	}

	func (m *myStruct) hello(name string) string {
		return fmt.Sprintf("Hello %s!", name)
	}

	var sv myStruct
	sp := &myStruct{}
	msg0 := sv.hello("World0")
	msg1 := sp.hello("World1")

	const (
		ca = "hello"
		cb = "piyo"
	)

	func returnInterface() interface{method(int)float32} {
		panic("not implemented")
	}

	inter := returnInterface()

	f := fact(10)
	var pi, pi2 float32 = 3.14, 6.28

	var reader renamedio.Reader
	`, &Config{})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, []byte(result.Src), "testdata/simple.golden")
}

func TestConvert_novar(t *testing.T) {
	result := Convert(`
	func f(n int64) int64 {
		return n * n
	}
	`, &Config{})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, []byte(result.Src), "testdata/novar.golden")
}

func TestConvert_errorUndeclared(t *testing.T) {
	// Variables must be declared explicitly.
	result := Convert(`
	var y = 10
	x = y * y
	`, &Config{})
	if result.Err == nil || !strings.Contains(result.Err.Error(), "undeclared name: x") {
		t.Errorf("Unexpected error: %v", result.Err)
		return
	}
	// Although, it's valid at the file scope in Go, it's invalid in lgo.
	result = Convert(`
	var x = y * y
	var y = 10
	`, &Config{})
	if result.Err == nil || !strings.Contains(result.Err.Error(), "undeclared name: y") {
		t.Errorf("Unexpected error: %v", result.Err)
		return
	}
}

func TestConvert_withOld(t *testing.T) {
	im := defaultImporter
	bufio, err := im.Import("bufio")
	if err != nil {
		t.Error(err)
	}
	// Variables must be declared explicitly.
	result := Convert(`
	import pkg1 "io/ioutil"

	var r = NewReader(nil)
	c := pkg1.NopCloser(r)
	`, &Config{
		Olds: []types.Object{bufio.Scope().Lookup("NewReader")},
	})
	if err != nil {
		t.Error(err)
		return
	}
	checkGolden(t, []byte(result.Src), "testdata/withold.golden")
}

func TestConvert_withOldPkgDup(t *testing.T) {
	// This test demonstrates how old values are renamed if the package where an old value is defined is also imported in source code.
	// This situation would not happen in the real world because old values must be defined in lgo-packages which should not be imported
	// by import statements.
	//
	// In this test, use another importer other than defaultImporter here because
	// we need to deferentiate Object from old values and the same Object
	// referred in source code (NewReader and bufio.NewReader).
	im := importer.Default()

	bufio, err := im.Import("bufio")
	if err != nil {
		t.Error(err)
	}
	// Variables must be declared explicitly.
	result := Convert(`
	import "bufio"

	var r0 = NewReader(nil)
	var r1 = bufio.NewReader(nil)
	`, &Config{
		Olds: []types.Object{bufio.Scope().Lookup("NewReader")},
	})
	if err != nil {
		t.Error(err)
		return
	}
	checkGolden(t, []byte(result.Src), "testdata/withold_pkgdup.golden")
}

func TestConvert_twoLgo(t *testing.T) {
	result := Convert(`
		func f(n int) int {
			return n * n
		}
		type st struct {
			value int
		}
		func (s *st) getValue() float32 {
			return float32(s.value)
		}
	
		func getUnnamedStruct() struct{x int} {
			return struct{x int}{10}
		}
		`, &Config{LgoPkgPath: "lgo/pkg0"})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	pkg0 := result.Pkg
	f := pkg0.Scope().Lookup("f")
	st := pkg0.Scope().Lookup("st")
	gu := pkg0.Scope().Lookup("getUnnamedStruct")
	result = Convert(`
		a := f(3)
		s := st{
			value: 20,
		}
		b := s.value
		c := s.getValue()
		d := interface{getValue() float32}(&s)
		f := d.getValue()
	
		g := getUnnamedStruct()
		var h struct{x int} = g
		`, &Config{
		Olds: []types.Object{f, st, gu},
	})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/twolgo.golden")
}

func TestConvert_twoLgo2(t *testing.T) {
	result := Convert(`
	x := 10
	y := 20
	`, &Config{LgoPkgPath: "lgo/pkg0"})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	pkg0 := result.Pkg
	// x in RHS of the first line refers to the old x.
	result = Convert(`
	x := x * x
	func f() int {
		return x + y
	}
	`, &Config{
		Olds: []types.Object{
			pkg0.Scope().Lookup("x"),
			pkg0.Scope().Lookup("y"),
		},
	})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/twolgo2.golden")

	result = Convert(`
	func f() int {
		return x + y
	}
	`, &Config{
		Olds: []types.Object{
			pkg0.Scope().Lookup("x"),
			pkg0.Scope().Lookup("y"),
		},
	})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/twolgo3.golden")
}

func TestConvert_rename(t *testing.T) {
	result := Convert(`
	func f(n int) int {
		return n * n
	}
	func Fn(n int) int {
		b := func() int {return 10}
		return b()
	}
	type st struct {
		value int
	}
	func (s *st) getValue() float32 {
		return float32(s.value)
	}

	type myInter interface {
		Method0()
		method()
	}

	func getInter() interface{method()} {
		var i myInter
		return i
	}

	v := f(3)
	getInter().method()
	myInter(nil).Method0()
	s := st{
		value: 34,
	}
	`, &Config{
		DefPrefix: "Def_",
		RefPrefix: "Ref_",
	})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/rename.golden")
}

func TestConvert_renameRefOtherPkgs(t *testing.T) {
	result := Convert(`
		func f(n int) int {
			return n * n
		}
		type st struct {
			value int
		}
		func (s *st) getValue() float32 {
			return float32(s.value)
		}
		`, &Config{LgoPkgPath: "lgo/pkg0"})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	pkg0 := result.Pkg
	f := pkg0.Scope().Lookup("f")
	st := pkg0.Scope().Lookup("st")
	result = Convert(`
		a := f(3)
		s := st{
			value: 20,
		}
		var i interface{} = &s
		i.(*st).getValue()
		// Renaming to access unexported names in other packages is broken.
		func myFunc() {
			a := f(3)
			s := st{
				value: a,
			}
			var i interface{} = &s
			i.(*st).getValue()
		}
		`, &Config{
		DefPrefix: "Def_",
		RefPrefix: "Ref_",
		Olds:      []types.Object{f, st},
	})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/rename_other_pkgs.golden")
}

func TestConvert_passImport(t *testing.T) {
	result := Convert(`
	import (
		"fmt"
		logger "log"
		"io/ioutil"
	)

	f, _ := ioutil.TempFile("a", "b")
	`, &Config{LgoPkgPath: "lgo/pkg0"})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	// fmt and logger are stripped. "os" is imported implicitly.
	checkGolden(t, result.Src, "testdata/passimport0.golden")

	var names []string
	for _, im := range result.Imports {
		names = append(names, im.Name())
	}
	// Although "os" is imported implicitly, it's not exported to result.Imports.
	expNames := []string{"fmt", "ioutil", "logger"}
	if !reflect.DeepEqual(names, expNames) {
		t.Errorf("Expected %#v but got %#v", expNames, names)
	}

	result = Convert(`
	fmt.Println("Hello fmt!")
	logger.Println("Hello log!")
	`, &Config{
		OldImports: result.Imports,
	})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/passimport1.golden")
}

func TestConvert_lastExpr(t *testing.T) {
	result := Convert(`
	x := 10
	x * x
	`, &Config{LgoPkgPath: "lgo/pkg0"})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/last_expr0.golden")

	result = Convert(`
	func f() {}
	f()
	`, &Config{LgoPkgPath: "lgo/pkg0"})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/last_expr1.golden")

	result = Convert(`
	func f() (int, float32) {
		return 10, 2.1
	}
	f()
	`, &Config{LgoPkgPath: "lgo/pkg0"})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/last_expr2.golden")

	result = Convert(`
	func f() int {
		return 123
	}
	f()
	`, &Config{LgoPkgPath: "lgo/pkg0"})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/last_expr3.golden")
}

func TestConvert_emptyResult(t *testing.T) {
	result := Convert(`
	import (
		"fmt"
		"os"
	)
	`, &Config{LgoPkgPath: "lgo/pkg0"})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	if result.Src != nil {
		t.Errorf("Expected empty but got %q", result.Src)
	}
	var imports []string
	for _, im := range result.Imports {
		imports = append(imports, im.Name())
	}
	sort.Strings(imports)
	exp := []string{"fmt", "os"}
	if !reflect.DeepEqual(imports, exp) {
		t.Errorf("Expected %#v but got %#v", exp, imports)
	}
}

func TestConvert_lgoctxBuiltin(t *testing.T) {
	result := Convert(`
	for {
		select {
			case <-_ctx.Done():
				break
		}
	}
	`, &Config{LgoPkgPath: "lgo/pkg0"})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/lgoctx_builtin.golden")
}

func TestConvert_autoExitCode(t *testing.T) {
	result := Convert(`
	func light(x int) int {
		y := x * x
		return y
	}

	func fcall(x int) int {
		x = light(x)
		x += 10
		x = light(x)
		x -= 10
		return light(x)
	}

	func ifstmt() {
		x := light(2)
		if x > 10 {
		}

		x = light(3)
		x += light(4)
		if y := light(10); x - y < 0 {
		}

		x = light(4)
		if x < light(10) {
		}
	}

	func forstmt() {
		x := light(1)
		for i := 0; i < 10; i++ {
			x += i
		}
		y := light(0)
		for i := light(y);; {
			y += i
		}
	}

	func switchstmt() int {
		x := light(2)
		switch x {
		case x * x:
			x = 10
		}

		x = light(3)
		switch x {
		case light(4):
			x = 10
		}

		// Inject exits into switch bodies.
		switch x := light(10); x {
		case 10:
			light(x)
			light(x + 1)
		default:
			light(x + 2)
			light(x + 3)
		}
		return x
	}

	func deferstmt() int {
		x := light(2)
		defer light(light(4))
		y := light(3)
		defer light(5)
		z := light(10)
		defer func() {
			z += light(20)
			for i := 0; i < x; i++ {
				z += light(30)
			}
			f := func() {
				z += 1
			}
			f()
			f()
		}()
		return x * y + z
	}

	for i := 0; i < 100; i++ {
	}
	for {}

	`, &Config{LgoPkgPath: "lgo/pkg0", AutoExitCode: true})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/autoexit.golden")
}

func TestConvert_autoExitCodeImportOnly(t *testing.T) {
	result := Convert(`
	import (
		"fmt"
		"os"
	)
	`, &Config{LgoPkgPath: "lgo/pkg0", AutoExitCode: true})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	if len(result.Src) != 0 {
		t.Errorf("Expected an empty src but got %q", result.Src)
	}
}

func TestConvert_autoExitCodeVarOnly(t *testing.T) {
	result := Convert(`var x int`, &Config{LgoPkgPath: "lgo/pkg0", AutoExitCode: true})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/autoexit_varonly.golden")
}

func TestConvert_registerVars(t *testing.T) {
	result := Convert(`
	a := 10
	b := 3.4
	var c string
	func f(n int) int { return n * n }
	`, &Config{LgoPkgPath: "lgo/pkg0", RegisterVars: true})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/register_vars.golden")
}

func TestConvert_wrapGoStmt(t *testing.T) {
	result := Convert(`
	f := func(x, y int) int { return x + y }
	go func(x int){
		go f(x, 20)
	}(10)
	`, &Config{LgoPkgPath: "lgo/pkg0", RegisterVars: true})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/wrap_gostmt.golden")
}

// Demostrates how converter keeps comments.
func TestConvert_comments(t *testing.T) {
	result := Convert(`// Top-level comment
	// The second line of the top-level comment

	// dangling comments 
	
	// fn does nothing
	func fn() {
		// Do nothing
	}

	// MyType represents something
	type MyType struct {
		Name string  // name
		Age int // age
	}

	// Hello returns a hello message
	func (m *MyType) Hello() string {
		return "Hello " + m.Name
	}

	type MyInterface interface {
		// DoSomething does something
		DoSomething(x int) float32
		Hello() string // Say hello
	}

	var (
		x int = 10  // Something
		// y is string
		y = "hello"
	)
	const (
		c = "constant"  // This is constant
		// d is also constant
		d = 123
	)

    // alice is Alice
	alice := &MyType{"Alice", 12}
	bob := &MyType{"Bob", 45}  // bob is Bob

	// i is interface
	var i interface{} = alice
	var j interface{} = bob // j is also interface
	`, &Config{LgoPkgPath: "lgo/pkg0"})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/comments.golden")
}

func TestConvert_commentFirstLine(t *testing.T) {
	result := Convert(`// fn does nothing
	func fn() {
		// Do nothing
	}`, &Config{LgoPkgPath: "lgo/pkg0"})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/comments_firstline.golden")
}

func TestConvert_commentFirstLineWithCore(t *testing.T) {
	result := Convert(`// fn does nothing
	func fn() {
	}
	<-_ctx.Done()
	`, &Config{LgoPkgPath: "lgo/pkg0"})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/comments_firstline__withcore.golden")
}

func TestConvert_commentFirstLineSlashAsterisk(t *testing.T) {
	result := Convert(`/* fn does nothing */
	func fn() {
		// Do nothing
	}`, &Config{LgoPkgPath: "lgo/pkg0"})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	// TODO: Fix this case
	checkGolden(t, result.Src, "testdata/comments_firstline_slashasterisk.golden")
}

func TestConvert_commentFirstTrailing(t *testing.T) {
	result := Convert(`const x = 10 // x is const int
		`, &Config{LgoPkgPath: "lgo/pkg0"})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/comments_firstline_trailing.golden")
}

func TestConvert_commentLastLine(t *testing.T) {
	result := Convert(`const x int = 123 // x is x`, &Config{LgoPkgPath: "lgo/pkg0"})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	checkGolden(t, result.Src, "testdata/comments_lastline.golden")
}

func Test_prependPrefixToID(t *testing.T) {
	prefix := "Ref_"
	tests := []struct {
		name   string
		expect string
	}{
		{name: "x", expect: "Ref_x"},
		{name: "x.y", expect: "x.Ref_y"},
	}
	for _, tt := range tests {
		ident := &ast.Ident{Name: tt.name}
		prependPrefixToID(ident, prefix)
		if ident.Name != tt.expect {
			t.Errorf("Expected %q for %q but got %q", tt.expect, tt.name, ident.Name)
		}
	}
}
