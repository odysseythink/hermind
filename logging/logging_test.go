package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestWithRequestIDRoundTrip(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-1")
	if RequestID(ctx) != "req-1" {
		t.Errorf("unexpected request id: %q", RequestID(ctx))
	}
}

func TestWithRequestIDGenerates(t *testing.T) {
	ctx := WithRequestID(context.Background(), "")
	if RequestID(ctx) == "" {
		t.Error("expected generated id")
	}
}

func TestContextHandlerInjectsRequestID(t *testing.T) {
	buf := &bytes.Buffer{}
	base := slog.NewJSONHandler(buf, nil)
	h := &contextHandler{inner: base}
	logger := slog.New(h)
	ctx := WithRequestID(context.Background(), "req-42")
	logger.InfoContext(ctx, "hello")
	if !strings.Contains(buf.String(), "req-42") {
		t.Errorf("expected request_id in log, got %s", buf.String())
	}
	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Errorf("invalid json: %v", err)
	}
	if rec["request_id"] != "req-42" {
		t.Errorf("request_id = %v", rec["request_id"])
	}
}
