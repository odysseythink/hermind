package platforms

import (
	"context"
	"testing"
)

func TestACP_ClosureDelegatesToTestListen(t *testing.T) {
	d, ok := Get("acp")
	if !ok || d.Test == nil {
		t.Fatal("acp descriptor missing Test closure")
	}
	err := d.Test(context.Background(), map[string]string{"addr": "127.0.0.1:0"})
	if err != nil {
		t.Errorf("unexpected: %v", err)
	}
}
