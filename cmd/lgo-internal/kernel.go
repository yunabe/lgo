package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"runtime/debug"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang/glog"
	"github.com/yunabe/lgo/cmd/lgo-internal/liner"
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
					glog.Error(err)
				}
				break
			}
		}
		if err := r.Close(); err != nil {
			glog.Errorf("Failed to close a reader pipe: %v", err)
		}
		done <- struct{}{}
	}()
	close = func() error {
		*file = orig
		return w.Close()
	}
	return close, nil
}

type jupyterDisplayer func(data *scaffold.DisplayData, update bool)

func init() {
	// Initialize the seed to use it from display.
	rand.Seed(time.Now().UnixNano())
}

func (d jupyterDisplayer) display(data *scaffold.DisplayData, id *string) {
	update := false
	if id != nil {
		if *id == "" {
			var buf [16]byte
			rand.Read(buf[:])
			var enc [32]byte
			hex.Encode(enc[:], buf[:])
			*id = "displayid_" + string(enc[:])
		} else {
			update = true
		}
		if data.Transient == nil {
			data.Transient = make(map[string]interface{})
		}
		data.Transient["display_id"] = *id
	}
	d(data, update)
}

func (d jupyterDisplayer) displayString(contentType, content string, id *string) {
	d.display(&scaffold.DisplayData{
		Data: map[string]interface{}{
			contentType: content,
		},
	}, id)
}

func (d jupyterDisplayer) displayBytes(contentType string, content []byte, id *string) {
	d.display(&scaffold.DisplayData{
		Data: map[string]interface{}{
			contentType: content,
		},
	}, id)
}

func (d jupyterDisplayer) JavaScript(s string, id *string) {
	d.displayString("application/javascript", s, id)
}
func (d jupyterDisplayer) HTML(s string, id *string)     { d.displayString("text/html", s, id) }
func (d jupyterDisplayer) Markdown(s string, id *string) { d.displayString("text/markdown", s, id) }
func (d jupyterDisplayer) Latex(s string, id *string)    { d.displayString("text/latex", s, id) }
func (d jupyterDisplayer) SVG(s string, id *string)      { panic("Not implemented") }
func (d jupyterDisplayer) PNG(b []byte, id *string)      { d.displayBytes("image/png", b, id) }
func (d jupyterDisplayer) JPEG(b []byte, id *string)     { d.displayBytes("image/jpeg", b, id) }
func (d jupyterDisplayer) GIF(b []byte, id *string)      { d.displayBytes("image/gif", b, id) }
func (d jupyterDisplayer) PDF(b []byte, id *string)      { d.displayBytes("application/pdf", b, id) }
func (d jupyterDisplayer) Text(s string, id *string)     { d.displayString("text/plain", s, id) }

func (h *handlers) HandleExecuteRequest(ctx context.Context, r *scaffold.ExecuteRequest, stream func(string, string), displayData func(data *scaffold.DisplayData, update bool)) *scaffold.ExecuteResult {
	h.execCount++
	rDone := make(chan struct{})
	soClose, err := pipeOutput(func(msg string) {
		stream("stdout", msg)
	}, &os.Stdout, rDone)
	if err != nil {
		glog.Errorf("Failed to open stdout pipe: %v", err)
		return &scaffold.ExecuteResult{
			Status:         "error",
			ExecutionCount: h.execCount,
		}
	}
	seClose, err := pipeOutput(func(msg string) {
		stream("stderr", msg)
	}, &os.Stderr, rDone)
	if err != nil {
		glog.Errorf("Failed to open stderr pipe: %v", err)
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
		if err = h.runner.Run(lgoCtx, r.Code); err != nil {
			runner.PrintError(os.Stderr, err)
		}
	}()
	soClose()
	seClose()
	<-rDone
	<-rDone
	if err != nil {
		return &scaffold.ExecuteResult{
			Status:         "error",
			ExecutionCount: h.execCount,
		}
	}
	return &scaffold.ExecuteResult{
		Status:         "ok",
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
	matches, start, end := h.runner.Complete(context.Background(), req.Code, offset)
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
		glog.Errorf("Failed to inspect: %v", err)
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

func (*handlers) HandleIsComplete(req *scaffold.IsCompleteRequest) *scaffold.IsCompleteReply {
	cont, indent := liner.ContinueLineString(req.Code)
	if cont {
		return &scaffold.IsCompleteReply{
			Status: "incomplete",
			// Use 4-spaces instead of "\t" because jupyter console prints "^I" for "\t".
			Indent: strings.Repeat("    ", indent),
		}
	}
	return &scaffold.IsCompleteReply{
		Status: "complete",
	}
}

// kernelLogWriter forwards messages to the current os.Stderr, which is change on every execution.
type kernelLogWriter struct{}

func (kernelLogWriter) Write(p []byte) (n int, err error) {
	return os.Stderr.Write(p)
}

func kernelMain(gopath, lgopath string, sessID *runner.SessionID) {
	log.SetOutput(kernelLogWriter{})
	server, err := scaffold.NewServer(*connectionFile, &handlers{
		runner: runner.NewLgoRunner(gopath, lgopath, sessID),
	})
	if err != nil {
		glog.Fatalf("Failed to create a server: %v", err)
	}

	// Start the server loop
	server.Loop()
	// clean-up
	glog.Infof("Clean the session: %s", sessID.Marshal())
	runner.CleanSession(gopath, lgopath, sessID)
}
