package memprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/hermind/tool/embedding"
)

// MetaClaw is a Provider that extracts typed memories (episodic, semantic,
// preference) from conversation turns and indexes them with optional vector
// embeddings. It registers metaclaw_remember and metaclaw_recall tools.
type MetaClaw struct {
	store     storage.Storage
	llm       provider.Provider
	embedder  embedding.Embedder
	sessionID string
}

// NewMetaClaw constructs a MetaClaw provider.
func NewMetaClaw(store storage.Storage, llm provider.Provider, emb embedding.Embedder) *MetaClaw {
	return &MetaClaw{
		store:    store,
		llm:      llm,
		embedder: emb,
	}
}

func (mc *MetaClaw) Name() string { return "metaclaw" }

func (mc *MetaClaw) Initialize(ctx context.Context, sessionID string) error {
	if mc.store == nil {
		return fmt.Errorf("metaclaw: storage is required")
	}
	mc.sessionID = sessionID
	return nil
}

func (mc *MetaClaw) Shutdown(ctx context.Context) error { return nil }

// SyncTurn extracts memories from the conversation turn using the LLM,
// if available. If the LLM is nil or the extraction fails, this is a no-op.
func (mc *MetaClaw) SyncTurn(ctx context.Context, userMsg, assistantMsg string) error {
	if mc.llm == nil {
		return nil
	}

	memories, err := mc.extractMemories(ctx, userMsg, assistantMsg)
	if err != nil {
		// Best-effort: extraction failure is not fatal
		return nil
	}

	for _, mem := range memories {
		if err := mc.saveMemory(ctx, mem.Content, mem.MemType); err != nil {
			// Continue saving other memories even if one fails
			continue
		}
	}

	return nil
}

// extractMemories calls the LLM to extract typed memories from a conversation turn.
// Returns a list of memory objects with Content and MemType fields.
func (mc *MetaClaw) extractMemories(ctx context.Context, userMsg, assistantMsg string) ([]*storage.Memory, error) {
	prompt := fmt.Sprintf(`You are a memory extraction assistant. Given the following conversation turn, extract up to 3 distinct memories worth preserving for future context.

Each memory must be one of:
- "episodic": a specific event or action that occurred
- "semantic": a fact or piece of knowledge
- "preference": a user preference or working style

Reply ONLY with a JSON array. Each item: {"content": "...", "type": "episodic|semantic|preference"}.
If there is nothing worth remembering, reply with an empty array [].

User: %s
Assistant: %s`, userMsg, assistantMsg)

	req := &provider.Request{
		SystemPrompt: "You are a memory extraction assistant.",
		Messages: []message.Message{
			{
				Role:    message.RoleUser,
				Content: message.TextContent(prompt),
			},
		},
	}

	resp, err := mc.llm.Complete(ctx, req)
	if err != nil {
		return nil, err
	}

	text := resp.Message.Content.Text()

	// Strip markdown code fences if present
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimSuffix(text, "```")
	} else if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
	}
	text = strings.TrimSpace(text)

	var items []struct {
		Content string `json:"content"`
		Type    string `json:"type"`
	}
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		return nil, err
	}

	var out []*storage.Memory
	for _, item := range items {
		if strings.TrimSpace(item.Content) == "" {
			continue
		}
		// Normalize type to lowercase
		memType := strings.ToLower(strings.TrimSpace(item.Type))
		out = append(out, &storage.Memory{
			Content: item.Content,
			MemType: memType,
		})
	}

	return out, nil
}

// saveMemory persists a memory to storage, optionally embedding it.
func (mc *MetaClaw) saveMemory(ctx context.Context, content, memType string) error {
	now := time.Now().UTC()
	id := fmt.Sprintf("mc_%d", now.UnixNano())

	mem := &storage.Memory{
		ID:        id,
		Content:   content,
		MemType:   memType,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Embed the content if an embedder is available
	if mc.embedder != nil {
		vec, err := mc.embedder.Embed(ctx, content)
		if err == nil && len(vec) > 0 {
			encoded, err := embedding.EncodeVector(vec)
			if err == nil {
				mem.Vector = encoded
			}
		}
	}

	return mc.store.SaveMemory(ctx, mem)
}

// Recall retrieves memories matching a query, optionally reranked by vector similarity.
func (mc *MetaClaw) Recall(ctx context.Context, query string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 5
	}

	opts := &storage.MemorySearchOptions{Limit: limit}

	// If an embedder is available, embed the query for vector reranking
	if mc.embedder != nil {
		vec, err := mc.embedder.Embed(ctx, query)
		if err == nil && len(vec) > 0 {
			opts.QueryVector = vec
		}
	}

	mems, err := mc.store.SearchMemories(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(mems))
	for _, m := range mems {
		out = append(out, m.Content)
	}
	return out, nil
}

// RegisterTools registers metaclaw_remember and metaclaw_recall tools.
func (mc *MetaClaw) RegisterTools(reg *tool.Registry) {
	reg.Register(&tool.Entry{
		Name:        "metaclaw_remember",
		Toolset:     "memory",
		Description: "Store a typed memory in the MetaClaw memory store.",
		Emoji:       "🧠",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "metaclaw_remember",
				Description: "Store a fact or event in the MetaClaw memory store with automatic type classification.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{
    "content":{"type":"string"},
    "type":{"type":"string","enum":["episodic","semantic","preference"]}
  },
  "required":["content","type"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Content string `json:"content"`
				Type    string `json:"type"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if strings.TrimSpace(args.Content) == "" {
				return tool.ToolError("content is required"), nil
			}
			if err := mc.saveMemory(ctx, args.Content, args.Type); err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"ok": true}), nil
		},
	})

	reg.Register(&tool.Entry{
		Name:        "metaclaw_recall",
		Toolset:     "memory",
		Description: "Recall memories from the MetaClaw store.",
		Emoji:       "🔍",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "metaclaw_recall",
				Description: "Search MetaClaw memories by semantic query (with optional vector reranking).",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{
    "query":{"type":"string"},
    "limit":{"type":"number"}
  },
  "required":["query"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Query string `json:"query"`
				Limit int    `json:"limit,omitempty"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			results, err := mc.Recall(ctx, args.Query, args.Limit)
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{"results": results}), nil
		},
	})
}
