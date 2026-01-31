package mcpio

import (
	"bufio"
	"bytes"
	"testing"
)

func TestReadWriteMessage_BackToBack(t *testing.T) {
	msg1 := []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	msg2 := []byte(`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{}}`)

	var buf bytes.Buffer
	if err := WriteMessage(&buf, msg1); err != nil {
		t.Fatalf("write msg1: %v", err)
	}
	if err := WriteMessage(&buf, msg2); err != nil {
		t.Fatalf("write msg2: %v", err)
	}

	reader := bufio.NewReader(&buf)
	got1, err := ReadMessage(reader)
	if err != nil {
		t.Fatalf("read msg1: %v", err)
	}
	got2, err := ReadMessage(reader)
	if err != nil {
		t.Fatalf("read msg2: %v", err)
	}

	if string(got1) != string(msg1) {
		t.Fatalf("msg1 mismatch: %s", string(got1))
	}
	if string(got2) != string(msg2) {
		t.Fatalf("msg2 mismatch: %s", string(got2))
	}
}
