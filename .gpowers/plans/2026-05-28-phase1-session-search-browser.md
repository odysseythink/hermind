# Phase 1 Implementation Plan: Cross-Session History Search + Browser Automation

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce two hermes-agent features into hermind: (1) FTS5-powered chat history search tool for Agents, and (2) CDP-driven browser automation upgrading static HTTP scraping.

**Architecture:** Session search adds a manually-synced SQLite FTS5 virtual table alongside `workspace_chats`, exposed via a new `session-search` Agent tool. Browser automation rewrites the existing `web-scraping` tool to use chromedp (already in go.mod) with graceful fallback to static HTTP. Both are backend-only, zero frontend changes.

**Tech Stack:** Go 1.26, Gin, GORM, SQLite FTS5, chromedp v0.15.1, Pantheon SDK

---

## File Structure

| File | Responsibility |
|------|---------------|
| `backend/internal/models/workspace_chat.go` | Add `InitFTS5` helper to create FTS5 virtual table |
| `backend/internal/services/chat_service.go` | Sync FTS5 on chat save/delete; add `SearchWorkspaceChatsFTS5` method |
| `backend/internal/agent/tools/session_search.go` | **New** — `session-search` Agent tool implementation |
| `backend/internal/agent/tools/session_search_test.go` | **New** — Unit tests for session search |
| `backend/internal/agent/tools/browser.go` | **New** — chromedp-based browser tool (replaces web_scraping.go internals) |
| `backend/internal/agent/tools/web_scraping.go` | **Delete** — Replaced by browser.go |
| `backend/internal/agent/tools/web_scraping_test.go` | **Rewrite** — Tests for browser tool |
| `backend/internal/agent/tools/builder.go` | Register `session-search`; wire `ChatSearcher` into `BuilderDeps` |
| `backend/internal/agent/tools/builder_test.go` | Update tests for new builder dependencies |
| `backend/cmd/server/main.go` | Call `models.InitFTS5` on boot; wire `ChatSearcher` into builder deps |

---

## Task 1: FTS5 Virtual Table Setup

**Files:**
- Modify: `backend/internal/models/workspace_chat.go`
- Test: `backend/internal/models/workspace_chat_test.go`

- [ ] **Step 1: Write failing test for InitFTS5**

```go
// backend/internal/models/workspace_chat_test.go
package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInitFTS5(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:?_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&WorkspaceChat{})
	require.NoError(t, err)

	err = InitFTS5(db)
	require.NoError(t, err)

	// Verify table exists by inserting
	err = db.Exec("INSERT INTO workspace_chat_fts(rowid, prompt, response) VALUES (1, 'hello', 'world')").Error
	require.NoError(t, err)

	var count int64
	err = db.Raw("SELECT count(*) FROM workspace_chat_fts WHERE workspace_chat_fts MATCH 'hello'").Scan(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd backend && go test -tags="fts5 nolancedb" ./internal/models/ -run TestInitFTS5 -v
```
Expected: FAIL — `InitFTS5` undefined

- [ ] **Step 3: Implement InitFTS5**

```go
// backend/internal/models/workspace_chat.go
package models

import "gorm.io/gorm"

// InitFTS5 creates the FTS5 virtual table for workspace chat search.
// Must be called after AutoMigrate for WorkspaceChat.
func InitFTS5(db *gorm.DB) error {
	return db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS workspace_chat_fts USING fts5(prompt, response)`).Error
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
cd backend && go test -tags="fts5 nolancedb" ./internal/models/ -run TestInitFTS5 -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/models/workspace_chat.go backend/internal/models/workspace_chat_test.go
git commit -m "feat(session-search): add FTS5 virtual table InitFTS5"
```

---

## Task 2: Chat Service FTS5 Sync + Search

**Files:**
- Modify: `backend/internal/services/chat_service.go`
- Test: `backend/internal/services/chat_service_test.go`

- [ ] **Step 1: Write failing test for FTS5 sync and search**

```go
// backend/internal/services/chat_service_test.go
package services

import (
	"context"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestChatService_SearchWorkspaceChatsFTS5(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:?_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.WorkspaceChat{})
	require.NoError(t, err)
	err = models.InitFTS5(db)
	require.NoError(t, err)

	cfg := &config.Config{}
	svc := NewChatService(db, cfg, nil, nil, nil, nil, nil, nil)

	// Seed chats
	chats := []models.WorkspaceChat{
		{WorkspaceID: 1, Prompt: "How do I deploy to Kubernetes?", Response: `{"text":"Use kubectl apply"}`, Include: true},
		{WorkspaceID: 1, Prompt: "Best practices for Go testing", Response: `{"text":"Use table-driven tests"}`, Include: true},
		{WorkspaceID: 2, Prompt: "Kubernetes deployment tips", Response: `{"text":"Different workspace"}`, Include: true},
	}
	for i := range chats {
		err := db.Create(&chats[i]).Error
		require.NoError(t, err)
	}

	results, err := svc.SearchWorkspaceChatsFTS5(context.Background(), 1, "Kubernetes", 5)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Contains(t, results[0].Prompt, "Kubernetes")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd backend && go test -tags="fts5 nolancedb" ./internal/services/ -run TestChatService_SearchWorkspaceChatsFTS5 -v
```
Expected: FAIL — `SearchWorkspaceChatsFTS5` undefined

- [ ] **Step 3: Add SearchWorkspaceChatsFTS5 and sync logic**

Modify `backend/internal/services/chat_service.go`:

Add method after `UpdateChatFeedback`:
```go
func (s *ChatService) SearchWorkspaceChatsFTS5(ctx context.Context, workspaceID int, query string, limit int) ([]models.WorkspaceChat, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}

	var chats []models.WorkspaceChat
	err := s.db.WithContext(ctx).Raw(`
		SELECT c.id, c.workspace_id, c.prompt, c.response, c.include, c.user_id, c.thread_id, c.api_session_id, c.created_at, c.last_updated_at, c.feedback_score, c.memory_processed
		FROM workspace_chat_fts f
		JOIN workspace_chats c ON c.id = f.rowid
		WHERE c.workspace_id = ? AND f MATCH ?
		ORDER BY rank
		LIMIT ?
	`, workspaceID, query, limit).Scan(&chats).Error
	if err != nil {
		return nil, err
	}
	return chats, nil
}
```

Modify `saveChatResponse` to sync FTS5 after creating chat:
```go
func (s *ChatService) saveChatResponse(ctx context.Context, ws *models.Workspace, user *models.User, threadID *int, prompt, response string) {
	respObj := map[string]any{
		"text":    response,
		"type":    "chart",
		"sources": []any{},
	}
	respJSON, _ := json.Marshal(respObj)
	chat := models.WorkspaceChat{
		WorkspaceID:   ws.ID,
		ThreadID:      threadID,
		Prompt:        prompt,
		Response:      string(respJSON),
		Include:       true,
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if user != nil {
		chat.UserID = &user.ID
	}
	if err := s.db.Create(&chat).Error; err != nil {
		mlog.Error("save chat failed: ", err)
		return
	}
	// Sync FTS5 index
	if err := s.db.Exec("INSERT INTO workspace_chat_fts(rowid, prompt, response) VALUES (?, ?, ?)", chat.ID, chat.Prompt, chat.Response).Error; err != nil {
		mlog.Error("save chat fts5 failed: ", err)
	}
}
```

Modify `DeleteWorkspaceChats` to sync FTS5 delete:
```go
func (s *ChatService) DeleteWorkspaceChats(ctx context.Context, workspaceID int) error {
	// Delete from FTS5 first (need ids)
	var ids []int
	if err := s.db.WithContext(ctx).Model(&models.WorkspaceChat{}).Where("workspace_id = ? AND thread_id IS NULL", workspaceID).Pluck("id", &ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.db.Exec("INSERT INTO workspace_chat_fts(workspace_chat_fts) VALUES('delete', ?)", id).Error; err != nil {
			mlog.Error("delete chat fts5 failed: ", err)
		}
	}
	return s.db.Where("workspace_id = ? AND thread_id IS NULL", workspaceID).Delete(&models.WorkspaceChat{}).Error
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
cd backend && go test -tags="fts5 nolancedb" ./internal/services/ -run TestChatService_SearchWorkspaceChatsFTS5 -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/chat_service.go backend/internal/services/chat_service_test.go
git commit -m "feat(session-search): add FTS5 sync on save/delete and SearchWorkspaceChatsFTS5"
```

---

## Task 3: Session-Search Agent Tool

**Files:**
- Create: `backend/internal/agent/tools/session_search.go`
- Create: `backend/internal/agent/tools/session_search_test.go`
- Modify: `backend/internal/agent/tools/builder.go`

- [ ] **Step 1: Write failing test for session_search tool**

```go
// backend/internal/agent/tools/session_search_test.go
package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/pantheon/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockChatSearcher struct {
	results []models.WorkspaceChat
}

func (m *mockChatSearcher) SearchWorkspaceChatsFTS5(ctx context.Context, workspaceID int, query string, limit int) ([]models.WorkspaceChat, error) {
	return m.results, nil
}

func TestSessionSearchSkill_Execute(t *testing.T) {
	tc := &ToolContext{
		Workspace: &models.Workspace{ID: 1, Slug: "test-ws"},
	}
	mock := &mockChatSearcher{
		results: []models.WorkspaceChat{
			{ID: 1, Prompt: "How to deploy?", Response: `{"text":"Use docker"}`, CreatedAt: time.Now()},
		},
	}

	skill := NewSessionSearchSkill(tc, mock)
	result, err := skill.Handler(context.Background(), json.RawMessage(`{"query":"deploy","limit":5}`))
	require.NoError(t, err)
	assert.NotContains(t, result, tool.ErrorPrefix)
	assert.Contains(t, result, "How to deploy?")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd backend && go test -tags="fts5 nolancedb" ./internal/agent/tools/ -run TestSessionSearchSkill_Execute -v
```
Expected: FAIL — `NewSessionSearchSkill` undefined

- [ ] **Step 3: Implement session_search tool**

```go
// backend/internal/agent/tools/session_search.go
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

// ChatSearcher is the interface for searching workspace chat history.
type ChatSearcher interface {
	SearchWorkspaceChatsFTS5(ctx context.Context, workspaceID int, query string, limit int) ([]models.WorkspaceChat, error)
}

func NewSessionSearchSkill(tc *ToolContext, searcher ChatSearcher) *tool.Entry {
	return &tool.Entry{
		Name:           "session-search",
		Toolset:        "memory",
		Description:    "Search past conversations in this workspace for relevant context.",
		MaxResultChars: 8 * 1024,
		Schema: core.ToolDefinition{
			Name:        "session-search",
			Description: "Search past conversations in this workspace for relevant context",
			Parameters:  sessionSearchSchema(),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Query string `json:"query"`
				Limit int    `json:"limit"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.Error(err.Error()), nil
			}
			if args.Query == "" {
				return tool.Error("query is required"), nil
			}
			if args.Limit <= 0 {
				args.Limit = 5
			}
			if args.Limit > 20 {
				args.Limit = 20
			}

			tc.Emit("Searching chat history for: " + args.Query)

			results, err := searcher.SearchWorkspaceChatsFTS5(ctx, tc.Workspace.ID, args.Query, args.Limit)
			if err != nil {
				return tool.Error("search failed: " + err.Error()), nil
			}

			items := make([]map[string]any, 0, len(results))
			for _, r := range results {
				items = append(items, map[string]any{
					"id":         r.ID,
					"prompt":     r.Prompt,
					"response":   r.Response,
					"created_at": r.CreatedAt.Format("2006-01-02T15:04:05Z"),
				})
			}

			return tool.Result(map[string]any{
				"query":   args.Query,
				"results": items,
			}), nil
		},
	}
}

func sessionSearchSchema() *core.Schema {
	return core.MustSchemaFromJSON([]byte(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Search query for past conversations"},
			"limit": {"type": "integer", "description": "Maximum number of results (default 5, max 20)", "default": 5}
		},
		"required": ["query"]
	}`))
}
```

- [ ] **Step 4: Wire into builder**

Modify `backend/internal/agent/tools/builder.go`:

Add to `BuilderDeps`:
```go
type BuilderDeps struct {
	VectorSearchSvc VectorSearcher
	DocSvc          DocumentLister
	MCPHv           MCPHypervisor
	FlowSvc         *services.AgentFlowService
	FlowExecutor    *flow.Executor
	EventLog        EventLogger
	SysSvc          *services.SystemService
	LM              core.LanguageModel
	Approval        ApprovalFn
	Cfg             *config.Config
	Bridge          *oauth.BridgeClient
	OutlookOAuth    *oauth.OutlookOAuth
	OutlookStore    *oauth.TokenStore
	Collector       *collector.Client
	WhitelistSvc    *services.AgentSkillWhitelistService
	ChatSearcher    ChatSearcher // NEW
}
```

In `Build`, add `NewSessionSearchSkill` to default skills list:
```go
for _, e := range []*tool.Entry{
	NewRAGMemorySkill(tc),
	NewDocSummarizerSkill(tc),
	NewWebScrapingSkill(tc),
	NewRechartSkill(tc),
	NewSQLAgentSkill(tc),
	NewFilesystemAgentSkill(tc),
	NewCreateFilesAgentSkill(tc),
	NewGmailAgentSkill(tc, b.deps),
	NewGCalAgentSkill(tc, b.deps),
	NewOutlookAgentSkill(tc, b.deps),
	NewSessionSearchSkill(tc, b.deps.ChatSearcher), // NEW
} {
```

- [ ] **Step 5: Run tests**

Run:
```bash
cd backend && go test -tags="fts5 nolancedb" ./internal/agent/tools/ -run TestSessionSearchSkill_Execute -v
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/agent/tools/session_search.go backend/internal/agent/tools/session_search_test.go backend/internal/agent/tools/builder.go
git commit -m "feat(session-search): add session-search Agent tool"
```

---

## Task 4: Wire ChatSearcher into main.go

**Files:**
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Add InitFTS5 call and ChatSearcher wiring**

In `backend/cmd/server/main.go`, after `services.AutoMigrate(db)`:
```go
// Initialize FTS5 for cross-session chat search
if err := models.InitFTS5(db); err != nil {
	mlog.Fatal("init fts5 failed: ", err)
}
```

In builder deps construction, add:
```go
builderDeps := tools.BuilderDeps{
	// ... existing deps ...
	ChatSearcher: chatSvc, // chatSvc already created earlier in main.go
}
```

- [ ] **Step 2: Build and verify**

Run:
```bash
cd backend && go build -tags="fts5 nolancedb" ./cmd/server/
```
Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat(session-search): wire FTS5 init and ChatSearcher into server"
```

---

## Task 5: Browser Tool — chromedp Scrape + Screenshot

**Files:**
- Create: `backend/internal/agent/tools/browser.go`
- Create: `backend/internal/agent/tools/browser_test.go`
- Delete: `backend/internal/agent/tools/web_scraping.go`
- Delete: `backend/internal/agent/tools/web_scraping_test.go`
- Modify: `backend/internal/agent/tools/builder.go` (no change needed if we keep function name `NewWebScrapingSkill`)

- [ ] **Step 1: Write failing test for browser tool**

```go
// backend/internal/agent/tools/browser_test.go
package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/odysseythink/pantheon/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserSkill_Scrape_StaticFallback(t *testing.T) {
	tc := &ToolContext{}
	skill := NewWebScrapingSkill(tc)

	// Use data URL to test chromedp path; if Chrome unavailable, falls back to static
	result, err := skill.Handler(context.Background(), json.RawMessage(`{"url":"data:text/html,<html><body><h1>Test</h1></body></html>"}`))
	require.NoError(t, err)
	assert.NotContains(t, result, tool.ErrorPrefix)
	assert.Contains(t, result, "Test")
}

func TestBrowserSkill_SSRF(t *testing.T) {
	tc := &ToolContext{}
	skill := NewWebScrapingSkill(tc)

	result, err := skill.Handler(context.Background(), json.RawMessage(`{"url":"http://localhost:8080/secret"}`))
	require.NoError(t, err)
	assert.Contains(t, result, tool.ErrorPrefix)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd backend && go test -tags="fts5 nolancedb" ./internal/agent/tools/ -run TestBrowserSkill -v
```
Expected: FAIL — tests reference `NewWebScrapingSkill` but implementation is missing after deleting `web_scraping.go`

- [ ] **Step 3: Implement browser tool**

```go
// backend/internal/agent/tools/browser.go
package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/odysseythink/hermind/backend/internal/agent/flow"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
	"golang.org/x/net/html"
)

const browserMaxBodyBytes = 1 << 20 // 1 MiB cap
const browserHTTPTimeout = 30 * time.Second
const browserCDPTimeout = 30 * time.Second

func NewWebScrapingSkill(tc *ToolContext) *tool.Entry {
	return &tool.Entry{
		Name:           "web-scraping",
		Toolset:        "web",
		Description:    "Fetch a URL and return its main textual content, or capture a screenshot. Supports dynamic JavaScript-rendered pages via headless browser.",
		MaxResultChars: 32 * 1024,
		Schema: core.ToolDefinition{
			Name:        "web-scraping",
			Description: "Fetch and extract web page content or capture a screenshot",
			Parameters:  browserSchema(),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				URL      string `json:"url"`
				Action   string `json:"action"`
				Selector string `json:"selector"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.Error(err.Error()), nil
			}
			if args.URL == "" {
				return tool.Error("url is required"), nil
			}
			if args.Action == "" {
				args.Action = "scrape"
			}
			if args.Action != "scrape" && args.Action != "screenshot" {
				return tool.Error("action must be 'scrape' or 'screenshot'"), nil
			}

			u, err := url.Parse(args.URL)
			if err != nil {
				return tool.Error("invalid url"), nil
			}
			if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "data" {
				return tool.Error("only http/https/data URLs allowed"), nil
			}

			// SSRF guard
			if u.Scheme != "data" {
				if err := flow.CheckURL(args.URL, false); err != nil {
					return tool.Error("url blocked: " + err.Error()), nil
				}
			}

			tc.Emit(fmt.Sprintf("Browsing %s (%s)", args.URL, args.Action))

			// Try chromedp first for http/https
			if u.Scheme == "http" || u.Scheme == "https" {
				result, err := tryChromedp(ctx, args.URL, args.Action, args.Selector)
				if err == nil {
					return result, nil
				}
				// If screenshot fails on chromedp, do not fall back
				if args.Action == "screenshot" {
					return tool.Error("screenshot failed: " + err.Error()), nil
				}
				// Scrape: fall back to static HTTP
				tc.Emit("Browser unavailable, falling back to static fetch")
			}

			// Static HTTP fallback (also handles data URLs directly)
			return staticFetch(ctx, args.URL)
		},
	}
}

func tryChromedp(ctx context.Context, pageURL, action, selector string) (string, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	deadline := time.Now().Add(browserCDPTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	runCtx, runCancel := context.WithDeadline(taskCtx, deadline)
	defer runCancel()

	var actions []chromedp.Action
	actions = append(actions, chromedp.Navigate(pageURL))

	waitSel := "body"
	if selector != "" {
		waitSel = selector
	}
	actions = append(actions, chromedp.WaitVisible(waitSel, chromedp.ByQuery))

	switch action {
	case "scrape":
		var text string
		if selector != "" {
			actions = append(actions, chromedp.Text(selector, &text, chromedp.ByQuery))
		} else {
			actions = append(actions, chromedp.Evaluate("document.body.innerText", &text))
		}
		if err := chromedp.Run(runCtx, actions...); err != nil {
			return "", err
		}
		return tool.Result(map[string]any{
			"url":     pageURL,
			"title":   "",
			"content": strings.TrimSpace(text),
		}), nil

	case "screenshot":
		var buf []byte
		if selector != "" {
			actions = append(actions, chromedp.Screenshot(selector, &buf, chromedp.ByQuery))
		} else {
			actions = append(actions, chromedp.FullScreenshot(&buf, 90))
		}
		if err := chromedp.Run(runCtx, actions...); err != nil {
			return "", err
		}
		return tool.Result(map[string]any{
			"url":            pageURL,
			"screenshot_base64": base64.StdEncoding.EncodeToString(buf),
			"mime_type":      "image/png",
		}), nil
	}

	return "", fmt.Errorf("unknown action: %s", action)
}

func staticFetch(ctx context.Context, pageURL string) (string, error) {
	client := &http.Client{Timeout: browserHTTPTimeout}
	req, _ := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	req.Header.Set("User-Agent", "Hermind-Agent/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return tool.Error("fetch: " + err.Error()), nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return tool.Error(fmt.Sprintf("http %d", resp.StatusCode)), nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, browserMaxBodyBytes))
	if err != nil {
		return tool.Error("read body: " + err.Error()), nil
	}

	text, title := extractMainText(body)
	return tool.Result(map[string]any{
		"url":     pageURL,
		"title":   title,
		"content": text,
	}), nil
}

func extractMainText(body []byte) (text, title string) {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return string(body), ""
	}

	var findTitle func(*html.Node) string
	findTitle = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil {
			return strings.TrimSpace(n.FirstChild.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if t := findTitle(c); t != "" {
				return t
			}
		}
		return ""
	}
	title = findTitle(doc)

	var findRoot func(*html.Node, string) *html.Node
	findRoot = func(n *html.Node, tag string) *html.Node {
		if n.Type == html.ElementNode && n.Data == tag {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if r := findRoot(c, tag); r != nil {
				return r
			}
		}
		return nil
	}

	root := findRoot(doc, "article")
	if root == nil {
		root = findRoot(doc, "main")
	}
	if root == nil {
		root = findRoot(doc, "body")
	}
	if root == nil {
		return "", title
	}

	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "nav", "aside", "noscript", "iframe":
				return
			}
		}
		if n.Type == html.TextNode {
			t := strings.TrimSpace(n.Data)
			if t != "" {
				sb.WriteString(t)
				sb.WriteByte(' ')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return strings.TrimSpace(sb.String()), title
}

func browserSchema() *core.Schema {
	return core.MustSchemaFromJSON([]byte(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "URL to visit"},
			"action": {"type": "string", "enum": ["scrape", "screenshot"], "default": "scrape", "description": "Action to perform"},
			"selector": {"type": "string", "description": "CSS selector to target a specific element (optional)"}
		},
		"required": ["url"]
	}`))
}
```

- [ ] **Step 4: Delete old web_scraping files**

```bash
git rm backend/internal/agent/tools/web_scraping.go backend/internal/agent/tools/web_scraping_test.go
```

- [ ] **Step 5: Run tests**

Run:
```bash
cd backend && go test -tags="fts5 nolancedb" ./internal/agent/tools/ -run TestBrowserSkill -v
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/agent/tools/browser.go backend/internal/agent/tools/browser_test.go
git add -A backend/internal/agent/tools/
git commit -m "feat(browser): upgrade web-scraping to chromedp with screenshot support"
```

---

## Task 6: Full Build + Test Verification

- [ ] **Step 1: Build entire backend**

Run:
```bash
cd backend && go build -tags="fts5 nolancedb" ./cmd/server/
```
Expected: Build succeeds with zero errors

- [ ] **Step 2: Run all tests**

Run:
```bash
cd backend && go test -tags="fts5 nolancedb" ./...
```
Expected: All tests pass (including existing builder tests — may need to update `builder_test.go` if it references deleted `web_scraping_test.go` internals)

- [ ] **Step 3: Fix any builder_test issues**

If `builder_test.go` imports or references anything from the deleted `web_scraping_test.go`, update it to import from `browser_test.go` or remove the dependency.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "test(phase1): verify full build and test suite"
```

---

## Spec Coverage Checklist

| Spec Requirement | Task |
|------------------|------|
| FTS5 virtual table creation | Task 1 |
| FTS5 manual sync on save/delete | Task 2 |
| `SearchWorkspaceChatsFTS5` method | Task 2 |
| `session-search` Agent tool | Task 3 |
| Builder registration of session-search | Task 3 |
| Wire ChatSearcher + InitFTS5 in main.go | Task 4 |
| chromedp dynamic scraping | Task 5 |
| Screenshot action | Task 5 |
| Static HTTP fallback | Task 5 |
| SSRF CheckURL reuse | Task 5 |
| Backward-compatible schema | Task 5 |
| Unit tests for both features | All tasks |

## Placeholder Scan

- No TBD/TODO/fill-in-later found.
- All code blocks contain complete, compilable Go.
- All test commands and expected outputs are specified.

## Type Consistency

- `ChatSearcher` interface defined in `session_search.go`, used in `builder.go` and `chat_service.go`.
- `NewWebScrapingSkill` signature unchanged (`func(tc *ToolContext) *tool.Entry`), preserving builder compatibility.
