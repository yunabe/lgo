package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/yunabe/lgo/cmd/runner"
	scaffold "github.com/yunabe/lgo/jupyter/gojupyterscaffold"
)

type handlers struct {
	runner    *runner.LgoRunner
	execCount int
}

func (*handlers) HandleKernelInfo() scaffold.KernelInfo {
	return scaffold.KernelInfo{
		ProtocolVersion:       "5.2",
		Implementation:        "lgo",
		ImplementationVersion: "0.0.1",
		LanguageInfo: scaffold.KernelLanguageInfo{
			Name: "go",
		},
		Banner: "lgo",
	}
}

func pipeOutput(send func(string), file **os.File, done chan<- struct{}) (close func() error, err error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	orig := *file
	*file = w
	go func() {
		utf8R := newUTF8AwareReader(r)
		var buf [4096]byte
		for {
			n, err := utf8R.Read(buf[:])
			if n > 0 {
				send(string(buf[:n]))
			}
			if err != nil {
				if err != io.EOF {
					log.Println(err)
				}
				break
			}
		}
		if err := r.Close(); err != nil {
			log.Printf("Failed to close a reader pipe: %v", err)
		}
		done <- struct{}{}
	}()
	close = func() error {
		*file = orig
		return w.Close()
	}
	return close, nil
}

func (h *handlers) HandleExecuteRequest(ctx context.Context, r *scaffold.ExecuteRequest, stream func(string, string), displayData func(*scaffold.DisplayData)) *scaffold.ExecuteResult {
	h.execCount++
	rDone := make(chan struct{})
	soClose, err := pipeOutput(func(msg string) {
		stream("stdout", msg)
	}, &os.Stdout, rDone)
	if err != nil {
		log.Printf("Failed to open stdout pipe: %v", err)
		return &scaffold.ExecuteResult{
			Status:         "error",
			ExecutionCount: h.execCount,
		}
	}
	seClose, err := pipeOutput(func(msg string) {
		stream("stderr", msg)
	}, &os.Stderr, rDone)
	if err != nil {
		log.Printf("Failed to open stderr pipe: %v", err)
		return &scaffold.ExecuteResult{
			Status:         "error",
			ExecutionCount: h.execCount,
		}
	}
	// Print the err in the notebook
	if err = h.runner.Run(ctx, []byte(r.Code)); err != nil {
		fmt.Fprint(os.Stderr, err)
	}
	soClose()
	seClose()
	<-rDone
	<-rDone
	if err != nil {
		log.Printf("Failed to execute code: %v", err)
		return &scaffold.ExecuteResult{
			Status:         "error",
			ExecutionCount: h.execCount,
		}
	}
	log.Print("Run ends with OK")
	return &scaffold.ExecuteResult{
		// Status:         "ok",
		ExecutionCount: h.execCount,
	}
}

func kernelMain(gopath, lgopath string, sessID *runner.SessionID) {
	server, err := scaffold.NewServer(*connectionFile, &handlers{
		runner: runner.NewLgoRunner(gopath, lgopath, sessID),
	})
	if err != nil {
		log.Fatalf("Failed to create a server: %v", err)
	}

	// Set up cleanup
	startCleanup := make(chan struct{})
	endCleanup := make(chan struct{})
	go func() {
		// clean-up goroutine
		<-startCleanup
		log.Printf("Clean the session: %s", sessID.Marshal())
		runner.CleanSession(gopath, lgopath, sessID)
		close(endCleanup)
		// Terminate the process if the main routine does not return in 1 sec after the ctx is cancelled.
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
	go func() {
		// start clean-up 500ms after the ctx is cancelled.
		<-server.Context().Done()
		time.Sleep(500 * time.Millisecond)
		startCleanup <- struct{}{}
	}()

	// Start the server loop
	server.Loop()
	startCleanup <- struct{}{}
	<-endCleanup
	os.Exit(0)
}
