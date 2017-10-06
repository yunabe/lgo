package runner

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const idPrefix = "sess"

type SessionID struct {
	Time int64 `json:"time"`
}

func NewSessionID() *SessionID {
	return &SessionID{time.Now().UnixNano()}
}

func (s *SessionID) Marshal() string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(fmt.Errorf("Unexpected error: %v", err))
	}
	h := make([]byte, hex.EncodedLen(len(b)))
	hex.Encode(h, b)
	return idPrefix + string(h)
}

func (s *SessionID) Unmarshal(h string) error {
	if !strings.HasPrefix(h, idPrefix) {
		return fmt.Errorf("Expected %s prefix but got %s", idPrefix, h)
	}
	h = h[len(idPrefix):]
	b := make([]byte, hex.DecodedLen(len(h)))
	if _, err := hex.Decode(b, []byte(h)); err != nil {
		return err
	}
	return json.Unmarshal(b, s)
}
