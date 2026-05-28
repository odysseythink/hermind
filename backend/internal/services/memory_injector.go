package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/reranker"
	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/core"
)

// SettingsReader is the minimal interface MemoryInjector needs. SystemService
// satisfies it via a thin adapter — see Task 6.
type SettingsReader interface {
	MemoriesEnabled(ctx context.Context) bool
}

type MemoryInjector struct {
	memSvc   *MemoryService
	settings SettingsReader
	rerank   reranker.Reranker
}

func NewMemoryInjector(memSvc *MemoryService, settings SettingsReader, r reranker.Reranker) *MemoryInjector {
	return &MemoryInjector{memSvc: memSvc, settings: settings, rerank: r}
}

// PromptWithMemories appends a "## Things I Remember About You" section to base.
// Safe to call when memSvc/settings are nil — returns base unchanged.
func (mi *MemoryInjector) PromptWithMemories(ctx context.Context, base string,
	userID *int, workspaceID int, currentMessage string, history []core.Message) string {

	if mi == nil || mi.memSvc == nil || mi.settings == nil {
		return base
	}
	if !mi.settings.MemoriesEnabled(ctx) {
		return base
	}

	globals, _ := mi.memSvc.ListGlobal(ctx, userID)
	wsMems, _ := mi.memSvc.ListWorkspace(ctx, userID, workspaceID)
	if len(globals) == 0 && len(wsMems) == 0 {
		return base
	}

	selected := wsMems
	if len(wsMems) > models.MaxInjectedWorkspaceLimit {
		query := buildRerankQuery(currentMessage, history)
		texts := make([]string, len(wsMems))
		for i, m := range wsMems {
			texts[i] = m.Content
		}
		if ranked, err := mi.rerank.Rerank(ctx, query, texts, models.MaxInjectedWorkspaceLimit); err == nil {
			out := make([]models.Memory, 0, len(ranked))
			for _, r := range ranked {
				if r.Index >= 0 && r.Index < len(wsMems) {
					out = append(out, wsMems[r.Index])
				}
			}
			selected = out
		} else {
			mlog.Warning("memory inject rerank failed, using recency", mlog.Err(err))
			selected = wsMems[:models.MaxInjectedWorkspaceLimit]
		}
	}
	if len(selected) > models.MaxInjectedWorkspaceLimit {
		selected = selected[:models.MaxInjectedWorkspaceLimit]
	}

	// Stamp last-used fire-and-forget
	ids := make([]int, 0, len(globals)+len(selected))
	for _, m := range globals {
		ids = append(ids, m.ID)
	}
	for _, m := range selected {
		ids = append(ids, m.ID)
	}
	go func(stampIDs []int) {
		_ = mi.memSvc.UpdateLastUsed(context.Background(), stampIDs)
	}(ids)

	var b strings.Builder
	b.WriteString(base)
	b.WriteString("\n\n## Things I Remember About You\n")
	for _, m := range globals {
		fmt.Fprintf(&b, "- %s\n", m.Content)
	}
	for _, m := range selected {
		fmt.Fprintf(&b, "- %s\n", m.Content)
	}
	return strings.TrimRight(b.String(), "\n")
}

// buildRerankQuery concatenates the current message with up to the last 3
// user-role history texts.
func buildRerankQuery(currentMessage string, history []core.Message) string {
	parts := []string{currentMessage}
	count := 0
	for i := len(history) - 1; i >= 0 && count < 3; i-- {
		m := history[i]
		if m.Role != core.MESSAGE_ROLE_USER {
			continue
		}
		// Best-effort text extraction; pantheon's text part lives in Content[0].TextPart.
		for _, p := range m.Content {
			if tp, ok := p.(core.TextPart); ok {
				parts = append(parts, tp.Text)
				break
			}
		}
		count++
	}
	return strings.Join(parts, " ")
}
