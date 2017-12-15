package core

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestExecutionContextCancel(t *testing.T) {
	atomic.StoreUint32(&isRunning, 0)
	startExec(LgoContext{Context: context.Background()}, func() {})

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

func TestMainCounters(t *testing.T) {
	tests := []struct {
		name    string
		body    func()
		message string
	}{
		{
			name: "ok",
			body: func() {},
		}, {
			name:    "fail",
			message: "main routine failed",
			body:    func() { panic("fail") },
		}, {
			name:    "cancel",
			message: "main routine canceled",
			body:    func() { panic(Bailout) },
		}, {
			name:    "gofail",
			message: "1 goroutine failed",
			body: func() {
				state := InitGoroutine()
				go func() {
					defer FinalizeGoroutine(state)
					panic("fail")
				}()
			},
		}, {
			name:    "gocancel",
			message: "1 goroutine canceled",
			body: func() {
				state := InitGoroutine()
				go func() {
					defer FinalizeGoroutine(state)
					panic(Bailout)
				}()
			},
		}, {
			name:    "gofailmulti",
			message: "2 goroutines failed",
			body: func() {
				for i := 0; i < 2; i++ {
					state := InitGoroutine()
					go func() {
						defer FinalizeGoroutine(state)
						panic("fail")
					}()
				}
			},
		}, {
			name:    "gocancelmulti",
			message: "3 goroutines canceled",
			body: func() {
				for i := 0; i < 3; i++ {
					state := InitGoroutine()
					go func() {
						defer FinalizeGoroutine(state)
						panic(Bailout)
					}()
				}
			},
		}, {
			name:    "gomix",
			message: "1 goroutine failed, 2 goroutines canceled",
			body: func() {
				state := InitGoroutine()
				go func() {
					defer FinalizeGoroutine(state)
					panic("fail")
				}()
				for i := 0; i < 2; i++ {
					state := InitGoroutine()
					go func() {
						defer FinalizeGoroutine(state)
						panic(Bailout)
					}()
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			atomic.StoreUint32(&isRunning, 0)
			ch := make(chan struct{})
			state := startExec(LgoContext{Context: context.Background()}, func() {
				<-ch
				tc.body()
			})
			initMsg := "main routine is hanging"
			if msg := state.counterMessage(); msg != initMsg {
				t.Fatalf("Got %q; want %q", msg, initMsg)
			}
			close(ch)
			var msg string
			if err := finalizeExec(state); err != nil {
				msg = err.Error()
			}
			if msg != tc.message {
				t.Fatalf("Got %q; want %q", msg, tc.message)
			}
			if err := state.Context.Err(); err != context.Canceled {
				// state.Context must be canceled.
				t.Errorf("Unexpected err: %v", err)
			}
		})
	}
}

func TestFinalizeExecTimeout(t *testing.T) {
	execWaitDuration = 10 * time.Millisecond

	atomic.StoreUint32(&isRunning, 0)
	state := startExec(LgoContext{Context: context.Background()}, func() {
		time.Sleep(100 * time.Millisecond)
	})
	state.cancel()
	var msg string
	if err := finalizeExec(state); err != nil {
		msg = err.Error()
	}
	want := "main routine is hanging"
	if msg != want {
		t.Fatalf("Got %q; want %q", msg, want)
	}
	if err := state.Context.Err(); err != context.Canceled {
		// state.Context must be canceled.
		t.Errorf("Unexpected err: %v", err)
	}
}
