package platforms

import (
	"context"
	"net"
	"testing"
)

func TestAPIServer_Success(t *testing.T) {
	if err := testListen(context.Background(), "127.0.0.1:0"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestAPIServer_PortInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("setup listener: %v", err)
	}
	defer ln.Close()

	if err := testListen(context.Background(), ln.Addr().String()); err == nil {
		t.Error("expected bind conflict error")
	}
}

func TestAPIServer_InvalidAddr(t *testing.T) {
	if err := testListen(context.Background(), "not a real addr"); err == nil {
		t.Error("expected parse error")
	}
}
