package skills

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/odysseythink/hermind/storage"
	sqlitestore "github.com/odysseythink/hermind/storage/sqlite"
	"github.com/stretchr/testify/require"
)

func TestComputeLibraryHashEmptyDir(t *testing.T) {
	dir := t.TempDir()
	h, err := computeLibraryHash(dir)
	require.NoError(t, err)
	require.NotEmpty(t, h, "even an empty dir produces a deterministic hash")
}

func TestComputeLibraryHashDeterministic(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("hello"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.md"), []byte("world"), 0o644))
	h1, err := computeLibraryHash(dir)
	require.NoError(t, err)
	h2, err := computeLibraryHash(dir)
	require.NoError(t, err)
	require.Equal(t, h1, h2)
}

func TestComputeLibraryHashContentSensitive(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.md")
	require.NoError(t, os.WriteFile(p, []byte("hello"), 0o644))
	h1, _ := computeLibraryHash(dir)
	require.NoError(t, os.WriteFile(p, []byte("hello!"), 0o644))
	h2, _ := computeLibraryHash(dir)
	require.NotEqual(t, h1, h2, "1-byte content change must change hash")
}

func TestComputeLibraryHashMtimeInvariant(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.md")
	require.NoError(t, os.WriteFile(p, []byte("hello"), 0o644))
	h1, _ := computeLibraryHash(dir)
	// touch — change mtime without changing content
	future := time.Now().Add(time.Hour)
	require.NoError(t, os.Chtimes(p, future, future))
	h2, _ := computeLibraryHash(dir)
	require.Equal(t, h1, h2, "mtime change must not affect hash")
}

func TestComputeLibraryHashOnlyMd(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("x"), 0o644))
	h1, _ := computeLibraryHash(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("y"), 0o644))
	h2, _ := computeLibraryHash(dir)
	require.Equal(t, h1, h2, "non-.md files must be ignored")
}

func newSkillsTestStore(t *testing.T) storage.Storage {
	t.Helper()
	dir := t.TempDir()
	st, err := sqlitestore.Open(filepath.Join(dir, "state.db"))
	require.NoError(t, err)
	require.NoError(t, st.Migrate())
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestTrackerRefreshFirstCallBumps(t *testing.T) {
	skillDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "a.md"), []byte("hi"), 0o644))
	store := newSkillsTestStore(t)
	tr := NewTracker(store, skillDir)
	bumped, err := tr.Refresh(context.Background())
	require.NoError(t, err)
	require.True(t, bumped)
	cur, err := tr.Current(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), cur.Seq)
}

func TestTrackerRefreshSecondCallNoBump(t *testing.T) {
	skillDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "a.md"), []byte("hi"), 0o644))
	store := newSkillsTestStore(t)
	tr := NewTracker(store, skillDir)
	_, _ = tr.Refresh(context.Background())
	bumped, err := tr.Refresh(context.Background())
	require.NoError(t, err)
	require.False(t, bumped)
	cur, _ := tr.Current(context.Background())
	require.Equal(t, int64(1), cur.Seq)
}

func TestTrackerRefreshAfterEditBumps(t *testing.T) {
	skillDir := t.TempDir()
	p := filepath.Join(skillDir, "a.md")
	require.NoError(t, os.WriteFile(p, []byte("hi"), 0o644))
	store := newSkillsTestStore(t)
	tr := NewTracker(store, skillDir)
	_, _ = tr.Refresh(context.Background())
	require.NoError(t, os.WriteFile(p, []byte("hello"), 0o644))
	bumped, _ := tr.Refresh(context.Background())
	require.True(t, bumped)
	cur, _ := tr.Current(context.Background())
	require.Equal(t, int64(2), cur.Seq)
}

func TestTrackerRefreshEmitsAuditOnBump(t *testing.T) {
	skillDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "a.md"), []byte("hi"), 0o644))
	store := newSkillsTestStore(t)
	tr := NewTracker(store, skillDir)
	bumped, err := tr.Refresh(context.Background())
	require.NoError(t, err)
	require.True(t, bumped)
	events, err := store.ListMemoryEvents(context.Background(), 10, 0, []string{"skills.generation_bumped"})
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "skills.generation_bumped", events[0].Kind)
	require.Contains(t, string(events[0].Data), `"new_seq":1`)
}

func TestTrackerRefreshNoAuditOnNoBump(t *testing.T) {
	skillDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "a.md"), []byte("hi"), 0o644))
	store := newSkillsTestStore(t)
	tr := NewTracker(store, skillDir)
	_, _ = tr.Refresh(context.Background())
	_, _ = tr.Refresh(context.Background()) // no-bump
	events, _ := store.ListMemoryEvents(context.Background(), 10, 0, []string{"skills.generation_bumped"})
	require.Len(t, events, 1, "no-bump must not append a second event")
}
