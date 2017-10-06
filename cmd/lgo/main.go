package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"syscall"

	"github.com/yunabe/lgo/cmd/lgo/install"
	"github.com/yunabe/lgo/cmd/runner"
)

const usage = `lgo is a tool to build and execute Go code interactively.append

Usage:

    lgo commands [arguments]

The commands are;

    install    install lgo into $LGOPATH. You need to run this command before using lgo
	kernel     run a jupyter notebook kernel
	run        run Go code defined in files
	repl       ...
	clean      clean temporary files created by lgo
`

var commandStrRe = regexp.MustCompile("[a-z]+")

func printUsageAndExit() {
	fmt.Fprint(os.Stderr, usage)
	os.Exit(1)
}

func runLgoInternal(subcommand string, extraArgs []string) {
	// TODO: Consolidate this logic to check env variables.
	if runtime.GOOS != "linux" {
		log.Fatal("lgo only supports Linux")
	}
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		log.Fatal("GOPATH is not set")
	}
	lgopath := os.Getenv("LGOPATH")
	if lgopath == "" {
		log.Fatal("LGOPATH is empty")
	}
	lgopath, err := filepath.Abs(lgopath)
	if err != nil {
		log.Fatalf("Failed to get the absolute path of LGOPATH: %v", err)
	}

	lgoInternal := filepath.Join(lgopath, "bin", "lgo-internal")
	if _, err = os.Stat(lgoInternal); os.IsNotExist(err) {
		log.Fatal("lgo is not installed in LGOPATH. Please run `lgo install` first")
	}

	// These signals are ignored because no goroutine reads sigch.
	// We do not use signal.Ignore because we do not change the signal handling behavior of child processes.
	sigch := make(chan os.Signal)
	signal.Notify(sigch, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP, syscall.SIGINT)

	sessID := runner.NewSessionID()
	var args []string
	args = append(args, "--subcommand="+subcommand)
	args = append(args, "--sess_id="+sessID.Marshal())
	args = append(args, extraArgs...)
	cmd := exec.Command(lgoInternal, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("lgo-internal failed: %v", err)
	}
	// In case lgo-internal exists before cleaning files (e.g. os.Exit is called)
	runner.CleanSession(gopath, lgopath, sessID)
}

func runMain() {
	fs := flag.NewFlagSet("lgo run", flag.ExitOnError)
	fs.Parse(os.Args[2:])
	runLgoInternal("run", fs.Args())
}

func kernelMain() {
	fs := flag.NewFlagSet("lgo kernel", flag.ExitOnError)
	connectionFile := fs.String("connection_file", "", "jupyter kernel connection file path.")
	fs.Parse(os.Args[2:])
	runLgoInternal("kernel", []string{"--connection_file=" + *connectionFile})
}

func main() {
	if len(os.Args) <= 1 {
		printUsageAndExit()
	}
	cmd := os.Args[1]
	if !commandStrRe.MatchString(cmd) {
		printUsageAndExit()
	}
	switch cmd {
	case "install":
		install.Main()
	case "kernel":
		kernelMain()
	case "run":
		runMain()
	case "clean":
		fmt.Fprint(os.Stderr, "not implemented")
	case "help":
		printUsageAndExit()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", cmd)
		os.Exit(1)
	}
}
