// Function declarations to access unexported methods in runtime package.
// This technique to access unexported methods is used in other packages
// (e.g. byteIndex in https://golang.org/src/net/parse.go)
//
// Do not add import "C" to this file because //go:linkname w/o function body is ignored for some reason
// if import "C" is declared in the same file.
// (See "Getting go:linkname to work" in http://www.alangpierce.com/blog/2016/03/17/adventures-in-go-accessing-unexported-functions/)

package runner

import (
	_ "unsafe" // for go:linkname
)

//go:linkname modulesinit runtime.modulesinit
func modulesinit()

//go:linkname typelinksinit runtime.typelinksinit
func typelinksinit()

//go:linkname itabsinit runtime.itabsinit
func itabsinit()
