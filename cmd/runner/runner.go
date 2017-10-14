package runner

import (
	"context"
	"errors"
	"fmt"
	"go/types"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"unsafe"

	"github.com/yunabe/lgo/converter"
	"github.com/yunabe/lgo/core"
)

/*
#cgo linux LDFLAGS: -ldl
#include <dlfcn.h>
*/
import "C"

func loadShared(ctx context.Context, buildPkgDir, pkgPath string) (canceled bool) {
	core.Runctx = ctx
	defer func() {
		core.Runctx = nil
		r := recover()
		if r == core.Bailout {
			canceled = true
		} else if r != nil {
			// repanic
			panic(r)
		}
	}()
	loadSharedInternal(buildPkgDir, pkgPath)
	return
}

func loadSharedInternal(buildPkgDir, pkgPath string) {
	// This code is implemented based on https://golang.org/src/plugin/plugin_dlopen.go
	sofile := "lib" + strings.Replace(pkgPath, "/", "-", -1) + ".so"
	handle := C.dlopen(C.CString(path.Join(buildPkgDir, sofile)), C.RTLD_NOW|C.RTLD_GLOBAL)
	if handle == nil {
		panic("Failed to open shared object.")
	}

	// Initialize freshly loaded modules
	// c.f. plugin_lastmoduleinit in https://golang.org/src/runtime/plugin.go
	modulesinit()
	typelinksinit()
	itabsinit()

	// Don't forget to call init.
	// TODO: Write unit tests to confirm this.
	initFuncPC := C.dlsym(handle, C.CString(pkgPath+".init"))
	if initFuncPC != nil {
		// Note: init does not exist if the library does not use external libraries.
		initFuncP := &initFuncPC
		initFunc := *(*func())(unsafe.Pointer(&initFuncP))
		initFunc()
	}

	lgoInitFuncPC := C.dlsym(handle, C.CString(pkgPath+".lgo_init"))
	if lgoInitFuncPC != nil {
		// lgo_init does not exist if lgo source includes only declarations.
		lgoInitFuncP := &lgoInitFuncPC
		lgoInitFunc := *(*func())(unsafe.Pointer(&lgoInitFuncP))
		lgoInitFunc()
	}
}

var ErrInterrupted = errors.New("interrupted")

type LgoRunner struct {
	gopath    string
	lgopath   string
	sessID    *SessionID
	execCount int64
	vars      map[string]types.Object
	imports   map[string]*types.PkgName
}

func NewLgoRunner(gopath, lgopath string, sessID *SessionID) *LgoRunner {
	return &LgoRunner{
		gopath:  gopath,
		lgopath: lgopath,
		sessID:  sessID,
		vars:    make(map[string]types.Object),
		imports: make(map[string]*types.PkgName),
	}
}

func (rn *LgoRunner) ExecCount() int64 {
	return rn.execCount
}

func (rn *LgoRunner) cleanFiles(pkgPath string) {
	// Delete src files
	os.RemoveAll(path.Join(rn.gopath, "src", pkgPath))
	libname := "lib" + strings.Replace(pkgPath, "/", "-", -1) + ".so"
	os.RemoveAll(path.Join(rn.lgopath, "pkg", libname))
	os.RemoveAll(path.Join(rn.lgopath, "pkg", pkgPath))
}

func (rn *LgoRunner) isCtxDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func (rn *LgoRunner) Run(ctx context.Context, src []byte) error {
	rn.execCount++
	sessDir := "github.com/yunabe/lgo/" + rn.sessID.Marshal()
	pkgPath := path.Join(sessDir, fmt.Sprintf("exec%d", rn.execCount))
	var olds []types.Object
	for _, obj := range rn.vars {
		olds = append(olds, obj)
	}
	var oldImports []*types.PkgName
	for _, im := range rn.imports {
		oldImports = append(oldImports, im)
	}
	result := converter.Convert(string(src), &converter.Config{
		Olds:         olds,
		OldImports:   oldImports,
		DefPrefix:    "LgoExport_",
		RefPrefix:    "LgoExport_",
		LgoPkgPath:   pkgPath,
		AutoExitCode: true,
		RegisterVars: true,
	})
	// converted, pkg, _, err
	if result.Err != nil {
		return result.Err
	}
	for _, name := range result.Pkg.Scope().Names() {
		rn.vars[name] = result.Pkg.Scope().Lookup(name)
	}
	for _, im := range result.Imports {
		rn.imports[im.Name()] = im
	}
	if len(result.Src) == 0 {
		// No declarations or expressions in the original source (e.g. only import statements).
		return nil
	}
	pkgDir := path.Join(rn.gopath, "src", pkgPath)
	os.MkdirAll(pkgDir, 0766)
	filePath := path.Join(pkgDir, "src.go")
	err := ioutil.WriteFile(filePath, result.Src, 0666)
	if err != nil {
		return err
	}
	buildPkgDir := path.Join(rn.lgopath, "pkg")
	cmd := exec.CommandContext(ctx, "go", "install", "-buildmode=shared", "-linkshared", "-pkgdir", buildPkgDir, pkgPath)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to build a shared library of %s: %v", pkgPath, err)
	}
	cancelled := loadShared(ctx, buildPkgDir, pkgPath)
	if cancelled {
		return ErrInterrupted
	}
	return nil
}

func (rn *LgoRunner) CleanUp() error {
	return CleanSession(rn.gopath, rn.lgopath, rn.sessID)
}
