package core

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"runtime/debug"
)

// SelfPkgPath is the package path of this package
const SelfPkgPath = "github.com/yunabe/lgo/core"

type LgoPrinter interface {
	Println(args ...interface{})
}

var lgoPrinters = make(map[LgoPrinter]bool)

// Runctx is Context that is referred from lgo code as "runctx".
var Runctx context.Context

var Bailout = errors.New("cancelled")

// If Runctx is nil or canceled, quite.
func ExitIfCtxDone() {
	if Runctx == nil {
		panic("runctx is nil")
	}
	select {
	case <-Runctx.Done():
		panic(Bailout)
	default:
		return
	}
}

func RegisterLgoPrinter(p LgoPrinter) {
	lgoPrinters[p] = true
}

func UnregisterLgoPrinter(p LgoPrinter) {
	delete(lgoPrinters, p)
}

func LgoPrintln(args ...interface{}) {
	for p := range lgoPrinters {
		p.Println(args...)
	}
}

var AllVars = make(map[string][]interface{})

func ZeroClearAllVars() {
	for _, vars := range AllVars {
		for _, p := range vars {
			v := reflect.ValueOf(p)
			v.Elem().Set(reflect.New(v.Type().Elem()).Elem())
		}
	}
	// Return memory to OS.
	debug.FreeOSMemory()
	runtime.GC()
}

func LgoRegisterVar(name string, p interface{}) {
	v := reflect.ValueOf(p)
	if v.Kind() != reflect.Ptr {
		panic("cannot register a non-pointer")
	}
	AllVars[name] = append(AllVars[name], p)
}
