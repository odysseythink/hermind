# Phase 1 Design: Cross-Session History Search + Browser Automation

## Overview

This document specifies two independent features to be introduced into hermind from hermes-agent:
1. **Cross-Session History Search** (`session-search`) — FTS5-powered chat history retrieval within a workspace.
2. **Browser Automation** — Upgrade static HTTP scraping to Chrome DevTools Protocol (CDP) with fallback.

Both features are backend-only and introduce no frontend changes.

---

## Feature 1: Cross-Session History Search

### Problem

`RAGMemory` searches **documents**, not **past conversations**. When a user asks "what did I ask about last week?" or "summarize my previous analysis", the Agent has no access to chat history beyond the current in-context window (`maxRounds=50`).

### Solution

Use SQLite FTS5 (already enabled via `fts5` build tag) to index `WorkspaceChat` records. Expose a new Agent tool `session-search` that performs full-text queries and returns relevant past exchanges.

### Architecture

```
User/Agent
    │
    ▼
session-search tool (query, limit)
    │
    ▼
FTS5 MATCH query
    │
    ▼
JOIN workspace_chats ON workspace_chat_fts.rowid = workspace_chats.id
WHERE workspace_id = ? AND workspace_chat_fts MATCH ?
ORDER BY rank
LIMIT ?
    │
    ▼
Return [{id, prompt, response, created_at}]
```

### FTS5 Schema

```sql
CREATE VIRTUAL TABLE IF NOT EXISTS workspace_chat_fts USING fts5(prompt, response);
```

- `rowid` is manually set to `workspace_chats.id` on INSERT.
- No `content=` option — manual sync is simpler and avoids GORM AutoMigrate issues with virtual tables.
- No `UNINDEXED` columns needed because workspace filtering happens in the JOIN clause after FTS5 ranking.

### Sync Strategy

| Operation | FTS5 Action |
|-----------|-------------|
| `saveChatResponse` (new chat) | `INSERT INTO workspace_chat_fts(rowid, prompt, response) VALUES (?, ?, ?)` |
| `DeleteWorkspaceChats` | `INSERT INTO workspace_chat_fts(workspace_chat_fts) VALUES('delete', ?)` for each deleted id |
| `UpdateChat` (response changed) | Out of scope for Phase 1 — chat edits are rare and FTS5 stale data is acceptable |

### Agent Tool

**Name**: `session-search`
**Toolset**: `memory`
**Description**: "Search past conversations in this workspace for relevant context."
**Parameters**:
- `query` (string, required): Natural language or keyword search query.
- `limit` (int, optional, default 5, max 20): Maximum number of results.

**Returns**:
```json
{
  "results": [
    {"id": 123, "prompt": "...", "response": "...", "created_at": "2026-05-20T10:00:00Z"}
  ]
}
```

### Implementation Files

| File | Change |
|------|--------|
| `backend/internal/models/workspace_chat.go` | Add `InitFTS5(db *gorm.DB)` helper to create virtual table |
| `backend/internal/services/chat_service.go` | `saveChatResponse`: sync insert into FTS5. `DeleteWorkspaceChats`: sync delete from FTS5. |
| `backend/internal/agent/tools/session_search.go` | **New** — tool implementation |
| `backend/internal/agent/tools/builder.go` | Register `NewSessionSearchSkill(tc)` in default skills list |
| `backend/internal/agent/tools/session_search_test.go` | **New** — unit tests |

### Performance Considerations

- FTS5 `MATCH` + `JOIN` + `workspace_id` filter is O(FTS index scan) + O(JOIN). For typical workspace chat volumes (<10K records), query time is <10ms.
- No additional memory overhead — FTS5 stores its index on disk.

---

## Feature 2: Browser Automation

### Problem

The existing `web-scraping` tool performs **static HTTP fetching** and HTML parsing. It fails on:
- Single-page applications (React/Vue sites requiring JS execution)
- Content behind login walls (cannot execute JS or maintain cookies)
- Lazy-loaded or dynamically rendered content

### Solution

Upgrade the `web-scraping` tool to use **chromedp** (already in `go.mod` at `v0.15.1`) for dynamic page rendering, with automatic fallback to static HTTP scraping when Chrome is unavailable.

### Architecture

```
Agent calls web-scraping(url, action, selector)
    │
    ▼
SSRF CheckURL(url, allowPrivate=false)
    │
    ▼
Try chromedp:
  - Allocate headless browser context (30s timeout)
  - Navigate to URL
  - Wait for body visible
  - Extract text OR capture screenshot
    │
┌───┴───┐
│Success│  Failure (Chrome not installed / timeout)
▼       ▼
Return  Fallback to static HTTP (existing logic)
```

### Schema (Backward Compatible)

```json
{
  "type": "object",
  "properties": {
    "url": {"type": "string", "description": "URL to visit"},
    "action": {
      "type": "string",
      "enum": ["scrape", "screenshot"],
      "default": "scrape",
      "description": "Action to perform"
    },
    "selector": {
      "type": "string",
      "description": "CSS selector to target a specific element (optional)"
    }
  },
  "required": ["url"]
}
```

- Existing calls with only `url` continue to work (default `action=scrape`).
- `action=scrape` returns `{url, title, content}` (same shape as before).
- `action=screenshot` returns `{url, screenshot_base64, mime_type: "image/png"}`.
- `selector` restricts scraping/screenshot to a specific DOM element.

### Fallback Strategy

1. **chromedp path**: Attempt to launch headless Chrome and render the page.
2. **On failure** (Chrome binary not found, timeout, crash): Log warning and fallback to static HTTP fetch + HTML text extraction (existing `extractMainText` logic).
3. **Screenshot failure**: If `action=screenshot` and chromedp fails, return error — static fetch cannot produce screenshots.

### SSRF Protection

Reuse `agent/flow/ssrf_guard.go:CheckURL(rawURL, allowPrivate)`:
- Reject non-http/https schemes.
- Reject `localhost`.
- Resolve hostname and reject private IP ranges (RFC1918, loopback, link-local).

### chromedp Integration

- **Independent context per call**: Each tool invocation creates its own `chromedp.Context` with a 30-second deadline. No shared browser allocator to avoid resource leaks in long-running Agent sessions.
- **No dependency on collector chromedp adapter**: While `collector/external/chromedp.go` exists, the Agent tool uses its own lightweight chromedp setup to avoid coupling with collector lifecycle and to support per-call customization (e.g., viewport size for screenshots).
- **Resource cleanup**: `defer cancel()` on both allocator and task contexts.

### Implementation Files

| File | Change |
|------|--------|
| `backend/internal/agent/tools/web_scraping.go` | **Rewrite** — chromedp scrape/screenshot + static fallback |
| `backend/internal/agent/tools/web_scraping_test.go` | **Rewrite** — test chromedp path (with data URL) and fallback path |
| `backend/internal/agent/tools/builder.go` | No change — `NewWebScrapingSkill` signature unchanged |

### Deployment Considerations

- **Docker**: Add `chromium` package to Dockerfile for headless browser support.
- **macOS/Linux dev**: Requires Chrome or Chromium installed. If absent, graceful fallback to static scraping.
- **Screenshot storage**: Screenshots are returned as base64 inline in tool result. No persistent storage needed.

---

## Testing Strategy

### Unit Tests

| Feature | Test |
|---------|------|
| session-search | `TestSessionSearchSkill_Schema` — validates tool schema |
| session-search | `TestSessionSearchSkill_Execute` — mock DB search, assert result shape |
| browser | `TestBrowserSkill_Scrape_StaticFallback` — without Chrome, falls back to HTTP |
| browser | `TestBrowserSkill_Scrape_Chromedp` — with data URL, extracts text via chromedp |
| browser | `TestBrowserSkill_Screenshot` — with data URL, captures base64 PNG |
| browser | `TestBrowserSkill_SSRF` — asserts private URLs are rejected |

### Integration Tests

- `TestSessionSearchFTS5_EndToEnd`: Create workspace chats → search → assert ranked results.
- `TestBrowserChromedp_RealURL`: Optional test against `example.com` (skipped in CI if Chrome unavailable).

---

## Dependencies

| Package | Status | Note |
|---------|--------|------|
| `github.com/chromedp/chromedp` | Already in `go.mod` (`v0.15.1`) | No new dependency |
| `github.com/chromedp/cdproto` | Already in `go.mod` (indirect) | Will become direct |
| SQLite FTS5 | Already enabled | `fts5` build tag required |

---

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Chrome not installed in production | Graceful fallback to static HTTP for `scrape`; screenshot returns error |
| FTS5 virtual table breaks GORM AutoMigrate | Create FTS5 table via raw SQL outside GORM AutoMigrate |
| chromedp memory leak in long Agent sessions | Per-call independent context with strict 30s timeout and defer cancel |
| Tool schema change breaks existing flows | `action` has default `"scrape"`; existing calls with only `url` unchanged |

---

## Out of Scope

The following are identified as future enhancements, not part of Phase 1:

- **Session search**: Cross-workspace search, LLM-based result summarization, automatic injection into system prompt (without tool call).
- **Browser**: Cookie persistence, form filling, multi-step navigation, click/type actions, PDF rendering.
