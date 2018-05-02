package converter

import (
	"fmt"
	"testing"
)

func TestFormat(t *testing.T) {
	tests := []struct {
		name string
		src  string
		err  string
		want string
		// If true, use a golden file instead of watn.
		useGolden bool
	}{{
		name: "error",
		src:  "\"",
		err:  "1:1: string literal not terminated",
	}, {
		name: "oneline",
		src:  "x:=10",
		want: "x := 10",
	}, {
		name: "twolines",
		src:  "x := 10\ny:=x*x",
		want: "x := 10\ny := x * x",
	}, {
		name: "leading_trailing_emptylines",
		src: `

var x int

`,
		want: "var x int",
	}, {
		name: "fixindent",
		src: `func f(x int) {
if(x > 0) {
return x
}
return 0
}
`,
		want: `func f(x int) {
    if x > 0 {
        return x
    }
    return 0
}`,
	}, {
		name: "sort_imports",
		src: `import (
"sort"

"fmt"
"github.com/lgo"
"bytes"
)
}`,
		want: `import (
    "sort"

    "bytes"
    "fmt"
    "github.com/lgo"
)`,
	}, {
		name: "comment_only1",
		src:  "// comment",
		want: "// comment",
	}, {
		name: "comment_only2",
		src:  "/* comnent */",
		want: "/* comnent */",
	}, {
		name:      "golden",
		useGolden: true,
		src: `// first line comment

// f is a func
func f() {

//   f body
}
// g is a func
func g(){}

// h is a func
func h() {/*do nothing*/}

// i is an int
var i int

type s struct{
n float32
   xy int
abc string

// comment1
z bool // comment2
}
`,
	},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Format(tc.src)
			if err == nil && tc.err != "" {
				t.Fatalf("got no error; want %q", tc.err)
			}
			if err != nil && err.Error() != tc.err {
				t.Fatalf("got %q; want %q", err, tc.err)
			}
			if tc.useGolden {
				checkGolden(t, got, fmt.Sprintf("testdata/format_%s.golden", tc.name))
				return
			}
			if got != tc.want {
				t.Fatalf("got %q; want %q", got, tc.want)
			}
		})
	}
}
