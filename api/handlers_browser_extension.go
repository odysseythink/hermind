package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/odysseythink/hermind/tool"
)

// ============================================================================
// 1. Browser Extension Scrape (existing — for popup "send current page")
// ============================================================================

type browserExtensionScrapeRequest struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Format    string `json:"format"` // "text" or "html"
	Timestamp int64  `json:"timestamp,omitempty"`
}

type browserExtensionScrapeResponse struct {
	Success bool   `json:"success"`
	ID      string `json:"id"`
	Error   string `json:"error,omitempty"`
}

type browserExtensionCheckResponse struct {
	Connected bool   `json:"connected"`
	Version   string `json:"version"`
	Error     string `json:"error,omitempty"`
}

// browserExtensionItem represents a stored scrape in the index.
type browserExtensionItem struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	Format    string `json:"format"`
	Timestamp int64  `json:"timestamp"`
}

// extensionStorage manages filesystem storage for browser extension content.
type extensionStorage struct {
	root string
	mu   sync.RWMutex
}

func newExtensionStorage(root string) *extensionStorage {
	return &extensionStorage{root: root}
}

func (es *extensionStorage) dir() string         { return filepath.Join(es.root, "browser-extension") }
func (es *extensionStorage) indexPath() string    { return filepath.Join(es.dir(), "index.json") }
func (es *extensionStorage) contentPath(id string) string { return filepath.Join(es.dir(), id+".md") }
func (es *extensionStorage) init() error          { return os.MkdirAll(es.dir(), 0755) }

func (es *extensionStorage) save(req *browserExtensionScrapeRequest) (string, error) {
	if err := es.init(); err != nil {
		return "", err
	}
	id := generateExtensionID()
	if req.Timestamp == 0 {
		req.Timestamp = time.Now().Unix()
	}
	content := fmt.Sprintf("# %s\n\nURL: %s\nDate: %s\n\n---\n\n%s",
		req.Title, req.URL, time.Unix(req.Timestamp, 0).Format(time.RFC3339), req.Content)
	if err := os.WriteFile(es.contentPath(id), []byte(content), 0644); err != nil {
		return "", err
	}
	es.mu.Lock()
	defer es.mu.Unlock()
	items, _ := es.readIndex()
	items = append(items, browserExtensionItem{ID: id, URL: req.URL, Title: req.Title, Format: req.Format, Timestamp: req.Timestamp})
	if len(items) > 100 {
		for _, old := range items[:len(items)-100] {
			_ = os.Remove(es.contentPath(old.ID))
		}
		items = items[len(items)-100:]
	}
	if err := es.writeIndex(items); err != nil {
		return "", err
	}
	return id, nil
}

func (es *extensionStorage) readIndex() ([]browserExtensionItem, error) {
	data, err := os.ReadFile(es.indexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return []browserExtensionItem{}, nil
		}
		return nil, err
	}
	var items []browserExtensionItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (es *extensionStorage) writeIndex(items []browserExtensionItem) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(es.indexPath(), data, 0644)
}

func (es *extensionStorage) list(limit int) ([]browserExtensionItem, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()
	items, err := es.readIndex()
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Timestamp > items[j].Timestamp })
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return items, nil
}

func (es *extensionStorage) read(id string) (string, error) {
	data, err := os.ReadFile(es.contentPath(id))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func generateExtensionID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) extensionStorage() *extensionStorage {
	return newExtensionStorage(s.opts.InstanceRoot)
}

func (s *Server) extensionAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next(w, r)
	}
}

func (s *Server) handleBrowserExtensionCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, browserExtensionCheckResponse{Connected: true, Version: s.opts.Version})
}

func (s *Server) handleBrowserExtensionScrape(w http.ResponseWriter, r *http.Request) {
	var req browserExtensionScrapeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, browserExtensionScrapeResponse{Success: false, Error: "Invalid JSON: " + err.Error()})
		return
	}
	if req.URL == "" {
		writeJSONStatus(w, http.StatusBadRequest, browserExtensionScrapeResponse{Success: false, Error: "url is required"})
		return
	}
	if req.Content == "" {
		writeJSONStatus(w, http.StatusBadRequest, browserExtensionScrapeResponse{Success: false, Error: "content is required"})
		return
	}
	store := s.extensionStorage()
	id, err := store.save(&req)
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, browserExtensionScrapeResponse{Success: false, Error: "Failed to save content: " + err.Error()})
		return
	}
	writeJSON(w, browserExtensionScrapeResponse{Success: true, ID: id})
}

// ============================================================================
// 2. Browser Control — bidirectional command channel
// ============================================================================

// browserTask represents a command sent to the extension.
type browserTask struct {
	ID        string          `json:"id"`
	Action    string          `json:"action"`
	URL       string          `json:"url,omitempty"`
	Selector  string          `json:"selector,omitempty"`
	Value     string          `json:"value,omitempty"`
	Direction string          `json:"direction,omitempty"`
	Amount    int             `json:"amount,omitempty"`
	DurationMs int            `json:"duration_ms,omitempty"`
	TabIndex  int             `json:"tab_index,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// browserTaskResult is what the extension reports back.
type browserTaskResult struct {
	TaskID    string `json:"task_id"`
	Success   bool   `json:"success"`
	Content   string `json:"content,omitempty"`    // text/html/screenshot-base64
	Error     string `json:"error,omitempty"`
	URL       string `json:"url,omitempty"`        // current URL after action
	Title     string `json:"title,omitempty"`      // page title
}

// taskQueue manages pending tasks and their result channels.
type taskQueue struct {
	mu      sync.Mutex
	pending map[string]*pendingTask
}

type pendingTask struct {
	task   browserTask
	result chan browserTaskResult
}

func newTaskQueue() *taskQueue {
	return &taskQueue{pending: make(map[string]*pendingTask)}
}

func (q *taskQueue) enqueue(task browserTask) <-chan browserTaskResult {
	q.mu.Lock()
	defer q.mu.Unlock()
	ch := make(chan browserTaskResult, 1)
	q.pending[task.ID] = &pendingTask{task: task, result: ch}
	return ch
}

func (q *taskQueue) dequeue() *browserTask {
	q.mu.Lock()
	defer q.mu.Unlock()
	for id, pt := range q.pending {
		// Simple FIFO: return the first one found
		_ = id
		t := pt.task
		return &t
	}
	return nil
}

func (q *taskQueue) complete(result browserTaskResult) bool {
	q.mu.Lock()
	pt, ok := q.pending[result.TaskID]
	if ok {
		delete(q.pending, result.TaskID)
	}
	q.mu.Unlock()
	if ok {
		pt.result <- result
		close(pt.result)
		return true
	}
	return false
}

func (q *taskQueue) cancel(taskID string) {
	q.mu.Lock()
	pt, ok := q.pending[taskID]
	if ok {
		delete(q.pending, taskID)
	}
	q.mu.Unlock()
	if ok {
		pt.result <- browserTaskResult{TaskID: taskID, Success: false, Error: "cancelled"}
		close(pt.result)
	}
}

// Global task queue (per-server instance).
// Server uses the package-level defaultTaskQueue for browser_control tasks.

func (s *Server) handleBrowserExtensionPoll(w http.ResponseWriter, r *http.Request) {
	defaultTaskQueue.mu.Lock()
	var task *browserTask
	for _, pt := range defaultTaskQueue.pending {
		t := pt.task
		task = &t
		break
	}
	defaultTaskQueue.mu.Unlock()

	if task == nil {
		writeJSON(w, map[string]interface{}{"empty": true})
		return
	}
	writeJSON(w, task)
}

func (s *Server) handleBrowserExtensionResult(w http.ResponseWriter, r *http.Request) {
	var result browserTaskResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if !defaultTaskQueue.complete(result) {
		writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": "task not found or already completed"})
		return
	}
	writeJSON(w, map[string]bool{"success": true})
}

// ============================================================================
// 3. browser_control tool
// ============================================================================

// BrowserControlSchema is the JSON schema for the browser_control tool.
const BrowserControlSchema = `{
  "type": "object",
  "properties": {
    "action":     { "type": "string", "enum": ["navigate", "click", "fill", "scroll", "screenshot", "extract_text", "extract_html", "wait", "switch_tab", "list_tabs", "close_tab"], "description": "Browser action to perform" },
    "url":        { "type": "string", "description": "URL for navigate action" },
    "selector":   { "type": "string", "description": "CSS selector for click/fill actions" },
    "value":      { "type": "string", "description": "Text to fill into input" },
    "direction":  { "type": "string", "enum": ["up", "down"], "description": "Scroll direction" },
    "amount":     { "type": "integer", "description": "Scroll amount in pixels (default 500)" },
    "duration_ms":{ "type": "integer", "description": "Wait duration in milliseconds (default 2000)" },
    "tab_index":  { "type": "integer", "description": "Tab index for switch_tab/close_tab" }
  },
  "required": ["action"]
}`

type browserControlArgs struct {
	Action     string `json:"action"`
	URL        string `json:"url,omitempty"`
	Selector   string `json:"selector,omitempty"`
	Value      string `json:"value,omitempty"`
	Direction  string `json:"direction,omitempty"`
	Amount     int    `json:"amount,omitempty"`
	DurationMs int    `json:"duration_ms,omitempty"`
	TabIndex   int    `json:"tab_index,omitempty"`
}

// NewBrowserControlHandler creates the handler for browser_control.
func NewBrowserControlHandler(pollURL string, apiKey string) tool.Handler {
	return func(ctx context.Context, args json.RawMessage) (string, error) {
		var a browserControlArgs
		if err := json.Unmarshal(args, &a); err != nil {
			return tool.ToolError(err.Error()), nil
		}

		// Validate action-specific required fields
		switch a.Action {
		case "navigate":
			if a.URL == "" {
				return tool.ToolError("navigate action requires 'url'"), nil
			}
		case "click", "fill":
			if a.Selector == "" {
				return tool.ToolError(a.Action + " action requires 'selector'"), nil
			}
			if a.Action == "fill" && a.Value == "" {
				return tool.ToolError("fill action requires 'value'"), nil
			}
		}

		// Build the task
		task := browserTask{
			ID:        generateExtensionID(),
			Action:    a.Action,
			URL:       a.URL,
			Selector:  a.Selector,
			Value:     a.Value,
			Direction: a.Direction,
			Amount:    a.Amount,
			DurationMs: a.DurationMs,
			TabIndex:  a.TabIndex,
			CreatedAt: time.Now(),
		}

		// For the handler to work, we need access to the Server's taskQueue.
		// Since tool.Handler is a standalone function, we cannot easily access
		// the Server instance. Instead, we use a package-level registry.
		ch := defaultTaskQueue.enqueue(task)

		// Wait for result with context timeout
		select {
		case result := <-ch:
			if !result.Success {
				return tool.ToolError(result.Error), nil
			}
			// Build response with content + metadata
			resp := struct {
				Content string `json:"content,omitempty"`
				URL     string `json:"url,omitempty"`
				Title   string `json:"title,omitempty"`
			}{
				Content: result.Content,
				URL:     result.URL,
				Title:   result.Title,
			}
			return tool.ToolResult(resp), nil
		case <-ctx.Done():
			defaultTaskQueue.cancel(task.ID)
			return tool.ToolError("browser extension did not respond in time (ensure the extension is installed and running)"), nil
		case <-time.After(30 * time.Second):
			defaultTaskQueue.cancel(task.ID)
			return tool.ToolError("browser extension did not respond within 30 seconds (ensure the extension is installed and running)"), nil
		}
	}
}

// Package-level default task queue for the browser_control tool.
// The Server's HTTP handlers also interact with this same queue.
var defaultTaskQueue = newTaskQueue()

// ============================================================================
// 4. browser_extension_read tool (existing)
// ============================================================================

// BrowserExtensionReadSchema is the JSON schema for the browser_extension_read tool.
const BrowserExtensionReadSchema = `{
  "type": "object",
  "properties": {
    "id":      { "type": "string", "description": "Specific document ID to read. If omitted, returns the most recent items." },
    "limit":   { "type": "integer", "description": "Number of recent items to list when id is omitted (default 5, max 20)", "minimum": 1, "maximum": 20 }
  }
}`

type browserExtensionReadArgs struct {
	ID    string `json:"id,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// BrowserExtensionReadItem is a single document.
type BrowserExtensionReadItem struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	Format    string `json:"format"`
	Timestamp int64  `json:"timestamp"`
	Content   string `json:"content,omitempty"`
}

// NewBrowserExtensionReadHandler creates the handler for browser_extension_read.
func NewBrowserExtensionReadHandler(instanceRoot string) tool.Handler {
	return func(_ context.Context, args json.RawMessage) (string, error) {
		var a browserExtensionReadArgs
		if err := json.Unmarshal(args, &a); err != nil {
			return tool.ToolError(err.Error()), nil
		}
		if a.Limit <= 0 {
			a.Limit = 5
		}
		if a.Limit > 20 {
			a.Limit = 20
		}

		store := newExtensionStorage(instanceRoot)

		if a.ID != "" {
			content, err := store.read(a.ID)
			if err != nil {
				return tool.ToolError(fmt.Sprintf("document not found: %s", a.ID)), nil
			}
			items, _ := store.list(0)
			var meta browserExtensionItem
			for _, it := range items {
				if it.ID == a.ID {
					meta = it
					break
				}
			}
			return tool.ToolResult(struct {
				Items []BrowserExtensionReadItem `json:"items"`
			}{
				Items: []BrowserExtensionReadItem{{
					ID:        a.ID,
					URL:       meta.URL,
					Title:     meta.Title,
					Format:    meta.Format,
					Timestamp: meta.Timestamp,
					Content:   content,
				}},
			}), nil
		}

		items, err := store.list(a.Limit)
		if err != nil {
			return tool.ToolError(err.Error()), nil
		}

		result := make([]BrowserExtensionReadItem, 0, len(items))
		for _, it := range items {
			content, _ := store.read(it.ID)
			result = append(result, BrowserExtensionReadItem{
				ID:        it.ID,
				URL:       it.URL,
				Title:     it.Title,
				Format:    it.Format,
				Timestamp: it.Timestamp,
				Content:   content,
			})
		}
		return tool.ToolResult(struct {
			Items []BrowserExtensionReadItem `json:"items"`
		}{Items: result}), nil
	}
}
