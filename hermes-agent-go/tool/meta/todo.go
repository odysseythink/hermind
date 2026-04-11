// Package meta provides "agent meta" tools — ones that change how
// the model manages its own work: todo list, clarify, checkpoint,
// session search, send_message, approval.
package meta

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nousresearch/hermes-agent/tool"
)

// TodoItem is one entry in the in-session todo list.
type TodoItem struct {
	ID        int       `json:"id"`
	Text      string    `json:"text"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
}

// TodoList is an in-memory list keyed by session. It is deliberately
// simple — Plan 18 and Plan 11c will move persistence into SQLite
// and per-profile.
type TodoList struct {
	mu    sync.Mutex
	next  int
	items map[int]*TodoItem
}

func NewTodoList() *TodoList {
	return &TodoList{items: make(map[int]*TodoItem)}
}

func (l *TodoList) Add(text string) *TodoItem {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.next++
	item := &TodoItem{ID: l.next, Text: text, CreatedAt: time.Now().UTC()}
	l.items[item.ID] = item
	return item
}

func (l *TodoList) Complete(id int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if it, ok := l.items[id]; ok {
		it.Done = true
		return true
	}
	return false
}

func (l *TodoList) Remove(id int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.items[id]; ok {
		delete(l.items, id)
		return true
	}
	return false
}

func (l *TodoList) All() []*TodoItem {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]*TodoItem, 0, len(l.items))
	for _, it := range l.items {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// RegisterTodo wires the todo_* handlers into reg.
func RegisterTodo(reg *tool.Registry, list *TodoList) {
	reg.Register(&tool.Entry{
		Name:        "todo_add",
		Toolset:     "meta",
		Description: "Add an item to the session todo list.",
		Emoji:       "➕",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name: "todo_add",
				Description: "Append a todo item.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{"text":{"type":"string"}},
  "required":["text"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct{ Text string `json:"text"` }
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Text) == "" {
				return tool.ToolError("text is required"), nil
			}
			item := list.Add(args.Text)
			return tool.ToolResult(map[string]any{"id": item.ID}), nil
		},
	})

	reg.Register(&tool.Entry{
		Name:        "todo_list",
		Toolset:     "meta",
		Description: "List the current session todos.",
		Emoji:       "📝",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name: "todo_list",
				Description: "List todos.",
				Parameters: json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			return tool.ToolResult(map[string]any{"items": list.All()}), nil
		},
	})

	reg.Register(&tool.Entry{
		Name:        "todo_done",
		Toolset:     "meta",
		Description: "Mark a todo done.",
		Emoji:       "✅",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name: "todo_done",
				Description: "Mark the todo with the given id as done.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{"id":{"type":"number"}},
  "required":["id"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct{ ID int `json:"id"` }
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if !list.Complete(args.ID) {
				return tool.ToolError("unknown todo id"), nil
			}
			return tool.ToolResult(map[string]any{"ok": true}), nil
		},
	})
}
