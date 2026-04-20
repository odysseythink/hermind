package stdio

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestDecodeRequest_String(t *testing.T) {
	raw := []byte(`{"jsonrpc":"2.0","id":7,"method":"prompt","params":{"sessionId":"s1","prompt":[{"type":"text","text":"hi"}]}}`)
	req, err := DecodeRequest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "prompt" {
		t.Errorf("method = %q", req.Method)
	}
	var id json.Number
	_ = json.Unmarshal(req.ID, &id)
	if id != "7" {
		t.Errorf("id = %q", id)
	}
}

func TestEncodeResponse_ResultOnly(t *testing.T) {
	var buf bytes.Buffer
	resp := &Response{
		ID:     json.RawMessage(`7`),
		Result: json.RawMessage(`{"stopReason":"end_turn"}`),
	}
	if err := EncodeResponse(&buf, resp); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if got[len(got)-1] != '\n' {
		t.Error("missing trailing newline")
	}
	if !bytes.Contains([]byte(got), []byte(`"jsonrpc":"2.0"`)) {
		t.Errorf("missing jsonrpc marker: %s", got)
	}
	if bytes.Contains([]byte(got), []byte(`"error"`)) {
		t.Errorf("error present when Result set: %s", got)
	}
}

func TestEncodeResponse_ErrorOnly(t *testing.T) {
	var buf bytes.Buffer
	resp := &Response{
		ID: json.RawMessage(`7`),
		Error: &Error{
			Code:    CodeMethodNotFound,
			Message: "method not found",
		},
	}
	_ = EncodeResponse(&buf, resp)
	if !bytes.Contains(buf.Bytes(), []byte(`"code":-32601`)) {
		t.Errorf("got %s", buf.String())
	}
}

func TestEncodeNotification(t *testing.T) {
	var buf bytes.Buffer
	_ = EncodeNotification(&buf, "session/update", map[string]any{
		"sessionId": "s1",
	})
	if !bytes.Contains(buf.Bytes(), []byte(`"method":"session/update"`)) {
		t.Errorf("got %s", buf.String())
	}
	if bytes.Contains(buf.Bytes(), []byte(`"id"`)) {
		t.Errorf("notifications must not carry id: %s", buf.String())
	}
}

func TestDecodeRequest_NotificationDetection(t *testing.T) {
	raw := []byte(`{"jsonrpc":"2.0","method":"session/cancel","params":{"sessionId":"s1"}}`)
	req, err := DecodeRequest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !req.IsNotification() {
		t.Errorf("expected notification, got ID=%q", string(req.ID))
	}
}
