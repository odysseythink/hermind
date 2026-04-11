package memprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/tool"
)

func TestSupermemorySyncTurnAndRecall(t *testing.T) {
	var addCount, searchCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/v3/memories") && r.Method == "POST":
			atomic.AddInt32(&addCount, 1)
			_, _ = w.Write([]byte(`{"id":"sm_1"}`))
		case strings.HasSuffix(r.URL.Path, "/v3/search") && r.Method == "POST":
			atomic.AddInt32(&searchCount, 1)
			_, _ = w.Write([]byte(`{"results":[{"content":"user likes cycling"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewSupermemory(config.SupermemoryConfig{
		BaseURL: srv.URL,
		APIKey:  "sm-key",
		UserID:  "user-42",
	})
	if p.Name() != "supermemory" {
		t.Fatalf("name = %q", p.Name())
	}

	ctx := context.Background()
	if err := p.Initialize(ctx, "sess-x"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := p.SyncTurn(ctx, "I ride bikes", "cool"); err != nil {
		t.Fatalf("SyncTurn: %v", err)
	}
	if atomic.LoadInt32(&addCount) != 1 {
		t.Errorf("expected 1 add call, got %d", addCount)
	}

	reg := tool.NewRegistry()
	p.RegisterTools(reg)
	args, _ := json.Marshal(map[string]string{"query": "hobbies"})
	res, err := reg.Dispatch(ctx, "supermemory_recall", args)
	if err != nil {
		t.Fatalf("Dispatch supermemory_recall: %v", err)
	}
	if !strings.Contains(res, "cycling") {
		t.Errorf("expected recall to contain cycling, got %s", res)
	}
	if atomic.LoadInt32(&searchCount) != 1 {
		t.Errorf("expected 1 search call, got %d", searchCount)
	}
}
