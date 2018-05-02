package converter

import (
	"go/token"
	"reflect"
	"strings"
	"testing"
)

func TestIdentifierAt(t *testing.T) {
	type args struct {
		src string
		idx int
	}
	tests := []struct {
		name      string
		args      args
		wantStart int
		wantEnd   int
	}{
		{
			name:      "basic",
			args:      args{"abc", 0},
			wantStart: 0,
			wantEnd:   3,
		}, {
			name:      "basic",
			args:      args{"_a", 0},
			wantStart: 0,
			wantEnd:   2,
		}, {
			args:      args{"abc", 1},
			wantStart: 0,
			wantEnd:   3,
		}, {
			args:      args{"abc", 3},
			wantStart: 0,
			wantEnd:   3,
		}, {
			args:      args{"abc", 10},
			wantStart: -1,
			wantEnd:   -1,
		}, {
			args:      args{"abc", -1},
			wantStart: -1,
			wantEnd:   -1,
		}, {
			args:      args{"1034", 2},
			wantStart: -1,
			wantEnd:   -1,
		}, {
			args:      args{"a034", 2},
			wantStart: 0,
			wantEnd:   4,
		}, {
			args:      args{"a+b", 2},
			wantStart: 2,
			wantEnd:   3,
		}, {
			args:      args{"a+b", 1},
			wantStart: 0,
			wantEnd:   1,
		}, {
			name:      "multibytes",
			args:      args{"こんにちは", 6},
			wantStart: 0,
			wantEnd:   15,
		}, {
			name:      "multibytes_invalidpos",
			args:      args{"こんにちは", 5},
			wantStart: -1,
			wantEnd:   -1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd := identifierAt(tt.args.src, tt.args.idx)
			if gotStart != tt.wantStart {
				t.Errorf("identifierAt() gotStart = %v, want %v", gotStart, tt.wantStart)
			}
			if gotEnd != tt.wantEnd {
				t.Errorf("identifierAt() gotEnd = %v, want %v", gotEnd, tt.wantEnd)
			}
		})
	}
}

func Test_findLastDot(t *testing.T) {
	type args struct {
		src string
		idx int
	}
	tests := []struct {
		name        string
		args        args
		wantDot     int
		wantIDStart int
		wantIDEnd   int
	}{
		{
			name:        "basic",
			args:        args{"ab.cd", 3},
			wantDot:     2,
			wantIDStart: 3,
			wantIDEnd:   5,
		}, {
			name:        "eos",
			args:        args{"ab.cd", 5},
			wantDot:     2,
			wantIDStart: 3,
			wantIDEnd:   5,
		}, {
			name:        "dot",
			args:        args{"ab.cd", 2},
			wantDot:     -1,
			wantIDStart: -1,
			wantIDEnd:   -1,
		}, {
			name:        "space",
			args:        args{"ab.  cd", 6},
			wantDot:     2,
			wantIDStart: 5,
			wantIDEnd:   7,
		}, {
			name:        "newline",
			args:        args{"ab.\ncd", 5},
			wantDot:     2,
			wantIDStart: 4,
			wantIDEnd:   6,
		}, {
			name:        "not_dot",
			args:        args{"a.b/cd", 4},
			wantDot:     -1,
			wantIDStart: -1,
			wantIDEnd:   -1,
		}, {
			name:        "empty_src",
			args:        args{"", 0},
			wantDot:     -1,
			wantIDStart: -1,
			wantIDEnd:   -1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDot, gotIDStart, gotIDEnd := findLastDot(tt.args.src, tt.args.idx)
			if gotDot != tt.wantDot {
				t.Errorf("findLastDot() gotDot = %v, want %v", gotDot, tt.wantDot)
			}
			if gotIDStart != tt.wantIDStart {
				t.Errorf("findLastDot() gotIDStart = %v, want %v", gotIDStart, tt.wantIDStart)
			}
			if gotIDEnd != tt.wantIDEnd {
				t.Errorf("findLastDot() gotIDEnd = %v, want %v", gotIDEnd, tt.wantIDEnd)
			}
		})
	}
}

func Test_isPosInFuncBody(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{"before", `func sum(a, b int) int[cur] { return a + b }`, false},
		{"brace_open", `func sum(a, b int) int [cur]{ return a + b }`, false},
		{"first", `func sum(a, b int) int {[cur] return a + b }`, true},
		{"last", `func sum(a, b int) int { return a + b[cur] }`, true},
		{"brace_close", `func sum(a, b int) int { return a + b [cur]}`, true},
		{"after", `func sum(a, b int) int { return a + b }[cur]`, false},
		{"funclit", `f := func (a, b int) int { [cur]return a + b }`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := tt.src
			var pos token.Pos
			pos = token.Pos(strings.Index(src, "[cur]") + 1)
			if pos == token.NoPos {
				t.Error("[cur] not found in src")
				return
			}
			src = strings.Replace(src, "[cur]", "", -1)
			_, blk, err := parseLesserGoString(src)
			if err != nil {
				t.Errorf("Failed to parse: %v", err)
				return
			}
			if got := isPosInFuncBody(blk, pos); got != tt.want {
				t.Errorf("isPosInFuncBody() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComplete(t *testing.T) {
	const selectorSpecExample = `
type T0 struct {
	x int
}

func (*T0) M0()

type T1 struct {
	y int
}

func (T1) M1()

type T2 struct {
	z int
	T1
	*T0
}

func (*T2) M2()

type Q *T2

var t T2     // with t.T0 != nil
var p *T2    // with p != nil and (*p).T0 != nil
var q Q = p
`
	tests := []struct {
		name        string
		src         string
		want        []string
		ignoreWant  bool
		wantInclude []string
		wantExclude []string
	}{
		{
			name: "go_keyword",
			src: `
			import (
				"bytes"
			)
			go bytes.sp[cur]`,
			want: []string{"Split", "SplitAfter", "SplitAfterN", "SplitN"},
		}, {
			name: "go_keyword_in_func",
			src: `
			import (
				"bytes"
			)
			func f() {
				go bytes.sp[cur]`,
			want: []string{"Split", "SplitAfter", "SplitAfterN", "SplitN"},
		}, {
			name: "go_with_defer_keyword",
			src: `
			import (
				"bytes"
			)
			func f(){
			}
			defer f()
			go bytes.sp[cur]`,
			want: []string{"Split", "SplitAfter", "SplitAfterN", "SplitN"},
		}, {
			name: "defer_before_go_keyword",
			src: `
			func foo(){
			}
			func bar(){
			}
			defer fo[cur]
			go bar()`,
			want: []string{"foo"},
		}, {
			name: "defer_between_2_go_keywords",
			src: `
			func foo(){
			}
			func bar(){
			}
			go bar()
			defer fo[cur]
			go bar()`,
			want: []string{"foo"},
		}, {
			name: "non_go_defer_function_call_with_go_keyword",
			src: `
			func foo(){
			}
			func bar(){
			}
			fo[cur]
			go bar()`,
			want: []string{"foo"},
		}, {
			name: "package",
			src: `
			import (
				"bytes"
			)
			var buf bytes.sp[cur]`,
			want: []string{"Split", "SplitAfter", "SplitAfterN", "SplitN"},
		}, {
			name: "package_in_func",
			src: `
			import (
				"bytes"
			)
			func f() {
				var buf bytes.sp[cur]`,
			want: []string{"Split", "SplitAfter", "SplitAfterN", "SplitN"},
		}, {
			name: "package_upper",
			src: `
			import (
				"bytes"
			)
			var buf bytes.SP[cur]`,
			want: []string{"Split", "SplitAfter", "SplitAfterN", "SplitN"},
		}, {
			name: "value",
			src: `
			import (
				"bytes"
			)
			var buf bytes.Buffer
			buf.un[cur]`,
			want: []string{"UnreadByte", "UnreadRune"},
		}, {
			name: "value_in_func",
			src: `
			import (
				"bytes"
			)
			func f() {
				var buf bytes.Buffer
				buf.un[cur]`,
			want: []string{"UnreadByte", "UnreadRune"},
		}, {
			name: "pointer",
			src: `
			import (
				"bytes"
			)
			var buf *bytes.Buffer
			buf.un[cur]`,
			want: []string{"UnreadByte", "UnreadRune"},
		}, {
			name: "selector_example1",
			src: `
			[selector_example]
			t.[cur]`,
			want: []string{"M0", "M1", "M2", "T0", "T1", "x", "y", "z"},
		}, {
			name: "selector_example2",
			src: `
			[selector_example]
			p.[cur]`,
			want: []string{"M0", "M1", "M2", "T0", "T1", "x", "y", "z"},
		}, {
			name: "selector_example3",
			src: `
			[selector_example]
			q.[cur]`,
			want: []string{"T0", "T1", "x", "y", "z"},
		}, {
			// ".(" is parsed as TypeAssertExpr.
			name: "dot_paren",
			src: `
			[selector_example]
			q.[cur](`,
			want: []string{"T0", "T1", "x", "y", "z"},
		}, {
			name: "before_type_assert",
			src: `
			[selector_example]
			var x interface{}
			x.(T0).[cur]`,
			want: []string{"M0", "x"},
		}, {
			name: "before_type_switch",
			src: `
			[selector_example]
			type I0 interface {
				M0()
			}
			var i I0
			switch i.[cur](type) {
			default:
			}`,
			want: []string{"M0"},
		}, {
			name: "lgo_context",
			src: `
			_ctx.val[cur]`,
			want: []string{"Value"},
		}, {
			name: "lgo_context_infunc",
			src: `
			func f() {
				_ctx.val[cur]
			}`,
			want: []string{"Value"},
		}, {
			name: "id_simple",
			src: `
			abc := 100
			xyz := "hello"
			[cur]
			zzz := 1.23
			`,
			ignoreWant:  true,
			wantInclude: []string{"abc", "xyz"},
			wantExclude: []string{"zzz"},
		}, {
			name: "id_upper",
			src: `
			abc := 100
			xyz := "hello"
			XY[cur]
			zzz := 1.23
			`,
			want: []string{"xyz"},
		}, {
			name: "id_camel_case",
			src: `
			func testFunc(){}
			testf[cur]
			`,
			want: []string{"testFunc"},
		}, {
			name: "id_partial",
			src: `
			abc := 100
			xyz := "hello"
			xy[cur]
			`,
			want: []string{"xyz"},
		}, {
			name: "id_in_func",
			src: `
			func fn() {
				abc := 100
				xyz := "hello"
				[cur]
				zzz := 1.23
			}`,
			ignoreWant:  true,
			wantInclude: []string{"abc", "xyz", "int64"},
			wantExclude: []string{"zzz"},
		}, {
			name: "id_partial_in_func",
			src: `
			func fn() {
				abc := 100
				xyz := "hello"
				xy[cur]
			}`,
			want: []string{"xyz"},
		}, {
			name: "sort",
			src: `
			type data struct {
				abc int
				DEF int
				xyz int
			}
			var d data
			d.[cur]
			`,
			want: []string{"abc", "DEF", "xyz"},
		}, {
			// https://github.com/yunabe/lgo/issues/18
			name:        "bug18",
			src:         `var [cur]`,
			ignoreWant:  true,
			wantInclude: []string{"int64"},
		}, {
			name: "bug17",
			src: `
			import "bytes"
			var buf bytes.Buffer
			buf.[cur]
			y := 10`,
			ignoreWant: true,
			// TODO: Fix issue #17.
			// wantInclude: []string{"Bytes", "Grow", "Len"},
		}, {
			// Similar to bug17, but Complete works in this case.
			name: "bug17ok",
			src: `
			import "bytes"
			var buf bytes.Buffer
			buf.un[cur]
			y := 10`,
			want: []string{"UnreadByte", "UnreadRune"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := tt.src
			src = strings.Replace(src, "[selector_example]", selectorSpecExample, -1)
			pos := token.Pos(strings.Index(src, "[cur]") + 1)
			if pos <= 0 {
				t.Error("[cur] not found")
				return
			}
			got, _, _ := Complete(strings.Replace(src, "[cur]", "", -1), pos, &Config{})
			if !tt.ignoreWant && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Expected %#v but got %#v", tt.want, got)
			}
			if len(tt.wantInclude) == 0 && len(tt.wantExclude) == 0 {
				return
			}
			m := make(map[string]bool)
			for _, c := range got {
				m[c] = true
			}
			for _, c := range tt.wantInclude {
				if !m[c] {
					t.Errorf("%q is not suggested; Got %#v", c, got)
				}
			}
			for _, c := range tt.wantExclude {
				if m[c] {
					t.Errorf("%q is suggested unexpectedly", c)
				}
			}
		})
	}
}

func TestCompleteKeywords(t *testing.T) {
	// Checks autocomplete works even if identifiers have keyword prefixes.
	// https://golang.org/ref/spec#Keywords
	kwds := []string{
		"break", "default", "func", "interface", "select",
		"case", "defer", "go", "map", "struct",
		"chan", "else", "goto", "package", "switch",
		"const", "fallthrough", "if", "range", "type",
		"continue", "for", "import", "return", "var",
	}
	tests := []struct {
		name string
		code string
		want []string
	}{
		{
			name: "id",
			code: `
			var [kwd]xyz, [kwd]abc int
			[kwd][cur]`,
			want: []string{"[kwd]abc", "[kwd]xyz"},
		}, {
			name: "idspace",
			code: `
			var [kwd]def, [kwd]ghi int
			[kwd][cur] + 10`,
			want: []string{"[kwd]def", "[kwd]ghi"},
		}, {
			name: "dot",
			code: `
			type data struct {
			  [kwd]123 int
			  [kwd]456 string
			}
			var d data
			d.[kwd][cur]`,
			want: []string{"[kwd]123", "[kwd]456"},
		},
	}
	for _, kwd := range kwds {
		for _, src := range tests {
			t.Run(kwd+"_"+src.name, func(t *testing.T) {
				code := strings.Replace(src.code, "[kwd]", kwd, -1)
				pos := token.Pos(strings.Index(code, "[cur]") + 1)
				if pos <= 0 {
					t.Fatal("[cur] not found")
					return
				}
				got, _, _ := Complete(strings.Replace(code, "[cur]", "", -1), pos, &Config{})
				var want []string
				for _, w := range src.want {
					want = append(want, strings.Replace(w, "[kwd]", kwd, -1))
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("got %v; want %v", got, want)
				}
			})
		}
	}
}
