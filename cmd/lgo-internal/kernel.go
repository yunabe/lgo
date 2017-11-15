package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"time"
	"unicode/utf8"

	"github.com/yunabe/lgo/cmd/runner"
	"github.com/yunabe/lgo/core"
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

type jupyterDisplayer func(*scaffold.DisplayData)

// var DisplayDataUnavailable = errors.New("display_data is not available")
// type emptyDataDisplayer struct{}

func (d jupyterDisplayer) displayString(contentType, content string) {
	d(&scaffold.DisplayData{
		Data: map[string]interface{}{
			contentType: content,
		},
	})
}

func (d jupyterDisplayer) displayBytes(contentType string, content []byte) {
	d(&scaffold.DisplayData{
		Data: map[string]interface{}{
			contentType: content,
		},
	})
}

func (d jupyterDisplayer) JavaScript(s string, id *string) {
	d.displayString("application/javascript", s)
}
func (d jupyterDisplayer) HTML(s string, id *string)     { d.displayString("text/html", s) }
func (d jupyterDisplayer) Markdown(s string, id *string) { d.displayString("text/markdown", s) }
func (d jupyterDisplayer) Latex(s string, id *string)    { d.displayString("text/latex", s) }
func (d jupyterDisplayer) SVG(s string, id *string)      { panic("Not implemented") }
func (d jupyterDisplayer) PNG(b []byte, id *string)      { d.displayBytes("image/png", b) }
func (d jupyterDisplayer) JPEG(b []byte, id *string)     { d.displayBytes("image/jpeg", b) }
func (d jupyterDisplayer) GIF(b []byte, id *string)      { d.displayBytes("image/gif", b) }
func (d jupyterDisplayer) PDF(b []byte, id *string)      { d.displayBytes("application/pdf", b) }
func (d jupyterDisplayer) Text(s string, id *string)     { d.displayString("text/plain", s) }

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
	lgoCtx := core.LgoContext{
		Context: ctx, Display: jupyterDisplayer(displayData),
	}
	func() {
		defer func() {
			p := recover()
			if p != nil {
				// The return value of debug.Stack() ends with \n.
				fmt.Fprintf(os.Stderr, "panic: %v\n\n%s", p, debug.Stack())
			}
		}()
		// Print the err in the notebook
		if err = h.runner.Run(lgoCtx, []byte(r.Code)); err != nil {
			runner.PrintError(os.Stderr, err)
		}
	}()
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

func runeOffsetToByteOffset(s string, roff int) int {
	var runes int
	for boff := range s {
		runes++
		if runes > roff {
			return boff
		}
	}
	return len(s)
}

func (h *handlers) HandleComplete(req *scaffold.CompleteRequest) *scaffold.CompleteReply {
	// Not implemented
	offset := runeOffsetToByteOffset(req.Code, req.CursorPos)
	matches, start, end, err := h.runner.Complete(context.Background(), req.Code, offset)
	if err != nil {
		log.Printf("Failed to complete: %v", err)
		return nil
	}
	if len(matches) == 0 {
		return nil
	}
	runeStart := utf8.RuneCountInString(req.Code[:start])
	runeEnd := runeStart + utf8.RuneCountInString(req.Code[start:end])
	return &scaffold.CompleteReply{
		Matches:     matches,
		Status:      "ok",
		CursorStart: runeStart,
		CursorEnd:   runeEnd,
	}
}

func (h *handlers) HandleInspect(r *scaffold.InspectRequest) *scaffold.InspectReply {
	doc, err := h.runner.Inspect(context.Background(), r.Code, runeOffsetToByteOffset(r.Code, r.CursorPos))
	if err != nil {
		log.Printf("Failed to inspect: %v", err)
		return nil
	}
	if doc == "" {
		return nil
	}
	return &scaffold.InspectReply{
		Status: "ok",
		Found:  true,
		Data: map[string]interface{}{
			"text/plain": doc,
		},
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
