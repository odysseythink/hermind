package memorylayer

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/storage/sqlite"
)

func TestLifecycle_LoadsCoreThenForesight(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "c1", Content: "core one", MemType: "core", Status: "active",
	})
	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "c2", Content: "core two", MemType: "core", Status: "active",
	})
	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "f1", Content: "foresight near",
		MemType: "foresight", Status: "active",
		ExpiresAt: now.Add(3 * 24 * time.Hour),
	})
	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "f2", Content: "foresight far",
		MemType: "foresight", Status: "active",
		ExpiresAt: now.Add(30 * 24 * time.Hour),
	})

	lc := NewLifecycle(store, LifecycleConfig{
		InjectCoreOnStart:      true,
		InjectForesightOnStart: true,
		CoreMaxCount:           10,
		CoreMaxTokens:          600,
		ForesightMaxCount:      3,
		ForesightDaysAhead:     7,
	})

	out, err := lc.OnSessionStart(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 pinned (2 core + 1 near foresight), got %d", len(out))
	}
	if !strings.Contains(out[0].Content, "core") {
		t.Errorf("core must lead, got %q", out[0].Content)
	}
	if !strings.Contains(out[2].Content, "near") {
		t.Errorf("near foresight must be last, got %q", out[2].Content)
	}
}

func TestLifecycle_CoreMaxTokensTrims(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}

	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "c1", Content: strings.Repeat("a", 300), MemType: "core", Status: "active",
	})
	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "c2", Content: strings.Repeat("b", 300), MemType: "core", Status: "active",
	})
	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "c3", Content: strings.Repeat("c", 300), MemType: "core", Status: "active",
	})

	lc := NewLifecycle(store, LifecycleConfig{
		InjectCoreOnStart: true,
		CoreMaxCount:      10,
		CoreMaxTokens:     500,
	})

	out, err := lc.OnSessionStart(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 core entry (300 ≤ 500, next would push to 600 > 500), got %d", len(out))
	}
}

func TestLifecycle_NoCoreNoForesight(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}

	lc := NewLifecycle(store, LifecycleConfig{
		InjectCoreOnStart:      true,
		InjectForesightOnStart: true,
	})

	out, err := lc.OnSessionStart(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty slice, got %d", len(out))
	}
}

func TestLifecycle_ExpiredForesightExcluded(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "f1", Content: "expired foresight",
		MemType: "foresight", Status: "active",
		ExpiresAt: now.Add(-1 * time.Hour),
	})

	lc := NewLifecycle(store, LifecycleConfig{
		InjectForesightOnStart: true,
		ForesightMaxCount:      3,
		ForesightDaysAhead:     7,
	})

	out, err := lc.OnSessionStart(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("expected 0 (expired foresight excluded), got %d", len(out))
	}
}

func TestLifecycle_DisabledTogglesAreNoOp(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}

	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "c1", Content: "core", MemType: "core", Status: "active",
	})
	_ = store.SaveMemory(context.Background(), &storage.Memory{
		ID: "f1", Content: "foresight", MemType: "foresight", Status: "active",
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	})

	lc := NewLifecycle(store, LifecycleConfig{
		InjectCoreOnStart:      false,
		InjectForesightOnStart: false,
	})

	out, err := lc.OnSessionStart(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("expected 0 when both toggles disabled, got %d", len(out))
	}
}
