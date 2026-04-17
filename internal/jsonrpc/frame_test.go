package jsonrpc

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeRequest(t *testing.T) {
	req, err := DecodeRequest([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{"k":"v"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "ping" {
		t.Errorf("method = %q", req.Method)
	}
	if string(req.Params) != `{"k":"v"}` {
		t.Errorf("params = %s", req.Params)
	}
}

func TestEncodeResponse_Result(t *testing.T) {
	var buf bytes.Buffer
	_ = EncodeResponse(&buf, &Response{
		ID:     json.RawMessage(`1`),
		Result: json.RawMessage(`{"ok":true}`),
	})
	out := buf.String()
	if !strings.Contains(out, `"jsonrpc":"2.0"`) || !strings.HasSuffix(out, "\n") {
		t.Errorf("got %q", out)
	}
}

func TestEncodeNotification_NoID(t *testing.T) {
	var buf bytes.Buffer
	_ = EncodeNotification(&buf, "hello", map[string]string{"x": "y"})
	if strings.Contains(buf.String(), `"id"`) {
		t.Errorf("notifications must omit id: %s", buf.String())
	}
}
