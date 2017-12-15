package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/yunabe/lgo/cmd/lgo-internal/liner"
	"github.com/yunabe/lgo/cmd/runner"
	"github.com/yunabe/lgo/core"
	"golang.org/x/sys/unix"
)

var (
	subcomandFlag  = flag.String("subcommand", "", "lgo subcommand")
	sessIDFlag     = flag.String("sess_id", "", "lgo session id")
	connectionFile = flag.String("connection_file", "", "jupyter kernel connection file path. This flag is used with kernel subcommand")
)

type printer struct{}

func (*printer) Println(args ...interface{}) {
	for _, arg := range args {
		fmt.Println(arg)
	}
}

func createRunContext(parent context.Context, sigint <-chan os.Signal) (ctx context.Context, cancel func()) {
	ctx, cancel = context.WithCancel(parent)
	go func() {
		select {
		case <-sigint:
			cancel()
		case <-ctx.Done():
		}
	}()
	return
}

func exitProcess() {
	glog.Flush()
	os.Exit(0)
}

func createProcessContext(withSigint bool) context.Context {
	// Use SIGUSR1 to notify the death of the parent process.
	unix.Prctl(unix.PR_SET_PDEATHSIG, uintptr(syscall.SIGUSR1), 0, 0, 0)

	sigch := make(chan os.Signal)
	ctx, cancel := context.WithCancel(context.Background())
	signals := []os.Signal{syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP, syscall.SIGUSR1}
	if withSigint {
		signals = append(signals, syscall.SIGINT)
	}
	signal.Notify(sigch, signals...)
	go func() {
		sig := <-sigch
		if sig == syscall.SIGUSR1 {
			glog.Info("The parent process died. Cancelling the internal process")
		} else {
			glog.Infof("Received a signal (%s). Cancelling the internal process", sig)
		}
		cancel()
	}()
	return ctx
}

func fromFiles(ctx context.Context, rn *runner.LgoRunner) {
	for _, path := range flag.Args() {
		src, err := ioutil.ReadFile(path)
		if err != nil {
			glog.Errorf("Failed to read %s: %v", path, err)
			return
		}
		if err = rn.Run(core.LgoContext{Context: ctx}, string(src)); err != nil {
			glog.Error(err)
			return
		}
	}
}

func fromStdin(ctx context.Context, rn *runner.LgoRunner) {
	ln := liner.NewLiner()
	ln.SetCompleter(func(lines []string) []string {
		if len(lines) == 0 {
			return nil
		}
		src := strings.Join(lines, "\n")
		last := lines[len(lines)-1]
		matches, start, end := rn.Complete(ctx, src, len(src))
		if len(matches) == 0 {
			return nil
		}
		start = start - (len(src) - len(last))
		end = end - (len(src) - len(last))
		if start < 0 || start > len(src) || end < 0 || end > len(src) {
			return nil
		}
		for i, m := range matches {
			matches[i] = last[:start] + m + last[end:]
		}
		return matches
	})
	sigint := make(chan os.Signal)
	signal.Notify(sigint, syscall.SIGINT)
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		default:
		}
		src, err := ln.Next()
		if err != nil {
			if err != io.EOF {
				glog.Errorf("liner returned non-EOF error unexpectedly: %v", err)
			}
			return
		}
		runCtx, cancel := context.WithCancel(ctx)
		go func() {
			select {
			case <-sigint:
				cancel()
			case <-runCtx.Done():
			}
		}()
		func() {
			defer func() {
				cancel()
				p := recover()
				if p != nil {
					// The return value of debug.Stack() ends with \n.
					fmt.Fprintf(os.Stderr, "panic: %v\n\n%s", p, debug.Stack())
				}
			}()
			if err := rn.Run(core.LgoContext{Context: runCtx}, src); err != nil {
				glog.Error(err)
			}
		}()
	}
}

func main() {
	flag.Parse()
	if *sessIDFlag == "" {
		glog.Fatal("--sess_id is not set")
	}
	var sessID runner.SessionID
	if err := sessID.Unmarshal(*sessIDFlag); err != nil {
		glog.Fatalf("--sess_id=%s is invalid: %v", *sessIDFlag, err)
	}
	if *subcomandFlag == "" {
		glog.Fatal("--subcomand is not set")
	}

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		glog.Fatal("GOPATH is not set")
	}
	lgopath := os.Getenv("LGOPATH")
	if lgopath == "" {
		glog.Fatal("LGOPATH is empty")
	}
	lgopath, err := filepath.Abs(lgopath)
	if err != nil {
		glog.Fatalf("Failed to get the absolute path of LGOPATH: %v", err)
	}
	core.RegisterLgoPrinter(&printer{})

	if *subcomandFlag == "kernel" {
		kernelMain(gopath, lgopath, &sessID)
		exitProcess()
	}

	rn := runner.NewLgoRunner(gopath, lgopath, &sessID)
	defer func() {
		err := rn.CleanUp()
		if err != nil {
			glog.Errorf("Clean up failure: %v", err)
		}
	}()

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT)
	interrupt := make(chan struct{})
	go func() {
		// This goroutine leaks!
		for {
			<-sigch
			interrupt <- struct{}{}
		}
	}()

	useFiles := len(flag.Args()) > 0
	ctx := createProcessContext(useFiles)

	// Set up cleanup
	startCleanup := make(chan struct{})
	endCleanup := make(chan struct{})
	go func() {
		// clean-up goroutine
		<-startCleanup
		glog.Infof("Clean the session: %s", sessID.Marshal())
		runner.CleanSession(gopath, lgopath, &sessID)
		close(endCleanup)
		// Terminate the process if the main routine does not return in 1 sec after the ctx is cancelled.
		time.Sleep(500 * time.Millisecond)
		exitProcess()
	}()
	go func() {
		// start clean-up 500ms after the ctx is cancelled.
		<-ctx.Done()
		time.Sleep(500 * time.Millisecond)
		startCleanup <- struct{}{}
	}()
	if len(flag.Args()) > 0 {
		fromFiles(ctx, rn)
	} else {
		fromStdin(ctx, rn)
	}
	startCleanup <- struct{}{}
	<-endCleanup
	exitProcess()
}
