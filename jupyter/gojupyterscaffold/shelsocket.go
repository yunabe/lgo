package gojupyterscaffold

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/golang/glog"
	zmq "github.com/pebbe/zmq4"
)

var errLoopEnd = errors.New("end of loop")

type contextAndCancel struct {
	ctx    context.Context
	cancel func()
}

type iopubSocket struct {
	socket    *zmq.Socket
	mutex     *sync.Mutex
	hmacKey   []byte
	serverCtx context.Context
	ongoing   map[*contextAndCancel]bool
}

func newIOPubSocket(serverCtx context.Context, zmqCtx *zmq.Context, cinfo *connectionInfo) (*iopubSocket, error) {
	iopub, err := zmqCtx.NewSocket(zmq.PUB)
	if err != nil {
		return nil, fmt.Errorf("Failed to open iopub socket: %v", err)
	}
	if err := iopub.Bind(cinfo.getAddr(cinfo.IOPubPort)); err != nil {
		return nil, fmt.Errorf("Failed to bind shell socket: %v", err)
	}
	return &iopubSocket{
		socket:    iopub,
		mutex:     &sync.Mutex{},
		hmacKey:   []byte(cinfo.Key),
		serverCtx: serverCtx,
		ongoing:   make(map[*contextAndCancel]bool),
	}, nil
}

func (s *iopubSocket) close() error {
	return s.socket.Close()
}

func (s *iopubSocket) addOngoingContext() *contextAndCancel {
	ctx, cancel := context.WithCancel(s.serverCtx)
	ctxCancel := &contextAndCancel{ctx: ctx, cancel: cancel}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.ongoing[ctxCancel] = true
	return ctxCancel
}

func (s *iopubSocket) removeOngoingContext(ctx *contextAndCancel) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.ongoing, ctx)
}

func (s *iopubSocket) cancelOngoings() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for e := range s.ongoing {
		e.cancel()
	}
}

func (s *iopubSocket) WithOngoingContext(f func(ctx context.Context) error, parent *message) (err error) {
	// We may want to call addOngoingContext in the goroutine for zmq loop.
	// TODO: Reconsider this deeply.
	ctxCancel := s.addOngoingContext()
	if err := s.publishStatus("busy", parent); err != nil {
		return err
	}
	defer func() {
		s.removeOngoingContext(ctxCancel)
		if ierr := s.publishStatus("idle", parent); ierr != nil && err == nil {
			err = ierr
		}
	}()
	return f(ctxCancel.ctx)
}

func (s *iopubSocket) sendMessage(msg *message) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return msg.Send(s.socket, s.hmacKey)
}

func (s *iopubSocket) publishStatus(status string, parent *message) error {
	var msg message
	// TODO: Change the format of Identity to kernel.<uuid>.MsgType.
	// http://jupyter-client.readthedocs.io/en/latest/messaging.html#the-wire-protocol
	msg.Identity = [][]byte{[]byte("status")}
	msg.Header.MsgType = "status"
	msg.Header.Version = "5.2"
	msg.Header.Username = "username"
	msg.Header.MsgID = genMsgID()
	msg.ParentHeader = parent.Header
	msg.Content = &struct {
		ExecutionState string `json:"execution_state"`
	}{
		ExecutionState: status,
	}
	return s.sendMessage(&msg)
}

// http://jupyter-client.readthedocs.io/en/latest/messaging.html#streams-stdout-stderr-etc
func (s *iopubSocket) sendStream(name, text string, parent *message) {
	var msg message
	msg.Identity = [][]byte{[]byte("stream")}
	msg.Header.MsgType = "stream"
	msg.Header.Version = "5.2"
	msg.Header.Username = "username"
	msg.Header.MsgID = genMsgID()

	msg.ParentHeader = parent.Header
	msg.Content = &struct {
		Name string `json:"name"`
		Text string `json:"text"`
	}{
		Name: name,
		Text: text,
	}
	if err := s.sendMessage(&msg); err != nil {
		glog.Errorf("Failed to send stream: %v", err)
	}
}

func (s *iopubSocket) sendDisplayData(data *DisplayData, parent *message, update bool) {
	var msg message
	msgType := "display_data"
	if update {
		if data.Transient["display_id"] == nil {
			glog.Warning("update_display_data with no display_id")
		}
		msgType = "update_display_data"
	}
	msg.Identity = [][]byte{[]byte(msgType)}
	msg.Header.MsgType = msgType
	msg.Header.Version = "5.2"
	msg.Header.Username = "username"
	msg.Header.MsgID = genMsgID()
	msg.ParentHeader = parent.Header
	msg.Content = data
	if err := s.sendMessage(&msg); err != nil {
		glog.Errorf("Failed to send stream: %v", err)
	}
}

type shellSocket struct {
	name          string
	hmacKey       []byte
	socket        *zmq.Socket
	resultPush    *zmq.Socket
	resultPushMux sync.Mutex
	resultPull    *zmq.Socket
	iopub         *iopubSocket

	handlers  RequestHandlers
	ctx       context.Context
	cancelCtx func()

	execQueue *executeQueue
}

func newShellSocket(serverCtx context.Context, zmqCtx *zmq.Context, name string, cinfo *connectionInfo, iopub *iopubSocket, handlers RequestHandlers, cancelCtx func(), execQueue *executeQueue) (*shellSocket, error) {
	var routerAddr string
	if name == "shell" {
		routerAddr = cinfo.getAddr(cinfo.ShellPort)
	} else if name == "control" {
		routerAddr = cinfo.getAddr(cinfo.ControlPort)
	} else {
		return nil, fmt.Errorf("Unknown shell socket name: %q", name)
	}

	sock, err := zmqCtx.NewSocket(zmq.ROUTER)
	if err != nil {
		return nil, fmt.Errorf("Failed to open %s socket: %v", name, err)
	}
	if err := sock.Bind(routerAddr); err != nil {
		return nil, fmt.Errorf("Failed to bind %s socket: %v", name, err)
	}
	resultPush, err := zmqCtx.NewSocket(zmq.PUSH)
	if err != nil {
		return nil, err
	}
	inprocAddr := fmt.Sprintf("inproc://result-for-%s-socket", name)
	if err := resultPush.Bind(inprocAddr); err != nil {
		return nil, err
	}
	resultPull, err := zmqCtx.NewSocket(zmq.PULL)
	if err != nil {
		return nil, err
	}
	if err := resultPull.Connect(inprocAddr); err != nil {
		return nil, err
	}
	return &shellSocket{
		name:       name,
		hmacKey:    []byte(cinfo.Key),
		socket:     sock,
		resultPush: resultPush,
		resultPull: resultPull,
		iopub:      iopub,
		handlers:   handlers,
		ctx:        serverCtx,
		cancelCtx:  cancelCtx,
		execQueue:  execQueue,
	}, nil
}

func (s *shellSocket) close() (err error) {
	if cerr := s.socket.Close(); cerr != nil {
		err = cerr
	}
	if cerr := s.resultPush.Close(); cerr != nil {
		err = cerr
	}
	if cerr := s.resultPull.Close(); cerr != nil {
		err = cerr
	}
	return
}

// pushResult sends a message to shellSocket so that it will be sent to the client.
// This method is goroutine-safe.
func (s *shellSocket) pushResult(msg *message) error {
	s.resultPushMux.Lock()
	defer s.resultPushMux.Unlock()
	return msg.Send(s.resultPush, s.hmacKey)
}

// notifyLoopEnd notifies the end of the loop to the goroutine in loop().
func (s *shellSocket) notifyLoopEnd() error {
	s.resultPushMux.Lock()
	defer s.resultPushMux.Unlock()
	// Notes:
	// You need to send at least one message with SendMessage.
	// You can not use a zero-length messsages to notify the end of loop because
	// the zero-length messages are not sent to the receiver.
	_, err := s.resultPush.SendMessage("END_OF_LOOP")
	return err
}

func (s *shellSocket) loop() {
	poller := zmq.NewPoller()
	poller.Add(s.socket, zmq.POLLIN)
	poller.Add(s.resultPull, zmq.POLLIN)
loop:
	for {
		polled, err := poller.Poll(-1)
		if isEINTR(err) {
			// It seems like poller.Poll sometimes return EINTR when a signal is sent
			// even if a signal handler for SIGINT is registered.
			glog.Info("zmq.Poll was interrupted")
			continue
		}
		if err != nil {
			glog.Errorf("Poll on %s socket failed: %v", s.name, err)
			continue
		}
		for _, p := range polled {
			switch p.Socket {
			case s.socket:
				if err := s.handleMessages(); err != nil {
					glog.Errorf("Failed to handle a message on %s socket: %v", s.name, err)
				}
			case s.resultPull:
				err := s.handleResultPull()
				if err == errLoopEnd {
					glog.Infof("Exiting polling loop for %s", s.name)
					break loop
				}
				if err != nil {
					glog.Infof("Failed to handle a message on the result socket of %s: %v", s.name, err)
				}
			default:
				panic(errors.New("zmq.Poll returned an unexpected socket"))
			}
		}
	}
}

func (s *shellSocket) sendKernelInfo(req *message) error {
	return s.iopub.WithOngoingContext(func(_ context.Context) error {
		var info KernelInfo
		info = s.handlers.HandleKernelInfo()
		res := newMessageWithParent(req)

		// https://github.com/jupyter/notebook/blob/master/notebook/services/kernels/handlers.py#L174
		res.Header.MsgType = "kernel_info_reply"
		res.Content = &info
		return res.Send(s.socket, s.hmacKey)
	}, req)
}

func (s *shellSocket) handleMessages() error {
	msgs, err := s.socket.RecvMessageBytes(0)
	if err != nil {
		return fmt.Errorf("Failed to receive data from %s: %v", s.name, err)
	}
	var msg message
	err = msg.Unmarshal(msgs, s.hmacKey)
	if err != nil {
		return fmt.Errorf("Failed to unmarshal messages from %s: %v", s.name, err)
	}
	switch typ := msg.Header.MsgType; typ {
	case "kernel_info_request":
		if err := s.sendKernelInfo(&msg); err != nil {
			glog.Errorf("Failed to handle kernel_info_request: %v", err)
		}
	case "shutdown_request":
		glog.Info("received shutdown_request.")
		s.cancelCtx()
		// TODO: Send shutdown_reply
	case "execute_request":
		s.execQueue.push(&msg, s)
	case "complete_request":
		go func() {
			reply := s.handlers.HandleComplete(msg.Content.(*CompleteRequest))
			if reply == nil {
				reply = &CompleteReply{
					Status: "ok",
				}
			}
			if reply.Status == "ok" && reply.Matches == nil {
				// matches must not be null because `jupyter console` can not accept null for matches as of 2017/12.
				// https://goo.gl/QRd5rG
				reply.Matches = make([]string, 0)
			}
			res := newMessageWithParent(&msg)
			res.Header.MsgType = "complete_reply"
			res.Content = reply
			s.pushResult(res)
		}()
	case "inspect_request":
		go func() {
			reply := s.handlers.HandleInspect(msg.Content.(*InspectRequest))
			if reply == nil {
				reply = &InspectReply{
					Status: "ok",
					Found:  false,
				}
			}
			res := newMessageWithParent(&msg)
			res.Header.MsgType = "inspect_reply"
			res.Content = reply
			s.pushResult(res)
		}()
	case "is_complete_request":
		go func() {
			reply := s.handlers.HandleIsComplete(msg.Content.(*IsCompleteRequest))
			if reply == nil {
				reply = &IsCompleteReply{Status: "unknown"}
			}
			res := newMessageWithParent(&msg)
			res.Header.MsgType = "is_complete_reply"
			res.Content = reply
			s.pushResult(res)
		}()
	default:
		glog.Warningf("Unsupported MsgType in %s: %q", s.name, typ)
	}
	return nil
}

// Forward a message on result_pull to socket.
func (s *shellSocket) handleResultPull() error {
	msgs, err := s.resultPull.RecvMessageBytes(0)
	if err != nil {
		return err
	}
	if len(msgs) == 1 && string(msgs[0]) == "END_OF_LOOP" {
		return errLoopEnd
	}
	// For some reasons, execute_reply is not handled correctly
	// unless we unmarshal and marshal msgs rather than just forwarding them.
	var msg message
	if err := msg.Unmarshal(msgs, s.hmacKey); err != nil {
		return err
	}
	return msg.Send(s.socket, s.hmacKey)
}
