package gojupyterscaffold

import (
	"encoding/json"
	"testing"
)

func TestVerifyHMac(t *testing.T) {
	msgs := [][]byte{
		[]byte("8307DB769F7E4B5F8B4D48735D36BDE3"),
		[]byte("<IDS|MSG>"),
		[]byte("ad1deb3813fcf7d3bdcf046d9a5c7276c87e4f0b98deb784413e0e36bc35e986"),
		[]byte("{\"username\":\"username\",\"version\":\"5.0\",\"msg_id\":\"DBAFA475AEE94EAE8860A5F46B561661\",\"msg_type\":\"kernel_info_request\",\"session\":\"8307DB769F7E4B5F8B4D48735D36BDE3\"}"),
		[]byte("{}"),
		[]byte("{}"),
		[]byte("{}"),
	}
	key := []byte("37485811-fb40116f79cb23af4056c7a8")
	var msg message
	err := msg.Unmarshal(msgs, key)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(msg.Identity) != 1 {
		t.Errorf("Unexpected size: %d", len(msg.Identity))
	}
	if msg.Header.Username != "username" {
		t.Errorf("Unexpected Username: %s", msg.Header.Username)
	}
	if msg.ParentHeader.Username != "" {
		t.Errorf("Unexpected Username: %s", msg.Header.Username)
	}
	_, err = msg.Marshal(key)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestMessageHeader(t *testing.T) {
	var header messageHeader
	err := json.Unmarshal([]byte("{\"username\":\"username\",\"version\":\"5.0\",\"msg_id\":\"DBAFA475AEE94EAE8860A5F46B561661\",\"msg_type\":\"kernel_info_request\",\"session\":\"8307DB769F7E4B5F8B4D48735D36BDE3\"}"), &header)
	if err != nil {
		t.Error(err)
	}
	expected := messageHeader{MsgID: "DBAFA475AEE94EAE8860A5F46B561661", Username: "username", Session: "8307DB769F7E4B5F8B4D48735D36BDE3", Date: "", MsgType: "kernel_info_request", Version: "5.0"}
	if header != expected {
		t.Errorf("Unexpected header: %#v", header)
	}
}
