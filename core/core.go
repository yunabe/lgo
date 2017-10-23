package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
)

// SelfPkgPath is the package path of this package
const SelfPkgPath = "github.com/yunabe/lgo/core"

// isRunning indicates lgo execution is running.
// This var is used to improve the performance of ExitIfCtxDone.
// To access this var, use atomic.Store/LoadUint32.
var isRunning uint32

type ExecutionState struct {
	Context     context.Context
	cancelCtx   func()
	canceled    bool
	cancelMu    sync.Mutex
	goroutineWg sync.WaitGroup
}

func newExecutionState(parent context.Context) *ExecutionState {
	ctx, cancel := context.WithCancel(parent)
	e := &ExecutionState{
		Context:   ctx,
		cancelCtx: cancel,
	}
	go func() {
		<-parent.Done()
		e.cancel()
	}()
	return e
}

func (e *ExecutionState) cancel() {
	e.cancelMu.Lock()
	if e.canceled {
		e.cancelMu.Unlock()
		return
	}
	e.canceled = true
	e.cancelMu.Unlock()

	if getExecState() == e {
		atomic.StoreUint32(&isRunning, 0)
	}
	e.cancelCtx()
}

// execState should be protected with a mutex because
// InitGoroutine, FinalizeGoroutine and ExitIfCtxDone might be called after
// a lgo execution finishes and execState is modified if there are goroutines which
// are not terminated properly when the context is canceled.
var execState *ExecutionState
var execStateMu sync.Mutex

// canceledCtx is used to return an canceled context when GetExecContext() is invoked when execState is nil.
var canceledCtx context.Context

func init() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	canceledCtx = ctx
}

func GetExecContext() context.Context {
	if e := getExecState(); e != nil {
		return e.Context
	}
	return canceledCtx
}

func getExecState() *ExecutionState {
	execStateMu.Lock()
	defer execStateMu.Unlock()
	return execState
}

func setExecState(e *ExecutionState) {
	execStateMu.Lock()
	defer execStateMu.Unlock()
	execState = e
}

func StartExec(parent context.Context) {
	atomic.StoreUint32(&isRunning, 1)
	setExecState(newExecutionState(parent))
}

func FinalizeExec() {
	e := getExecState()
	e.cancel()
	e.goroutineWg.Wait()
	setExecState(nil)
}

func InitGoroutine() (e *ExecutionState) {
	e = getExecState()
	if e == nil {
		return
	}
	e.goroutineWg.Add(1)
	return
}

func FinalizeGoroutine(e *ExecutionState) {
	r := recover()
	if r != nil && r != Bailout {
		fmt.Fprintf(os.Stderr, "panic: %v\n\n%s", r, debug.Stack())
	}
	cur := getExecState()
	if cur != e || cur == nil {
		// The goroutine did not terminated properly in the previous run.
		return
	}
	e.goroutineWg.Done()
	if r != nil {
		// paniced
		e.cancel()
	}
	return
}

type LgoPrinter interface {
	Println(args ...interface{})
}

var lgoPrinters = make(map[LgoPrinter]bool)

var Bailout = errors.New("canceled")

func ExitIfCtxDone() {
	running := atomic.LoadUint32(&isRunning)
	if running == 1 {
		// If running, do nothing.
		return
	}
	// Slow operation
	select {
	case <-GetExecContext().Done():
		panic(Bailout)
	default:
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
