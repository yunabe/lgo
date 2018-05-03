package gojupyterscaffold

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	zmq "github.com/pebbe/zmq4"
)

var (
	identityDelim = []byte("<IDS|MSG>")
)

type message struct {
	Identity     [][]byte
	Header       messageHeader
	ParentHeader messageHeader
	Metadata     interface{}
	Content      interface{}
}

// http://jupyter-client.readthedocs.io/en/latest/messaging.html#general-message-format
type messageHeader struct {
	MsgID    string `json:"msg_id"`
	Username string `json:"username"`
	Session  string `json:"session"`
	Date     string `json:"date"`
	MsgType  string `json:"msg_type"`
	Version  string `json:"version"`
}

func newContentForMsgType(header *messageHeader) interface{} {
	switch header.MsgType {
	case "execute_request":
		return &ExecuteRequest{}
	case "complete_request":
		return &CompleteRequest{}
	case "inspect_request":
		return &InspectRequest{}
	case "is_complete_request":
		return &IsCompleteRequest{}
	case "gofmt_request":
		return &GoFmtRequest{}
	}
	return nil
}

func unmarshalJSONToInterface(b []byte, i interface{}) (interface{}, error) {
	if i == nil {
		tmp := make(map[string]interface{})
		i = &tmp
	}
	if err := json.Unmarshal(b, i); err != nil {
		return nil, err
	}
	return i, nil
}

func validateMessages(msgs [][]byte, key []byte) error {
	if len(msgs) < 5 {
		return fmt.Errorf("Too short messages: %d", len(msgs))
	}
	mac := hmac.New(sha256.New, key)
	// header, parent header, metadata, and content are signed with hmac.
	for _, msg := range msgs[1:5] {
		mac.Write(msg)
	}
	// Decode the hex signature
	sig := make([]byte, hex.DecodedLen(len(msgs[0])))
	hex.Decode(sig, msgs[0])
	// Verify the signature
	if !hmac.Equal(mac.Sum(nil), sig) {
		return errors.New("HMAC was invalid")
	}
	return nil
}

func (m *message) Unmarshal(bs [][]byte, key []byte) error {
	delimIdx := -1
	for i, b := range bs {
		if bytes.Equal(b, identityDelim) {
			delimIdx = i
			break
		}
	}
	if delimIdx < 0 {
		return fmt.Errorf("Identity deliminator %s not found", identityDelim)
	}
	bodies := bs[delimIdx+1:]
	if err := validateMessages(bodies, key); err != nil {
		return err
	}
	m.Identity = bs[:delimIdx]
	m.Header = messageHeader{}
	if err := json.Unmarshal(bodies[1], &m.Header); err != nil {
		return err
	}
	m.ParentHeader = messageHeader{}
	if err := json.Unmarshal(bodies[2], &m.ParentHeader); err != nil {
		return err
	}
	if i, err := unmarshalJSONToInterface(bodies[3], m.Metadata); err == nil {
		m.Metadata = i
	} else {
		return err
	}
	m.Content = newContentForMsgType(&m.Header)
	if i, err := unmarshalJSONToInterface(bodies[4], m.Content); err == nil {
		m.Content = i
	} else {
		return err
	}
	return nil
}

func (m *message) Marshal(key []byte) (bs [][]byte, err error) {
	bs = m.Identity
	bs = append(bs, identityDelim)
	chunks := []interface{}{&m.Header, &m.ParentHeader, m.Metadata, m.Content}
	var bodies [][]byte
	for _, chunk := range chunks {
		var data []byte
		if chunk == nil {
			data = []byte("{}")
		} else {
			data, err = json.Marshal(chunk)
			if err != nil {
				return nil, err
			}
		}
		bodies = append(bodies, data)
	}
	mac := hmac.New(sha256.New, key)
	for _, body := range bodies {
		mac.Write(body)
	}
	sig := mac.Sum(nil)
	hexSig := make([]byte, hex.EncodedLen(len(sig)))
	hex.Encode(hexSig, sig)

	bs = append(bs, hexSig)
	return append(bs, bodies...), nil
}

func (m *message) Send(sock *zmq.Socket, key []byte) error {
	bs, err := m.Marshal(key)
	if err != nil {
		return fmt.Errorf("Failed to marshal kernelinfo: %v", err)
	}
	args := make([]interface{}, len(bs))
	for i, b := range bs {
		args[i] = b
	}
	if _, err := sock.SendMessage(args...); err != nil {
		return err
	}
	return nil
}

func genMsgID() string {
	var id [32]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic(fmt.Sprintf("rand.Read failed: %v", err))
	}
	var hexID [64]byte
	hex.Encode(hexID[:], id[:])
	return string(hexID[:])
}

func newMessageWithParent(parent *message) *message {
	// https://github.com/ipython/ipykernel/blob/master/ipykernel/kernelbase.py#L222
	// idents should be copied from the parent.
	msg := message{
		Identity:     parent.Identity,
		ParentHeader: parent.Header,
	}
	msg.Header.Session = parent.Header.Session
	msg.Header.Version = "5.2"
	msg.Header.Username = "username"
	msg.Header.MsgID = genMsgID()
	return &msg
}
