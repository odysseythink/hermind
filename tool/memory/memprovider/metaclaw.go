package memprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/hermind/tool/embedding"
	"github.com/odysseythink/hermind/tool/memory/memprovider/citesink"
)

// MetaClaw is a Provider that extracts typed memories (episodic, semantic,
// preference) from conversation turns and indexes them with optional vector
// embeddings. It registers metaclaw_remember and metaclaw_recall tools.
type MetaClaw struct {
	store     storage.Storage
	llm       provider.Provider
	embedder  embedding.Embedder
	sessionID string
	skillsCfg *config.SkillsConfig

	mu           sync.Mutex
	recentBuf    []TurnPair
	recentCap    int
	sinceRefresh int
	summaryEvery int // 0 disables refresh; Task 8 activates it
}

// TurnPair is one user/assistant exchange kept in MetaClaw's rolling
// buffer for working-summary generation.
type TurnPair struct {
	User      string
	Assistant string
	Timestamp time.Time
}

// NewMetaClaw constructs a MetaClaw provider.
func NewMetaClaw(store storage.Storage, llm provider.Provider, emb embedding.Embedder, skillsCfg *config.SkillsConfig) *MetaClaw {
	return &MetaClaw{
		store:     store,
		llm:       llm,
		embedder:  emb,
		skillsCfg: skillsCfg,
		recentCap: 20,
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

func (mc *MetaClaw) Shutdown(ctx context.Context) error {
	if mc.store == nil {
		return nil
	}
	_, _ = Consolidate(ctx, mc.store, nil)
	return nil
}

// SyncTurn extracts memories from the conversation turn using the LLM,
// if available. If the LLM is nil or the extraction fails, this is a no-op.
func (mc *MetaClaw) SyncTurn(ctx context.Context, userMsg, assistantMsg string) error {
	mc.mu.Lock()
	mc.recentBuf = append(mc.recentBuf, TurnPair{
		User: userMsg, Assistant: assistantMsg, Timestamp: time.Now().UTC(),
	})
	if len(mc.recentBuf) > mc.recentCap {
		mc.recentBuf = mc.recentBuf[len(mc.recentBuf)-mc.recentCap:]
	}
	mc.sinceRefresh++
	mc.mu.Unlock()

	// Check if working summary refresh is triggered
	mc.mu.Lock()
	trigger := mc.summaryEvery > 0 && mc.sinceRefresh >= mc.summaryEvery
	if trigger {
		mc.sinceRefresh = 0
	}
	mc.mu.Unlock()
	if trigger {
		go mc.refreshWorkingSummary(context.Background())
	}

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

// Recall retrieves memories matching a query, returning typed snippets
// with stable IDs so the feedback loop can credit specific entries.
// The working_summary row (if present and active) is always placed in
// slot 0; the remaining limit-1 slots come from hybrid search.
func (mc *MetaClaw) Recall(ctx context.Context, query string, limit int) ([]InjectedMemory, error) {
	if limit <= 0 {
		limit = 5
	}

	out := make([]InjectedMemory, 0, limit)

	if ws, err := mc.store.GetMemory(ctx, "working_summary"); err == nil &&
		ws != nil && (ws.Status == "" || ws.Status == storage.MemoryStatusActive) {
		out = append(out, InjectedMemory{ID: ws.ID, Content: ws.Content})
		limit--
	}

	if limit <= 0 {
		return out, nil
	}

	// Fetch the current skills generation seq for decay calculation
	currentSeq := int64(0)
	if gen, err := mc.store.GetSkillsGeneration(ctx); err == nil && gen != nil {
		currentSeq = gen.Seq
	}

	// Determine the half-life to apply. Default to 5 if config is nil or zero;
	// explicitly set 0 disables decay (per storage layer contract).
	halfLife := 5
	if mc.skillsCfg != nil && mc.skillsCfg.GenerationHalfLife > 0 {
		halfLife = mc.skillsCfg.GenerationHalfLife
	}

	opts := &storage.MemorySearchOptions{
		Limit:              limit,
		CurrentSkillsSeq:   currentSeq,
		GenerationHalfLife: halfLife,
	}
	if mc.embedder != nil {
		vec, err := mc.embedder.Embed(ctx, query)
		if err == nil && len(vec) > 0 {
			opts.QueryVector = vec
		}
	}

	mems, err := mc.store.SearchMemories(ctx, query, opts)
	if err != nil {
		return out, err
	}
	for _, m := range mems {
		if m.ID == "working_summary" {
			continue // already prepended
		}
		out = append(out, InjectedMemory{ID: m.ID, Content: m.Content})
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
			recalled, err := mc.Recall(ctx, args.Query, args.Limit)
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			texts := make([]string, 0, len(recalled))
			for _, r := range recalled {
				texts = append(texts, r.Content)
			}
			return tool.ToolResult(map[string]any{"results": texts}), nil
		},
	})

	reg.Register(&tool.Entry{
		Name:        "metaclaw_consolidate",
		Toolset:     "memory",
		Description: "Deduplicate and decay MetaClaw memories.",
		Emoji:       "🧹",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "metaclaw_consolidate",
				Description: "Run a consolidation pass: mark near-duplicate memories as superseded, optionally archive stale episodic memories.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{
    "mem_type":{"type":"string","enum":["episodic","semantic","preference",""]},
    "decay_days":{"type":"number"}
  }
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				MemType   string  `json:"mem_type,omitempty"`
				DecayDays float64 `json:"decay_days,omitempty"`
			}
			_ = json.Unmarshal(raw, &args)
			opts := &ConsolidateOptions{MemType: args.MemType}
			if args.DecayDays > 0 {
				opts.DecayAfter = time.Duration(args.DecayDays * float64(24*time.Hour))
			}
			rep, err := Consolidate(ctx, mc.store, opts)
			if err != nil {
				return tool.ToolError(err.Error()), nil
			}
			return tool.ToolResult(map[string]any{
				"scanned":    rep.Scanned,
				"superseded": rep.Superseded,
				"archived":   rep.Archived,
			}), nil
		},
	})

	reg.Register(&tool.Entry{
		Name:        "metaclaw_cite_memory",
		Toolset:     "memory",
		Description: "Record that a memory influenced the current reply.",
		Emoji:       "📌",
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "metaclaw_cite_memory",
				Description: "Signal that the specified memory was used when forming the reply.",
				Parameters: json.RawMessage(`{
  "type":"object",
  "properties":{"memory_id":{"type":"string"}},
  "required":["memory_id"]
}`),
			},
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				MemoryID string `json:"memory_id"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.ToolError("invalid arguments: " + err.Error()), nil
			}
			if args.MemoryID == "" {
				return tool.ToolError("memory_id is required"), nil
			}
			citesink.Cite(ctx, args.MemoryID)
			return tool.ToolResult(map[string]any{"ok": true}), nil
		},
	})
}

// SetSummaryEvery configures how often (in SyncTurn calls) a rolling
// working-summary aux call is triggered. Zero disables.
func (mc *MetaClaw) SetSummaryEvery(n int) {
	mc.mu.Lock()
	mc.summaryEvery = n
	mc.mu.Unlock()
}

func (mc *MetaClaw) refreshWorkingSummary(ctx context.Context) {
	mc.mu.Lock()
	snapshot := make([]TurnPair, len(mc.recentBuf))
	copy(snapshot, mc.recentBuf)
	mc.mu.Unlock()
	if mc.llm == nil || len(snapshot) == 0 {
		return
	}

	var transcript strings.Builder
	for _, p := range snapshot {
		fmt.Fprintf(&transcript, "User: %s\nAssistant: %s\n\n", p.User, p.Assistant)
	}

	resp, err := mc.llm.Complete(ctx, &provider.Request{
		SystemPrompt: "Produce a terse one-paragraph rolling summary of the user's recent activity and current context. Focus on goals, open tasks, and decisions. Replace any prior summary rather than appending. Under 150 words.",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent(transcript.String())},
		},
	})
	if err != nil {
		return
	}
	summary := strings.TrimSpace(resp.Message.Content.Text())
	if summary == "" {
		return
	}

	now := time.Now().UTC()
	created := now
	if existing, err := mc.store.GetMemory(ctx, "working_summary"); err == nil && existing != nil {
		created = existing.CreatedAt
	}
	_ = mc.store.SaveMemory(ctx, &storage.Memory{
		ID:        "working_summary",
		Content:   summary,
		MemType:   storage.MemTypeWorkingSummary,
		CreatedAt: created,
		UpdatedAt: now,
		Status:    storage.MemoryStatusActive,
	})
}

// RecentBufferSnapshot returns a copy of the rolling turn buffer.
// Safe to call concurrently with SyncTurn.
func (mc *MetaClaw) RecentBufferSnapshot() []TurnPair {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	out := make([]TurnPair, len(mc.recentBuf))
	copy(out, mc.recentBuf)
	return out
}
