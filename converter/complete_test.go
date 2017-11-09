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
			gotStart, gotEnd := identifierAt([]byte(tt.args.src), tt.args.idx)
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
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDot, gotIDStart, gotIDEnd := findLastDot([]byte(tt.args.src), tt.args.idx)
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
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
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
			name: "value",
			src: `
			import (
				"bytes"
			)
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
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := token.Pos(strings.Index(tt.src, "[cur]") + 1)
			if pos <= 0 {
				t.Error("[cur] not found")
				return
			}
			got, _, _ := Complete([]byte(strings.Replace(tt.src, "[cur]", "", -1)), pos, &Config{})
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Expected %#v but got %#v", tt.want, got)
			}
		})
	}
}
