package main

import (
	"context"
	"flag"
	"fmt"
	"html"
	"os"
	"time"

	scaffold "github.com/yunabe/lgo/jupyter/gojupyterscaffold"
)

var connectionFile = flag.String("connection_file", "", "")

type handlers struct{}

func (*handlers) HandleKernelInfo() scaffold.KernelInfo {
	return scaffold.KernelInfo{
		ProtocolVersion:       "5.2",
		Implementation:        "GoJupyterScaffoldKernel",
		ImplementationVersion: "1.2.3",
		LanguageInfo: scaffold.KernelLanguageInfo{
			Name: "javascript",
		},
		Banner: "gojupyterscaffold example",
	}
}

func (*handlers) HandleExecuteRequest(
	ctx context.Context,
	r *scaffold.ExecuteRequest,
	stream func(string, string),
	displayData func(data *scaffold.DisplayData, update bool)) *scaffold.ExecuteResult {
	var i int
	tick := time.Tick(time.Second)
	cancelled := false
loop:
	for ; i < 5; i++ {
		displayData(&scaffold.DisplayData{
			Data: map[string]interface{}{
				"text/plain": fmt.Sprintf("Loop: i == %d", i),
			},
			Transient: map[string]interface{}{
				"display_id": "hogehoge",
			},
			Metadata: map[string]interface{}{},
		}, i > 0)
		stream("stdout", fmt.Sprintf("hello --- %d ---", i))
		stream("stderr", "world")
		displayData(&scaffold.DisplayData{
			Data: map[string]interface{}{
				"text/html": "Hello <b>World</b>",
			},
			Metadata: map[string]interface{}{},
		}, false)
		select {
		case <-tick:
		case <-ctx.Done():
			cancelled = true
			break loop
		}
	}
	res := &scaffold.ExecuteResult{ExecutionCount: i}
	if cancelled {
		res.Status = "error"
		stream("stderr", "Cancel!")
	} else {
		res.Status = "ok"
		stream("stdout", "Done!")
	}
	return res
}

func (*handlers) HandleComplete(req *scaffold.CompleteRequest) *scaffold.CompleteReply {
	return &scaffold.CompleteReply{
		Status:      "ok",
		Matches:     []string{"abc", "xyz", "123", "こんにちは"},
		CursorStart: req.CursorPos,
		CursorEnd:   req.CursorPos,
	}
}

func (*handlers) HandleInspect(req *scaffold.InspectRequest) *scaffold.InspectReply {
	return &scaffold.InspectReply{
		Status: "ok",
		Found:  true,
		Data: map[string]interface{}{
			// text/plain is shown when Shift+Tab is pushed.
			"text/plain": fmt.Sprintf("Code: %q, pos: %d, detail: %d", req.Code, req.CursorPos, req.DetailLevel),
			// text/html is shown when an inspection window is expanded.
			"text/html": fmt.Sprintf("code: <pre>%s</pre>pos: <b>%d</b><br>detail: <i>%d</i>",
				html.EscapeString(req.Code), req.CursorPos, req.DetailLevel),
		},
	}
}

func (*handlers) HandleIsComplete(req *scaffold.IsCompleteRequest) *scaffold.IsCompleteReply {
	return nil
}

func main() {
	flag.Parse()
	fmt.Printf("os.Args == %+v\n", os.Args)
	fmt.Printf("connection_file == %s\n", *connectionFile)

	server, err := scaffold.NewServer(*connectionFile, &handlers{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create a server: %v\n", err)
		os.Exit(1)
	}
	server.Loop()
}
