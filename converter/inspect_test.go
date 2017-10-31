package converter

import (
	"go/token"
	"strings"
	"testing"
)

func TestInspectRefs(t *testing.T) {
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
			name: "global variable",
			src: `
			import (
				"fmt"
			)

			var (
				s int
			)

			func sum(x, y int) { [cur]s = x + y }`,
			// TODO: Fix this
			doc: "var cmd/hello.s int",
		},
		{
			name: "global_const",
			src: `
			const myVal = 10
			x := [cur]myVal * 10
			`,
			// TODO: Fix this
			doc: "const cmd/hello.myVal untyped int",
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
		},
		{
			name: "interface_method",
			src: `
			import (
				"bytes"
				"io"
			)

			var buf bytes.Buffer
			var r io.Reader = &buf
			r.[cur]Read(nil)`,
			query: "io.Reader.Read",
		},
		{
			name: "type",
			src: `
			import (
				"flag"
			)

			f := flag.F[cur]lag{}`,
			query: "flag.Flag",
		},
		{
			name: "field",
			src: `
			import (
				"flag"
			)

			f := flag.Flag{[cur]Name: "myflag"}`,
		},
		{
			name: "invalid type",
			src: `
			var x foobar
			[cur]x + 10`,
		},
		{
			name: "invalid const val",
			src: `
			func sum(x, y int) int { return x + y }
			const x = sum(10, 20)
			[cur]x + 10`,
			// TODO: Fix this
			doc: "const cmd/hello.x invalid type",
		},
		{
			name: "invalid syntax",
			src:  `[cur]x := 3 +`,
		},
		{
			name: "invalid syntax after cur",
			src: `[cur]x := 3 + 4
			y := x +`,
			doc: "var cmd/hello.x int",
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
			if tt.doc != doc {
				t.Errorf("Expected %q but got %q", tt.doc, doc)
			}
			if tt.query != query {
				t.Errorf("Expected %q but got %q", tt.query, query)
			}
		})
	}
}
