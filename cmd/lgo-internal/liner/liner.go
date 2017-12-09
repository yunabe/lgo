package liner

import (
	"go/ast"
	"go/scanner"
	"go/token"
	"io"
	"strings"

	"github.com/peterh/liner"
	"github.com/yunabe/lgo/parser"
)

func parseLesserGoString(src string) (f *parser.LGOBlock, err error) {
	return parser.ParseLesserGoFile(token.NewFileSet(), "", src, 0)
}

func isUnexpectedEOF(errs scanner.ErrorList, lines []string) bool {
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

func nextIndent(src string) int {
	sc := &scanner.Scanner{}
	fs := token.NewFileSet()
	var msgs []string
	sc.Init(fs.AddFile("", -1, len(src)), []byte(src), func(_ token.Position, msg string) {
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

func dropEmptyLine(lines []string) []string {
	r := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) > 0 {
			r = append(r, line)
		}
	}
	return r
}

func hasTypeOrMethodDecl(b *parser.LGOBlock) bool {
	for _, s := range b.Stmts {
		decl, _ := s.(*ast.DeclStmt)
		if decl == nil {
			continue
		}
		if f, _ := decl.Decl.(*ast.FuncDecl); f != nil && f.Recv != nil {
			// methods
			return true
		}
		if gen, _ := decl.Decl.(*ast.GenDecl); gen != nil && gen.Tok == token.TYPE {
			// type decl
			return true
		}
	}
	return false
}

// continueForMethods returns true if lines have type or method declaration and the last line is not empty.
func continueForMethods(b *parser.LGOBlock, lines []string) (bool, int) {
	if len(lines) == 0 || !hasTypeOrMethodDecl(b) {
		// Note: len(lines) check exists just for safety.
		return false, 0
	}
	last := lines[len(lines)-1]
	if strings.TrimSpace(last) == "" {
		return false, 0
	}
	return true, 0
}

func continueLine(lines []string) (bool, int) {
	dropped := dropEmptyLine(lines)
	src := strings.Join(dropped, "\n")
	b, err := parseLesserGoString(src)
	if err == nil {
		return continueForMethods(b, lines)
	}
	if errs, ok := err.(scanner.ErrorList); !ok || !isUnexpectedEOF(errs, dropped) {
		return continueForMethods(b, lines)
	}
	return true, nextIndent(src)
}

func ContinueLineString(s string) (bool, int) {
	return continueLine(strings.Split(s, "\n"))
}

type Liner struct {
	liner *liner.State
	// lines keeps the intermediate input to use it from complete
	lines []string
}

func NewLiner() *Liner {
	return &Liner{
		liner: liner.NewLiner(),
	}
}

func (l *Liner) Next() (string, error) {
	l.lines = nil
	var indent int
	for {
		var prompt string
		if l.lines == nil {
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
			if l.lines == nil {
				return "", io.EOF
			}
			return "", nil
		}
		l.lines = append(l.lines, line)
		var cont bool
		cont, indent = continueLine(l.lines)
		if !cont {
			content := strings.Join(l.lines, "\n")
			if len(content) > 0 {
				l.liner.AppendHistory(content)
			}
			return content, nil
		}
	}
}

// SetCompleter sets the completion function that Liner will call to fetch completion candidates when the user presses tab.
func (l *Liner) SetCompleter(f func(lines []string) []string) {
	l.liner.SetCompleter(func(line string) []string {
		return f(append(l.lines, line))
	})
}
