package runner

import (
	"encoding/json"
	"testing"
)

func TestSessID_marshalJSON(t *testing.T) {
	id := SessionID{
		Time: 1234,
	}
	b, err := json.Marshal(&id)
	if err != nil {
		t.Error(err)
		return
	}
	expected := "{\"time\":1234}"
	if string(b) != expected {
		t.Errorf("Expected %s but got %s", expected, b)
	}
}

func TestSessID_marshal(t *testing.T) {
	id := SessionID{
		Time: 12345,
	}
	expS := "sess7b2274696d65223a31323334357d"
	s := id.Marshal()
	if s != expS {
		t.Errorf("Expected %q but got %q", expS, s)
	}

	var newID SessionID
	if err := newID.Unmarshal(s); err != nil {
		t.Error(err)
	}
	var expTime int64 = 12345
	if newID.Time != expTime {
		t.Errorf("Expected %d but got %d", expTime, newID.Time)
	}
}
