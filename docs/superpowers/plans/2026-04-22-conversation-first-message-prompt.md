# Conversation First-Message Prompt Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make conversations lazy-created on the user's first message, freeze `system_prompt` as `default + "\n\n" + first_user_message`, derive title from the first 10 runes (operator-editable), wipe existing rows via a lightweight schema-versioned migration, and surface both web-chat and IM-gateway sessions in a single unified list.

**Architecture:** Backend: new pure `DeriveTitle`, refactor `agent.Engine.ensureSession` to return `(*Session, created_bool, error)` with composition, `RunConversation` uses the frozen stored prompt, `sessionrun.Run` publishes a `session_created` SSE event after creation. Storage: add a tiny `schema_meta` table + version runner with a v2 step that wipes `sessions`/`messages`. Frontend: widen Session schema, poll + focus-refetch for IM-originated sessions, double-click rename on `SessionItem`, reducer `chat/session/created` with full DTO, remove the optimistic-placeholder behavior of `newSession`.

**Tech Stack:** Go 1.21+, SQLite (modernc.org/sqlite), go-chi/chi v5, React 18 + TypeScript + Vite, zod, Vitest + React Testing Library.

**Spec:** [2026-04-22-conversation-first-message-prompt-design.md](../specs/2026-04-22-conversation-first-message-prompt-design.md)

---

## Task 1: `DeriveTitle` pure function

**Files:**
- Create: `agent/title.go`
- Test: `agent/title_test.go`

- [ ] **Step 1: Write the failing test**

```go
// agent/title_test.go
package agent

import "testing"

func TestDeriveTitle(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"empty",           "",                          ""},
		{"whitespace only", "   \n\t  ",                 ""},
		{"short ascii",     "hi",                        "hi"},
		{"short cjk",       "你好",                       "你好"},
		{"exactly 10",      "abcdefghij",                "abcdefghij"},
		{"over 10 ascii",   "the quick brown fox jumps", "the quick "},
		{"over 10 cjk",     "一二三四五六七八九十十一十二", "一二三四五六七八九十"},
		{"newlines become spaces", "hello\nworld", "hello worl"},
		{"crlf becomes spaces",    "a\r\nb",      "a  b"},
		{"trim then cut",   "   hello world   ",         "hello worl"},
		{"emoji rune",      "🎉🎉🎉celebrate now!",      "🎉🎉🎉celebra"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveTitle(tc.in)
			if got != tc.want {
				t.Errorf("DeriveTitle(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./agent/ -run TestDeriveTitle -v`
Expected: FAIL — `undefined: DeriveTitle`

- [ ] **Step 3: Write minimal implementation**

```go
// agent/title.go
package agent

import "strings"

// titleMaxRunes caps DeriveTitle output length in Unicode code points.
const titleMaxRunes = 10

// DeriveTitle produces a short display title from the user's first message:
// replaces newlines with spaces, trims surrounding whitespace, truncates to
// titleMaxRunes code points. Empty input returns an empty string — callers
// render a localized "Untitled" in that case.
func DeriveTitle(msg string) string {
	s := strings.ReplaceAll(msg, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) > titleMaxRunes {
		runes = runes[:titleMaxRunes]
	}
	return string(runes)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./agent/ -run TestDeriveTitle -v`
Expected: PASS — all 11 cases.

- [ ] **Step 5: Commit**

```bash
git add agent/title.go agent/title_test.go
git commit -m "feat(agent): add DeriveTitle helper for conversation titles

Pure function: replaces CR/LF with spaces, trims, truncates to 10 Unicode
code points. Handles ASCII, CJK, and emoji correctly.
"
```

---

## Task 2: SQLite `schema_meta` table + version-runner scaffolding

**Files:**
- Modify: `storage/sqlite/migrate.go`
- Test: `storage/sqlite/migrate_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
// storage/sqlite/migrate_test.go
package sqlite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrate_CreatesSchemaMetaAtV1(t *testing.T) {
	store := newTestStore(t)
	var value string
	err := store.db.QueryRowContext(
		context.Background(),
		`SELECT value FROM schema_meta WHERE key = 'version'`,
	).Scan(&value)
	require.NoError(t, err)
	// Until Task 3 ships, freshly-initialized DBs stay at version 1.
	assert.Equal(t, "1", value)
}

func TestMigrate_Idempotent(t *testing.T) {
	store := newTestStore(t)
	// Migrate a second time; must not fail, must not bump version unnecessarily.
	require.NoError(t, store.Migrate())
	var value string
	require.NoError(t, store.db.QueryRowContext(
		context.Background(),
		`SELECT value FROM schema_meta WHERE key = 'version'`,
	).Scan(&value))
	assert.Equal(t, "1", value)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./storage/sqlite/ -run TestMigrate -v`
Expected: FAIL — `no such table: schema_meta`

- [ ] **Step 3: Write minimal implementation**

Replace the body of `migrate.go`:

```go
// storage/sqlite/migrate.go
package sqlite

import (
	"database/sql"
	"fmt"
)

// schemaSQL is the full initial schema. Designed to match the Python hermes
// state.db layout for compatibility. schema_meta tracks the applied version
// so incremental migrations beyond v1 can run idempotently.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL DEFAULT 'cli',
    user_id TEXT DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    model_config TEXT DEFAULT '{}',
    system_prompt TEXT DEFAULT '',
    parent_session_id TEXT DEFAULT '',
    started_at REAL NOT NULL,
    ended_at REAL,
    end_reason TEXT DEFAULT '',
    message_count INTEGER NOT NULL DEFAULT 0,
    tool_call_count INTEGER NOT NULL DEFAULT 0,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    cache_write_tokens INTEGER NOT NULL DEFAULT 0,
    reasoning_tokens INTEGER NOT NULL DEFAULT 0,
    billing_provider TEXT DEFAULT '',
    billing_base_url TEXT DEFAULT '',
    estimated_cost_usd REAL NOT NULL DEFAULT 0,
    actual_cost_usd REAL NOT NULL DEFAULT 0,
    cost_status TEXT DEFAULT '',
    title TEXT DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_sessions_source ON sessions(source);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_started_at ON sessions(started_at);

CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    tool_call_id TEXT DEFAULT '',
    tool_calls TEXT DEFAULT '',
    tool_name TEXT DEFAULT '',
    timestamp REAL NOT NULL,
    token_count INTEGER NOT NULL DEFAULT 0,
    finish_reason TEXT DEFAULT '',
    reasoning TEXT DEFAULT '',
    reasoning_details TEXT DEFAULT '',
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);

CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
    content,
    content='messages',
    content_rowid='id'
);

CREATE TRIGGER IF NOT EXISTS messages_fts_insert AFTER INSERT ON messages BEGIN
    INSERT INTO messages_fts(rowid, content) VALUES (new.id, new.content);
END;

CREATE TRIGGER IF NOT EXISTS messages_fts_delete AFTER DELETE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, content) VALUES ('delete', old.id, old.content);
END;

CREATE TRIGGER IF NOT EXISTS messages_fts_update AFTER UPDATE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, content) VALUES ('delete', old.id, old.content);
    INSERT INTO messages_fts(rowid, content) VALUES (new.id, new.content);
END;

CREATE TABLE IF NOT EXISTS memories (
    id TEXT PRIMARY KEY,
    user_id TEXT DEFAULT '',
    content TEXT NOT NULL,
    category TEXT DEFAULT '',
    tags TEXT DEFAULT '',
    metadata TEXT DEFAULT '{}',
    created_at REAL NOT NULL,
    updated_at REAL NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_memories_user ON memories(user_id);
CREATE INDEX IF NOT EXISTS idx_memories_created ON memories(created_at);

CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
    content,
    content='memories',
    content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS memories_fts_insert AFTER INSERT ON memories BEGIN
    INSERT INTO memories_fts(rowid, content) VALUES (new.rowid, new.content);
END;

CREATE TRIGGER IF NOT EXISTS memories_fts_delete AFTER DELETE ON memories BEGIN
    INSERT INTO memories_fts(memories_fts, rowid, content) VALUES ('delete', old.rowid, old.content);
END;

CREATE TRIGGER IF NOT EXISTS memories_fts_update AFTER UPDATE ON memories BEGIN
    INSERT INTO memories_fts(memories_fts, rowid, content) VALUES ('delete', old.rowid, old.content);
    INSERT INTO memories_fts(rowid, content) VALUES (new.rowid, new.content);
END;

CREATE TABLE IF NOT EXISTS schema_meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
INSERT OR IGNORE INTO schema_meta (key, value) VALUES ('version', '1');
`

// currentSchemaVersion is the highest version this binary knows about. Any
// stored version less than this triggers incremental migration steps in
// Migrate(). When adding a new step, bump this constant AND add the matching
// case in applyVersion.
const currentSchemaVersion = 1

// Migrate applies the base schema, then runs any versioned migration steps
// up to currentSchemaVersion. Idempotent: safe to call on an up-to-date DB.
func (s *Store) Migrate() error {
	if _, err := s.db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("sqlite: migrate base: %w", err)
	}
	for {
		v, err := s.schemaVersion()
		if err != nil {
			return err
		}
		if v >= currentSchemaVersion {
			return nil
		}
		if err := s.applyVersion(v + 1); err != nil {
			return fmt.Errorf("sqlite: migrate v%d: %w", v+1, err)
		}
	}
}

func (s *Store) schemaVersion() (int, error) {
	var raw string
	err := s.db.QueryRow(
		`SELECT value FROM schema_meta WHERE key = 'version'`,
	).Scan(&raw)
	if err != nil {
		return 0, fmt.Errorf("sqlite: read schema version: %w", err)
	}
	var v int
	if _, err := fmt.Sscanf(raw, "%d", &v); err != nil {
		return 0, fmt.Errorf("sqlite: parse schema version %q: %w", raw, err)
	}
	return v, nil
}

// applyVersion dispatches to the step that bumps the DB from v-1 to v.
// Runs in a single transaction so partial failures leave the version unchanged.
func (s *Store) applyVersion(v int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	switch v {
	// Task 3 adds case 2.
	default:
		return fmt.Errorf("no migration step for v%d", v)
	}
	// Unreachable while currentSchemaVersion == 1.
	_, _ = tx.Exec(`UPDATE schema_meta SET value = ? WHERE key = 'version'`, fmt.Sprintf("%d", v))
	return tx.Commit()
}

// _ silences the sql import when applyVersion has no real cases yet.
var _ = sql.ErrNoRows
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./storage/sqlite/ -run TestMigrate -v`
Expected: PASS — both tests green. Also run the full package to make sure nothing else broke: `go test ./storage/sqlite/ -v`.

- [ ] **Step 5: Commit**

```bash
git add storage/sqlite/migrate.go storage/sqlite/migrate_test.go
git commit -m "feat(storage): add schema_meta table and version-runner

Introduces lightweight incremental migration infrastructure. currentSchemaVersion
is 1 — v2 (row wipe) lands in the next commit.
"
```

---

## Task 3: SQLite v2 step (wipe sessions + messages)

**Files:**
- Modify: `storage/sqlite/migrate.go`
- Test: `storage/sqlite/migrate_test.go`

- [ ] **Step 1: Write the failing test**

Append to `storage/sqlite/migrate_test.go`:

```go
func TestMigrate_V2_WipesSessionsAndMessages(t *testing.T) {
	store := newTestStore(t)  // migrated to v1
	ctx := context.Background()
	now := time.Now().UTC()

	require.NoError(t, store.CreateSession(ctx, &storage.Session{
		ID: "s1", Source: "cli", Model: "m", StartedAt: now,
	}))
	require.NoError(t, store.AddMessage(ctx, "s1", &storage.StoredMessage{
		Role: "user", Content: `"hi"`, Timestamp: now,
	}))
	require.NoError(t, store.SaveMemory(ctx, &storage.Memory{
		ID: "mem1", Content: "kept across migrations", CreatedAt: now, UpdatedAt: now,
	}))

	// Force the runner back to v1 so Migrate() advances to v2.
	_, err := store.db.ExecContext(ctx,
		`UPDATE schema_meta SET value = '1' WHERE key = 'version'`)
	require.NoError(t, err)

	require.NoError(t, store.Migrate())

	var sessionCount, messageCount, memoryCount int
	require.NoError(t, store.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sessions`).Scan(&sessionCount))
	require.NoError(t, store.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM messages`).Scan(&messageCount))
	require.NoError(t, store.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM memories`).Scan(&memoryCount))

	assert.Equal(t, 0, sessionCount, "v2 should wipe sessions")
	assert.Equal(t, 0, messageCount, "v2 should wipe messages")
	assert.Equal(t, 1, memoryCount, "v2 must NOT touch memories")

	var version string
	require.NoError(t, store.db.QueryRowContext(ctx,
		`SELECT value FROM schema_meta WHERE key = 'version'`).Scan(&version))
	assert.Equal(t, "2", version)
}

func TestMigrate_V2_Idempotent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	// First Migrate() in newTestStore already ran. Populate, then re-Migrate.
	require.NoError(t, store.CreateSession(ctx, &storage.Session{
		ID: "survivor", Source: "cli", Model: "m", StartedAt: time.Now().UTC(),
	}))
	require.NoError(t, store.Migrate())

	var count int
	require.NoError(t, store.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sessions`).Scan(&count))
	// Already at v2; second Migrate() is a no-op; row survives.
	assert.Equal(t, 1, count)
}
```

Also update the top import block of `migrate_test.go`:

```go
import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/storage"
)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./storage/sqlite/ -run TestMigrate -v`
Expected: `TestMigrate_V2_WipesSessionsAndMessages` FAILs — either "no migration step for v2" or version stays at 1.

- [ ] **Step 3: Write minimal implementation**

Modify `migrate.go`:

1. Bump `currentSchemaVersion` from `1` to `2`.
2. Replace the `applyVersion` function body with a real v2 case:

```go
const currentSchemaVersion = 2

// applyVersion dispatches to the step that bumps the DB from v-1 to v.
// Runs in a single transaction so partial failures leave the version unchanged.
func (s *Store) applyVersion(v int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	switch v {
	case 2:
		if _, err := tx.Exec(`DELETE FROM messages`); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM sessions`); err != nil {
			return err
		}
	default:
		return fmt.Errorf("no migration step for v%d", v)
	}

	if _, err := tx.Exec(
		`UPDATE schema_meta SET value = ? WHERE key = 'version'`,
		fmt.Sprintf("%d", v),
	); err != nil {
		return err
	}
	return tx.Commit()
}
```

3. Delete the now-unneeded `var _ = sql.ErrNoRows` line and the `database/sql` import if sql isn't used elsewhere in the file.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./storage/sqlite/ -v`
Expected: PASS — all migrate tests green, nothing else broken.

- [ ] **Step 5: Commit**

```bash
git add storage/sqlite/migrate.go storage/sqlite/migrate_test.go
git commit -m "feat(storage): v2 migration wipes sessions + messages

Operators upgrading from v1 start fresh. Memories, skills, and schema stay put.
Runs inside a transaction so partial failure leaves version at 1.
"
```

---

## Task 4: Refactor `ensureSession` — compose prompt, derive title, return session

**Files:**
- Modify: `agent/conversation.go` (lines 47-62 and 266-283)
- Test: `agent/conversation_test.go` (extend existing)

- [ ] **Step 1: Write the failing test**

First check existing engine tests so we use the same helpers:

```bash
grep -n "NewEngineWithToolsAndAux\|sqlite.New\|storage\.Storage" agent/*_test.go | head -10
```

Match whichever constructor + in-memory sqlite (via `storage/sqlite.Open(":memory:")`) pattern is already used. Create `agent/ensure_session_test.go`:

```go
// agent/ensure_session_test.go
package agent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/storage/sqlite"
)

func newTestStoreForEngine(t *testing.T) storage.Storage {
	t.Helper()
	s, err := sqlite.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, s.Migrate())
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func newEngineWithStorage(t *testing.T, store storage.Storage, platform string) *Engine {
	t.Helper()
	return NewEngineWithToolsAndAux(nil, nil, store, nil, config.AgentConfig{MaxTurns: 3}, platform)
}

func TestEnsureSession_NewRow_ComposesPromptAndTitle(t *testing.T) {
	store := newTestStoreForEngine(t)
	eng := newEngineWithStorage(t, store, "web")

	opts := &RunOptions{
		SessionID:   "s-new-1",
		UserMessage: "Build me a haiku generator",
		Model:       "claude-opus-4-7",
	}

	sess, created, err := eng.ensureSession(context.Background(), opts, "You are helpful.", opts.UserMessage, opts.Model)
	require.NoError(t, err)
	require.NotNil(t, sess)
	assert.True(t, created)
	assert.Equal(t, "You are helpful.\n\nBuild me a haiku generator", sess.SystemPrompt)
	assert.Equal(t, "Build me ", sess.Title)  // 10 runes
	assert.Equal(t, "web", sess.Source)
}

func TestEnsureSession_ExistingRow_Unchanged(t *testing.T) {
	store := newTestStoreForEngine(t)
	ctx := context.Background()
	require.NoError(t, store.CreateSession(ctx, &storage.Session{
		ID:           "s-existing",
		Source:       "telegram",
		Model:        "claude-opus-4-7",
		SystemPrompt: "You are helpful.\n\nfirst ever",
		Title:        "first ever",
		StartedAt:    time.Now().UTC(),
	}))
	eng := newEngineWithStorage(t, store, "web")

	sess, created, err := eng.ensureSession(ctx, &RunOptions{
		SessionID:   "s-existing",
		UserMessage: "second attempt", // MUST be ignored
	}, "new default prompt", "second attempt", "")
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, "You are helpful.\n\nfirst ever", sess.SystemPrompt)
	assert.Equal(t, "first ever", sess.Title)
}

func TestEnsureSession_EmptyFirstMessage_FallsBackToDefault(t *testing.T) {
	store := newTestStoreForEngine(t)
	eng := newEngineWithStorage(t, store, "web")

	sess, created, err := eng.ensureSession(context.Background(), &RunOptions{
		SessionID: "s-empty", UserMessage: "   ",
	}, "default", "   ", "m")
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, "default", sess.SystemPrompt)
	assert.Equal(t, "", sess.Title)
}
```

If the `:memory:` path doesn't work with the current sqlite driver, fall back to `t.TempDir()+"/test.db"` — SQLite won't care about the real filesystem path for a short-lived test.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./agent/ -run TestEnsureSession -v`
Expected: FAIL — either function signature mismatch (`ensureSession` currently returns only `error`) or composition doesn't happen.

- [ ] **Step 3: Write minimal implementation**

Replace `ensureSession` (around line 266-283):

```go
// ensureSession creates a new session row if it doesn't exist. On new rows,
// the stored system prompt is composed as `defaultPrompt + "\n\n" + firstMsg`
// and title is DeriveTitle(firstMsg). Returns the session (existing or
// freshly created) and a bool indicating whether this call created it.
func (e *Engine) ensureSession(
	ctx context.Context,
	opts *RunOptions,
	defaultPrompt, firstMsg, model string,
) (*storage.Session, bool, error) {
	if s, err := e.storage.GetSession(ctx, opts.SessionID); err == nil {
		return s, false, nil
	} else if !errors.Is(err, storage.ErrNotFound) {
		return nil, false, err
	}

	composed := defaultPrompt
	if strings.TrimSpace(firstMsg) != "" {
		composed = defaultPrompt + "\n\n" + firstMsg
	}
	s := &storage.Session{
		ID:           opts.SessionID,
		Source:       e.platform,
		UserID:       opts.UserID,
		Model:        model,
		SystemPrompt: composed,
		Title:        DeriveTitle(firstMsg),
		StartedAt:    time.Now().UTC(),
	}
	if err := e.storage.CreateSession(ctx, s); err != nil {
		return nil, false, err
	}
	return s, true, nil
}
```

Add `"strings"` to the import block if it isn't already there.

Update the call site in `RunConversation` (around line 54-56):

```go
var effectivePrompt string = systemPrompt
var sessionCreated bool
if e.storage != nil {
	sess, created, err := e.ensureSession(ctx, opts, systemPrompt, opts.UserMessage, model)
	if err != nil {
		return nil, fmt.Errorf("engine: ensure session: %w", err)
	}
	effectivePrompt = sess.SystemPrompt
	sessionCreated = created
	if err := e.persistMessage(ctx, opts.SessionID, &history[len(history)-1]); err != nil {
		return nil, fmt.Errorf("engine: persist user message: %w", err)
	}
	if sessionCreated && e.onSessionCreated != nil {
		e.onSessionCreated(sess) // Task 6 wires this
	}
}
```

Down in the loop (around line 99), change `req.SystemPrompt = systemPrompt` to `req.SystemPrompt = effectivePrompt`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./agent/ -v`
Expected: PASS — new tests green, no regressions.

- [ ] **Step 5: Commit**

```bash
git add agent/conversation.go agent/conversation_test.go agent/title.go agent/title_test.go
git commit -m "refactor(agent): ensureSession composes prompt+title and returns session

- ensureSession now returns (*storage.Session, created bool, error).
- On new row: system_prompt = default + '\n\n' + firstMsg (frozen), title
  derived from first 10 runes of firstMsg.
- RunConversation uses the stored (effective) prompt on every turn so later
  config changes don't leak into long-running sessions.
"
```

---

## Task 5: `Engine.SetSessionCreatedCallback` hook

**Files:**
- Modify: `agent/engine.go`
- Test: `agent/engine_callbacks_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
// agent/engine_callbacks_test.go
package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/storage"
)

func TestSetSessionCreatedCallback_FiresOnNewRow(t *testing.T) {
	store := newTestStoreForEngine(t)  // from Task 4
	eng := newEngineWithStorage(t, store, "web")

	var captured *storage.Session
	eng.SetSessionCreatedCallback(func(s *storage.Session) { captured = s })

	_, _, err := eng.ensureSession(context.Background(), &RunOptions{
		SessionID: "s-cb", UserMessage: "hi",
	}, "default", "hi", "m")
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, "s-cb", captured.ID)
	assert.Equal(t, "hi", captured.Title)
}

func TestSetSessionCreatedCallback_SilentOnExistingRow(t *testing.T) {
	store := newTestStoreForEngine(t)
	ctx := context.Background()
	require.NoError(t, store.CreateSession(ctx, &storage.Session{
		ID: "s-existing-cb", Source: "cli", Model: "m",
		SystemPrompt: "p", Title: "t", StartedAt: time.Now().UTC(),
	}))
	eng := newEngineWithStorage(t, store, "web")

	var fired bool
	eng.SetSessionCreatedCallback(func(*storage.Session) { fired = true })
	_, _, err := eng.ensureSession(ctx, &RunOptions{
		SessionID: "s-existing-cb", UserMessage: "ignored",
	}, "default", "ignored", "m")
	require.NoError(t, err)
	assert.False(t, fired, "callback must not fire on existing session")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./agent/ -run SessionCreatedCallback -v`
Expected: FAIL — `SetSessionCreatedCallback` undefined.

- [ ] **Step 3: Write minimal implementation**

In `agent/engine.go`:

Add field to the `Engine` struct (alongside `onStreamDelta`, `onToolStart`, etc.):

```go
// (inside struct) onSessionCreated func(*storage.Session)
```

Add the setter (following the `SetStreamDeltaCallback` pattern near line 127):

```go
// SetSessionCreatedCallback registers a callback invoked exactly once per
// session, immediately after ensureSession materializes a new row. Must be
// set before RunConversation. Calling after is undefined behavior.
func (e *Engine) SetSessionCreatedCallback(fn func(s *storage.Session)) {
	e.onSessionCreated = fn
}
```

In the Task-4-revised `RunConversation` block where `sessionCreated` is checked, the call `e.onSessionCreated(sess)` should already be in place.

Add `"github.com/odysseythink/hermind/storage"` to `agent/engine.go` imports if not present.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./agent/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add agent/engine.go agent/engine_callbacks_test.go
git commit -m "feat(agent): SetSessionCreatedCallback hook

Fires once per session when ensureSession writes a new row. sessionrun.Run
uses it to publish session_created to the stream hub.
"
```

---

## Task 6: `sessionrun.Run` publishes `session_created` StreamEvent

**Files:**
- Modify: `api/sessionrun/runner.go`
- Modify: `api/sessionrun_bridge.go` (if the event type mapping lives there — grep first)
- Test: `api/sessionrun/runner_test.go` (create or extend)

- [ ] **Step 1: Explore existing test patterns**

```bash
ls api/sessionrun/
grep -n "func Test\|stubProvider\|fakeProvider" api/sessionrun/*_test.go 2>/dev/null | head -20
```

If `api/sessionrun/runner_test.go` already exists with a capturing hub + stub provider + test-deps helper, reuse them. Otherwise scaffold in one file.

Write a failing test. `sessionrun.Event` is `{Type string; SessionID string; Data any}` and the hub interface is `EventPublisher{ Publish(Event) }`.

```go
// api/sessionrun/runner_session_created_test.go
package sessionrun

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/storage"
)

type capturingHub struct {
	events []Event
}

func (h *capturingHub) Publish(e Event) { h.events = append(h.events, e) }

func (h *capturingHub) eventsOfType(t string) []Event {
	var out []Event
	for _, e := range h.events {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

func TestRun_PublishesSessionCreated_OnBrandNewSession(t *testing.T) {
	hub := &capturingHub{}
	deps := newTestRunDeps(t, hub)  // see note below

	err := Run(context.Background(), deps, Request{
		SessionID:   "brand-new-sess",
		UserMessage: "Build me a haiku generator",
		Model:       "stub",
	})
	require.NoError(t, err)

	created := hub.eventsOfType("session_created")
	require.Len(t, created, 1, "expected exactly one session_created event")
	assert.Equal(t, "brand-new-sess", created[0].SessionID)

	dto, ok := created[0].Data.(map[string]any)
	require.True(t, ok, "Data must be map[string]any")
	assert.Equal(t, "brand-new-sess", dto["id"])
	assert.Equal(t, "Build me ", dto["title"])
}

func TestRun_NoSessionCreated_OnExistingSession(t *testing.T) {
	hub := &capturingHub{}
	deps := newTestRunDeps(t, hub)
	require.NoError(t, deps.Storage.CreateSession(context.Background(), &storage.Session{
		ID: "already-there", Source: "web", Model: "stub", SystemPrompt: "p", Title: "t",
	}))

	err := Run(context.Background(), deps, Request{
		SessionID:   "already-there",
		UserMessage: "hi again",
		Model:       "stub",
	})
	require.NoError(t, err)

	assert.Empty(t, hub.eventsOfType("session_created"))
}
```

For `newTestRunDeps`: if a similar helper is already in this package (`grep -n "func newTest\|Deps{" api/sessionrun/*_test.go`), reuse it. Otherwise, create `api/sessionrun/testhelpers_test.go` with:

- a stub `provider.Provider` that returns a minimal one-turn non-tool response (model this on any existing stub — `agent/conversation_test.go` or `provider/*_test.go` likely have one; copy that pattern, don't invent new fields)
- a `newTestRunDeps` function that returns a `Deps` wired with `sqlite.Open(":memory:")` + the stub provider + the hub argument

Keep the stub minimal — we only need `Run` to reach `engine.ensureSession` and then terminate cleanly.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./api/sessionrun/ -v`
Expected: FAIL — no `session_created` event in hub.

- [ ] **Step 3: Write minimal implementation**

In `api/sessionrun/runner.go`, after the `engine.SetToolResultCallback(...)` block (around line 94) but before the first `deps.Hub.Publish(...)` of the `running` status (line 96), add:

```go
engine.SetSessionCreatedCallback(func(s *storage.Session) {
	deps.Hub.Publish(Event{
		Type:      "session_created",
		SessionID: s.ID,
		Data: map[string]any{
			"id":            s.ID,
			"title":         s.Title,
			"source":        s.Source,
			"model":         s.Model,
			"started_at":    s.StartedAt.Unix(),
			"ended_at":      0,
			"message_count": s.MessageCount,
		},
	})
})
```

Add `"github.com/odysseythink/hermind/storage"` to the import block. Prefer reusing a DTO struct if one is already accessible from this package (do a quick grep for `SessionDTO` before committing to the inline map).

In the web SSE handler (likely `api/sse.go` or `api/stream_bridge.go` — find it via `grep -rn "StreamEvent" api/ | head`), confirm the JSON marshaling of the event passes through without filtering unknown types. If it has an allowlist, add `"session_created"`. If it doesn't, no action.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./api/sessionrun/ ./api/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/sessionrun/runner.go api/sessionrun/runner_test.go api/sse.go
git commit -m "feat(api): publish session_created SSE event on new session

Wires the agent Engine's SetSessionCreatedCallback into sessionrun.Run's
StreamHub so the web sidebar can insert a row when the goroutine's
ensureSession creates a brand-new session.
"
```

---

## Task 7: `PATCH /api/sessions/{id}` endpoint — rename title

**Files:**
- Modify: `api/handlers_sessions.go` (add the handler)
- Modify: `api/server.go` (register the route)
- Test: `api/handlers_sessions_test.go`

- [ ] **Step 1: Write the failing test**

The existing tests use `newTestServerWithStore` and `mockStorage` from `api/handlers_sessions_test.go`. That mock's `UpdateSession` is currently a no-op — we must upgrade it in a small preamble edit before the new tests will pass.

First, edit `api/handlers_sessions_test.go::mockStorage.UpdateSession` (currently around line 58-60) to actually apply the update:

```go
func (m *mockStorage) UpdateSession(ctx context.Context, id string, u *storage.SessionUpdate) error {
	for _, s := range m.sessions {
		if s.ID == id {
			if u.Title != "" {
				s.Title = u.Title
			}
			if u.EndedAt != nil {
				s.EndedAt = u.EndedAt
			}
			if u.EndReason != "" {
				s.EndReason = u.EndReason
			}
			if u.MessageCount != nil {
				s.MessageCount = *u.MessageCount
			}
			return nil
		}
	}
	return storage.ErrNotFound
}
```

Then append the new tests to the same file:

```go
func TestPatchSession_RenamesTitle(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSession("sess-rename")

	rr := httptest.NewRecorder()
	body := strings.NewReader(`{"title":"new title"}`)
	req := httptest.NewRequest("PATCH", "/api/sessions/sess-rename", body)
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	var dto SessionDTO
	if err := json.NewDecoder(rr.Body).Decode(&dto); err != nil {
		t.Fatal(err)
	}
	if dto.Title != "new title" {
		t.Errorf("title = %q, want %q", dto.Title, "new title")
	}
	if dto.ID != "sess-rename" {
		t.Errorf("id = %q, want %q", dto.ID, "sess-rename")
	}
}

func TestPatchSession_EmptyTitle_Returns400(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSession("s1")

	for _, body := range []string{`{"title":""}`, `{"title":"   "}`} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("PATCH", "/api/sessions/s1", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer t")
		req.Header.Set("Content-Type", "application/json")
		s.Router().ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("body=%q: code=%d, want 400", body, rr.Code)
		}
	}
}

func TestPatchSession_TooLong_Returns400(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSession("s2")
	body := `{"title":"` + strings.Repeat("x", 201) + `"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/sessions/s2", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("code=%d, want 400", rr.Code)
	}
}

func TestPatchSession_NotFound_Returns404(t *testing.T) {
	s, _ := newTestServerWithStore(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/sessions/ghost",
		strings.NewReader(`{"title":"anything"}`))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("code=%d, want 404", rr.Code)
	}
}

func TestPatchSession_MissingToken_Returns401(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSession("s3")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/sessions/s3",
		strings.NewReader(`{"title":"new"}`))
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("code=%d, want 401", rr.Code)
	}
}
```

Imports to ensure at the top of the file: `"net/http"`, `"net/http/httptest"`, `"strings"`, `"encoding/json"`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./api/ -run TestPatchSession -v`
Expected: FAIL — route not registered (404 on all) or handler undefined.

- [ ] **Step 3: Write minimal implementation**

Add to `api/handlers_sessions.go` (this file already contains `handleSessionsList`, `handleSessionGet`, `dtoFromSession`, etc. — follow their style exactly):

```go
// handleSessionPatch updates a session's title. Only title is editable in
// this version — system_prompt stays frozen at creation time.
func (s *Server) handleSessionPatch(w http.ResponseWriter, r *http.Request) {
	if s.opts.Storage == nil {
		http.Error(w, "storage not configured", http.StatusServiceUnavailable)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
		return
	}
	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	title := strings.TrimSpace(body.Title)
	if title == "" {
		http.Error(w, "title must not be empty", http.StatusBadRequest)
		return
	}
	if utf8.RuneCountInString(title) > 200 {
		http.Error(w, "title too long (max 200 runes)", http.StatusBadRequest)
		return
	}

	err := s.opts.Storage.UpdateSession(r.Context(), id, &storage.SessionUpdate{Title: title})
	if errors.Is(err, storage.ErrNotFound) {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sess, err := s.opts.Storage.GetSession(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, dtoFromSession(sess))
}
```

Imports to ensure on the file: `"encoding/json"`, `"strings"`, `"unicode/utf8"` (the others — `errors`, `net/http`, `strconv`, `time`, `chi/v5`, `storage` — are already there).

In `api/server.go::buildRouter()`, inside the existing `r.Route("/api", func(r chi.Router) { ... })` block (around line 143-171), add one line right after the existing `r.Get("/sessions/{id}", s.handleSessionGet)`:

```go
r.Patch("/sessions/{id}", s.handleSessionPatch)
```

The `r.Use(auth)` at the top of that block covers all routes inside it, so the 401 test passes automatically.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./api/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/handlers_sessions.go api/handlers_sessions_test.go api/server.go
git commit -m "feat(api): PATCH /api/sessions/{id} renames session title

Validates 1 ≤ len(title) ≤ 200 runes, maps storage.ErrNotFound to 404,
returns fresh SessionDTO on success.
"
```

---

## Task 8: Widen frontend `SessionSummarySchema`

**Files:**
- Modify: `web/src/api/schemas.ts`
- Modify: `web/src/state/chat.ts` (SessionSummary type → import from schemas OR widen)
- Test: `web/src/api/schemas.test.ts` (extend)

- [ ] **Step 1: Write the failing test**

```typescript
// web/src/api/schemas.test.ts — append
import { describe, it, expect } from 'vitest';
import { SessionSummarySchema } from './schemas';

describe('SessionSummarySchema (widened)', () => {
  it('accepts the full backend DTO shape', () => {
    const parsed = SessionSummarySchema.parse({
      id: 'abc',
      title: 'hi there',
      source: 'web',
      model: 'claude-opus-4-7',
      started_at: 1713724000,
      ended_at: 0,
      message_count: 3,
    });
    expect(parsed.source).toBe('web');
    expect(parsed.message_count).toBe(3);
  });

  it('allows optional fields to be missing', () => {
    const parsed = SessionSummarySchema.parse({
      id: 'abc',
      source: 'web',
    });
    expect(parsed.title).toBeUndefined();
  });

  it('requires source', () => {
    expect(() => SessionSummarySchema.parse({ id: 'abc' })).toThrow();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && bun run test --run schemas.test`
Expected: FAIL — `source` not on schema.

- [ ] **Step 3: Write minimal implementation**

In `web/src/api/schemas.ts` replace the `SessionSummarySchema` block:

```typescript
export const SessionSummarySchema = z.object({
  id: z.string(),
  title: z.string().optional(),
  source: z.string(),
  model: z.string().optional(),
  started_at: z.number().optional(),
  ended_at: z.number().optional(),
  message_count: z.number().optional(),
});
export type SessionSummary = z.infer<typeof SessionSummarySchema>;
```

Any consumer still referencing `updated_at` on a `SessionSummary` must be updated. Find them:

```bash
cd web && grep -rn "updated_at\|updatedAt" src/
```

Update each to use `started_at` (the backend uses `started_at` as the primary timestamp; don't invent a separate `updatedAt` concept). Specifically `state/chat.ts` has its own local `SessionSummary` type with `updatedAt: number` — replace it with the imported schema-derived type OR keep it local but rename the field.

In `state/chat.ts`:

```typescript
import type { SessionSummary } from '../api/schemas';
// ...remove the local `export type SessionSummary = ...` block
```

And update the reducer's `chat/session/created` case to spread a full SessionSummary (this gets finalized in Task 9, but for now use a stub that widens the action payload):

```typescript
| { type: 'chat/session/created'; session: SessionSummary }
// ...
case 'chat/session/created':
  return {
    ...state,
    sessions: state.sessions.some((s) => s.id === action.session.id)
      ? state.sessions
      : [action.session, ...state.sessions],
    activeSessionId: action.session.id,
  };
```

Update `state/chat.test.ts` accordingly — the existing test at line 13 passes `{id, title}`; change it to `{session: {id, source: 'web', title: 'New conversation'}}`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd web && bun run test`
Expected: PASS — all schemas + chat-state tests green.

- [ ] **Step 5: Commit**

```bash
git add web/src/api/schemas.ts web/src/state/chat.ts web/src/state/chat.test.ts web/src/api/schemas.test.ts
git commit -m "refactor(web): widen SessionSummary to match backend DTO

SessionSummary now carries source/model/started_at/ended_at/message_count.
chat/session/created reducer action takes a full DTO. updated_at references
replaced with started_at throughout.
"
```

---

## Task 9: `useSessionList` polling + focus-refetch + remove optimistic insert

**Files:**
- Modify: `web/src/hooks/useSessionList.ts`
- Test: `web/src/hooks/useSessionList.test.ts`

- [ ] **Step 1: Write the failing tests**

```typescript
// web/src/hooks/useSessionList.test.ts — replace existing file contents
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { useSessionList } from './useSessionList';

function mockFetchOnce(payload: unknown) {
  return vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
    ok: true,
    status: 200,
    json: async () => payload,
    headers: new Headers({ 'content-type': 'application/json' }),
  } as Response);
}

describe('useSessionList', () => {
  beforeEach(() => { vi.useFakeTimers(); });
  afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks(); });

  it('loads sessions on mount', async () => {
    mockFetchOnce({ sessions: [{ id: 's1', title: 'hi', source: 'web' }], total: 1 });
    const { result } = renderHook(() => useSessionList());
    await waitFor(() => expect(result.current.sessions.length).toBe(1));
    expect(result.current.sessions[0].id).toBe('s1');
  });

  it('newSession returns a uuid but does NOT insert a placeholder', async () => {
    mockFetchOnce({ sessions: [], total: 0 });
    const { result } = renderHook(() => useSessionList());
    await waitFor(() => expect(result.current.sessions.length).toBe(0));
    let id: string | undefined;
    act(() => { id = result.current.newSession(); });
    expect(id).toMatch(/^[0-9a-f-]{36}$/);
    expect(result.current.sessions.length).toBe(0);
  });

  it('insertSession adds a new row (idempotent on same id)', async () => {
    mockFetchOnce({ sessions: [], total: 0 });
    const { result } = renderHook(() => useSessionList());
    await waitFor(() => expect(result.current.sessions.length).toBe(0));
    act(() => result.current.insertSession({ id: 's-new', title: 'hey', source: 'web' }));
    expect(result.current.sessions.length).toBe(1);
    act(() => result.current.insertSession({ id: 's-new', title: 'hey', source: 'web' }));
    expect(result.current.sessions.length).toBe(1); // idempotent
  });

  it('renameSession updates title locally', async () => {
    mockFetchOnce({ sessions: [{ id: 's1', title: 'old', source: 'web' }], total: 1 });
    const { result } = renderHook(() => useSessionList());
    await waitFor(() => expect(result.current.sessions.length).toBe(1));
    act(() => result.current.renameSession('s1', 'new name'));
    expect(result.current.sessions[0].title).toBe('new name');
  });

  it('polls /api/sessions every 10s and merges results', async () => {
    mockFetchOnce({ sessions: [], total: 0 });  // initial mount
    const { result } = renderHook(() => useSessionList());
    await waitFor(() => expect(result.current.sessions.length).toBe(0));

    mockFetchOnce({
      sessions: [{ id: 'poll-1', title: 'from gateway', source: 'telegram' }],
      total: 1,
    });
    await act(async () => { vi.advanceTimersByTime(10_000); });
    await waitFor(() => expect(result.current.sessions.length).toBe(1));
    expect(result.current.sessions[0].source).toBe('telegram');
  });

  it('refetches on window.focus', async () => {
    mockFetchOnce({ sessions: [], total: 0 });
    const { result } = renderHook(() => useSessionList());
    await waitFor(() => expect(result.current.sessions.length).toBe(0));

    mockFetchOnce({
      sessions: [{ id: 'focus-1', title: 'late arrival', source: 'feishu' }],
      total: 1,
    });
    await act(async () => { window.dispatchEvent(new Event('focus')); });
    await waitFor(() => expect(result.current.sessions.length).toBe(1));
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd web && bun run test --run useSessionList`
Expected: FAIL — `newSession` still inserts, no `insertSession`/`renameSession` exports, no polling.

- [ ] **Step 3: Write minimal implementation**

Replace `web/src/hooks/useSessionList.ts`:

```typescript
import { useCallback, useEffect, useRef, useState } from 'react';
import { apiFetch } from '../api/client';
import { SessionsListResponseSchema, type SessionSummary } from '../api/schemas';

const POLL_INTERVAL_MS = 10_000;

export function useSessionList() {
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const refetch = useCallback(() => {
    abortRef.current?.abort();
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    apiFetch('/api/sessions?limit=50', {
      schema: SessionsListResponseSchema,
      signal: ctrl.signal,
    })
      .then((r) => {
        // Merge: server-authoritative for known ids; preserve local-only rows
        // (e.g. just-inserted via SSE before the server indexes them).
        setSessions((prev) => {
          const byId = new Map<string, SessionSummary>();
          for (const s of r.sessions) byId.set(s.id, s);
          for (const s of prev) if (!byId.has(s.id)) byId.set(s.id, s);
          return [...byId.values()].sort(
            (a, b) => (b.started_at ?? 0) - (a.started_at ?? 0),
          );
        });
      })
      .catch((e) => {
        if (ctrl.signal.aborted) return;
        setError(e instanceof Error ? e.message : 'load failed');
      });
  }, []);

  // Initial load + polling + focus-refetch
  useEffect(() => {
    refetch();
    const timer = setInterval(refetch, POLL_INTERVAL_MS);
    const onFocus = () => refetch();
    window.addEventListener('focus', onFocus);
    return () => {
      clearInterval(timer);
      window.removeEventListener('focus', onFocus);
      abortRef.current?.abort();
    };
  }, [refetch]);

  const newSession = useCallback(() => crypto.randomUUID(), []);

  const insertSession = useCallback((session: SessionSummary) => {
    setSessions((prev) =>
      prev.some((s) => s.id === session.id) ? prev : [session, ...prev],
    );
  }, []);

  const renameSession = useCallback((id: string, title: string) => {
    setSessions((prev) =>
      prev.map((s) => (s.id === id ? { ...s, title } : s)),
    );
  }, []);

  return { sessions, error, newSession, insertSession, renameSession, refetch };
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd web && bun run test --run useSessionList`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/hooks/useSessionList.ts web/src/hooks/useSessionList.test.ts
git commit -m "feat(web): useSessionList polls + focus-refetches; newSession stops inserting placeholder

- Poll /api/sessions every 10s and on window.focus for IM-originated sessions.
- newSession() now returns a fresh UUID without mutating the sidebar list.
- New insertSession/renameSession expose local mutations for SSE + PATCH flows.
"
```

---

## Task 10: `useChatStream` handles `session_created` SSE event

**Files:**
- Modify: `web/src/hooks/useChatStream.ts`
- Test: `web/src/hooks/useChatStream.test.ts`

- [ ] **Step 1: Explore and write the failing test**

Find how existing event types are handled: `grep -n "type.*token\|type.*status\|type.*tool_call" web/src/hooks/useChatStream.ts`. The test follows the existing dispatch-capturing pattern.

```typescript
// web/src/hooks/useChatStream.test.ts — append (or integrate with existing file)
it('dispatches chat/session/created on session_created SSE event', async () => {
  const dispatch = vi.fn();
  // Open an EventSource-style stream for session 's-new'. Your existing test
  // setup likely mocks EventSource — reuse that harness. Pseudocode:
  const es = mockEventSourceFor('s-new');
  renderHook(() => useChatStream('s-new', dispatch));

  act(() => {
    es.emit('session_created', JSON.stringify({
      id: 's-new',
      title: 'Build me ',
      source: 'web',
      model: 'm',
      started_at: 1713724000,
      ended_at: 0,
      message_count: 1,
    }));
  });

  expect(dispatch).toHaveBeenCalledWith({
    type: 'chat/session/created',
    session: expect.objectContaining({ id: 's-new', title: 'Build me ' }),
  });
});
```

Match the mock EventSource setup used by the existing useChatStream tests — do NOT invent new infrastructure.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && bun run test --run useChatStream`
Expected: FAIL — no dispatch on `session_created`.

- [ ] **Step 3: Write minimal implementation**

In `useChatStream.ts` add a handler alongside `token`, `status`, `tool_call` cases:

```typescript
case 'session_created': {
  const payload = SessionSummarySchema.parse(data);
  dispatch({ type: 'chat/session/created', session: payload });
  break;
}
```

Import `SessionSummarySchema` at the top of the file.

Also — since the sidebar now relies on `useSessionList.insertSession` (not the reducer's session list), either:

(a) Wire `useChatStream` to also call `insertSession` directly (prop-drill it in), OR
(b) Have `ChatWorkspace` watch the reducer's `state.sessions` and mirror to `useSessionList.insertSession` when a new id appears.

Pick (a): modify `useChatStream(sessionId, dispatch, onSessionCreated?)` to accept an optional `onSessionCreated(session: SessionSummary) => void` callback. In the handler:

```typescript
case 'session_created': {
  const payload = SessionSummarySchema.parse(data);
  dispatch({ type: 'chat/session/created', session: payload });
  onSessionCreated?.(payload);
  break;
}
```

Then update `ChatWorkspace.tsx`:

```typescript
const { sessions, newSession, insertSession, renameSession } = useSessionList();
useChatStream(sessionId, dispatch, insertSession);
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd web && bun run test --run useChatStream`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/hooks/useChatStream.ts web/src/hooks/useChatStream.test.ts web/src/components/chat/ChatWorkspace.tsx
git commit -m "feat(web): handle session_created SSE event

useChatStream now dispatches chat/session/created and calls an
onSessionCreated callback so the sidebar list in useSessionList inserts the
fresh row without a full refetch.
"
```

---

## Task 11: `ChatWorkspace` — "+ New conversation" stops inserting; routes to UUID

**Files:**
- Modify: `web/src/components/chat/ChatWorkspace.tsx`
- Test: `web/src/components/chat/ChatWorkspace.test.tsx` (extend)

- [ ] **Step 1: Write the failing test**

Locate the existing ChatWorkspace integration test. Append:

```typescript
it('"+ New conversation" click routes to a fresh sessionId but does NOT insert into the sidebar', async () => {
  // mock /api/sessions to return [] so the sidebar starts empty
  mockFetchOnce({ sessions: [], total: 0 });

  const onChangeSession = vi.fn();
  render(<ChatWorkspace sessionId={null} onChangeSession={onChangeSession} providerConfigured={true} />);

  await waitFor(() => {
    expect(screen.queryByText('New conversation')).not.toBeInTheDocument();
  });

  const btn = screen.getByRole('button', { name: /\+ New conversation/i });
  await userEvent.click(btn);

  // onChangeSession called with a fresh UUID; sidebar still shows no rows.
  expect(onChangeSession).toHaveBeenCalledWith(expect.stringMatching(/^[0-9a-f-]{36}$/));
  expect(screen.queryByText('New conversation')).not.toBeInTheDocument();
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && bun run test --run ChatWorkspace`
Expected: FAIL — likely the old optimistic placeholder still appears.

- [ ] **Step 3: Write minimal implementation**

Nothing in `ChatWorkspace.tsx` should need to change structurally — the behavior flip comes from `useSessionList.newSession()` no longer inserting (Task 9). Verify the button's onClick:

```typescript
onNew={() => {
  const id = newSession();
  onChangeSession(id);
}}
```

If the current code dispatches `chat/session/created` here (optimistic), remove that dispatch. Session rows appear later via the SSE handler in Task 10.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd web && bun run test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/chat/ChatWorkspace.tsx web/src/components/chat/ChatWorkspace.test.tsx
git commit -m "feat(web): + New conversation no longer inserts placeholder row

Clicking + New conversation routes to a fresh UUID (enabling SSE subscription)
but defers sidebar insertion until the backend emits session_created.
"
```

---

## Task 12: `SessionItem` double-click rename (editing state + PATCH call)

**Files:**
- Modify: `web/src/components/chat/SessionItem.tsx`
- Modify: `web/src/components/chat/SessionItem.module.css` (editing input styles)
- Modify: `web/src/components/chat/SessionList.tsx` (pass `onRename`)
- Modify: `web/src/components/chat/ChatSidebar.tsx` (plumb `onRename` from ChatWorkspace)
- Test: `web/src/components/chat/SessionItem.test.tsx`

- [ ] **Step 1: Write the failing tests**

```typescript
// web/src/components/chat/SessionItem.test.tsx — new file or append
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import SessionItem from './SessionItem';

const baseSession = { id: 's1', title: 'alpha', source: 'web' };

describe('SessionItem rename', () => {
  it('double-click enters editing state with preselected input', async () => {
    render(<SessionItem session={baseSession} active={false} onSelect={() => {}} onRename={() => {}} />);
    await userEvent.dblClick(screen.getByText('alpha'));
    const input = screen.getByDisplayValue('alpha');
    expect(input).toHaveFocus();
    expect((input as HTMLInputElement).selectionStart).toBe(0);
    expect((input as HTMLInputElement).selectionEnd).toBe('alpha'.length);
  });

  it('Enter commits: onRename called, exits editing', async () => {
    const onRename = vi.fn().mockResolvedValue(undefined);
    render(<SessionItem session={baseSession} active={false} onSelect={() => {}} onRename={onRename} />);
    await userEvent.dblClick(screen.getByText('alpha'));
    const input = screen.getByDisplayValue('alpha');
    await userEvent.clear(input);
    await userEvent.type(input, 'beta');
    await userEvent.keyboard('{Enter}');
    expect(onRename).toHaveBeenCalledWith('s1', 'beta');
  });

  it('Esc cancels: onRename NOT called, exits editing', async () => {
    const onRename = vi.fn();
    render(<SessionItem session={baseSession} active={false} onSelect={() => {}} onRename={onRename} />);
    await userEvent.dblClick(screen.getByText('alpha'));
    const input = screen.getByDisplayValue('alpha');
    await userEvent.clear(input);
    await userEvent.type(input, 'changed');
    await userEvent.keyboard('{Escape}');
    expect(onRename).not.toHaveBeenCalled();
    // After Esc the original title renders back in the readonly view.
    expect(screen.getByText('alpha')).toBeInTheDocument();
  });

  it('blur commits with trimmed value', async () => {
    const onRename = vi.fn().mockResolvedValue(undefined);
    render(<SessionItem session={baseSession} active={false} onSelect={() => {}} onRename={onRename} />);
    await userEvent.dblClick(screen.getByText('alpha'));
    const input = screen.getByDisplayValue('alpha');
    await userEvent.clear(input);
    await userEvent.type(input, '  gamma  ');
    await userEvent.tab(); // blur
    expect(onRename).toHaveBeenCalledWith('s1', 'gamma');
  });

  it('empty title blocks save (no API call, stays in editing state)', async () => {
    const onRename = vi.fn();
    render(<SessionItem session={baseSession} active={false} onSelect={() => {}} onRename={onRename} />);
    await userEvent.dblClick(screen.getByText('alpha'));
    const input = screen.getByDisplayValue('alpha');
    await userEvent.clear(input);
    await userEvent.keyboard('{Enter}');
    expect(onRename).not.toHaveBeenCalled();
    expect(input).toHaveFocus();
  });

  it('renders source badge', () => {
    render(<SessionItem session={{ ...baseSession, source: 'telegram' }} active={false} onSelect={() => {}} onRename={() => {}} />);
    expect(screen.getByText(/telegram/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd web && bun run test --run SessionItem`
Expected: FAIL — missing editing behavior, missing badge.

- [ ] **Step 3: Write minimal implementation**

Rewrite `SessionItem.tsx`:

```typescript
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { SessionSummary } from '../../api/schemas';
import styles from './SessionItem.module.css';

const TITLE_MAX_RUNES = 200;

function runeLen(s: string): number {
  // Array.from splits on code points (close enough for our 200-rune cap; we
  // intentionally don't pull in a grapheme lib — same rule the backend uses).
  return Array.from(s).length;
}

type Props = {
  session: SessionSummary;
  active: boolean;
  onSelect: (id: string) => void;
  onRename: (id: string, title: string) => Promise<void> | void;
};

export default function SessionItem({ session, active, onSelect, onRename }: Props) {
  const { t } = useTranslation('ui');
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(session.title ?? '');
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (editing && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [editing]);

  function commit() {
    const trimmed = draft.trim();
    if (trimmed === '' || runeLen(trimmed) > TITLE_MAX_RUNES) {
      // keep editing state; user must fix
      return;
    }
    if (trimmed === (session.title ?? '')) {
      setEditing(false);
      return;
    }
    Promise.resolve(onRename(session.id, trimmed))
      .then(() => setEditing(false))
      .catch(() => {
        // Parent is expected to toast; we just reset to the stored title.
        setDraft(session.title ?? '');
        setEditing(false);
      });
  }

  function cancel() {
    setDraft(session.title ?? '');
    setEditing(false);
  }

  const displayTitle = session.title && session.title.length > 0
    ? session.title
    : t('chat.untitled');

  return (
    <button
      type="button"
      className={`${styles.item} ${active ? styles.active : ''}`}
      onClick={() => !editing && onSelect(session.id)}
    >
      {editing ? (
        <input
          ref={inputRef}
          type="text"
          className={styles.editInput}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') { e.preventDefault(); commit(); }
            else if (e.key === 'Escape') { e.preventDefault(); cancel(); }
          }}
          onBlur={commit}
          onClick={(e) => e.stopPropagation()}
        />
      ) : (
        <span
          className={styles.title}
          title={t('chat.sidebar.doubleClickToRename')}
          onDoubleClick={(e) => { e.stopPropagation(); setDraft(session.title ?? ''); setEditing(true); }}
        >
          {displayTitle}
        </span>
      )}
      <span className={styles.sourceBadge}>{(session.source ?? '').toUpperCase()}</span>
    </button>
  );
}
```

Add CSS in `SessionItem.module.css` (following DESIGN.md — mono source badge):

```css
.item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: var(--space-2) var(--space-3);
  background: transparent;
  border: none;
  border-left: 2px solid transparent;
  color: var(--text);
  font-family: var(--font-sans);
  font-size: var(--fs-base);
  text-align: left;
  cursor: pointer;
  transition: background var(--t-fast) ease-out;
}
.item:hover { background: var(--hover-tint); }
.item.active { border-left-color: var(--accent); background: var(--hover-tint); }

.title { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.editInput {
  flex: 1;
  padding: 2px var(--space-2);
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: var(--r-sm);
  color: var(--text);
  font-family: var(--font-sans);
  font-size: var(--fs-base);
  outline: none;
}
.editInput:focus { border-color: var(--accent); }

.sourceBadge {
  padding: 1px var(--space-1);
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  letter-spacing: 0.06em;
  color: var(--muted);
  border: 1px solid var(--border);
  border-radius: var(--r-sm);
  margin-left: var(--space-2);
}
```

Propagate `onRename` through `SessionList.tsx` and `ChatSidebar.tsx`. In `ChatWorkspace.tsx`:

```typescript
async function handleRename(id: string, title: string) {
  try {
    await apiFetch(`/api/sessions/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      body: { title },
    });
    renameSession(id, title);
  } catch (err) {
    setToast(t('chat.renameFailed', { msg: err instanceof Error ? err.message : '' }));
    throw err; // let SessionItem reset its local draft
  }
}
// ...
<ChatSidebar sessions={sessions} ... onRename={handleRename} />
```

Add locale keys in `web/src/locales/en/ui.json` and `web/src/locales/zh-CN/ui.json`:

```json
"chat.untitled": "Untitled",
"chat.sidebar.doubleClickToRename": "Double-click to rename",
"chat.renameFailed": "Rename failed: {{msg}}"
```

zh-CN:

```json
"chat.untitled": "未命名",
"chat.sidebar.doubleClickToRename": "双击重命名",
"chat.renameFailed": "重命名失败：{{msg}}"
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd web && bun run test`
Expected: PASS — all 388 existing tests still green, 6 new `SessionItem` tests green.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/chat/SessionItem.tsx web/src/components/chat/SessionItem.module.css web/src/components/chat/SessionItem.test.tsx web/src/components/chat/SessionList.tsx web/src/components/chat/ChatSidebar.tsx web/src/components/chat/ChatWorkspace.tsx web/src/locales/en/ui.json web/src/locales/zh-CN/ui.json
git commit -m "feat(web): double-click to rename conversation + source badge

- SessionItem double-click → input → Enter/blur commits via PATCH.
- Esc cancels without hitting the API. Empty or >200 runes blocks save.
- Source badge renders in mono uppercase next to each title.
- New locale keys: chat.untitled, chat.sidebar.doubleClickToRename, chat.renameFailed.
"
```

---

## Task 13: End-to-end smoke — build binary and verify

**Files:** none (build + manual verify only)

- [ ] **Step 1: Full test suite**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
go test ./...
cd web && bun run test
```

Expected: all Go packages PASS (no regressions), all 394+ web tests PASS.

- [ ] **Step 2: Rebuild web assets and binary**

```bash
cd web && bun run build
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
rm -rf api/webroot/assets
cp -r web/dist/index.html api/webroot/
cp -r web/dist/assets api/webroot/
go build -o bin/hermind ./cmd/hermind
```

- [ ] **Step 3: Manual verify**

```bash
# Fresh DB in a temp location to see the migration kick in
HERMIND_HOME=/tmp/hermind-smoke bin/hermind web &
SMOKE_PID=$!
sleep 2
sqlite3 /tmp/hermind-smoke/hermind.db "SELECT value FROM schema_meta WHERE key='version'"
# Expected: 2
kill $SMOKE_PID
```

Open the web UI, send a first message, confirm:
- Sidebar shows a new row with the first 10 runes as title right after you hit send (SSE delivered).
- Double-click the title → input appears preselected → type a new title → Enter → row updates and the DB reflects it.
- Open a second browser tab; without doing anything, within ~10s the new session appears in that tab's sidebar too (polling).
- If Telegram is configured and enabled, send a message from Telegram — within 10s the gateway-created session appears in the web sidebar.

- [ ] **Step 4: Commit**

No changes expected — this task is a smoke test. If the smoke caught a bug, file a task back up and fix before committing anything new.

---

## Execution notes

- Tasks 1–7 are backend-only (Go). Run `go test ./...` between each.
- Tasks 8–12 are frontend-only (TS/React). Run `bun run test` between each. Don't rebuild the binary until Task 13.
- Keep the commits small per the template. If a step expands beyond 5 minutes of work, split it in the moment — don't batch.
