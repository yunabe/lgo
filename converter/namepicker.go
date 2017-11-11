package converter

import (
	"fmt"
	"go/ast"
	"go/types"
)

type namePicker struct {
	m map[string]bool
}

func newNamePicker(defs map[*ast.Ident]types.Object) *namePicker {
	m := make(map[string]bool)
	for id := range defs {
		m[id.Name] = true
	}
	return &namePicker{m}
}

func (p *namePicker) NewName(base string) string {
	if !p.m[base] {
		p.m[base] = true
		return base
	}
	for i := 0; ; i++ {
		name := fmt.Sprintf("%s%d", base, i)
		if !p.m[name] {
			p.m[name] = true
			return name
		}
	}
}
