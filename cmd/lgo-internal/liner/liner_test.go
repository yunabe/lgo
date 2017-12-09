package liner

import (
	"testing"
)

func TestContinueLine(t *testing.T) {
	tests := []struct {
		lines  []string
		expect bool
		indent int
	}{{
		lines:  []string{"x +"},
		expect: true,
	}, {
		lines:  []string{" y +"},
		expect: true,
	}, {
		lines:  []string{"func"},
		expect: true,
	}, {
		lines:  []string{"     func"},
		expect: true,
	}, {
		lines:  []string{"if"},
		expect: true,
	}, {
		// syntax error.
		lines:  []string{"if {"},
		expect: false,
	}, {
		lines:  []string{"for {"},
		expect: true,
		indent: 1,
	}, {
		lines:  []string{"for{for{for{{"},
		expect: true,
		indent: 4,
	}, {
		lines:  []string{"for{for{for{{}"},
		expect: true,
		indent: 3,
	}, {
		lines: []string{"func main()"},
		// This is false because `func f()<newline>{}` is invalid in Go`
		expect: false,
	}, {
		// Don't return true even if "missing function body" occurrs.
		lines:  []string{"func main()", "func main2(){}"},
		expect: false,
	}, {
		lines: []string{"import fmt"},
		// This must be true because `import fmt<newline>"fmt"` is invalid.
		// TODO: Fix this
		expect: true,
	}, {
		lines:  []string{"func main("},
		expect: true,
	}, {
		lines:  []string{"func main(x,"},
		expect: true,
	}, {
		lines: []string{"func main(x"},
		// Strickly speaking, this should be false if there is no possible valid statement with this.
		expect: true,
	}, {
		lines:  []string{"func ("},
		expect: true,
	}, {
		lines:  []string{"func (r interface{"},
		expect: true,
		indent: 1,
	}, {
		lines:  []string{"/* comment "},
		expect: true,
	}, {
		lines:  []string{"`raw string"},
		expect: true,
	}, {
		lines:  []string{"for {", "for {"},
		expect: true,
		indent: 2,
	}, {
		lines:  []string{"for {", " "},
		expect: true,
		indent: 1,
	}, {
		lines:  []string{"for {", "\t"},
		expect: true,
		indent: 1,
	}, {
		// This tests dropEmptyLine.
		lines:  []string{"for {", ""},
		expect: true,
		indent: 1,
	}, {
		lines:  []string{"type s struct {}"},
		expect: true,
	}, {
		lines:  []string{"type s struct {}", ""},
		expect: false,
	}, {
		lines:  []string{"type s struct {}", "   "},
		expect: false,
	}, {
		lines:  []string{"func (s) f(){}"},
		expect: true,
	}, {
		lines:  []string{"func (s) f(){}", ""},
		expect: false,
	},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			var lines []string
			for _, l := range tc.lines {
				lines = append(lines, l)
			}
			cont, indent := continueLine(lines)
			if cont != tc.expect {
				t.Errorf("Expected %v but got %v for %#v", tc.expect, cont, tc.lines)
				return
			}
			if indent != tc.indent {
				t.Errorf("Expected %d but got %d for %#v", tc.indent, indent, tc.lines)
			}
		})
	}
}
