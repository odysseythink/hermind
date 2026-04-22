# PR 2: purge-sessions — Single-Conversation Collapse Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collapse the `Session` abstraction from every layer of hermind. Delete `gateway/` (multi-platform bot framework) and `acp` (protocol bridge). Flatten the storage schema so `messages` is instance-scoped. Simplify the HTTP API, agent engine, cron runner, and frontend to match the "one instance = one conversation" model. Persist v1 data as a `state.db.v1-backup` file on upgrade.

**Architecture:**
- Storage schema v3: drops `sessions` table; `messages` loses `session_id`. Old `state.db` files (detected by presence of `sessions` table) are renamed to `state.db.v1-backup` and a fresh v3 DB is created.
- `storage.Storage` interface flattened: no session-scoped methods, no `sessionID` params on `AppendMessage`, `GetHistory`, `SearchMessages`, etc.
- `agent.Engine.RunConversation` signature: no `SessionID`/`UserID`; new `Ephemeral bool` flag for cron runs that don't persist to `messages`.
- HTTP API: `/api/sessions*` routes replaced by `/api/conversation*`. Single SSE stream at `/api/sse`.
- Cron: each job spins an ephemeral engine run with its own trajectory file; never touches the main conversation.
- Frontend: delete the sidebar / session list / settings drawer; `ChatWorkspace` becomes a single-column layout; `ConversationHeader` holds `[instance path | model dropdown | stop]`.

**Tech Stack:** Go 1.25, SQLite (mattn/go-sqlite3), chi router; React + Vite + Vitest; pnpm build.

**Spec reference:** `docs/superpowers/specs/2026-04-22-hermind-instance-single-session-design.md` §PR 2.

**Depends on:** PR 1 (`config-dir`) is already merged. This plan assumes `config.InstanceRoot()` is available.

---

## File Structure

### Deleted directories / files

```
gateway/                  # multi-platform bot framework (whole subtree)
gateway/platforms/        # slack/discord/telegram/irc/matrix/... adapters
gateway/acp/              # legacy ACP under gateway (distinct from cli/acp.go)
cli/gateway.go
cli/gateway_build.go
cli/gateway_test.go
cli/gateway_build_test.go
cli/gatewayctl/           # gateway controller
cli/acp.go
cli/acp_test.go
api/handlers_sessions.go
api/handlers_sessions_test.go
api/handlers_messages.go          # replaced by handlers_conversation.go
api/handlers_messages_test.go
api/handlers_session_run.go
api/handlers_session_run_test.go
api/session_registry.go
api/session_registry_test.go
api/session_patch_limits.go
api/sessionrun/                    # whole package
api/sessionrun_bridge.go
api/handlers_platforms.go
api/handlers_platforms_test.go
storage/sqlite/session.go          # Session CRUD impls
web/src/components/chat/ChatSidebar.{tsx,module.css}
web/src/components/chat/SessionList.{tsx,module.css}
web/src/components/chat/SessionItem.{tsx,module.css,test.tsx}
web/src/components/chat/NewChatButton.{tsx,module.css}
web/src/components/chat/SessionSettingsDrawer.{tsx,module.css,test.tsx}
web/src/components/chat/SettingsButton.{tsx,module.css}
web/src/hooks/useSessionList.{ts,test.ts}
web/src/shell/keyedInstances.{ts,test.ts}       # only if gateway-only;  audit first
web/src/shell/listInstances.{ts,test.ts}        # only if gateway-only;  audit first
web/src/shell/summaries.{tsx,test.tsx}
```

### New files

```
storage/sqlite/v1backup.go           # detects v1 sessions table and renames file
storage/sqlite/v1backup_test.go
storage/sqlite/conversation.go       # AppendMessage / GetHistory / conversation_state impl
api/handlers_conversation.go         # GET/POST /api/conversation + POST /api/conversation/cancel
api/handlers_conversation_test.go
api/handlers_sse.go                   # GET /api/sse (single stream)
api/handlers_sse_test.go
cli/cron_ephemeral_test.go           # ensure cron run doesn't touch GetHistory()
```

### Modified files

```
storage/storage.go                  # interface flatten
storage/types.go                    # remove SessionID/UserID fields from search/usage types
storage/sqlite/sqlite.go            # Open() calls v1backup check before migrate
storage/sqlite/migrate.go           # schemaSQL emits v3 tables; applyVersion step 3 is empty
storage/sqlite/message.go           # rewrite for no session_id
storage/sqlite/memory.go            # unchanged semantics; may need minor touch
storage/sqlite/tx.go                # Tx interface flatten
agent/engine.go                     # drop session-created callback; simplify
agent/conversation.go               # rewrite RunConversation (no SessionID; Ephemeral flag)
agent/compression.go                # drop sessionID from Compressor signatures
agent/trajectory.go                 # SessionID-free writer (filename driver instead)
cli/cron.go                         # ephemeral engine runs; per-job trajectory file
cli/bootstrap.go                    # storage path lookup unchanged; remove BuildGateway
cli/engine_deps.go                  # remove gateway tool registration
cli/root.go                         # remove newGatewayCmd, newAcpCmd; chat is default
cli/web.go                          # drop gateway controller wiring; simplify ServerOpts
api/server.go                       # strip session routes; wire conversation + sse
api/dto.go                          # drop Session DTOs; add ConversationHistory/MessagePost DTOs
api/sse.go                          # simplify: broadcast to all subscribers
api/stream.go / api/stream_hook.go  # drop SessionID from StreamEvent
web/src/components/chat/ChatWorkspace.tsx   # single-column; no sidebar; no settings
web/src/components/chat/ChatWorkspace.module.css
web/src/components/chat/ConversationHeader.tsx   # model dropdown + stop
web/src/components/chat/ConversationHeader.module.css
web/src/hooks/useChatStream.ts     # no session, SSE at /api/sse
web/src/state/chat.ts              # single-conversation reducer (no messagesBySession map)
web/src/state.ts                   # remove gateway-specific actions/selectors
web/src/api/schemas.ts             # drop SessionSummary, SessionPatch, etc.; add MetaResponse + history schemas
web/src/api/client.ts              # unchanged from PR 1
web/src/App.tsx                    # single mode (chat); drop shell/settings dual-mode + hash router
web/src/main.tsx                   # unchanged from PR 1
web/src/locales/en/ui.json         # strip session keys
web/src/locales/zh-CN/ui.json      # strip session keys
CHANGELOG.md                       # breaking changes entry
api/webroot/                       # rebuilt bundle at end
```

---

## Phase A — Storage Schema & Migration

## Task A1: Add v1 backup detection helper with TDD

**Files:**
- Create: `storage/sqlite/v1backup.go`
- Create: `storage/sqlite/v1backup_test.go`

- [ ] **Step 1: Write the failing test**

Create `storage/sqlite/v1backup_test.go`:

```go
package sqlite

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func makeV1SchemaFile(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE sessions (id TEXT PRIMARY KEY); CREATE TABLE messages (id INTEGER PRIMARY KEY, session_id TEXT);`)
	require.NoError(t, err)
}

func TestBackupLegacyDBIfNeeded_RenamesWhenSessionsTableExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	makeV1SchemaFile(t, path)

	backedUp, backupPath, err := backupLegacyDBIfNeeded(path)
	require.NoError(t, err)
	assert.True(t, backedUp)
	assert.Equal(t, filepath.Join(dir, "state.db.v1-backup"), backupPath)

	// Original gone, backup exists
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(backupPath)
	assert.NoError(t, err)
}

func TestBackupLegacyDBIfNeeded_NoOpWhenNoSessionsTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	db, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE messages (id INTEGER PRIMARY KEY)`)
	require.NoError(t, err)
	db.Close()

	backedUp, _, err := backupLegacyDBIfNeeded(path)
	require.NoError(t, err)
	assert.False(t, backedUp)

	// Original untouched
	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestBackupLegacyDBIfNeeded_NoOpWhenFileMissing(t *testing.T) {
	dir := t.TempDir()
	backedUp, _, err := backupLegacyDBIfNeeded(filepath.Join(dir, "nope.db"))
	require.NoError(t, err)
	assert.False(t, backedUp)
}

func TestBackupLegacyDBIfNeeded_AppendsSuffixOnCollision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	makeV1SchemaFile(t, path)

	// Pre-existing backup collides
	collide := filepath.Join(dir, "state.db.v1-backup")
	require.NoError(t, os.WriteFile(collide, []byte("prior"), 0o644))

	backedUp, backupPath, err := backupLegacyDBIfNeeded(path)
	require.NoError(t, err)
	assert.True(t, backedUp)
	assert.True(t,
		strings.HasPrefix(filepath.Base(backupPath), "state.db.v1-backup."),
		"got %s, want prefix state.db.v1-backup.", backupPath)

	// Both files present
	for _, p := range []string{collide, backupPath} {
		_, err := os.Stat(p)
		assert.NoError(t, err, "expected %s to exist", p)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./storage/sqlite/... -run TestBackupLegacyDBIfNeeded -v`
Expected: FAIL — `undefined: backupLegacyDBIfNeeded`.

- [ ] **Step 3: Write the implementation**

Create `storage/sqlite/v1backup.go`:

```go
package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// backupLegacyDBIfNeeded renames an existing state.db to state.db.v1-backup
// if it has a v1 `sessions` table. Returns (renamed, backupPath, error).
// If the source file doesn't exist or doesn't have a sessions table, it
// is a no-op.
//
// On collision with an existing .v1-backup, a unix-ms suffix is appended
// so no data is ever overwritten.
func backupLegacyDBIfNeeded(path string) (bool, string, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, "", nil
		}
		return false, "", err
	}

	hasSessions, err := probeSessionsTable(path)
	if err != nil {
		return false, "", err
	}
	if !hasSessions {
		return false, "", nil
	}

	backup := path + ".v1-backup"
	if _, err := os.Stat(backup); err == nil {
		backup = fmt.Sprintf("%s.%d", backup, time.Now().UnixMilli())
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, "", err
	}

	if err := os.Rename(path, backup); err != nil {
		return false, "", fmt.Errorf("sqlite: backup legacy db: %w", err)
	}
	return true, backup, nil
}

// probeSessionsTable opens the DB read-only and checks for a `sessions`
// table. Returns false on any error other than "file doesn't exist".
func probeSessionsTable(path string) (bool, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro", path)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return false, fmt.Errorf("sqlite: probe open: %w", err)
	}
	defer db.Close()

	row := db.QueryRow(`SELECT 1 FROM sqlite_master WHERE type='table' AND name='sessions' LIMIT 1`)
	var one int
	if err := row.Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("sqlite: probe scan: %w", err)
	}
	return one == 1, nil
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./storage/sqlite/... -run TestBackupLegacyDBIfNeeded -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add storage/sqlite/v1backup.go storage/sqlite/v1backup_test.go
git commit -m "feat(storage): backupLegacyDBIfNeeded renames v1 state.db on detection"
```

---

## Task A2: `sqlite.Open` calls backup + prints stderr notice

**Files:**
- Modify: `storage/sqlite/sqlite.go`

- [ ] **Step 1: Inspect and edit `Open`**

Open `storage/sqlite/sqlite.go`. Find the `Open(path string)` function. Prepend the backup check:

```go
func Open(path string) (*Store, error) {
	backedUp, backupPath, err := backupLegacyDBIfNeeded(path)
	if err != nil {
		return nil, fmt.Errorf("sqlite: %w", err)
	}
	if backedUp {
		fmt.Fprintln(os.Stderr,
			"hermind: legacy state.db (v1 multi-session) backed up to "+backupPath+".")
		fmt.Fprintln(os.Stderr,
			"  The new schema is single-conversation; your message history has been")
		fmt.Fprintln(os.Stderr,
			"  preserved in the backup but is not migrated.")
	}

	// ...existing Open body (sql.Open, pragmas, etc.)...
}
```

Add `"os"` and `"fmt"` imports if not already present.

- [ ] **Step 2: Add an integration test**

Append to `storage/sqlite/sqlite_test.go`:

```go
func TestOpen_BacksUpLegacyV1DBAndStartsFresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	makeV1SchemaFile(t, path)

	store, err := Open(path)
	require.NoError(t, err)
	defer store.Close()

	// Backup exists
	_, err = os.Stat(filepath.Join(dir, "state.db.v1-backup"))
	require.NoError(t, err)

	// Fresh DB has no sessions table (schema v3)
	rows, err := store.DB().Query(`SELECT name FROM sqlite_master WHERE type='table' AND name='sessions'`)
	require.NoError(t, err)
	defer rows.Close()
	assert.False(t, rows.Next(), "expected no sessions table in fresh v3 schema")
}
```

(If `store.DB()` isn't exposed, add a package-private accessor or run the query via `Tx` — pick the least invasive path.)

- [ ] **Step 3: Run tests**

Run: `go test ./storage/sqlite/... -v`
Expected: PASS (including new test).

- [ ] **Step 4: Commit**

```bash
git add storage/sqlite/sqlite.go storage/sqlite/sqlite_test.go
git commit -m "feat(storage): sqlite.Open backs up v1 state.db and starts fresh"
```

---

## Task A3: Rewrite `schemaSQL` for v3 (flat messages, conversation_state, no sessions)

**Files:**
- Modify: `storage/sqlite/migrate.go`

- [ ] **Step 1: Replace `schemaSQL` and bump `currentSchemaVersion`**

Replace the entire `schemaSQL` constant body with:

```go
const schemaSQL = `
CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    role TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    tool_call_id TEXT DEFAULT '',
    tool_calls TEXT DEFAULT '',
    tool_name TEXT DEFAULT '',
    timestamp REAL NOT NULL,
    token_count INTEGER NOT NULL DEFAULT 0,
    finish_reason TEXT DEFAULT '',
    reasoning TEXT DEFAULT '',
    reasoning_details TEXT DEFAULT ''
);

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

CREATE TABLE IF NOT EXISTS conversation_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    system_prompt_cache TEXT DEFAULT '',
    total_input_tokens INTEGER DEFAULT 0,
    total_output_tokens INTEGER DEFAULT 0,
    total_cache_read_tokens INTEGER DEFAULT 0,
    total_cache_write_tokens INTEGER DEFAULT 0,
    total_cost_usd REAL DEFAULT 0,
    updated_at REAL NOT NULL DEFAULT 0
);
INSERT OR IGNORE INTO conversation_state
    (id, updated_at) VALUES (1, 0);

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
INSERT OR IGNORE INTO schema_meta (key, value) VALUES ('version', '3');
`

const currentSchemaVersion = 3
```

- [ ] **Step 2: Rewrite `applyVersion`**

Replace the body of `applyVersion` with a no-op for v3 (since a fresh DB is created when v1 is detected; any pre-existing v2 DB in the wild is, per the spec, not supported for automatic migration — fail loudly):

```go
func (s *Store) applyVersion(v int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	switch v {
	case 3:
		// No-op: v3 is the first schema the current binary speaks.
		// Legacy v1/v2 DBs are detected by backupLegacyDBIfNeeded()
		// at Open() time and renamed out of the way.
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

- [ ] **Step 3: Update `migrate_test.go` references**

Open `storage/sqlite/migrate_test.go`. Any test that creates a v1-shaped DB and expects `applyVersion(2)` to truncate it needs to be rewritten — the new path is: create v1 file, call `Open`, expect backup + fresh v3. Delete obsolete tests and keep only the tests that exercise v3 schema creation on a fresh DB.

Minimum coverage after edits:

```go
func TestMigrate_FreshDBCreatesV3Schema(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "state.db"))
	require.NoError(t, err)
	defer store.Close()

	var ver string
	require.NoError(t, store.DB().QueryRow(
		`SELECT value FROM schema_meta WHERE key='version'`,
	).Scan(&ver))
	assert.Equal(t, "3", ver)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./storage/sqlite/... -v`
Expected: PASS. Build may still fail elsewhere (later tasks fix session.go, message.go, etc.).

- [ ] **Step 5: Commit**

```bash
git add storage/sqlite/migrate.go storage/sqlite/migrate_test.go
git commit -m "feat(storage): schema v3 — flat messages, conversation_state singleton"
```

---

## Task A4: Flatten `storage.Storage` interface and `storage/types.go`

**Files:**
- Modify: `storage/storage.go`
- Modify: `storage/types.go`

- [ ] **Step 1: Rewrite `storage/storage.go`**

```go
// storage/storage.go
package storage

import (
	"context"
	"errors"
)

var (
	ErrNotFound = errors.New("storage: not found")
)

// Storage is the root storage interface. Implementations must be safe
// for concurrent use.
type Storage interface {
	// Conversation: single, instance-scoped message log.
	AppendMessage(ctx context.Context, msg *StoredMessage) error
	GetHistory(ctx context.Context, limit, offset int) ([]*StoredMessage, error)
	SearchMessages(ctx context.Context, query string, opts *SearchOptions) ([]*SearchResult, error)

	// Conversation state (singleton row id=1).
	UpdateSystemPromptCache(ctx context.Context, prompt string) error
	UpdateUsage(ctx context.Context, usage *UsageUpdate) error

	// Memory — unchanged semantics.
	SaveMemory(ctx context.Context, memory *Memory) error
	GetMemory(ctx context.Context, id string) (*Memory, error)
	SearchMemories(ctx context.Context, query string, opts *MemorySearchOptions) ([]*Memory, error)
	DeleteMemory(ctx context.Context, id string) error

	// Transactions.
	WithTx(ctx context.Context, fn func(tx Tx) error) error

	// Lifecycle.
	Close() error
	Migrate() error
}

// Tx is the transaction-scoped interface.
type Tx interface {
	AppendMessage(ctx context.Context, msg *StoredMessage) error
	UpdateSystemPromptCache(ctx context.Context, prompt string) error
	UpdateUsage(ctx context.Context, usage *UsageUpdate) error
}
```

- [ ] **Step 2: Rewrite `storage/types.go`**

```go
// storage/types.go
package storage

import (
	"encoding/json"
	"time"
)

// StoredMessage is the persistence shape of a single conversation message.
// No session_id — messages belong to the instance.
type StoredMessage struct {
	ID               int64
	Role             string
	Content          string // JSON-encoded message.Content
	ToolCallID       string
	ToolCalls        json.RawMessage
	ToolName         string
	Timestamp        time.Time
	TokenCount       int
	FinishReason     string
	Reasoning        string
	ReasoningDetails string
}

// UsageUpdate holds a usage delta to add to the conversation_state row.
type UsageUpdate struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	ReasoningTokens  int
	CostUSD          float64
}

// SearchOptions controls FTS message search.
type SearchOptions struct {
	Limit int
}

// SearchResult is a single hit from SearchMessages.
type SearchResult struct {
	Message *StoredMessage
	Snippet string
	Rank    float64
}

// Memory is a persisted agent memory entry.
type Memory struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id,omitempty"`
	Content   string          `json:"content"`
	Category  string          `json:"category,omitempty"`
	Tags      []string        `json:"tags,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// MemorySearchOptions controls MemorySearch behavior.
type MemorySearchOptions struct {
	UserID string
	Tags   []string
	Limit  int
}
```

- [ ] **Step 3: Build expect-to-fail**

Run: `go build ./storage/...`
Expected: FAIL in `storage/sqlite/*.go` (session.go, message.go, tx.go, memory.go still reference the old types). That's by design — the next tasks fix these.

- [ ] **Step 4: Commit**

```bash
git add storage/storage.go storage/types.go
git commit -m "refactor(storage): flatten interface — instance-scoped messages only"
```

---

## Task A5: Rewrite `storage/sqlite` internals for flat schema

**Files:**
- Delete: `storage/sqlite/session.go`
- Create: `storage/sqlite/conversation.go`
- Modify: `storage/sqlite/message.go`
- Modify: `storage/sqlite/tx.go`
- Modify: `storage/sqlite/memory.go` (audit only)
- Modify: `storage/sqlite/interface_test.go`

- [ ] **Step 1: Delete `storage/sqlite/session.go`**

```bash
git rm storage/sqlite/session.go
```

- [ ] **Step 2: Rewrite `storage/sqlite/message.go`**

Replace its contents with a flat implementation. Core functions:

```go
// storage/sqlite/message.go
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/odysseythink/hermind/storage"
)

func (s *Store) AppendMessage(ctx context.Context, msg *storage.StoredMessage) error {
	return insertMessageExec(ctx, s.db, msg)
}

func (t *txImpl) AppendMessage(ctx context.Context, msg *storage.StoredMessage) error {
	return insertMessageExec(ctx, t.tx, msg)
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertMessageExec(ctx context.Context, ex execer, msg *storage.StoredMessage) error {
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now().UTC()
	}
	_, err := ex.ExecContext(ctx, `
		INSERT INTO messages
		  (role, content, tool_call_id, tool_calls, tool_name,
		   timestamp, token_count, finish_reason, reasoning, reasoning_details)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		msg.Role, msg.Content, msg.ToolCallID,
		string(msg.ToolCalls), msg.ToolName,
		toEpoch(msg.Timestamp),
		msg.TokenCount, msg.FinishReason, msg.Reasoning, msg.ReasoningDetails,
	)
	if err != nil {
		return fmt.Errorf("sqlite: append message: %w", err)
	}
	return nil
}

func (s *Store) GetHistory(ctx context.Context, limit, offset int) ([]*storage.StoredMessage, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, role, content, tool_call_id, tool_calls, tool_name,
		       timestamp, token_count, finish_reason, reasoning, reasoning_details
		FROM messages
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("sqlite: get history: %w", err)
	}
	defer rows.Close()

	var out []*storage.StoredMessage
	for rows.Next() {
		m := &storage.StoredMessage{}
		var ts float64
		var toolCalls string
		if err := rows.Scan(
			&m.ID, &m.Role, &m.Content, &m.ToolCallID,
			&toolCalls, &m.ToolName, &ts, &m.TokenCount,
			&m.FinishReason, &m.Reasoning, &m.ReasoningDetails,
		); err != nil {
			return nil, fmt.Errorf("sqlite: scan message: %w", err)
		}
		m.Timestamp = fromEpoch(ts)
		m.ToolCalls = []byte(toolCalls)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) SearchMessages(
	ctx context.Context,
	query string,
	opts *storage.SearchOptions,
) ([]*storage.SearchResult, error) {
	limit := 20
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.role, m.content, m.timestamp,
		       snippet(messages_fts, 0, '<b>', '</b>', '...', 16) AS snip,
		       bm25(messages_fts) AS rank
		FROM messages m
		JOIN messages_fts ON m.id = messages_fts.rowid
		WHERE messages_fts MATCH ?
		ORDER BY rank ASC
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite: search messages: %w", err)
	}
	defer rows.Close()

	var out []*storage.SearchResult
	for rows.Next() {
		sm := &storage.StoredMessage{}
		var ts float64
		var rank float64
		var snippet string
		if err := rows.Scan(&sm.ID, &sm.Role, &sm.Content, &ts, &snippet, &rank); err != nil {
			return nil, fmt.Errorf("sqlite: scan search: %w", err)
		}
		sm.Timestamp = fromEpoch(ts)
		out = append(out, &storage.SearchResult{Message: sm, Snippet: snippet, Rank: rank})
	}
	return out, rows.Err()
}
```

Keep `toEpoch` / `fromEpoch` helpers somewhere (they may already exist in `sqlite.go` — if not, add them).

- [ ] **Step 3: Create `storage/sqlite/conversation.go`**

```go
// storage/sqlite/conversation.go
package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/odysseythink/hermind/storage"
)

func (s *Store) UpdateSystemPromptCache(ctx context.Context, prompt string) error {
	return updateSystemPromptCacheExec(ctx, s.db, prompt)
}

func (t *txImpl) UpdateSystemPromptCache(ctx context.Context, prompt string) error {
	return updateSystemPromptCacheExec(ctx, t.tx, prompt)
}

func updateSystemPromptCacheExec(ctx context.Context, ex execer, prompt string) error {
	_, err := ex.ExecContext(ctx, `
		UPDATE conversation_state
		SET system_prompt_cache = ?, updated_at = ?
		WHERE id = 1
	`, prompt, toEpoch(time.Now().UTC()))
	if err != nil {
		return fmt.Errorf("sqlite: update system_prompt_cache: %w", err)
	}
	return nil
}

func (s *Store) UpdateUsage(ctx context.Context, u *storage.UsageUpdate) error {
	return updateUsageExec(ctx, s.db, u)
}

func (t *txImpl) UpdateUsage(ctx context.Context, u *storage.UsageUpdate) error {
	return updateUsageExec(ctx, t.tx, u)
}

func updateUsageExec(ctx context.Context, ex execer, u *storage.UsageUpdate) error {
	if u == nil {
		return nil
	}
	_, err := ex.ExecContext(ctx, `
		UPDATE conversation_state SET
		  total_input_tokens = total_input_tokens + ?,
		  total_output_tokens = total_output_tokens + ?,
		  total_cache_read_tokens = total_cache_read_tokens + ?,
		  total_cache_write_tokens = total_cache_write_tokens + ?,
		  total_cost_usd = total_cost_usd + ?,
		  updated_at = ?
		WHERE id = 1
	`,
		u.InputTokens, u.OutputTokens,
		u.CacheReadTokens, u.CacheWriteTokens,
		u.CostUSD, toEpoch(time.Now().UTC()),
	)
	if err != nil {
		return fmt.Errorf("sqlite: update usage: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Rewrite `storage/sqlite/tx.go`**

Simplify the transaction wrapper. It should expose `AppendMessage`, `UpdateSystemPromptCache`, `UpdateUsage` — no session methods.

```go
// storage/sqlite/tx.go
package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/odysseythink/hermind/storage"
)

type txImpl struct {
	tx *sql.Tx
}

func (s *Store) WithTx(ctx context.Context, fn func(tx storage.Tx) error) error {
	t, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin tx: %w", err)
	}
	if err := fn(&txImpl{tx: t}); err != nil {
		_ = t.Rollback()
		return err
	}
	return t.Commit()
}
```

- [ ] **Step 5: Audit `storage/sqlite/memory.go`**

The Memory interface is unchanged, but it may have session-coupled helpers. Grep:
```
grep -n "session\|Session\|SessionID" storage/sqlite/memory.go
```
If any hits: remove them. Memory was always instance-scoped per the spec.

- [ ] **Step 6: Update `interface_test.go`**

`storage/sqlite/interface_test.go` likely enforces that `*Store` and `*txImpl` satisfy the `storage.Storage` / `storage.Tx` interfaces via compile-time assertions. Update to the new interface shape:

```go
var (
	_ storage.Storage = (*Store)(nil)
	_ storage.Tx      = (*txImpl)(nil)
)
```

Remove any assertions on methods that no longer exist.

- [ ] **Step 7: Rewrite `memory_test.go` / `message_test.go` / `sqlite_test.go` calls**

Any test calling `AddMessage(ctx, sessionID, ...)` becomes `AppendMessage(ctx, ...)`. Any test calling `CreateSession`, `GetSession`, `ListSessions`, `UpdateSession`, `UpdateSystemPrompt(sessionID,…)`, `UpdateUsage(sessionID,…)` is either deleted (if it's testing session behavior) or rewritten (if it's testing something orthogonal that just happened to use sessions as a prop).

A representative replacement test:

```go
func TestAppendAndGetHistory(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "state.db"))
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	ctx := context.Background()
	require.NoError(t, store.AppendMessage(ctx, &storage.StoredMessage{
		Role:    "user",
		Content: `{"text":"hello"}`,
	}))
	require.NoError(t, store.AppendMessage(ctx, &storage.StoredMessage{
		Role:    "assistant",
		Content: `{"text":"hi"}`,
	}))

	hist, err := store.GetHistory(ctx, 100, 0)
	require.NoError(t, err)
	require.Len(t, hist, 2)
	assert.Equal(t, "user", hist[0].Role)
	assert.Equal(t, "assistant", hist[1].Role)
}
```

- [ ] **Step 8: Build and run the storage suite**

Run: `go test ./storage/...`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "refactor(storage/sqlite): flat messages + conversation_state singleton"
```

---

## Phase B — Delete gateway, acp, and session-coupled API layers

## Task B1: Delete `gateway/` and `cli/gatewayctl/`

**Files:**
- Delete: `gateway/` (whole directory)
- Delete: `cli/gatewayctl/` (whole directory)

- [ ] **Step 1: Delete the directories**

```bash
git rm -r gateway cli/gatewayctl
```

- [ ] **Step 2: Expect build failures**

Run: `go build ./...`
Expected: many errors in `cli/*.go`, `api/server.go`, anywhere referencing `gateway.*` or `gatewayctl.*`. That's fine — Task B2–B5 fix them.

- [ ] **Step 3: Commit**

```bash
git commit -m "chore: delete gateway/ and cli/gatewayctl/ (multi-platform bot framework)"
```

---

## Task B2: Delete `cli/gateway.go`, `cli/acp.go`, and wiring

**Files:**
- Delete: `cli/gateway.go`, `cli/gateway_build.go`, `cli/gateway_test.go`, `cli/gateway_build_test.go`, `cli/acp.go`, `cli/acp_test.go`
- Modify: `cli/root.go`
- Modify: `cli/engine_deps.go`
- Modify: `cli/web.go`

- [ ] **Step 1: Delete files**

```bash
git rm cli/gateway.go cli/gateway_build.go cli/gateway_test.go cli/gateway_build_test.go cli/acp.go cli/acp_test.go
```

- [ ] **Step 2: Update `cli/root.go`**

Remove `newGatewayCmd(app)` and `newAcpCmd(app)` (if present) from the `AddCommand(...)` call. Remaining list example:

```go
root.AddCommand(
    newRunCmd(app),
    newCronCmd(app),
    newSkillsCmd(app),
    newSetupCmd(app),
    newDoctorCmd(app),
    newAuthCmd(app),
    newModelsCmd(app),
    newPluginsCmd(app),
    newUpgradeCmd(app),
    newRLCmd(app),
    newMCPCmd(app),
    newWebCmd(app),
    newVersionCmd(),
)
```

Keep the default `root.RunE` that launches the web UI.

- [ ] **Step 3: Update `cli/engine_deps.go`**

Grep the file for references to `gateway`, `BuildGateway`, `GatewayController`. Remove all of them. The `BuildEngineDeps` function should end up smaller — roughly: primary provider + aux + storage + tool registry + skills.

- [ ] **Step 4: Update `cli/web.go`**

Remove the entire `ctrl := gatewayctl.New(...)` + `ctrl.Start(ctx)` + `defer ctrl.Shutdown` block from `runWeb`. Remove the `Controller` field from the `api.ServerOpts{}` literal. The post-PR-1, post-gateway-removal body is approximately:

```go
func runWeb(ctx context.Context, app *App, opts webRunOptions) error {
	if err := ensureStorage(app); err != nil {
		return err
	}

	deps, cleanup, err := BuildEngineDeps(ctx, app)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil && !errors.Is(err, errMissingAPIKey) {
		return fmt.Errorf("web: build engine deps: %w", err)
	}

	streams := api.NewMemoryStreamHub()
	srv, err := api.NewServer(&api.ServerOpts{
		Config:       app.Config,
		ConfigPath:   app.ConfigPath,
		InstanceRoot: app.InstanceRoot,
		Storage:      app.Storage,
		Version:      Version,
		Streams:      streams,
		Deps:         deps,
	})
	if err != nil {
		return err
	}

	var ln net.Listener
	if opts.Addr == "" {
		ln, err = listenRandomLocalhost()
		if err != nil {
			return fmt.Errorf("web: %w", err)
		}
	} else {
		ln, err = net.Listen("tcp", opts.Addr)
		if err != nil {
			return fmt.Errorf("web: listen %s: %w", opts.Addr, err)
		}
	}
	realAddr := "http://" + ln.Addr().String()
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	fmt.Fprintf(out, "hermind web listening on %s\n", realAddr)
	fmt.Fprintf(out, "instance:  %s\n", app.InstanceRoot)
	fmt.Fprintf(out, "open:      %s/\n", realAddr)

	if !opts.NoBrowser {
		go openBrowser(realAddr + "/")
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if opts.ExitAfter > 0 {
		time.AfterFunc(opts.ExitAfter, cancel)
	}

	httpSrv := &http.Server{
		Handler:           srv.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-runCtx.Done()
		shutCtx, c2 := context.WithTimeout(context.Background(), 2*time.Second)
		defer c2()
		_ = httpSrv.Shutdown(shutCtx)
	}()
	if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
```

- [ ] **Step 5: Build**

Run: `go build ./...`
Expected: still some errors in `api/*.go` (handlers_platforms, handlers_sessions, etc.). Those come off in the next task.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "chore(cli): delete gateway + acp subcommands and wiring"
```

---

## Task B3: Delete session-coupled API handlers + sessionrun package

**Files:**
- Delete: `api/handlers_sessions.go`, `api/handlers_sessions_test.go`
- Delete: `api/handlers_messages.go`, `api/handlers_messages_test.go`
- Delete: `api/handlers_session_run.go`, `api/handlers_session_run_test.go`
- Delete: `api/session_registry.go`, `api/session_registry_test.go`
- Delete: `api/session_patch_limits.go`
- Delete: `api/sessionrun/` (whole dir)
- Delete: `api/sessionrun_bridge.go`
- Delete: `api/handlers_platforms.go`, `api/handlers_platforms_test.go`

- [ ] **Step 1: Git remove**

```bash
git rm api/handlers_sessions.go api/handlers_sessions_test.go \
       api/handlers_messages.go api/handlers_messages_test.go \
       api/handlers_session_run.go api/handlers_session_run_test.go \
       api/session_registry.go api/session_registry_test.go \
       api/session_patch_limits.go \
       api/sessionrun_bridge.go \
       api/handlers_platforms.go api/handlers_platforms_test.go
git rm -r api/sessionrun
```

- [ ] **Step 2: Update `api/server.go`**

Rewrite `ServerOpts` and `buildRouter()`:

```go
type ServerOpts struct {
	Config       *config.Config
	ConfigPath   string
	InstanceRoot string
	Storage      storage.Storage
	Version      string
	Streams      StreamHub
	Deps         EngineDeps // see note below
}
```

Note: since `sessionrun.Deps` is gone, introduce a minimal `EngineDeps` struct in `api/server.go` (or a new `api/deps.go`):

```go
// EngineDeps is the bundle of providers, storage, and tools the
// conversation POST endpoint needs to run one engine turn.
type EngineDeps struct {
	Provider    provider.Provider
	AuxProvider provider.Provider
	Storage     storage.Storage
	ToolReg     *tool.Registry
	AgentCfg    config.AgentConfig
	Platform    string
}
```

(Add the appropriate imports to `api/server.go` or a new `api/deps.go` file.)

Rewrite `buildRouter()`:

```go
func (s *Server) buildRouter() chi.Router {
	r := chi.NewRouter()

	r.Route("/api", func(r chi.Router) {
		r.Get("/status", s.handleStatus)
		r.Get("/model/info", s.handleModelInfo)

		r.Get("/config", s.handleConfigGet)
		r.Put("/config", s.handleConfigPut)
		r.Get("/config/schema", s.handleConfigSchema)

		r.Get("/conversation", s.handleConversationGet)
		r.Post("/conversation/messages", s.handleConversationPost)
		r.Post("/conversation/cancel", s.handleConversationCancel)

		r.Get("/sse", s.handleSSE)

		r.Get("/tools", s.handleToolsList)
		r.Get("/skills", s.handleSkillsList)
		r.Get("/providers", s.handleProvidersList)
		r.Post("/providers/{name}/models", s.handleProvidersModels)
		r.Post("/fallback_providers/{index}/models", s.handleFallbackProvidersModels)
	})

	r.Get("/", s.handleIndex)
	r.Get("/ui/*", s.handleStatic)

	return r
}
```

Remove any remaining references to `SessionRegistry`, `sessionrun`, `GatewayController`, `ApplyResult`, `ErrApplyInProgress` etc. from `server.go`. `NewServer` should now return a `*Server` populated with `opts`, `router`, `streams`, and `deps` — nothing else.

- [ ] **Step 3: Rewrite `api/stream.go` / `api/stream_hook.go`**

Open each file. Remove `SessionID` from `StreamEvent`. Single broadcast hub (no per-session subscribers). The shape becomes:

```go
type StreamEvent struct {
	Type string
	Data map[string]any
}

type StreamHub interface {
	Publish(ev StreamEvent)
	Subscribe() (<-chan StreamEvent, func())
}
```

Rewrite the memory hub (`NewMemoryStreamHub`) accordingly — fan out each event to every active subscriber.

- [ ] **Step 4: Update `api/dto.go`**

Delete `SessionDTO`, `SessionListResponse`, `MessageDTO`, `MessagesResponse`, `MessageSubmitResponse`, `PlatformsSchemaResponse`, `RevealResponse`, and any other session/gateway DTOs. Add new DTOs:

```go
type StoredMessageDTO struct {
	ID               int64   `json:"id"`
	Role             string  `json:"role"`
	Content          string  `json:"content"`
	ToolCallID       string  `json:"tool_call_id,omitempty"`
	ToolName         string  `json:"tool_name,omitempty"`
	Timestamp        float64 `json:"timestamp"`
	FinishReason     string  `json:"finish_reason,omitempty"`
	Reasoning        string  `json:"reasoning,omitempty"`
}

type ConversationHistoryResponse struct {
	Messages []StoredMessageDTO `json:"messages"`
}

type ConversationPostRequest struct {
	UserMessage string `json:"user_message"`
	Model       string `json:"model,omitempty"`
}

type ConversationPostResponse struct {
	Accepted bool `json:"accepted"`
}
```

- [ ] **Step 5: Build**

Run: `go build ./...`
Expected: still failures in `api/sse.go`, `api/ws.go`, `agent/*`, `cli/*`. Those come next.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "chore(api): delete session/gateway handlers and sessionrun package"
```

---

## Phase C — Agent engine, API handlers, cron

## Task C1: Rewrite `agent/engine.go` + `agent/conversation.go` (no Session)

**Files:**
- Modify: `agent/engine.go`
- Modify: `agent/conversation.go`
- Modify: `agent/compression.go`

- [ ] **Step 1: `agent/engine.go` — remove session-created callback**

Remove:
- Field `onSessionCreated func(*storage.Session)`
- Method `SetSessionCreatedCallback(fn func(s *storage.Session))`

Keep everything else intact (provider, auxProvider, storage, tools, config, platform, prompt, compressor, aux, memory, onStreamDelta, onToolStart, onToolResult, activeSkills).

- [ ] **Step 2: Rewrite `agent/conversation.go:RunOptions` and `ConversationResult`**

```go
type RunOptions struct {
	UserMessage string
	Model       string
	Ephemeral   bool
	History     []message.Message // used when Ephemeral=true; empty starts fresh
}

type ConversationResult struct {
	Response   message.Message
	Messages   []message.Message
	Usage      message.Usage
	Iterations int
}
```

- [ ] **Step 3: Rewrite `RunConversation`**

```go
func (e *Engine) RunConversation(ctx context.Context, opts *RunOptions) (*ConversationResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	model := opts.Model
	if model == "" {
		model = "claude-opus-4-6"
	}

	var history []message.Message
	if opts.Ephemeral {
		history = append([]message.Message{}, opts.History...)
	} else if e.storage != nil {
		rows, err := e.storage.GetHistory(ctx, 0, 0)
		if err != nil {
			return nil, fmt.Errorf("engine: load history: %w", err)
		}
		for _, row := range rows {
			msg, err := messageFromStored(row)
			if err != nil {
				return nil, err
			}
			history = append(history, msg)
		}
	}

	// Append the new user message to the in-memory history
	userMsg := message.Message{
		Role:    message.RoleUser,
		Content: message.TextContent(opts.UserMessage),
	}
	history = append(history, userMsg)
	if e.memory != nil {
		e.memory.ObserveTurn(userMsg)
	}

	// Persist the user message (only when not ephemeral).
	if !opts.Ephemeral && e.storage != nil {
		if err := e.persistMessage(ctx, &userMsg); err != nil {
			return nil, fmt.Errorf("engine: persist user message: %w", err)
		}
	}

	var activeSkills []ActiveSkill
	if e.activeSkills != nil {
		activeSkills = e.activeSkills()
	}
	systemPrompt := e.prompt.Build(&PromptOptions{Model: model, ActiveSkills: activeSkills})

	var toolDefs []tool.ToolDefinition
	if e.tools != nil {
		toolDefs = e.tools.Definitions(nil)
	}

	budget := NewBudget(e.config.MaxTurns)
	totalUsage := message.Usage{}
	iterations := 0
	var lastResponse message.Message

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !budget.Consume() {
			break
		}
		iterations++

		if !opts.Ephemeral && e.compressor != nil && shouldCompress(history, e.config.Compression) {
			if newHistory, err := e.compressor.Compress(ctx, history); err == nil {
				history = newHistory
			}
		}

		req := &provider.Request{
			Model:        model,
			SystemPrompt: systemPrompt,
			Messages:     history,
			Tools:        toolDefs,
			MaxTokens:    4096,
		}

		resp, err := e.streamOnce(ctx, req)
		if err != nil {
			return nil, err
		}

		history = append(history, resp.Message)
		lastResponse = resp.Message
		if e.memory != nil {
			e.memory.ObserveTurn(resp.Message)
		}
		totalUsage.InputTokens += resp.Usage.InputTokens
		totalUsage.OutputTokens += resp.Usage.OutputTokens
		totalUsage.CacheReadTokens += resp.Usage.CacheReadTokens
		totalUsage.CacheWriteTokens += resp.Usage.CacheWriteTokens

		if !opts.Ephemeral && e.storage != nil {
			respCopy := resp
			txErr := e.storage.WithTx(ctx, func(tx storage.Tx) error {
				m := &history[len(history)-1]
				if err := e.persistMessageTx(ctx, tx, m); err != nil {
					return err
				}
				return tx.UpdateUsage(ctx, &storage.UsageUpdate{
					InputTokens:      respCopy.Usage.InputTokens,
					OutputTokens:     respCopy.Usage.OutputTokens,
					CacheReadTokens:  respCopy.Usage.CacheReadTokens,
					CacheWriteTokens: respCopy.Usage.CacheWriteTokens,
				})
			})
			if txErr != nil {
				return nil, fmt.Errorf("engine: persist response: %w", txErr)
			}
		}

		toolCalls := extractToolCalls(resp.Message.Content)
		if len(toolCalls) == 0 {
			break
		}

		toolResults := e.executeToolCalls(ctx, toolCalls)
		toolResultMsg := message.Message{
			Role:    message.RoleUser,
			Content: message.BlockContent(toolResults),
		}
		history = append(history, toolResultMsg)

		if !opts.Ephemeral && e.storage != nil {
			if err := e.persistMessage(ctx, &toolResultMsg); err != nil {
				return nil, fmt.Errorf("engine: persist tool result: %w", err)
			}
		}
	}

	return &ConversationResult{
		Response:   lastResponse,
		Messages:   history,
		Usage:      totalUsage,
		Iterations: iterations,
	}, nil
}

// persistMessage / persistMessageTx drop the sessionID parameter.
func (e *Engine) persistMessage(ctx context.Context, m *message.Message) error {
	stored, err := storedFromMessage(m)
	if err != nil {
		return err
	}
	return e.storage.AppendMessage(ctx, stored)
}

func (e *Engine) persistMessageTx(ctx context.Context, tx storage.Tx, m *message.Message) error {
	stored, err := storedFromMessage(m)
	if err != nil {
		return err
	}
	return tx.AppendMessage(ctx, stored)
}

// storedFromMessage loses the sessionID parameter.
func storedFromMessage(m *message.Message) (*storage.StoredMessage, error) {
	contentJSON, err := m.Content.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("engine: marshal content: %w", err)
	}
	return &storage.StoredMessage{
		Role:         string(m.Role),
		Content:      string(contentJSON),
		ToolCallID:   m.ToolCallID,
		ToolName:     m.ToolName,
		Timestamp:    time.Now().UTC(),
		FinishReason: m.FinishReason,
		Reasoning:    m.Reasoning,
	}, nil
}

// messageFromStored rebuilds a message.Message from a StoredMessage
// pulled out of storage.GetHistory. Needed because the engine now
// loads prior turns from the store instead of getting them from the
// caller.
func messageFromStored(row *storage.StoredMessage) (message.Message, error) {
	var content message.Content
	if err := content.UnmarshalJSON([]byte(row.Content)); err != nil {
		return message.Message{}, fmt.Errorf("engine: decode stored content: %w", err)
	}
	return message.Message{
		Role:         message.Role(row.Role),
		Content:      content,
		ToolCallID:   row.ToolCallID,
		ToolName:     row.ToolName,
		FinishReason: row.FinishReason,
		Reasoning:    row.Reasoning,
	}, nil
}
```

Delete the `ensureSession` function entirely — there's no session to ensure.

- [ ] **Step 4: Update `agent/compression.go`**

Grep it for `sessionID` / `SessionID`. Remove those parameters. `Compressor.Compress(ctx, history)` already takes the history slice; update any callers/tests that had the session param.

- [ ] **Step 5: Update `agent/engine_test.go` + `agent/conversation_test.go`**

Rewrite the tests to use the new `RunOptions` shape. Delete `TestRunConversation_PrefersSessionModelOverRunOptions` (session concept is gone; `opts.Model` always wins).

Add a new test:

```go
func TestRunConversation_EphemeralDoesNotPersist(t *testing.T) {
	store := openTestStore(t)
	e := agent.NewEngineWithTools(mockProvider(), store, nil, config.AgentConfig{MaxTurns: 2}, "test")

	_, err := e.RunConversation(context.Background(), &agent.RunOptions{
		UserMessage: "hi",
		Ephemeral:   true,
	})
	require.NoError(t, err)

	hist, err := store.GetHistory(context.Background(), 100, 0)
	require.NoError(t, err)
	assert.Empty(t, hist, "ephemeral run must not append to storage")
}
```

- [ ] **Step 6: Build and test**

Run: `go test ./agent/...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor(agent): flatten Engine — no Session; Ephemeral flag for cron"
```

---

## Task C2: Write `/api/conversation*` and `/api/sse` handlers

**Files:**
- Create: `api/handlers_conversation.go`
- Create: `api/handlers_sse.go`
- Create: `api/handlers_conversation_test.go`

- [ ] **Step 1: Write conversation handlers**

Create `api/handlers_conversation.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/odysseythink/hermind/agent"
)

func (s *Server) handleConversationGet(w http.ResponseWriter, r *http.Request) {
	if s.opts.Storage == nil {
		http.Error(w, "storage not configured", http.StatusServiceUnavailable)
		return
	}
	limit := atoiDefault(r.URL.Query().Get("limit"), 200)
	offset := atoiDefault(r.URL.Query().Get("offset"), 0)

	rows, err := s.opts.Storage.GetHistory(r.Context(), limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]StoredMessageDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, StoredMessageDTO{
			ID:           row.ID,
			Role:         row.Role,
			Content:      row.Content,
			ToolCallID:   row.ToolCallID,
			ToolName:     row.ToolName,
			Timestamp:    toEpoch(row.Timestamp),
			FinishReason: row.FinishReason,
			Reasoning:    row.Reasoning,
		})
	}
	writeJSON(w, ConversationHistoryResponse{Messages: out})
}

// runLock serializes conversation runs — one in-flight at a time.
var runMu sync.Mutex
var runCancel context.CancelFunc

func (s *Server) handleConversationPost(w http.ResponseWriter, r *http.Request) {
	if s.opts.Deps.Provider == nil {
		http.Error(w, "provider not configured", http.StatusServiceUnavailable)
		return
	}
	var body ConversationPostRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.UserMessage == "" {
		http.Error(w, "user_message required", http.StatusBadRequest)
		return
	}

	runMu.Lock()
	if runCancel != nil {
		runMu.Unlock()
		http.Error(w, "another turn is in flight", http.StatusConflict)
		return
	}
	runCtx, cancel := context.WithCancel(context.Background())
	runCancel = cancel
	runMu.Unlock()

	eng := agent.NewEngineWithToolsAndAux(
		s.opts.Deps.Provider, s.opts.Deps.AuxProvider, s.opts.Deps.Storage,
		s.opts.Deps.ToolReg, s.opts.Deps.AgentCfg, s.opts.Deps.Platform,
	)
	wireEngineToHub(eng, s.streams)

	go func() {
		defer func() {
			runMu.Lock()
			runCancel = nil
			runMu.Unlock()
			cancel()
		}()
		_, err := eng.RunConversation(runCtx, &agent.RunOptions{
			UserMessage: body.UserMessage,
			Model:       body.Model,
		})
		if err != nil {
			s.streams.Publish(StreamEvent{
				Type: "error",
				Data: map[string]any{"message": err.Error()},
			})
		}
		s.streams.Publish(StreamEvent{Type: "done"})
	}()

	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, ConversationPostResponse{Accepted: true})
}

func (s *Server) handleConversationCancel(w http.ResponseWriter, _ *http.Request) {
	runMu.Lock()
	defer runMu.Unlock()
	if runCancel != nil {
		runCancel()
		runCancel = nil
	}
	w.WriteHeader(http.StatusNoContent)
}
```

Add a helper that wires the engine's per-event callbacks to the stream hub:

```go
// wireEngineToHub forwards engine callbacks to the stream hub as
// chunk/tool_call/tool_result events. Kept minimal — the exact event
// shapes must match what useChatStream.ts expects (see PR 2 §Frontend).
func wireEngineToHub(eng *agent.Engine, hub StreamHub) {
	eng.SetStreamDeltaCallback(func(d *provider.StreamDelta) {
		if d == nil || d.TextDelta == "" {
			return
		}
		hub.Publish(StreamEvent{
			Type: "message_chunk",
			Data: map[string]any{"text": d.TextDelta},
		})
	})
	eng.SetToolStartCallback(func(call message.ContentBlock) {
		hub.Publish(StreamEvent{
			Type: "tool_call",
			Data: map[string]any{
				"id":    call.ToolUseID,
				"name":  call.ToolUseName,
				"input": call.ToolUseInput,
			},
		})
	})
	eng.SetToolResultCallback(func(call message.ContentBlock, result string) {
		hub.Publish(StreamEvent{
			Type: "tool_result",
			Data: map[string]any{
				"id":     call.ToolUseID,
				"result": result,
			},
		})
	})
}
```

Add any necessary imports (`"context"`, `"github.com/odysseythink/hermind/provider"`, `"github.com/odysseythink/hermind/message"`).

- [ ] **Step 2: Write SSE handler**

Create `api/handlers_sse.go`:

```go
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	events, unsub := s.streams.Subscribe()
	defer unsub()

	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			payload, err := json.Marshal(map[string]any{
				"type": ev.Type,
				"data": ev.Data,
			})
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
	}
}
```

- [ ] **Step 3: Write handler tests**

Create `api/handlers_conversation_test.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage"
)

func TestConversationGet_EmptyReturnsEmptyList(t *testing.T) {
	store := newMemoryFakeStore()
	srv, err := NewServer(&ServerOpts{
		Config:  &config.Config{},
		Version: "test",
		Storage: store,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/conversation", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body ConversationHistoryResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Empty(t, body.Messages)
}

func TestConversationGet_ReturnsAppendedMessages(t *testing.T) {
	store := newMemoryFakeStore()
	require.NoError(t, store.AppendMessage(context.Background(), &storage.StoredMessage{
		Role: "user", Content: `{"text":"hi"}`,
	}))

	srv, _ := NewServer(&ServerOpts{
		Config: &config.Config{}, Version: "test", Storage: store,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/conversation", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	var body ConversationHistoryResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Len(t, body.Messages, 1)
	assert.Equal(t, "user", body.Messages[0].Role)
}

func TestConversationPost_Returns503WhenNoProvider(t *testing.T) {
	srv, _ := NewServer(&ServerOpts{
		Config: &config.Config{}, Version: "test", Storage: newMemoryFakeStore(),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/conversation/messages",
		strings.NewReader(`{"user_message":"hi"}`))
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestOldSessionRoutesReturn404(t *testing.T) {
	srv, _ := NewServer(&ServerOpts{
		Config: &config.Config{}, Version: "test",
	})
	for _, path := range []string{"/api/sessions", "/api/sessions/abc", "/api/sessions/abc/messages"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code, "path %s", path)
	}
}
```

Write a minimal `newMemoryFakeStore()` test helper (or wire the real sqlite store with a temp dir). If there's an existing fake in a test helper file, reuse it.

- [ ] **Step 4: Build and run tests**

Run: `go test ./api/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/handlers_conversation.go api/handlers_sse.go api/handlers_conversation_test.go
git commit -m "feat(api): GET/POST /api/conversation + /api/sse (single stream)"
```

---

## Task C3: `/api/meta` exposes `current_model`

**Files:**
- Modify: `api/dto.go`
- Modify: `api/handlers_meta.go`

- [ ] **Step 1: Extend `StatusResponse`**

```go
type StatusResponse struct {
	Version       string `json:"version"`
	UptimeSec     int64  `json:"uptime_sec"`
	StorageDriver string `json:"storage_driver"`
	InstanceRoot  string `json:"instance_root"`
	CurrentModel  string `json:"current_model"`
}
```

- [ ] **Step 2: Populate `current_model` in `handleStatus`**

```go
func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, StatusResponse{
		Version:       s.opts.Version,
		UptimeSec:     int64(time.Since(s.bootedAt).Seconds()),
		StorageDriver: s.driverName(),
		InstanceRoot:  s.opts.InstanceRoot,
		CurrentModel:  s.opts.Config.Model,
	})
}
```

- [ ] **Step 3: Test**

Append to `api/server_test.go` (or `handlers_meta_test.go`):

```go
func TestHandleStatus_ReturnsCurrentModel(t *testing.T) {
	srv, _ := NewServer(&ServerOpts{
		Config:       &config.Config{Model: "anthropic/claude-opus-4-6"},
		Version:      "test",
		InstanceRoot: "/tmp/i",
	})
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	var body StatusResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "anthropic/claude-opus-4-6", body.CurrentModel)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./api/... -run TestHandleStatus -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/dto.go api/handlers_meta.go api/server_test.go
git commit -m "feat(api): /api/status exposes current_model"
```

---

## Task C4: Cron ephemeral runs + per-job trajectory

**Files:**
- Modify: `cli/cron.go`
- Modify: `agent/trajectory.go`
- Create: `cli/cron_ephemeral_test.go`

- [ ] **Step 1: Rewrite `buildCronJob`**

```go
func buildCronJob(jc config.CronJobConfig, sched cron.Schedule, prov provider.Provider, app *App) cron.Job {
	jobName := jc.Name
	prompt := jc.Prompt
	model := jc.Model
	return cron.Job{
		Name:     jobName,
		Schedule: sched,
		Run: func(ctx context.Context) error {
			ctx = logging.WithRequestID(ctx, uuid.NewString())
			// Isolated engine with no storage — cron runs do not touch
			// the main conversation's messages table.
			eng := agent.NewEngineWithTools(prov, nil, nil, app.Config.Agent, "cron")

			// Each job gets its own trajectory file.
			root, err := config.InstancePath("trajectories")
			if err != nil {
				return err
			}
			tw, err := agent.NewTrajectoryWriter(
				root,
				fmt.Sprintf("cron-%s-%d", jobName, time.Now().Unix()),
			)
			if err != nil {
				return err
			}
			defer tw.Close()
			eng.SetStreamDeltaCallback(func(d *provider.StreamDelta) {
				_ = tw.Write(agent.TrajectoryEvent{
					Kind:    "assistant",
					Content: d.TextDelta,
				})
			})

			_, err = eng.RunConversation(ctx, &agent.RunOptions{
				UserMessage: prompt,
				Model:       model,
				Ephemeral:   true,
				History:     nil,
			})
			return err
		},
	}
}
```

- [ ] **Step 2: Update `agent/trajectory.go`**

`TrajectoryEvent.SessionID` is no longer meaningful as a session discriminator — rename it to `JobID` (or just remove it; all callers pass a per-job name). For simplicity, remove it. Update `TrajectoryWriter.Write` / `NewTrajectoryWriter` signatures: the writer already takes a `dir, id string` — rename the second argument from `sessionID` to `name`. No behavior change.

Adjust any tests that populate `ev.SessionID`.

- [ ] **Step 3: Write the ephemeral assertion test**

Create `cli/cron_ephemeral_test.go`:

```go
package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/config"
)

func TestCronRun_DoesNotWriteToMainConversation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HERMIND_HOME", "")
	t.Chdir(tmp)

	app, err := NewApp()
	require.NoError(t, err)
	defer app.Close()

	require.NoError(t, ensureStorage(app))
	before, err := app.Storage.GetHistory(context.Background(), 1000, 0)
	require.NoError(t, err)
	require.Empty(t, before)

	// Directly exercise buildCronJob's Run function with a fake provider.
	job := buildCronJob(
		config.CronJobConfig{Name: "unit-test", Prompt: "hi", Schedule: "@every 1m"},
		nil, // schedule is unused by Run
		mockProviderOK(t),
		app,
	)
	require.NoError(t, job.Run(context.Background()))

	// Main conversation unchanged.
	after, err := app.Storage.GetHistory(context.Background(), 1000, 0)
	require.NoError(t, err)
	assert.Empty(t, after)

	// Trajectory file exists under <instance>/trajectories/.
	entries, err := os.ReadDir(filepath.Join(tmp, ".hermind", "trajectories"))
	require.NoError(t, err)
	var foundCron bool
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".jsonl" {
			foundCron = true
			break
		}
	}
	assert.True(t, foundCron, "expected a cron-*.jsonl in <instance>/trajectories/")
}

// mockProviderOK returns a fake provider.Provider whose Stream emits a
// single done event so RunConversation terminates immediately. First
// check agent/testing/ or internal/testprovider/ for an existing helper;
// if none, use this inline shim.
type mockProv struct{}

func (mockProv) Name() string                                                 { return "mock" }
func (mockProv) Complete(_ context.Context, _ *provider.Request) (*provider.Response, error) {
	return nil, nil
}
func (mockProv) Stream(_ context.Context, _ *provider.Request) (provider.Stream, error) {
	return &mockStream{}, nil
}
func (mockProv) ModelInfo(_ string) *provider.ModelInfo       { return nil }
func (mockProv) EstimateTokens(_, s string) (int, error)      { return len(s) / 4, nil }
func (mockProv) Available() bool                              { return true }

type mockStream struct{ done bool }

func (m *mockStream) Recv() (*provider.StreamEvent, error) {
	if m.done {
		return nil, io.EOF
	}
	m.done = true
	return &provider.StreamEvent{
		Type: provider.EventDone,
		Response: &provider.Response{
			Message: message.Message{
				Role:    message.RoleAssistant,
				Content: message.TextContent("ok"),
			},
		},
	}, nil
}
func (m *mockStream) Close() error { return nil }

func mockProviderOK(t *testing.T) provider.Provider {
	t.Helper()
	return mockProv{}
}
```

(If the codebase already has a mock provider helper, use it and delete the TODO stub.)

- [ ] **Step 4: Build and test**

Run: `go build ./... && go test ./cli/... -run TestCronRun_DoesNotWriteToMainConversation -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(cron): ephemeral runs with per-job trajectory; no main history writes"
```

---

## Phase D — Frontend

## Task D1: Delete session/sidebar/drawer components

**Files:**
- Delete: listed below

- [ ] **Step 1: Delete files**

```bash
git rm web/src/components/chat/ChatSidebar.{tsx,module.css} \
       web/src/components/chat/SessionList.{tsx,module.css} \
       web/src/components/chat/SessionItem.{tsx,module.css,test.tsx} \
       web/src/components/chat/NewChatButton.{tsx,module.css} \
       web/src/components/chat/SessionSettingsDrawer.{tsx,module.css,test.tsx} \
       web/src/components/chat/SettingsButton.{tsx,module.css} \
       web/src/hooks/useSessionList.{ts,test.ts}
```

- [ ] **Step 2: Build expect-to-fail**

Run: `cd web && pnpm build`
Expected: FAIL — `ChatWorkspace.tsx` still imports deleted modules.

- [ ] **Step 3: Commit**

```bash
git commit -m "chore(web): delete session sidebar, list, drawer, settings button"
```

---

## Task D2: Rewrite `ChatWorkspace` — single-column, no sidebar

**Files:**
- Modify: `web/src/components/chat/ChatWorkspace.tsx`
- Modify: `web/src/components/chat/ChatWorkspace.module.css`
- Modify: `web/src/components/chat/ChatWorkspace.test.tsx`

- [ ] **Step 1: Rewrite `ChatWorkspace.tsx`**

```tsx
import { useEffect, useReducer, useState } from 'react';
import { useTranslation } from 'react-i18next';
import ConversationHeader from './ConversationHeader';
import MessageList from './MessageList';
import ComposerBar from './ComposerBar';
import Toast from './Toast';
import styles from './ChatWorkspace.module.css';
import { useChatStream } from '../../hooks/useChatStream';
import { chatReducer, initialChatState } from '../../state/chat';
import { apiFetch, ApiError } from '../../api/client';
import { ConversationHistoryResponseSchema } from '../../api/schemas';

type Props = {
  instanceRoot: string;
  providerConfigured?: boolean;
  modelOptions: string[];
  currentModel: string;
};

export default function ChatWorkspace({
  instanceRoot,
  providerConfigured = true,
  modelOptions,
  currentModel,
}: Props) {
  const { t } = useTranslation('ui');
  const [state, dispatch] = useReducer(chatReducer, initialChatState);
  const [toast, setToast] = useState<string | null>(null);
  const [runtimeModel, setRuntimeModel] = useState<string>(currentModel);

  useChatStream(dispatch);

  // Load persisted history on mount.
  useEffect(() => {
    const ctrl = new AbortController();
    apiFetch('/api/conversation', {
      schema: ConversationHistoryResponseSchema,
      signal: ctrl.signal,
    })
      .then((r) =>
        dispatch({
          type: 'chat/history/loaded',
          messages: r.messages.map((m) => ({
            id: String(m.id),
            role: m.role,
            content: m.content,
            timestamp: m.timestamp,
          })),
        }),
      )
      .catch(() => {/* empty history is fine */});
    return () => ctrl.abort();
  }, []);

  async function handleSend() {
    const text = state.composer.text.trim();
    if (!text) return;
    dispatch({ type: 'chat/composer/setText', text: '' });
    dispatch({ type: 'chat/stream/start', userText: text });
    try {
      await apiFetch('/api/conversation/messages', {
        method: 'POST',
        body: { user_message: text, model: runtimeModel },
      });
    } catch (err) {
      dispatch({ type: 'chat/stream/rollbackUserMessage' });
      if (err instanceof ApiError) {
        if (err.status === 409) setToast(t('chat.errorBusy'));
        else if (err.status === 503) setToast(t('chat.errorNoProvider'));
        else setToast(t('chat.errorSendFailed', { msg: err.message }));
      } else {
        setToast(t('chat.errorSendFailed', { msg: err instanceof Error ? err.message : '' }));
      }
    }
  }

  async function handleStop() {
    try {
      await apiFetch('/api/conversation/cancel', { method: 'POST' });
    } catch (err) {
      console.warn('cancel failed', err);
    }
  }

  return (
    <div className={styles.workspace}>
      <ConversationHeader
        instanceRoot={instanceRoot}
        modelOptions={modelOptions}
        selectedModel={runtimeModel}
        onSelectModel={setRuntimeModel}
        onStop={handleStop}
        streaming={state.streaming.status === 'running'}
      />
      <MessageList
        messages={state.messages}
        streamingDraft={state.streaming.assistantDraft}
        streamingToolCalls={state.streaming.toolCalls}
      />
      {state.streaming.status === 'error' && state.streaming.error && (
        <div role="alert" className={styles.errorBanner}>
          {state.streaming.error}
        </div>
      )}
      <ComposerBar
        text={state.composer.text}
        onChangeText={(txt) => dispatch({ type: 'chat/composer/setText', text: txt })}
        onSend={handleSend}
        onStop={handleStop}
        disabled={!providerConfigured}
        streaming={state.streaming.status === 'running'}
        onSlashCommand={(cmd) => {
          if (cmd === 'clear') dispatch({ type: 'chat/composer/setText', text: '' });
        }}
      />
      {toast && <Toast message={toast} onDismiss={() => setToast(null)} />}
    </div>
  );
}
```

- [ ] **Step 2: Rewrite `ChatWorkspace.module.css`**

```css
.workspace {
  display: grid;
  grid-template-rows: auto 1fr auto;
  height: 100vh;
  min-height: 0;
}

.errorBanner {
  background: var(--bg-error);
  color: var(--fg-error);
  padding: 8px 16px;
  font-size: 13px;
  font-family: var(--font-mono, monospace);
  border-top: 1px solid var(--border-error);
}
```

- [ ] **Step 3: Rewrite `ChatWorkspace.test.tsx`**

Delete any test assertions about the sidebar, session list, new-chat button, or settings drawer. Keep assertions about:
- Header renders `instanceRoot`
- Sending a message fires POST `/api/conversation/messages`
- Stop button fires POST `/api/conversation/cancel`

Example:

```tsx
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ChatWorkspace from './ChatWorkspace';

// Mock apiFetch, SSE, etc. following the existing test scaffolding.

it('sends a user message to /api/conversation/messages', async () => {
  const fetchMock = vi.fn().mockResolvedValue({ accepted: true });
  vi.mock('../../api/client', () => ({
    apiFetch: fetchMock,
    ApiError: class extends Error {},
  }));

  render(
    <ChatWorkspace
      instanceRoot="/tmp/.hermind"
      modelOptions={['anthropic/claude-opus-4-6']}
      currentModel="anthropic/claude-opus-4-6"
    />,
  );
  await userEvent.type(screen.getByRole('textbox'), 'hello');
  await userEvent.click(screen.getByRole('button', { name: /send/i }));
  await waitFor(() => {
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/conversation/messages',
      expect.objectContaining({ method: 'POST' }),
    );
  });
});
```

- [ ] **Step 4: Run tests**

Run: `cd web && pnpm test --run ChatWorkspace`
Expected: PASS. The tests will still fail because `ConversationHeader`, `state/chat.ts`, `api/schemas.ts` haven't been updated. Those come in subsequent tasks.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(web): ChatWorkspace single-column, no sidebar/drawer"
```

---

## Task D3: Rewrite `ConversationHeader` with model dropdown + stop

**Files:**
- Modify: `web/src/components/chat/ConversationHeader.tsx`
- Modify: `web/src/components/chat/ConversationHeader.module.css`

- [ ] **Step 1: Rewrite `ConversationHeader.tsx`**

```tsx
import { useTranslation } from 'react-i18next';
import styles from './ConversationHeader.module.css';

type Props = {
  instanceRoot: string;
  modelOptions: string[];
  selectedModel: string;
  onSelectModel: (model: string) => void;
  onStop: () => void;
  streaming: boolean;
};

export default function ConversationHeader({
  instanceRoot,
  modelOptions,
  selectedModel,
  onSelectModel,
  onStop,
  streaming,
}: Props) {
  const { t } = useTranslation('ui');
  return (
    <header className={styles.header}>
      <span
        className={styles.instancePath}
        title={instanceRoot}
        dir="rtl"
        aria-label={t('chat.instance.label', { defaultValue: 'Instance' })}
      >
        {instanceRoot}
      </span>
      <div className={styles.spacer} />
      <select
        className={styles.modelSelect}
        value={selectedModel}
        onChange={(e) => onSelectModel(e.target.value)}
        aria-label={t('chat.modelDropdown', { defaultValue: 'Model' })}
      >
        {modelOptions.map((m) => (
          <option key={m} value={m}>
            {m}
          </option>
        ))}
      </select>
      <button
        type="button"
        className={styles.stopBtn}
        disabled={!streaming}
        onClick={onStop}
        aria-label={t('chat.stop', { defaultValue: 'Stop' })}
      >
        {t('chat.stop', { defaultValue: 'Stop' })}
      </button>
    </header>
  );
}
```

- [ ] **Step 2: Update `ConversationHeader.module.css`**

```css
.header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--border-subtle);
}
.instancePath {
  font-family: var(--font-mono, 'JetBrains Mono', monospace);
  font-size: 13px;
  color: var(--fg-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 40ch;
  direction: rtl;
  unicode-bidi: plaintext;
}
.spacer { flex: 1; }
.modelSelect {
  font-family: var(--font-mono, monospace);
  font-size: 13px;
  padding: 4px 8px;
  border: 1px solid var(--border-subtle);
  border-radius: 2px;
  background: var(--bg-inset);
}
.stopBtn {
  font-family: var(--font-mono, monospace);
  font-size: 13px;
  padding: 4px 12px;
  border: 1px solid var(--border-subtle);
  border-radius: 2px;
  background: transparent;
  cursor: pointer;
}
.stopBtn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
```

- [ ] **Step 3: Run tests**

Run: `cd web && pnpm test --run ConversationHeader`
Expected: the existing tests will fail because the prop shape changed. Rewrite them to match the new signature. Add:

```tsx
it('disables stop button when not streaming', () => {
  render(
    <ConversationHeader
      instanceRoot="/tmp/.hermind"
      modelOptions={['m1']}
      selectedModel="m1"
      onSelectModel={() => {}}
      onStop={() => {}}
      streaming={false}
    />,
  );
  expect(screen.getByRole('button', { name: /stop/i })).toBeDisabled();
});
```

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor(web): ConversationHeader — instance path + model dropdown + stop"
```

---

## Task D4: Rewrite chat state reducer (single conversation)

**Files:**
- Modify: `web/src/state/chat.ts`
- Modify: `web/src/state/chat.test.ts`

- [ ] **Step 1: Rewrite `state/chat.ts`**

```ts
export type ChatMessage = {
  id: string;
  role: string;
  content: string;
  timestamp: number;
};

export type ToolCall = {
  id: string;
  name: string;
  input: unknown;
  state: 'running' | 'done' | 'error';
  result?: string;
};

export type StreamingState = {
  status: 'idle' | 'running' | 'error';
  assistantDraft: string;
  toolCalls: ToolCall[];
  error?: string;
};

export type ChatState = {
  messages: ChatMessage[];
  composer: { text: string };
  streaming: StreamingState;
};

export const initialChatState: ChatState = {
  messages: [],
  composer: { text: '' },
  streaming: { status: 'idle', assistantDraft: '', toolCalls: [] },
};

export type ChatAction =
  | { type: 'chat/history/loaded'; messages: ChatMessage[] }
  | { type: 'chat/composer/setText'; text: string }
  | { type: 'chat/stream/start'; userText: string }
  | { type: 'chat/stream/token'; delta: string }
  | { type: 'chat/stream/toolCall'; call: ToolCall }
  | { type: 'chat/stream/toolResult'; id: string; result: string }
  | { type: 'chat/stream/done'; assistantText: string }
  | { type: 'chat/stream/error'; message: string }
  | { type: 'chat/stream/rollbackUserMessage' };

export function chatReducer(state: ChatState, action: ChatAction): ChatState {
  switch (action.type) {
    case 'chat/history/loaded':
      return { ...state, messages: action.messages };
    case 'chat/composer/setText':
      return { ...state, composer: { text: action.text } };
    case 'chat/stream/start':
      return {
        ...state,
        messages: [
          ...state.messages,
          {
            id: `user-${Date.now()}`,
            role: 'user',
            content: JSON.stringify({ text: action.userText }),
            timestamp: Date.now(),
          },
        ],
        streaming: { status: 'running', assistantDraft: '', toolCalls: [] },
      };
    case 'chat/stream/token':
      return {
        ...state,
        streaming: {
          ...state.streaming,
          assistantDraft: state.streaming.assistantDraft + action.delta,
        },
      };
    case 'chat/stream/toolCall':
      return {
        ...state,
        streaming: {
          ...state.streaming,
          toolCalls: [...state.streaming.toolCalls, action.call],
        },
      };
    case 'chat/stream/toolResult':
      return {
        ...state,
        streaming: {
          ...state.streaming,
          toolCalls: state.streaming.toolCalls.map((c) =>
            c.id === action.id ? { ...c, state: 'done', result: action.result } : c,
          ),
        },
      };
    case 'chat/stream/done':
      return {
        ...state,
        messages: [
          ...state.messages,
          {
            id: `asst-${Date.now()}`,
            role: 'assistant',
            content: action.assistantText,
            timestamp: Date.now(),
          },
        ],
        streaming: { status: 'idle', assistantDraft: '', toolCalls: [] },
      };
    case 'chat/stream/error':
      return {
        ...state,
        streaming: { ...state.streaming, status: 'error', error: action.message },
      };
    case 'chat/stream/rollbackUserMessage':
      return {
        ...state,
        messages: state.messages.slice(0, -1),
        streaming: { status: 'idle', assistantDraft: '', toolCalls: [] },
      };
  }
}
```

- [ ] **Step 2: Rewrite `state/chat.test.ts`**

Replace all tests with cases exercising each action above. Representative:

```ts
import { chatReducer, initialChatState } from './chat';

it('appends user message on stream/start', () => {
  const s = chatReducer(initialChatState, { type: 'chat/stream/start', userText: 'hi' });
  expect(s.messages).toHaveLength(1);
  expect(s.streaming.status).toBe('running');
});

it('rolls back user message on failure', () => {
  let s = chatReducer(initialChatState, { type: 'chat/stream/start', userText: 'hi' });
  s = chatReducer(s, { type: 'chat/stream/rollbackUserMessage' });
  expect(s.messages).toHaveLength(0);
  expect(s.streaming.status).toBe('idle');
});
```

- [ ] **Step 3: Run tests**

Run: `cd web && pnpm test --run state/chat`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor(web/state): single-conversation chat reducer"
```

---

## Task D5: Rewrite `useChatStream` for single SSE stream

**Files:**
- Modify: `web/src/hooks/useChatStream.ts`
- Modify: `web/src/hooks/useChatStream.test.ts`

- [ ] **Step 1: Rewrite the hook**

```ts
import { useEffect, useRef } from 'react';
import type { ChatAction } from '../state/chat';

type Dispatch = (a: ChatAction) => void;

export function useChatStream(dispatch: Dispatch) {
  const tokenBufRef = useRef('');
  const rafPendingRef = useRef(false);

  useEffect(() => {
    const es = new EventSource('/api/sse');

    function flushTokens() {
      rafPendingRef.current = false;
      if (tokenBufRef.current) {
        dispatch({ type: 'chat/stream/token', delta: tokenBufRef.current });
        tokenBufRef.current = '';
      }
    }

    es.onmessage = (ev) => {
      let parsed: { type?: string; data?: Record<string, unknown> };
      try {
        parsed = JSON.parse(ev.data);
      } catch {
        return;
      }
      switch (parsed.type) {
        case 'message_chunk': {
          const d = parsed.data as { text?: string } | undefined;
          if (typeof d?.text === 'string') {
            tokenBufRef.current += d.text;
            if (!rafPendingRef.current) {
              rafPendingRef.current = true;
              requestAnimationFrame(flushTokens);
            }
          }
          break;
        }
        case 'tool_call': {
          const d = parsed.data as Record<string, unknown>;
          dispatch({
            type: 'chat/stream/toolCall',
            call: {
              id: String(d.id ?? Date.now()),
              name: String(d.name ?? 'tool'),
              input: d.input ?? null,
              state: 'running',
            },
          });
          break;
        }
        case 'tool_result': {
          const d = parsed.data as Record<string, unknown>;
          dispatch({
            type: 'chat/stream/toolResult',
            id: String(d.id ?? ''),
            result: String(d.result ?? ''),
          });
          break;
        }
        case 'done': {
          dispatch({ type: 'chat/stream/done', assistantText: tokenBufRef.current });
          tokenBufRef.current = '';
          break;
        }
        case 'error': {
          const d = parsed.data as { message?: string } | undefined;
          dispatch({ type: 'chat/stream/error', message: d?.message ?? 'stream error' });
          break;
        }
      }
    };

    es.onerror = () => {
      dispatch({ type: 'chat/stream/error', message: 'SSE disconnected' });
    };

    return () => {
      es.close();
    };
  }, [dispatch]);
}
```

- [ ] **Step 2: Rewrite `useChatStream.test.ts`**

Delete session-created / session-updated tests. Add tests for `message_chunk`, `tool_call`, `tool_result`, `done`, `error`. Use the existing `EventSource` mock or `msw` / `vi.stubGlobal` pattern the project already has.

- [ ] **Step 3: Run tests**

Run: `cd web && pnpm test --run useChatStream`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor(web): useChatStream subscribes to single /api/sse stream"
```

---

## Task D6: Rewrite `api/schemas.ts` + drop session types

**Files:**
- Modify: `web/src/api/schemas.ts`
- Modify: `web/src/api/schemas.test.ts`

- [ ] **Step 1: Update schemas**

Delete: `SessionSummarySchema`, `SessionUpdatedPayloadSchema`, `SessionPatchSchema`, `MessagesResponseSchema`, `MessageSubmitResponseSchema`, `PlatformsSchemaResponseSchema`, `ApplyResultSchema`, and their TypeScript type exports.

Add:

```ts
export const StoredMessageSchema = z.object({
  id: z.number(),
  role: z.string(),
  content: z.string(),
  tool_call_id: z.string().optional(),
  tool_name: z.string().optional(),
  timestamp: z.number(),
  finish_reason: z.string().optional(),
  reasoning: z.string().optional(),
});
export type StoredMessage = z.infer<typeof StoredMessageSchema>;

export const ConversationHistoryResponseSchema = z.object({
  messages: z.array(StoredMessageSchema),
});
export type ConversationHistoryResponse = z.infer<typeof ConversationHistoryResponseSchema>;

export const MetaResponseSchema = z.object({
  version: z.string(),
  uptime_sec: z.number(),
  storage_driver: z.string(),
  instance_root: z.string(),
  current_model: z.string(),
});
export type MetaResponse = z.infer<typeof MetaResponseSchema>;
```

Keep `ConfigResponseSchema`, `ConfigSchemaResponseSchema`, `ProviderModelsResponseSchema` — the config editor still uses these.

- [ ] **Step 2: Update `api/schemas.test.ts`**

Delete tests for deleted schemas. Add tests for `MetaResponseSchema` and `ConversationHistoryResponseSchema`.

- [ ] **Step 3: Run tests**

Run: `cd web && pnpm test --run api/schemas`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor(web/api): drop session schemas; add MetaResponse + conversation history"
```

---

## Task D7: Simplify `App.tsx` — chat-only root

**Files:**
- Modify: `web/src/App.tsx`
- Modify: `web/src/App.test.tsx`
- Modify: `web/src/state.ts` (remove gateway slice; keep config editor slice)

- [ ] **Step 1: Rewrite `App.tsx`**

The app now renders `ChatWorkspace` by default. The config editor ("settings" mode) stays reachable via a top-bar toggle. All multi-session routing (`hashState.sessionId`, `onChangeSession`) is deleted.

Skeleton:

```tsx
import { useCallback, useEffect, useMemo, useReducer, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { apiFetch, ApiError } from './api/client';
import {
  ConfigResponseSchema,
  ConfigSchemaResponseSchema,
  MetaResponseSchema,
  ProviderModelsResponseSchema,
} from './api/schemas';
import { initialState, reducer, totalDirtyCount } from './state';
import TopBar from './components/shell/TopBar';
import SettingsSidebar from './components/shell/SettingsSidebar';
import SettingsPanel from './components/shell/SettingsPanel';
import Footer from './components/Footer';
import ChatWorkspace from './components/chat/ChatWorkspace';

export default function App() {
  const { t } = useTranslation('ui');
  const [state, dispatch] = useReducer(reducer, initialState);
  const [mode, setMode] = useState<'chat' | 'settings'>('chat');
  const [meta, setMeta] = useState<{ instanceRoot: string; currentModel: string }>({
    instanceRoot: '',
    currentModel: '',
  });

  useEffect(() => {
    const ctrl = new AbortController();
    (async () => {
      try {
        const [cfgSchema, cfg, metaResp] = await Promise.all([
          apiFetch('/api/config/schema', { schema: ConfigSchemaResponseSchema, signal: ctrl.signal }),
          apiFetch('/api/config', { schema: ConfigResponseSchema, signal: ctrl.signal }),
          apiFetch('/api/status', { schema: MetaResponseSchema, signal: ctrl.signal }),
        ]);
        dispatch({
          type: 'boot/loaded',
          descriptors: [],
          configSections: cfgSchema.sections,
          config: cfg.config,
        });
        setMeta({
          instanceRoot: metaResp.instance_root,
          currentModel: metaResp.current_model,
        });
      } catch (err) {
        if (ctrl.signal.aborted) return;
        dispatch({
          type: 'boot/failed',
          error: err instanceof Error ? err.message : t('status.bootFailed'),
        });
      }
    })();
    return () => ctrl.abort();
  }, []);

  // ... derive providerInstances, dirtyProviderKeys, etc. as in the old App.tsx
  // (KEEP the slices for the config editor: providers, mcp, cron, skills,
  //  memory, terminal, storage, agent, auxiliary.)
  // DELETE the slices: platforms, instances, dirtyInstanceKeys — those are
  // all gateway-related and the gateway group is gone.

  // Providers drive the chat model dropdown.
  const onFetchProviderModels = useCallback(async (key: string) => {
    const r = await apiFetch(`/api/providers/${encodeURIComponent(key)}/models`, {
      method: 'POST',
      schema: ProviderModelsResponseSchema,
    });
    dispatch({ type: 'provider/models/loaded', providerKey: key, models: r.models });
    return r;
  }, []);

  const allModels = useMemo(() => {
    const seen = new Set<string>();
    for (const models of Object.values(state.providerModels)) {
      for (const m of models) seen.add(m);
    }
    if (meta.currentModel) seen.add(meta.currentModel);
    return Array.from(seen).sort();
  }, [state.providerModels, meta.currentModel]);

  const providerConfigured = useMemo(
    () =>
      Object.keys(
        (state.config as { providers?: Record<string, unknown> }).providers ?? {},
      ).length > 0,
    [state.config],
  );

  if (state.status === 'booting') return <div style={{ padding: '2rem' }}>{t('status.loading')}</div>;
  if (state.status === 'error') {
    return (
      <div style={{ padding: '2rem', color: 'var(--error)' }}>
        {t('status.bootFailedPrefix')} {state.flash?.msg ?? t('status.unknownError')}
      </div>
    );
  }

  if (mode === 'chat') {
    return (
      <div className="app-shell chat-mode">
        <TopBar
          dirtyCount={0}
          status={state.status}
          onSave={() => {}}
          mode="chat"
          onModeChange={(m) => setMode(m)}
        />
        <ChatWorkspace
          instanceRoot={meta.instanceRoot}
          providerConfigured={providerConfigured}
          modelOptions={allModels}
          currentModel={meta.currentModel}
        />
      </div>
    );
  }

  // Settings mode — preserves the existing config editor, with all
  // gateway-related props removed. The implementer should open the
  // pre-PR2 App.tsx and copy the `return (...)` block's
  // <SettingsSidebar ... /> and <SettingsPanel ... /> call sites into
  // this branch, then apply these deletions:
  //
  // SettingsSidebar props to DELETE:
  //   instances, selectedKey, descriptors, dirtyInstanceKeys,
  //   onNewInstance, onApply (apply is gateway-only).
  // SettingsPanel props to DELETE:
  //   selectedKey, instance, originalInstance, descriptor,
  //   dirtyGateway, onField, onToggleEnabled, onDelete, onApply.
  // Dispatch actions to DELETE from any remaining wiring:
  //   'instance/create', 'instance/delete', 'edit/field',
  //   'edit/enabled'.
  //
  // Keep everything driving providers, mcp, cron, memory, skills,
  // terminal, storage, agent, auxiliary groups.
  return (
    <div className="app-shell settings-mode">
      <TopBar
        dirtyCount={totalDirtyCount(state)}
        status={state.status}
        onSave={async () => {
          await apiFetch('/api/config', { method: 'PUT', body: { config: state.config } });
        }}
        mode="settings"
        onModeChange={(m) => setMode(m)}
      />
      {/* Paste stripped-down SettingsSidebar + <main>SettingsPanel</main> here */}
      <Footer flash={state.flash} />
    </div>
  );
}
```

When cleaning up `SettingsSidebar` / `SettingsPanel` props, delete all keys tied to `gateway`, `platforms`, `instances`, `dirtyInstanceKeys`, `descriptors`. Retain the slices used by Models, Advanced, Skills, Memory, Terminal, Storage, Agent, Auxiliary, Cron groups.

- [ ] **Step 2: Update `state.ts`**

Remove the `instance/delete`, `instance/create`, and `edit/field` / `edit/enabled` actions (these operated on `config.gateway.platforms` specifically). Remove the `listInstances`, `dirtyCount`, `instanceDirty`, `groupDirty` selectors that read `config.gateway.platforms`.

Retain `edit/config-field`, `edit/config-scalar`, `edit/keyed-instance-field`, `list-instance/*`, `provider/models/loaded`, `shell/selectGroup` etc. — the config editor needs these.

Delete `GROUPS` entries for `gateway`. Update `GroupId` type.

- [ ] **Step 3: Update the config-editor sidebar and settings panel**

In `components/shell/SettingsSidebar.tsx` and `components/shell/SettingsPanel.tsx`, delete any UI that lists gateway instances. Gate the Gateway group header so it no longer renders.

- [ ] **Step 4: Build**

Run: `cd web && pnpm build`
Expected: PASS.

- [ ] **Step 5: Run tests**

Run: `cd web && pnpm test --run`
Expected: most pass; flaky tests that referenced `session` or `gateway` need pruning (next task).

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor(web): App.tsx chat-first; drop gateway slice from state.ts"
```

---

## Task D8: Audit `web/src/shell/` + drop session-only helpers

**Files:**
- Delete or modify: `web/src/shell/*.{ts,tsx,test.ts}`

- [ ] **Step 1: Audit the directory**

Run:
```bash
grep -ln "session\|Session\|sessionId\|gateway\|platforms" web/src/shell/*.ts web/src/shell/*.tsx
```

For each hit, classify:
- **Config editor concern** (groups, sections, first-subkey, hash router for settings) → keep, edit to remove gateway branches.
- **Multi-session UI concern** (summaries, keyedInstances when used by session list, listInstances when used by gateway platforms) → delete.

Concrete decisions from the spec:
- `keyedInstances.ts` and `keyedInstances.test.ts`: if used *only* by gateway and/or session UI → delete. If providers / mcp / memory providers also use it (likely), **keep** and only remove gateway-specific call sites.
- `listInstances.ts`: same. Likely used by fallback_providers / cron jobs list — keep.
- `summaries.tsx`: delete (session summary rendering).
- `groups.ts`: delete the `gateway` entry; keep the file.
- `sections.ts`, `firstSubkey.ts`: keep.
- `hash.ts`: simplify — drop `mode: 'chat'` branch that encoded `sessionId`. Keep the settings hash encoding (`#/settings/<group>/<sub>`).

- [ ] **Step 2: Execute the audit**

Deletes:
```bash
git rm web/src/shell/summaries.tsx web/src/shell/summaries.test.tsx
```

Edits:
- Open `web/src/shell/hash.ts`. Remove the chat sub-mode logic. Simplify `parseHash` to return either `{ mode: 'chat' }` (the default) or `{ mode: 'settings', groupId, sub }`.
- Open `web/src/shell/groups.ts`. Remove the `'gateway'` entry from `GROUPS` and the `GroupId` union.

- [ ] **Step 3: Run tests**

Run: `cd web && pnpm test --run shell/`
Expected: PASS. Delete orphaned tests and any `describe("migrateLegacyHash ...")` block that depended on gateway platform migration.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "chore(web/shell): drop session summaries + gateway branches"
```

---

## Task D9: Strip session-related i18n keys

**Files:**
- Modify: `web/src/locales/en/ui.json`
- Modify: `web/src/locales/zh-CN/ui.json`

- [ ] **Step 1: Inventory and remove**

Run:
```bash
grep -nE '"session|"Session|"chat.settings|"chat.rename|"chat.newConversation|"gateway' web/src/locales/en/ui.json web/src/locales/zh-CN/ui.json
```

Delete every matched key. Add new keys:

```json
"chat": {
  "instance": { "label": "Instance" },
  "modelDropdown": "Model",
  "stop": "Stop",
  "errorBusy": "Another turn is in flight",
  "errorNoProvider": "No LLM provider configured — open Settings to add an API key",
  "errorSendFailed": "Failed to send: {{msg}}"
}
```

Mirror in `zh-CN`.

- [ ] **Step 2: Type check + tests**

Run: `cd web && pnpm test --run && pnpm tsc --noEmit`
Expected: green.

- [ ] **Step 3: Commit**

```bash
git add web/src/locales
git commit -m "refactor(web/i18n): drop session/gateway keys; add instance/model/stop"
```

---

## Phase E — Wrap-up

## Task E1: CHANGELOG entry

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Prepend a 0.3.0 entry**

```markdown
## 0.3.0 — Instance-bound, single-conversation model

### Breaking
- The multi-session model is removed. Each hermind instance is a single
  persistent conversation. The UI no longer has a session sidebar,
  new-chat button, or per-session settings drawer.
- `hermind gateway` and `hermind acp` subcommands are deleted. The
  `gateway/` and related multi-platform bot adapters are gone. Users
  who need a bot framework should pin to the 0.2.x branch.
- HTTP API: all `/api/sessions*` routes are removed. Use:
  - `GET /api/conversation?limit=&offset=` for history
  - `POST /api/conversation/messages` to send a user message
  - `POST /api/conversation/cancel` to stop an in-flight run
  - `GET /api/sse` for the single streaming event source
- `state.db` schema v3: `sessions` table is dropped; `messages` loses
  its `session_id` column. On upgrade, an existing v1 DB is renamed to
  `state.db.v1-backup` (with a unix-ms suffix on collision) and a fresh
  v3 DB is created. **Your message history is preserved in the backup
  but is not migrated into the new schema.**
- Per-session `system_prompt` field is removed. System prompt lives in
  `config.yaml` under `agent.default_system_prompt`.

### Added
- Runtime model dropdown in the conversation header (ephemeral
  override — reloads reset to `config.yaml:model`).
- `GET /api/status` includes `current_model`.
- Cron jobs now run ephemerally: each run gets its own
  `<instance>/trajectories/cron-*.jsonl` and does not pollute the main
  conversation.
```

- [ ] **Step 2: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: CHANGELOG entry for 0.3.0 single-conversation pivot"
```

---

## Task E2: Rebuild `api/webroot/` bundle

**Files:**
- Modify: `api/webroot/*`

- [ ] **Step 1: Clean and rebuild**

```bash
cd web
pnpm install
pnpm build
cd ..
rm -rf api/webroot/*
cp -r web/dist/. api/webroot/
```

- [ ] **Step 2: Commit**

```bash
git add api/webroot
git commit -m "chore(webroot): rebuild embedded frontend bundle for 0.3.0"
```

---

## Task E3: Full verification

- [ ] **Step 1: Backend tests**

Run: `go test ./...`
Expected: all green. Watch for any leftover references to `storage.Session`, `storage.SessionUpdate`, `ListSessions`, `GetSession`, `CreateSession`, `SessionID`, etc.:
```bash
grep -rn "storage\.Session\|SessionID\|storage\.ListSessions\|storage\.CreateSession" --include='*.go' .
```
Fix any remaining hits before moving on.

- [ ] **Step 2: Frontend tests + typecheck**

```bash
cd web
pnpm test --run
pnpm tsc --noEmit
cd ..
```
Expected: green.

- [ ] **Step 3: Smoke test**

```bash
mkdir -p /tmp/purge-smoke
( cd /tmp/purge-smoke && go run ./cmd/hermind web --exit-after 5s --no-browser )
```

Expected output includes:
```
hermind web listening on http://127.0.0.1:3XXXX
instance:  /tmp/purge-smoke/.hermind
```
No token, no legacy session paths, state.db contains only `messages`, `memories`, `conversation_state`, `schema_meta`:
```bash
sqlite3 /tmp/purge-smoke/.hermind/state.db ".tables"
```

- [ ] **Step 4: Upgrade-path smoke test**

```bash
# Reset a scratch instance
rm -rf /tmp/migrate-smoke && mkdir -p /tmp/migrate-smoke
# Fake a v1 DB
sqlite3 /tmp/migrate-smoke/.hermind/state.db < /dev/stdin <<'SQL'
CREATE TABLE sessions (id TEXT PRIMARY KEY);
CREATE TABLE messages (id INTEGER PRIMARY KEY, session_id TEXT);
SQL
# Boot PR2 binary
( cd /tmp/migrate-smoke && go run ./cmd/hermind web --exit-after 3s --no-browser )
ls /tmp/migrate-smoke/.hermind/
```
Expected: `state.db.v1-backup` appears next to a fresh `state.db`; stderr printed the backup notice.

- [ ] **Step 5: Commit tracking**

No code changes. Verify `git status` is clean before opening the PR.

---

## Task E4: Open the PR

- [ ] **Step 1: `gh pr create`**

```bash
gh pr create --title "feat: purge Session abstraction — single conversation per instance" --body "$(cat <<'EOF'
## Summary

Implements the single-conversation redesign from
`docs/superpowers/specs/2026-04-22-hermind-instance-single-session-design.md`
§PR 2. Follows the already-merged PR 1 (`config-dir`).

- Storage schema v3: drops `sessions`, flattens `messages`, introduces
  `conversation_state` singleton. v1 DBs are renamed to
  `state.db.v1-backup` on first boot.
- Deletes `gateway/`, `cli/gatewayctl/`, `cli/gateway.go`,
  `cli/acp.go`, all session-coupled API handlers and the `sessionrun`
  package.
- `agent.Engine.RunConversation` no longer takes SessionID/UserID.
  New `Ephemeral` flag for cron runs.
- HTTP API replaced: `/api/conversation` + `/api/sse`.
- Frontend: single-column `ChatWorkspace`; header is
  `[instance path] [model dropdown] [stop]`; model dropdown is a
  runtime-only override.
- Cron jobs run ephemerally with per-job trajectory files.
- CHANGELOG entry for 0.3.0.

## Test plan
- [ ] `go test ./...` green
- [ ] `cd web && pnpm test --run && pnpm tsc --noEmit` green
- [ ] Smoke: fresh cwd → `.hermind/state.db` contains only v3 tables
- [ ] Smoke: v1 DB → backed up, fresh v3 DB created, stderr notice printed
- [ ] Smoke: cron job fires → trajectory file written, `GetHistory` unchanged
- [ ] Smoke: runtime model switch → next POST uses the new model;
      reload falls back to `config.yaml:model`
- [ ] `grep -rn 'storage.Session\|handlers_sessions\|sessionrun\|gateway' --include='*.go' .` returns nothing

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## PR 2 Acceptance Checklist

- [ ] `hermind gateway` and `hermind acp` subcommands do not exist.
- [ ] `state.db` contains only `messages`, `conversation_state`, `memories`, `memories_fts`, `messages_fts`, `schema_meta`.
- [ ] `/api/sessions*` routes return 404; `/api/conversation` works.
- [ ] Frontend has no sidebar, no new-chat button, no settings drawer. Header is `[instance path] [model dropdown] [stop]`.
- [ ] Runtime model change affects only the next request; reload reverts to `config.yaml:model`.
- [ ] Cron-triggered prompts do not appear in `GetHistory()`; `trajectories/cron-*.jsonl` is produced.
- [ ] Legacy v1 `state.db` is renamed to `state.db.v1-backup` and a fresh v3 DB is created.
- [ ] CHANGELOG has a 0.3.0 entry covering all breaking changes.
- [ ] `go test ./...` and `cd web && pnpm test --run && pnpm tsc --noEmit` are green.
- [ ] `api/webroot/` is rebuilt from `web/dist/`.
