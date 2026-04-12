package rl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWandBGetRunStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer wb-key" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"state":"running","config":{"learning_rate":{"value":0.0001}},"summary":{"loss":0.5,"step":100}}`))
	}))
	defer srv.Close()

	client := NewWandBClient("wb-key", srv.URL)
	status, err := client.GetRunStatus(context.Background(), "entity/project/runs/run123")
	if err != nil {
		t.Fatalf("GetRunStatus: %v", err)
	}
	if status.State != "running" {
		t.Errorf("state = %q", status.State)
	}
	if status.Summary["loss"] != 0.5 {
		t.Errorf("loss = %v", status.Summary["loss"])
	}
}

func TestWandBClientNotConfigured(t *testing.T) {
	client := NewWandBClient("", "")
	_, err := client.GetRunStatus(context.Background(), "entity/project/runs/run123")
	if err == nil {
		t.Error("expected error when not configured")
	}
}
