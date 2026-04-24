package agent

import (
	"context"
	"strings"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
)

// syncMemoryFeedback computes used/neglect for each injected memory by
// unioning three signals (aux-LLM verdict, explicit cite, substring
// presence in the assistant reply) and calls BumpMemoryUsage on the
// storage for each id. Errors per memory are swallowed (best-effort).
func syncMemoryFeedback(
	ctx context.Context,
	store storage.Storage,
	injected []memprovider.InjectedMemory,
	verdict *Verdict,
	cited []string,
	assistantReply string,
) {
	if store == nil {
		return
	}
	uses := syncMemoryFeedbackDecide(injected, verdict, cited, assistantReply)
	for id, used := range uses {
		_ = store.BumpMemoryUsage(ctx, id, used)
	}
}

// syncMemoryFeedbackDecide is the pure decision function split out for
// testing without a storage backend.
func syncMemoryFeedbackDecide(
	injected []memprovider.InjectedMemory,
	verdict *Verdict,
	cited []string,
	assistantReply string,
) map[string]bool {
	out := make(map[string]bool, len(injected))
	verdictSet := map[string]struct{}{}
	if verdict != nil {
		for _, id := range verdict.MemoriesUsed {
			verdictSet[id] = struct{}{}
		}
	}
	citeSet := map[string]struct{}{}
	for _, id := range cited {
		citeSet[id] = struct{}{}
	}
	replyLower := strings.ToLower(assistantReply)
	for _, m := range injected {
		used := false
		if _, ok := verdictSet[m.ID]; ok {
			used = true
		}
		if _, ok := citeSet[m.ID]; ok {
			used = true
		}
		if !used && substringInReply(replyLower, m.Content) {
			used = true
		}
		out[m.ID] = used
	}
	return out
}

// substringInReply returns true when a non-trivial chunk of the memory
// content appears in the assistant reply (case-insensitive). Uses the
// first 60 characters of the trimmed content as a probe to tolerate
// paraphrases of later passages.
func substringInReply(replyLower, content string) bool {
	probe := strings.TrimSpace(strings.ToLower(content))
	if len(probe) < 4 {
		return false
	}
	if len(probe) > 60 {
		probe = probe[:60]
	}
	return strings.Contains(replyLower, probe)
}
