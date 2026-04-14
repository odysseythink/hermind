package memprovider

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/storage/sqlite"
	"github.com/odysseythink/hermind/tool"
)

func TestHolographicRememberAndRecall(t *testing.T) {
	dir := t.TempDir()
	store, err := sqlite.Open(filepath.Join(dir, "holo.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	p := NewHolographic(store)
	ctx := context.Background()
	if err := p.Initialize(ctx, "sess-1"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := p.SyncTurn(ctx, "I like mountains", "got it"); err != nil {
		t.Fatalf("SyncTurn: %v", err)
	}

	reg := tool.NewRegistry()
	p.RegisterTools(reg)

	args, _ := json.Marshal(map[string]any{"content": "user enjoys hiking", "category": "preference"})
	if _, err := reg.Dispatch(ctx, "holographic_remember", args); err != nil {
		t.Fatalf("dispatch remember: %v", err)
	}

	findArgs, _ := json.Marshal(map[string]string{"query": "hiking"})
	res, err := reg.Dispatch(ctx, "holographic_recall", findArgs)
	if err != nil {
		t.Fatalf("dispatch recall: %v", err)
	}
	if !strings.Contains(res, "hiking") {
		t.Errorf("expected 'hiking' in result, got %s", res)
	}
}
