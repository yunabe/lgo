package main

import (
	"context"
	"flag"
	"fmt"
	"log"
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
	displayData func(*scaffold.DisplayData)) *scaffold.ExecuteResult {
	var i int
	tick := time.Tick(time.Second)
	cancelled := false
loop:
	for ; i < 10; i++ {
		stream("stdout", fmt.Sprintf("hello --- %d ---", i))
		stream("stderr", "world")
		displayData(&scaffold.DisplayData{
			Data: map[string]interface{}{
				"text/html": "Hello <b>World</b>",
			},
			Metadata: map[string]interface{}{},
		})
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

func main() {
	flag.Parse()
	log.Printf("os.Args == %+v", os.Args)
	log.Printf("connection_file == %s", *connectionFile)

	server, err := scaffold.NewServer(*connectionFile, &handlers{})
	if err != nil {
		log.Fatalf("Failed to create a server: %v", err)
	}
	server.Loop()
}
