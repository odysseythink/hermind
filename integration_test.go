// integration_test.go — end-to-end test for skill-generation tagging lifecycle
package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/skills"
	"github.com/odysseythink/hermind/storage"
	sqlitestore "github.com/odysseythink/hermind/storage/sqlite"
)

// TestSkillGenerationLifecycle exercises the end-to-end lifecycle:
// 1. Tracker with empty skill directory
// 2. Write a skill file → Refresh → bumped == true, seq == 1
// 3. Save a Memory → BumpMemoryUsage(used=true) → ReinforcedAtSeq == 1
// 4. Edit the skill file → Refresh → bumped == true, seq == 2
// 5. Query memory_events with kind == "skills.generation_bumped" → 2 events
func TestSkillGenerationLifecycle(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "skills")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	// Open a temp sqlite store and migrate.
	store, err := sqlitestore.Open(filepath.Join(tmp, "state.db"))
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	// Construct a Tracker with the empty skill directory.
	tracker := skills.NewTracker(store, skillDir)

	// Step 1: Initial library — write one skill file.
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "a.md"), []byte("alpha"), 0o644))
	bumped, err := tracker.Refresh(context.Background())
	require.NoError(t, err)
	require.True(t, bumped, "first Refresh after writing skill should bump")

	// Verify seq == 1.
	gen, err := tracker.Current(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), gen.Seq, "seq should be 1 after first bump")

	// Step 2: Save a Memory and reinforce it at seq=1.
	require.NoError(t, store.SaveMemory(context.Background(), &storage.Memory{
		ID:      "m1",
		Content: "test memory",
	}))
	require.NoError(t, store.BumpMemoryUsage(context.Background(), "m1", true))

	got, err := store.GetMemory(context.Background(), "m1")
	require.NoError(t, err)
	require.Equal(t, int64(1), got.ReinforcedAtSeq, "memory should be reinforced at seq=1")

	// Step 3: Bump library to seq=2 by editing the skill file.
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "a.md"), []byte("beta"), 0o644))
	bumped, err = tracker.Refresh(context.Background())
	require.NoError(t, err)
	require.True(t, bumped, "second Refresh after editing skill should bump")

	// Verify seq == 2.
	gen, err = tracker.Current(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(2), gen.Seq, "seq should be 2 after second bump")

	// Step 4: Query memory_events and verify 2 "skills.generation_bumped" events.
	events, err := store.ListMemoryEvents(context.Background(), 10, 0, []string{"skills.generation_bumped"})
	require.NoError(t, err)
	require.Len(t, events, 2, "should have exactly 2 generation_bumped events")

	// Verify the event kinds are correct.
	for _, ev := range events {
		require.Equal(t, "skills.generation_bumped", ev.Kind)
	}
}
