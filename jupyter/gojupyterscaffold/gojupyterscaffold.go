// Package gojupyterscaffold provides a scaffold of Jupyter kernel implemented by Go.
//
// References:
// https://github.com/ipython/ipykernel/blob/master/ipykernel/kernelbase.py
// https://github.com/jupyter/jupyter_client/blob/master/jupyter_client/session.py
//
// Misc:
// ZMQ pubsub with inproc is broken (https://github.com/JustinTulloss/zeromq.node/issues/22) though it's not used in this code now.
package gojupyterscaffold

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
	zmq "github.com/pebbe/zmq4"
)

// ConnectionInfo stores the contents of the kernel connection file created by Jupyter.
type connectionInfo struct {
	StdinPort       int    `json:"stdin_port"`
	IP              string `json:"ip"`
	ControlPort     int    `json:"control_port"`
	HBPort          int    `json:"hb_port"`
	SignatureScheme string `json:"signature_scheme"`
	Key             string `json:"key"`
	KernelName      string `json:"kernel_name"`
	ShellPort       int    `json:"shell_port"`
	Transport       string `json:"transport"`
	IOPubPort       int    `json:"iopub_port"`
}

func readConnectionInfo(connectionFile string) (*connectionInfo, error) {
	b, err := ioutil.ReadFile(connectionFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to read %s: %v", connectionFile, err)
	}
	glog.Infof("Connection info JSON: %v", string(b))
	var cinfo connectionInfo
	if err = json.Unmarshal(b, &cinfo); err != nil {
		return nil, fmt.Errorf("Failed to parse %s: %v", connectionFile, err)
	}
	glog.Infof("Connection info: %+v", cinfo)
	return &cinfo, nil
}

func (ci *connectionInfo) getAddr(port int) string {
	return fmt.Sprintf("%s://%s:%d", ci.Transport, ci.IP, port)
}

type Server struct {
	handlers RequestHandlers

	// ctx of this server and a func to cancel it.
	ctx       context.Context
	cancelCtx func()

	// ZMQ sockets
	shell   *shellSocket
	control *shellSocket
	iopub   *iopubSocket
	stdin   *zmq.Socket
	hb      *zmq.Socket

	// Attribute
	connInfo *connectionInfo

	execQueue *executeQueue
}

// NewServer returns a new jupyter kernel server.
func NewServer(connectionFile string, handlers RequestHandlers) (server *Server, err error) {
	serverCtx, cancelCtx := context.WithCancel(context.Background())
	defer func() {
		// Avoid ctx leak
		// https://golang.org/pkg/context/
		if server == nil {
			cancelCtx()
		}
	}()
	cinfo, err := readConnectionInfo(connectionFile)
	if err != nil {
		return nil, err
	}
	ctx, err := zmq.NewContext()
	if err != nil {
		return nil, err
	}

	iopub, err := newIOPubSocket(serverCtx, ctx, cinfo)
	if err != nil {
		return nil, fmt.Errorf("Failed to create iopub socket: %v", err)
	}

	execQueue := newExecuteQueue(serverCtx, iopub, handlers)
	shell, err := newShellSocket(serverCtx, ctx, "shell", cinfo, iopub, handlers, cancelCtx, execQueue)
	if err != nil {
		return nil, fmt.Errorf("Failed to create shell socket: %v", err)
	}
	control, err := newShellSocket(serverCtx, ctx, "control", cinfo, iopub, handlers, cancelCtx, execQueue)
	if err != nil {
		return nil, fmt.Errorf("Failed to create control socket: %v", err)
	}

	stdin, err := ctx.NewSocket(zmq.ROUTER)
	if err != nil {
		return nil, fmt.Errorf("Failed to open stdin socket: %v", err)
	}
	if err := stdin.Bind(cinfo.getAddr(cinfo.StdinPort)); err != nil {
		return nil, fmt.Errorf("Failed to bind shell socket: %v", err)
	}
	// Ref: Python version of HeartBeat
	// https://github.com/ipython/ipykernel/blob/master/ipykernel/heartbeat.py
	hb, err := ctx.NewSocket(zmq.REP)
	if err != nil {
		return nil, fmt.Errorf("Failed to open heartbeat socket: %v", err)
	}
	if err := hb.Bind(cinfo.getAddr(cinfo.HBPort)); err != nil {
		return nil, fmt.Errorf("Failed to bind heartbeat socket: %v", err)
	}
	return &Server{
		handlers:  handlers,
		ctx:       serverCtx,
		cancelCtx: cancelCtx,
		shell:     shell,
		control:   control,
		stdin:     stdin,
		iopub:     iopub,
		hb:        hb,
		connInfo:  cinfo,
		execQueue: execQueue,
	}, nil
}

// Context returns the context of the server
func (s *Server) Context() context.Context {
	return s.ctx
}

func (s *Server) monitorSigint() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT)
	go func() {
		for _ = range ch {
			glog.Info("Received SIGINT. Cancelling an ongoing execute_request")
			s.execQueue.cancelCurrent()
		}
	}()
}

func (s *Server) monitorTerminationSignals() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)
	go func() {
		for sig := range ch {
			glog.Infof("Received a signal (%s), terminating the kernel.", sig)
			s.cancelCtx()
		}
	}()
}

func isEINTR(err error) bool {
	if err == nil {
		return false
	}
	errno, ok := err.(syscall.Errno)
	return ok && errno == syscall.EINTR
}

// Loop starts the server main loop
func (s *Server) Loop() {
	go func() {
		glog.Info("Forwarding heartbeat requests")
		if err := zmq.Proxy(s.hb, s.hb, nil); err != nil {
			glog.Fatalf("Failed to echo heartbeat request: %v", err)
		}
		glog.Info("Quitting goroutine for heartbeat requests")
	}()
	s.monitorSigint()
	s.monitorTerminationSignals()

	execDone := make(chan struct{})
	sockDone := make(chan struct{})
	go func() {
		s.execQueue.loop()
		close(execDone)
	}()
	go func() {
		s.shell.loop()
		sockDone <- struct{}{}
	}()
	go func() {
		s.control.loop()
		sockDone <- struct{}{}
	}()
	<-execDone

	if err := s.shell.notifyLoopEnd(); err != nil {
		glog.Errorf("Failed to notify the loop end to shell socket: %v", err)
	}
	if err := s.control.notifyLoopEnd(); err != nil {
		glog.Errorf("Failed to notify the loop end to control socket: %v", err)
	}
	// Wait loop ends
	<-sockDone
	<-sockDone

	if err := s.iopub.close(); err != nil {
		glog.Errorf("Failed to close iopub socket: %v", err)
	}
	if err := s.shell.close(); err != nil {
		glog.Errorf("Failed to close shell socket: %v", err)
	}
	if err := s.control.close(); err != nil {
		glog.Errorf("Failed to close control socket: %v", err)
	}

	// TODO: Support stdin.
}
