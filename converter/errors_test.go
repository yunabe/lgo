// This file tests lgo compile error messages

package converter

import (
	"go/scanner"
	"reflect"
	"strings"
	"testing"
)

func TestConvertErrors(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		errors []string
	}{
		{
			name: "syntax errors",
			src: `
			func f() {
				x := y +
			}
			func f() {`,
			errors: []string{
				"3:4: expected operand, found '}'",
				"4:14: expected ';', found 'EOF'",
			},
		},
		{
			name: "undefined",
			src: `
			x := y
			x := 10
			`,
			errors: []string{
				"1:6: undeclared name: y",
				"2:6: no new variables on left side of :=",
			},
		},
		{
			name: "inside function",
			src: `
			func f(x int) int {
				return 1.23 * x
			}
			`,
			errors: []string{
				"2:12: 1.23 (untyped float constant) truncated to int",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Convert(strings.TrimSpace(tt.src), &Config{})
			err := result.Err
			if err == nil {
				t.Error("No error is reported")
				return
			}
			var errs []string
			if lst, ok := err.(ErrorList); ok {
				for _, e := range lst {
					errs = append(errs, e.Error())
				}
			} else if lst, ok := err.(scanner.ErrorList); ok {
				for _, e := range lst {
					errs = append(errs, e.Error())
				}
			} else {
				errs = []string{err.Error()}
			}
			if !reflect.DeepEqual(errs, tt.errors) {
				t.Errorf("Expected %#v but got %#v", tt.errors, errs)
			}
		})
	}

}
