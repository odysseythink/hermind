package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/core"
)

const (
	defaultObserverPrompt = `You are a memory extraction assistant. Given a recent conversation between
a user and an AI assistant, identify durable facts about the user worth
remembering for future conversations.

CRITERIA for a fact:
- Concrete (a specific preference, expertise, project, role, or stable belief)
- Re-usable across distinct conversation topics
- Not transient (not "is currently confused about Y")
- Not derivable from the assistant's general knowledge

Call the extract_candidate_facts tool. Each fact has:
- content: short third-person statement ("User prefers Go over Python")
- confidence: 0.0..1.0
- reasoning: one sentence explaining why this is durable

If no durable facts, call the tool with facts: [].

CONVERSATION:
{{CONVERSATION}}`

	defaultReflectorPrompt = `You are a memory curator. Given a set of candidate facts from the Observer
and the user's existing memories, decide which candidates to keep, dedupe,
or use to update an existing memory.

For each accepted memory call decide_memory_actions with:
- content: final memory text
- scope: "WORKSPACE" (specific to this workspace) or "GLOBAL" (applies everywhere)
- action: "create" (new memory) or "update" (revise an existing memory by id)
- updateId: integer (required when action == "update"; must reference an
  existing memory id from the lists below)
- reasoning: one sentence

RULES:
- Skip candidates that duplicate or are subsumed by existing memories
- Prefer "update" when a candidate refines an existing memory's content
- Never create a memory that contradicts an existing one — reject silently
- Available global slots: {{GLOBAL_SLOTS}}; do not exceed
- The workspace cap is 20; selecting more workspace creates will be truncated

EXISTING WORKSPACE MEMORIES (id: content):
{{WORKSPACE_MEMORIES}}

EXISTING GLOBAL MEMORIES (id: content):
{{GLOBAL_MEMORIES}}

CANDIDATE FACTS:
{{CANDIDATES}}`
)

// LLMClient is the slice of pantheon's LLM API the extractor needs.
type LLMClient interface {
	Generate(ctx context.Context, req *core.Request) (*core.Response, error)
}

type MemoryExtractor struct {
	memSvc     *MemoryService
	llm        LLMClient
	observerT  string
	reflectorT string
}

func NewMemoryExtractor(memSvc *MemoryService, llm LLMClient, observerT, reflectorT string) *MemoryExtractor {
	if observerT == "" {
		observerT = defaultObserverPrompt
	}
	if reflectorT == "" {
		reflectorT = defaultReflectorPrompt
	}
	return &MemoryExtractor{memSvc: memSvc, llm: llm, observerT: observerT, reflectorT: reflectorT}
}

type observerCandidate struct {
	Content    string  `json:"content"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
}

type reflectorAction struct {
	Content   string `json:"content"`
	Scope     string `json:"scope"`     // WORKSPACE | GLOBAL
	Action    string `json:"action"`    // create | update
	UpdateID  *int   `json:"updateId,omitempty"`
	Reasoning string `json:"reasoning"`
}

func (e *MemoryExtractor) ProcessGroup(ctx context.Context, userID *int, workspaceID int, chats []models.WorkspaceChat) error {
	if len(chats) == 0 {
		return nil
	}

	// 1. Observer
	convo := renderConversation(chats)
	obsPrompt := strings.ReplaceAll(e.observerT, "{{CONVERSATION}}", convo)
	candidates, err := e.runObserver(ctx, obsPrompt)
	if err != nil || len(candidates) == 0 {
		if err != nil {
			mlog.Warning("memory observer failed", mlog.Err(err))
		}
		return nil
	}

	// 2. Reflector
	wsMems, _ := e.memSvc.ListWorkspace(ctx, userID, workspaceID)
	glMems, _ := e.memSvc.ListGlobal(ctx, userID)
	globalSlots := models.GlobalMemoryLimit - len(glMems)
	if globalSlots <= 0 && len(wsMems) >= models.WorkspaceMemoryLimit {
		return nil
	}
	refPrompt := e.reflectorT
	refPrompt = strings.ReplaceAll(refPrompt, "{{WORKSPACE_MEMORIES}}", renderMems(wsMems))
	refPrompt = strings.ReplaceAll(refPrompt, "{{GLOBAL_MEMORIES}}", renderMems(glMems))
	refPrompt = strings.ReplaceAll(refPrompt, "{{GLOBAL_SLOTS}}", fmt.Sprintf("%d", globalSlots))
	refPrompt = strings.ReplaceAll(refPrompt, "{{CANDIDATES}}", renderCandidates(candidates))

	actions, err := e.runReflector(ctx, refPrompt)
	if err != nil || len(actions) == 0 {
		if err != nil {
			mlog.Warning("memory reflector failed", mlog.Err(err))
		}
		return nil
	}

	// 3. Apply
	extracted := make([]ExtractedAction, 0, len(actions))
	for _, a := range actions {
		extracted = append(extracted, ExtractedAction{
			Action: a.Action, Scope: a.Scope, Content: a.Content, UpdateID: a.UpdateID,
		})
	}
	_, err = e.memSvc.ApplyExtracted(ctx, userID, workspaceID, extracted, globalSlots)
	return err
}

func (e *MemoryExtractor) runObserver(ctx context.Context, prompt string) ([]observerCandidate, error) {
	resp, err := e.llm.Generate(ctx, &core.Request{
		SystemPrompt: "You extract durable user facts via the provided tool.",
		Messages: []core.Message{{
			Role:    core.MESSAGE_ROLE_USER,
			Content: []core.ContentParter{core.TextPart{Text: prompt}},
		}},
		Tools: []core.ToolDefinition{{Name: "extract_candidate_facts", Description: "Emit candidate facts", Parameters: candidateSchema()}},
	})
	if err != nil {
		return nil, err
	}
	args := firstToolCallArgs(resp)
	if args == "" {
		return nil, nil
	}
	var body struct{ Facts []observerCandidate `json:"facts"` }
	if err := json.Unmarshal([]byte(args), &body); err != nil {
		return nil, err
	}
	return body.Facts, nil
}

func (e *MemoryExtractor) runReflector(ctx context.Context, prompt string) ([]reflectorAction, error) {
	resp, err := e.llm.Generate(ctx, &core.Request{
		SystemPrompt: "You curate memories via the provided tool.",
		Messages: []core.Message{{
			Role:    core.MESSAGE_ROLE_USER,
			Content: []core.ContentParter{core.TextPart{Text: prompt}},
		}},
		Tools: []core.ToolDefinition{{Name: "decide_memory_actions", Description: "Decide actions", Parameters: actionSchema()}},
	})
	if err != nil {
		return nil, err
	}
	args := firstToolCallArgs(resp)
	if args == "" {
		return nil, nil
	}
	var body struct{ Memories []reflectorAction `json:"memories"` }
	if err := json.Unmarshal([]byte(args), &body); err != nil {
		return nil, err
	}
	return body.Memories, nil
}

func firstToolCallArgs(resp *core.Response) string {
	if resp == nil {
		return ""
	}
	for _, p := range resp.Message.Content {
		if t, ok := p.(core.ToolCallPart); ok {
			return t.Arguments
		}
	}
	return ""
}

func renderConversation(chats []models.WorkspaceChat) string {
	var b strings.Builder
	for _, c := range chats {
		fmt.Fprintf(&b, "USER: %s\nASSISTANT: %s\n\n", c.Prompt, truncateResp(c.Response))
	}
	return b.String()
}

func truncateResp(s string) string {
	if utf8.RuneCountInString(s) <= 600 {
		return s
	}
	runes := []rune(s)
	return string(runes[:600]) + "…"
}

func renderMems(mems []models.Memory) string {
	if len(mems) == 0 {
		return "(none)"
	}
	var b strings.Builder
	for _, m := range mems {
		fmt.Fprintf(&b, "%d: %s\n", m.ID, m.Content)
	}
	return b.String()
}

func renderCandidates(cs []observerCandidate) string {
	var b strings.Builder
	for _, c := range cs {
		fmt.Fprintf(&b, "- [%0.2f] %s — %s\n", c.Confidence, c.Content, c.Reasoning)
	}
	return b.String()
}

func candidateSchema() *core.Schema {
	return &core.Schema{
		Type: "object",
		Properties: map[string]*core.Schema{
			"facts": {
				Type: "array",
				Items: &core.Schema{
					Type: "object",
					Properties: map[string]*core.Schema{
						"content":    {Type: "string"},
						"confidence": {Type: "number"},
						"reasoning":  {Type: "string"},
					},
					Required: []string{"content"},
				},
			},
		},
		Required: []string{"facts"},
	}
}

func actionSchema() *core.Schema {
	return &core.Schema{
		Type: "object",
		Properties: map[string]*core.Schema{
			"memories": {
				Type: "array",
				Items: &core.Schema{
					Type: "object",
					Properties: map[string]*core.Schema{
						"content":   {Type: "string"},
						"scope":     {Type: "string", Enum: []string{"WORKSPACE", "GLOBAL"}},
						"action":    {Type: "string", Enum: []string{"create", "update"}},
						"updateId":  {Type: "integer"},
						"reasoning": {Type: "string"},
					},
					Required: []string{"content", "scope", "action"},
				},
			},
		},
		Required: []string{"memories"},
	}
}
