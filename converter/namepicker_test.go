package converter

import (
	"go/ast"
	"go/types"
	"testing"
)

func TestNamePicker(t *testing.T) {
	picker := newNamePicker(map[*ast.Ident]types.Object{
		&ast.Ident{Name: "y"}: nil,
	})
	x := picker.NewName("x")
	x0 := picker.NewName("x")
	if x != "x" || x0 != "x0" {
		t.Errorf("Expected (x, x0) but got (%s, %s)", x, x0)
	}

	y0 := picker.NewName("y")
	y1 := picker.NewName("y")
	if y0 != "y0" || y1 != "y1" {
		t.Errorf("Expected (y0, y1) but got (%s, %s)", y0, y1)
	}
}
