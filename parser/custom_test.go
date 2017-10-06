package parser

import (
	"go/token"
	"testing"
)

func parseLesserGoString(src string) (f *LGOBlock, err error) {
	return ParseLesserGoFile(token.NewFileSet(), "", []byte(src), 0)
}

func TestParseLesserGoString(t *testing.T) {
	_, err := parseLesserGoString(`
	var x = 10

	func f(i int) int {
		return i
	}

	(func(){fmt.Println("hello")}())
	`)
	if err != nil {
		t.Error(err)
		return
	}
}
