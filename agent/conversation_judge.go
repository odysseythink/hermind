package agent

import (
	"context"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
)

// Verdict summarizes a completed conversation for end-of-conversation
// consumers (memory reinforcement, skill extraction, telemetry).
type Verdict struct {
	// Outcome is one of "success", "struggle", "failure", or "unknown".
	Outcome string
	// MemoriesUsed lists the IDs of injected memories that materially
	// influenced the assistant reply, as judged by the aux LLM.
	MemoriesUsed []string
	// SkillsToExtract contains reusable patterns the judge recommends
	// persisting. Populated only when Outcome != "success".
	SkillsToExtract []SkillDraft
	// Reasoning is a terse natural-language note, for telemetry only.
	Reasoning string
}

// SkillDraft is a minimal description of a skill worth saving. The
// evolver writes it as a Markdown file under the skills directory.
type SkillDraft struct {
	Name        string
	Description string
	Body        string
}

// JudgeInput bundles everything a ConversationJudge implementation needs
// to score a completed conversation.
type JudgeInput struct {
	History          []message.Message
	InjectedMemories []memprovider.InjectedMemory
	InjectedSkills   []ActiveSkill
	Platform         string
}

// ConversationJudge scores a completed conversation once at its end.
// Implementations must be safe to leave nil (no-op behavior is wired in
// RunConversation). Implementations should be best-effort — failures
// must not propagate errors that abort the turn.
type ConversationJudge interface {
	Run(ctx context.Context, in JudgeInput) (*Verdict, error)
}
