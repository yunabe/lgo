package liner

import (
	"bytes"
	"go/scanner"
	"go/token"
	"io"
	"strings"

	"github.com/peterh/liner"
	"github.com/yunabe/lgo/parser"
)

func parseLesserGoString(src []byte) (f *parser.LGOBlock, err error) {
	return parser.ParseLesserGoFile(token.NewFileSet(), "", src, 0)
}

func isUnexpectedEOF(errs scanner.ErrorList, lines [][]byte) bool {
	for _, err := range errs {
		if err.Msg == "raw string literal not terminated" || err.Msg == "comment not terminated" {
			return true
		}
		if strings.Contains(err.Msg, "expected ") && err.Pos.Line == len(lines) &&
			err.Pos.Column == len(lines[len(lines)-1])+1 {
			return true
		}
	}
	return false
}

func nextIndent(src []byte) int {
	sc := &scanner.Scanner{}
	fs := token.NewFileSet()
	var msgs []string
	sc.Init(fs.AddFile("", -1, len(src)), src, func(_ token.Position, msg string) {
		msgs = append(msgs, msg)
	}, 0)
	var indent int
	for {
		_, tok, _ := sc.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.LBRACE {
			indent++
		} else if indent > 0 && tok == token.RBRACE {
			indent--
		}
	}
	return indent
}

func dropEmptyLine(lines [][]byte) [][]byte {
	r := make([][]byte, 0, len(lines))
	for _, line := range lines {
		if len(line) > 0 {
			r = append(r, line)
		}
	}
	return r
}

func continueLine(lines [][]byte) (bool, int) {
	lines = dropEmptyLine(lines)
	src := bytes.Join(lines, []byte("\n"))
	_, err := parseLesserGoString(src)
	if err == nil {
		return false, 0
	}
	if errs, ok := err.(scanner.ErrorList); !ok || !isUnexpectedEOF(errs, lines) {
		return false, 0
	}
	return true, nextIndent(src)
}

type Liner struct {
	liner *liner.State
}

func NewLiner() *Liner {
	return &Liner{
		liner: liner.NewLiner(),
	}
}

func (l *Liner) Next() ([]byte, error) {
	var lines [][]byte
	var indent int
	for {
		var prompt string
		if lines == nil {
			prompt = ">>> "
		} else {
			prompt = "... "
		}
		if indent > 0 {
			prompt += strings.Repeat("    ", int(indent))
		}
		// line does not include \n.
		line, err := l.liner.Prompt(prompt)
		if err == io.EOF {
			// Ctrl-D
			if lines == nil {
				return nil, io.EOF
			}
			return nil, nil
		}
		lines = append(lines, []byte(line))
		var cont bool
		cont, indent = continueLine(lines)
		if !cont {
			content := bytes.Join(lines, []byte("\n"))
			if len(content) > 0 {
				l.liner.AppendHistory(string(content))
			}
			return content, nil
		}
	}
}
