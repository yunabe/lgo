package converter

import (
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func TestInspect(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		doc   string
		query string
	}{
		{
			name: "local variable",
			src: `
			import (
				"fmt"
			)

			func sum(x, y int) int { return x + y }
			func f(x int) (y int) {
				s := sum(x, x)
				return [cur]s*x
			}`,
			doc: "var s int",
		},
		{
			name: "local const",
			src: `
			func f(x int) (y int) {
				const s = 10
				return [cur]s*x
			}`,
			doc: "const s untyped int",
		},
		{
			name: "global_variable",
			src: `
			import (
				"fmt"
			)

			var (
				s int
			)

			func sum(x, y int) { [cur]s = x + y }`,
			doc: "var s int",
		},
		{
			name: "global_const",
			src: `
			const myVal = 10
			x := [cur]myVal * 10
			`,
			doc: "const myVal untyped int",
		},
		{
			name: "func",
			src: `
			func fn(x int) int { return x * x }
			[cur]fn(10)
			`,
			doc: "func fn(x int) int",
		}, {
			name: "func_args",
			src: `
			func fn(x int) int { return x * x }
			fn([cur]`,
			doc: "func fn(x int) int",
		}, {
			name: "func_args_closed",
			src: `
			func fn(x int) int { return x * x }
			fn([cur])
			`,
			doc: "func fn(x int) int",
		}, {
			name: "func_args_after_close",
			src: `
			func fn(x int) int { return x * x }
			fn()[cur]
			`,
		}, {
			name: "func_args_id",
			src: `
			func fn(x int) int { return x * x }
			n := 10
			fn(n[cur])
			`,
			doc: "var n int",
		}, {
			name: "func_args_comma",
			src: `
			func fn(x int) int { return x * x }
			n := 10
			fn(n,[cur])
			`,
			doc: "func fn(x int) int",
		}, {
			name: "func_args_nested",
			src: `
			func fn(x int) int { return x * x }
			func xy(a int) int { return x * x }
			fn(xy([cur]
			`,
			doc: "func xy(a int) int",
		}, {
			name: "func_args_selector",
			src: `
			import "bytes"
			bytes.Compare(nil,[cur])`,
			query: "bytes.Compare",
		}, {
			name: "method",
			src: `
			type typ int
			func (t typ) Int() int { return int(t) }

			x := typ(123)
			x.[cur]Int()`,
			// TODO: Includes a receiver.
			doc: "func Int() int",
		},
		{
			name: "interface_method",
			src: `
			type hello interface {
				sayHello(name string)
			}
			var h hello
			h.[cur]sayHello()`,
			// TODO: Includes a receiver.
			doc: "func sayHello(name string)",
		},
		{
			name: "custom_type_var",
			src: `
			type message string
			var m message
			[cur]m`,
			// TODO: Remove "cmd/hello.".
			doc: "var m cmd/hello.message",
		},
		{
			name: "package",
			src: `
			import (
				"fmt"
			)

			[cur]fmt.Println(0, 1)`,
			query: "fmt",
		},
		{
			name: "renamed package",
			src: `
			import (
				pkg "fmt"
			)

			[cur]pkg.Println(0, 1)`,
			query: "fmt",
		},
		{
			name: "package var",
			src: `
			import (
				"fmt"
				"os"
			)

			fmt.Fprintln(os.[cur]Stderr, "error")`,
			query: "os.Stderr",
		},
		{
			name: "package const",
			src: `
			import (
				"io"
			)

			x := io.[cur]SeekStart`,
			query: "io.SeekStart",
		},
		{
			name: "package func",
			src: `
			import (
				"fmt"
			)

			fmt.P[cur]rintln(0, 1)`,
			query: "fmt.Println",
		},
		{
			name: "method",
			src: `
			import (
				"bytes"
			)

			var buf bytes.Buffer
			buf.[cur]Len()`,
			query: "bytes.Buffer.Len",
		},
		{
			name: "renamed pkg method",
			src: `
			import (
				b "bytes"
			)

			var buf b.Buffer
			buf.[cur]Len()`,
			query: "bytes.Buffer.Len",
		}, {
			name: "package_interface_method",
			src: `
			import (
				"bytes"
				"io"
			)

			var buf bytes.Buffer
			var r io.Reader = &buf
			r.[cur]Read(nil)`,
			query: "io.Reader.Read",
		}, {
			name: "type",
			src: `
			import (
				"flag"
			)

			f := flag.F[cur]lag{}`,
			query: "flag.Flag",
		}, {
			name: "field",
			src: `
			import (
				"flag"
			)

			f := flag.Flag{[cur]Name: "myflag"}`,
			query: "flag.Flag.Name",
		}, {
			name: "local_field_def",
			src:  "type st struct { [cur]val int }",
			doc:  "var val int",
		}, {
			name: "local_field_ref",
			src: `
			type st struct { val int }
			var x st
			x.[cur]val`,
			doc: "var val int",
		}, {
			name: "invalid_type",
			src: `
			var x foobar
			[cur]x + 10`,
		},
		{
			name: "invalid_const_val",
			src: `
			func sum(x, y int) int { return x + y }
			const x = sum(10, 20)
			[cur]x + 10`,
			// TODO: Fix this
			doc: "const x invalid type",
		},
		{
			name: "invalid syntax",
			src:  `[cur]x := 3 +`,
		},
		{
			name: "invalid_syntax_after_cur",
			src: `[cur]x := 3 + 4
			y := x +`,
			doc: "var x int",
		},
		{
			name: "right_after_id",
			src: `
			func f(x int) (y int) {
				s := x+1
				return s[cur]*x
			}`,
			doc: "var s int",
		},
		{
			name: "typename_struct",
			src: `
			type mytype struct {
				X int
				Y string
			}
			v := [cur]mytype{}`,
			doc: "type mytype struct{X int; Y string}",
		},
		{
			name: "typename_interface",
			src: `
			type mytype interface {
				Method(x int)
			}
			v := [cur]mytype(nil)`,
			doc: "type mytype interface{Method(x int)}",
		},
		{
			name: "typename_in_var",
			src: `
			type message string
			var m [cur]message`,
			doc: "type message string",
		},
		// def_ prefix tests test Inspect on identifiers that define objects.
		{
			name: "def_global_variable",
			src:  `var [cur]x = 10`,
			doc:  "var x int",
		},
		{
			name: "def_func",
			src: `
			func [cur]myFunc(x int) (y int) { return x * 2 }
			myFunc(10)`,
			doc: "func myFunc(x int) (y int)",
		},
		{
			name: "def_method",
			src: `
			type myType int
			func (myType) [cur]myMethod() {}`,
			// TODO: Print the receiver
			doc: "func myMethod()",
		},
		{
			name: "def_type",
			src:  `type [cur]myType int`,
			doc:  "type myType int",
		}, {
			name: "lgo_context",
			src:  `_[cur]ctx.Done()`,
			doc:  "var _ctx github.com/yunabe/lgo/core.LgoContext",
		}, {
			name:  "lgo_context_method",
			src:   `_ctx.[cur]Done()`,
			query: "context.Context.Done",
		}, {
			name: "lgo_context_infunc",
			src: `
			func f() {
				_ctx.[cur]Done()
			}`,
			query: "context.Context.Done",
		}, {
			name: "builtin_error_method",
			src: `
			var err error
			err.Error[cur]
			`,
			query: "builtin.error.Error",
		}, {
			name: "builtin_func",
			src: `
			var s []string
			s = ap[cur]pend(s, "hello")`,
			query: "builtin.append",
		}, {
			name:  "builtin_panic",
			src:   `pa[cur]nic("panic")`,
			query: "builtin.panic",
		}, {
			name:  "builtin_type",
			src:   `var f fl[cur]oat64`,
			query: "builtin.float64",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := tt.src
			pos := token.Pos(strings.Index(src, "[cur]") + 1)
			if pos == token.NoPos {
				t.Error("[cur] not found in src")
				return
			}

			obj, local := inspectObject(strings.Replace(src, "[cur]", "", -1), pos, &Config{})
			doc, query := getDocOrGoDocQuery(obj, local)
			var queryStr string
			if query != nil {
				queryStr = query.pkg
				if len(query.ids) > 0 {
					queryStr += "." + strings.Join(query.ids, ".")
				}
			}
			if tt.doc != doc {
				t.Errorf("Expected %q but got %q", tt.doc, doc)
			}
			if tt.query != queryStr {
				t.Errorf("Expected %q but got %q", tt.query, queryStr)
			}
		})
	}
}

func TestInspectWithOlds(t *testing.T) {
	result := Convert(`
	x := 10
	X := x + 10
	`, &Config{LgoPkgPath: "lgo/pkg0"})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	olds := []types.Object{result.Pkg.Scope().Lookup("x"),
		result.Pkg.Scope().Lookup("X"),
	}
	tests := []struct{ id, query string }{
		{"x", "lgo/pkg0.Def_x"},
		{"X", "lgo/pkg0.X"},
	}
	for _, tt := range tests {
		src := `y := x + X`
		doc, query := InspectIdent(src, token.Pos(strings.Index(src, tt.id)+1), &Config{
			Olds:      olds,
			DefPrefix: "Def_",
			// RefPrefix is not used.
			RefPrefix: "Ref_",
		})
		if doc != "" {
			t.Errorf("Expected an empty doc for %s but got %q", tt.id, doc)
		}
		if query != tt.query {
			t.Errorf("Expected %q for %s but got %q", tt.query, tt.id, query)
		}
	}
}

func TestInspectUnnamedStruct(t *testing.T) {
	result := Convert(`
	func Gen() struct{Val int} {
		return struct{Val int}{123}
	}
	`, &Config{LgoPkgPath: "lgo/pkg0"})
	if result.Err != nil {
		t.Error(result.Err)
		return
	}
	olds := []types.Object{
		result.Pkg.Scope().Lookup("Gen"),
	}
	src := `Gen().Val`
	doc, query := InspectIdent(src, token.Pos(strings.Index(src, "Val")+1), &Config{
		Olds:      olds,
		DefPrefix: "Def_",
		// RefPrefix is not used.
		RefPrefix: "Ref_",
	})
	if doc != "" {
		t.Errorf("Unexpected non-empty doc: %q", doc)
	}
	if query != "" {
		t.Errorf("Unexpected non-empty query: %q", query)
	}
}
