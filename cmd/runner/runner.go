package runner

import (
	"bytes"
	"context"
	"fmt"
	"go/scanner"
	"go/token"
	"go/types"
	"io"
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

func loadShared(ctx core.LgoContext, buildPkgDir, pkgPath string) error {
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
	if lgoInitFuncPC == nil {
		// lgo_init does not exist if lgo source includes only declarations.
		return nil
	}
	lgoInitFuncP := &lgoInitFuncPC
	lgoInitFunc := *(*func())(unsafe.Pointer(&lgoInitFuncP))
	return core.ExecLgoEntryPoint(ctx, func() {
		lgoInitFunc()
	})
}

func loadSharedInternal(buildPkgDir, pkgPath string) {
}

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

const maxErrLines = 5

// PrintError prints err to w.
// If err is scanner.ErrorList or convert.ErrorList, it expands internal errors.
func PrintError(w io.Writer, err error) {
	var length int
	var get func(int) error
	if lst, ok := err.(scanner.ErrorList); ok {
		length = len(lst)
		get = func(i int) error { return lst[i] }
	} else if lst, ok := err.(converter.ErrorList); ok {
		length = len(lst)
		get = func(i int) error { return lst[i] }
	} else {
		fmt.Fprintln(w, err.Error())
		return
	}
	for i := 0; i < maxErrLines && i < length; i++ {
		msg := get(i).Error()
		if i == maxErrLines-1 && i != length-1 {
			msg += fmt.Sprintf(" (and %d more errors)", length-1-i)
		}
		fmt.Fprintln(w, msg)
	}
}

const lgoExportPrefix = "LgoExport_"

func (rn *LgoRunner) Run(ctx core.LgoContext, src string) error {
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
	result := converter.Convert(src, &converter.Config{
		Olds:         olds,
		OldImports:   oldImports,
		DefPrefix:    lgoExportPrefix,
		RefPrefix:    lgoExportPrefix,
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
	if err := os.MkdirAll(pkgDir, 0766); err != nil {
		return err
	}
	filePath := path.Join(pkgDir, "src.go")
	err := ioutil.WriteFile(filePath, []byte(result.Src), 0666)
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
	return loadShared(ctx, buildPkgDir, pkgPath)
}

func (rn *LgoRunner) Complete(ctx context.Context, src string, index int) (matches []string, start, end int) {
	var olds []types.Object
	// TODO: Protect rn.vars and rn.imports with locks to make them goroutine safe.
	for _, obj := range rn.vars {
		olds = append(olds, obj)
	}
	var oldImports []*types.PkgName
	for _, im := range rn.imports {
		oldImports = append(oldImports, im)
	}
	matches, start, end = converter.Complete(src, token.Pos(index+1), &converter.Config{
		Olds:       olds,
		OldImports: oldImports,
		DefPrefix:  lgoExportPrefix,
		RefPrefix:  lgoExportPrefix,
	})
	return
}

// Inspect analyzes src and returns the document of an identifier at index (0-based).
func (rn *LgoRunner) Inspect(ctx context.Context, src string, index int) (string, error) {
	var olds []types.Object
	// TODO: Protect rn.vars and rn.imports with locks to make them goroutine safe.
	for _, obj := range rn.vars {
		olds = append(olds, obj)
	}
	var oldImports []*types.PkgName
	for _, im := range rn.imports {
		oldImports = append(oldImports, im)
	}
	doc, query := converter.InspectIdent(src, token.Pos(index+1), &converter.Config{
		Olds:       olds,
		OldImports: oldImports,
		DefPrefix:  lgoExportPrefix,
		RefPrefix:  lgoExportPrefix,
	})
	if doc != "" {
		return doc, nil
	}
	if query == "" {
		return "", nil
	}
	cmd := exec.CommandContext(ctx, "go", "doc", query)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.Replace(buf.String(), lgoExportPrefix, "", -1), nil
}
