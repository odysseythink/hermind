package server

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestServer_InitializeOverStdio(t *testing.T) {
	s, _ := newTestServer(t)
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n")
	var out bytes.Buffer
	if err := s.RunOnce(context.Background(), in, &out, 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"protocolVersion"`) {
		t.Errorf("got %s", out.String())
	}
}

func TestServer_UnknownMethodReturnsError(t *testing.T) {
	s, _ := newTestServer(t)
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"does_not_exist"}` + "\n")
	var out bytes.Buffer
	_ = s.RunOnce(context.Background(), in, &out, 1)
	if !strings.Contains(out.String(), `"code":-32601`) {
		t.Errorf("got %s", out.String())
	}
}
