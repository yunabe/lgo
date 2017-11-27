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

// A LgoContext carries a context of lgo execution.
type LgoContext struct {
	// Go context.Context.
	context.Context
	// Display displays non-text content in Jupyter Notebook.
	Display DataDisplayer
}

func lgoCtxWithCancel(ctx LgoContext) (LgoContext, context.CancelFunc) {
	goctx, cancel := context.WithCancel(ctx.Context)
	return LgoContext{goctx, ctx.Display}, cancel
}

// DataDisplayer is the interface that wraps Jupyter Notebook display_data protocol.
// The list of supported content types are based on Jupyter Notebook implementation[2].
// Each method receives a content and an display id. If id is nil, the method does not use id.
// If id is not nil and it points an empty string, the method reserves a new display ID and stores it to id.
// If id is not nil and it points a non-empty string, the method overwrites a content with the same ID in Jupyter Notebooks.
//
// References:
// [1] http://jupyter-client.readthedocs.io/en/latest/messaging.html#display-data
// [2] https://github.com/jupyter/notebook/blob/master/notebook/static/notebook/js/outputarea.js
type DataDisplayer interface {
	JavaScript(s string, id *string)
	HTML(s string, id *string)
	Markdown(s string, id *string)
	Latex(s string, id *string)
	SVG(s string, id *string)
	PNG(b []byte, id *string)
	JPEG(b []byte, id *string)
	GIF(b []byte, id *string)
	PDF(b []byte, id *string)
	Text(s string, id *string)
}

type ExecutionState struct {
	Context     LgoContext
	cancelCtx   func()
	canceled    bool
	cancelMu    sync.Mutex
	goroutineWg sync.WaitGroup
}

func newExecutionState(parent LgoContext) *ExecutionState {
	ctx, cancel := lgoCtxWithCancel(parent)
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
var canceledCtx LgoContext

func init() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	canceledCtx = LgoContext{Context: ctx}
}

func GetExecContext() LgoContext {
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

func StartExec(parent LgoContext) {
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
