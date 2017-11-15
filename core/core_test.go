package core

import (
	"context"
	"sync/atomic"
	"testing"
)

func TestExecutionContext(t *testing.T) {
	atomic.StoreUint32(&isRunning, 0)
	StartExec(LgoContext{Context: context.Background()})

	if running := atomic.LoadUint32(&isRunning); running != 1 {
		t.Errorf("Expected 1 but got %d", running)
	}
	e := getExecState()
	select {
	case <-e.Context.Done():
		t.Error("e.Context is canceled unexpectedly")
	default:
	}
	// Nothing happens
	ExitIfCtxDone()

	e.cancel()

	select {
	case <-e.Context.Done():
	default:
		t.Error("e.Context is not canceled")
	}
	if running := atomic.LoadUint32(&isRunning); running != 0 {
		t.Errorf("Expected 0 but got %d", running)
	}
	defer func() {
		r := recover()
		if r != Bailout {
			t.Errorf("ExitIfDone paniced with an expected value: %v", r)
		}
	}()
	// panic with Bailout
	ExitIfCtxDone()
}
