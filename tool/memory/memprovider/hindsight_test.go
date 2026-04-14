package memprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/tool"
)

func TestHindsightRetainRecallReflect(t *testing.T) {
	var retainHits, recallHits, reflectHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/retain") && r.Method == "POST":
			atomic.AddInt32(&retainHits, 1)
			_, _ = w.Write([]byte(`{"ok":true}`))
		case strings.HasSuffix(r.URL.Path, "/recall") && r.Method == "POST":
			atomic.AddInt32(&recallHits, 1)
			_, _ = w.Write([]byte(`{"results":[{"content":"user likes chess","score":0.9}]}`))
		case strings.HasSuffix(r.URL.Path, "/reflect") && r.Method == "POST":
			atomic.AddInt32(&reflectHits, 1)
			_, _ = w.Write([]byte(`{"answer":"you seem to enjoy chess"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewHindsight(config.HindsightConfig{
		BaseURL: srv.URL,
		APIKey:  "hs-key",
		BankID:  "hermind",
		Budget:  "mid",
	})
	if p.Name() != "hindsight" {
		t.Fatalf("name = %q", p.Name())
	}

	ctx := context.Background()
	if err := p.Initialize(ctx, "sess-1"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := p.SyncTurn(ctx, "I love chess", "noted"); err != nil {
		t.Fatalf("SyncTurn: %v", err)
	}
	if atomic.LoadInt32(&retainHits) != 1 {
		t.Errorf("retainHits = %d", retainHits)
	}

	reg := tool.NewRegistry()
	p.RegisterTools(reg)

	recallArgs, _ := json.Marshal(map[string]string{"query": "what hobbies"})
	res, err := reg.Dispatch(ctx, "hindsight_recall", recallArgs)
	if err != nil {
		t.Fatalf("dispatch recall: %v", err)
	}
	if !strings.Contains(res, "chess") {
		t.Errorf("missing chess in recall result: %s", res)
	}
	if atomic.LoadInt32(&recallHits) != 1 {
		t.Errorf("recallHits = %d", recallHits)
	}

	reflectArgs, _ := json.Marshal(map[string]string{"query": "what do I like"})
	res, err = reg.Dispatch(ctx, "hindsight_reflect", reflectArgs)
	if err != nil {
		t.Fatalf("dispatch reflect: %v", err)
	}
	if !strings.Contains(res, "chess") {
		t.Errorf("missing chess in reflect result: %s", res)
	}
	if atomic.LoadInt32(&reflectHits) != 1 {
		t.Errorf("reflectHits = %d", reflectHits)
	}

	retainArgs, _ := json.Marshal(map[string]string{"content": "remember this"})
	res, err = reg.Dispatch(ctx, "hindsight_retain", retainArgs)
	if err != nil {
		t.Fatalf("dispatch retain: %v", err)
	}
	if !strings.Contains(res, `"ok":true`) {
		t.Errorf("expected ok:true, got %s", res)
	}
}
