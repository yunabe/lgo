package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SelfPkgPath is the package path of this package
const SelfPkgPath = "github.com/yunabe/lgo/core"

// How long time we should wait for goroutines after a cancel operation.
var execWaitDuration = time.Second

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

type resultCounter struct {
	active uint
	fail   uint
	cancel uint
	mu     sync.Mutex
}

func (c *resultCounter) add() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active++
}

// recordResult records a result of a routine based on the value of recover().
func (c *resultCounter) recordResult(r interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active--
	if c.active < 0 {
		panic("active is negative")
	}
	if r == nil {
		return
	}
	if r == Bailout {
		c.cancel += 1
		return
	}
	fmt.Fprintf(os.Stderr, "panic: %v\n\n%s", r, debug.Stack())
	c.fail += 1
}

func (c *resultCounter) recordResultInDefer() {
	c.recordResult(recover())
}

type ExecutionState struct {
	Context   LgoContext
	cancelCtx func()
	canceled  bool
	cancelMu  sync.Mutex

	mainCounter resultCounter
	subCounter  resultCounter
	routineWait sync.WaitGroup
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

func (e *ExecutionState) counterMessage() string {
	var msgs []string
	func() {
		e.mainCounter.mu.Lock()
		defer e.mainCounter.mu.Unlock()
		if e.mainCounter.fail > 0 {
			msgs = append(msgs, "main routine failed")
		} else if e.mainCounter.cancel > 0 {
			msgs = append(msgs, "main routine canceled")
		} else if e.mainCounter.active > 0 {
			msgs = append(msgs, "main routine is hanging")
		}
	}()
	func() {
		e.subCounter.mu.Lock()
		defer e.subCounter.mu.Unlock()
		if c := e.subCounter.fail; c > 1 {
			msgs = append(msgs, fmt.Sprintf("%d goroutines failed", c))
		} else if c == 1 {
			msgs = append(msgs, fmt.Sprintf("%d goroutine failed", c))
		}
		if c := e.subCounter.cancel; c > 1 {
			msgs = append(msgs, fmt.Sprintf("%d goroutines canceled", c))
		} else if c == 1 {
			msgs = append(msgs, fmt.Sprintf("%d goroutine canceled", c))
		}
		if c := e.subCounter.active; c > 1 {
			msgs = append(msgs, fmt.Sprintf("%d goroutines are hanging", c))
		} else if c == 1 {
			msgs = append(msgs, fmt.Sprintf("%d goroutine is hanging", c))
		}
	}()
	return strings.Join(msgs, ", ")
}

func (e *ExecutionState) waitRoutines() {
	ctx, done := context.WithCancel(context.Background())
	go func() {
		e.routineWait.Wait()
		done()
		// Don't forget to cancel the current ctx to avoid ctx leak.
		e.cancel()
	}()
	go func() {
		<-e.Context.Done()
		time.Sleep(execWaitDuration)
		done()
	}()
	// Wait done is called.
	<-ctx.Done()
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

func resetExecState(e *ExecutionState) {
	execStateMu.Lock()
	defer execStateMu.Unlock()
	if execState == e {
		execState = nil
	}
}

func ExecLgoEntryPoint(parent LgoContext, main func()) error {
	return finalizeExec(startExec(parent, main))
}

func startExec(parent LgoContext, main func()) *ExecutionState {
	atomic.StoreUint32(&isRunning, 1)
	e := newExecutionState(parent)
	setExecState(e)

	e.routineWait.Add(1)
	e.mainCounter.add()
	go func() {
		defer e.routineWait.Done()
		defer e.mainCounter.recordResultInDefer()
		main()
	}()
	return e
}

func finalizeExec(e *ExecutionState) error {
	e.waitRoutines()
	resetExecState(e)
	if msg := e.counterMessage(); msg != "" {
		return errors.New(msg)
	}
	return nil
}

func InitGoroutine() (e *ExecutionState) {
	e = getExecState()
	if e == nil {
		return
	}
	e.routineWait.Add(1)
	e.subCounter.add()
	return
}

func FinalizeGoroutine(e *ExecutionState) {
	r := recover()
	e.subCounter.recordResult(r)
	e.routineWait.Done()
	if r != nil {
		// paniced, cancel other routines.
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
