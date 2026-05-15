package agent

import (
	"context"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/pantheon/extensions/judge"
)

// Re-exports of pantheon/extensions/judge types so existing call sites
// in hermind continue to compile.
type (
	Verdict           = judge.Verdict
	SkillDraft        = judge.SkillDraft
	InjectedMemory    = judge.InjectedMemory
	ActiveSkill       = judge.ActiveSkill
)

// JudgeInput bundles everything a ConversationJudge implementation needs
// to score a completed conversation.
type JudgeInput struct {
	History          []message.HermindMessage
	InjectedMemories []InjectedMemory
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
