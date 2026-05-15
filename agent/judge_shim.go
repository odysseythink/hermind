package agent

import (
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/extensions/judge"
)

// Re-exports of pantheon/extensions/judge so existing call sites
// in hermind continue to compile.
type (
	Verdict           = judge.Verdict
	SkillDraft        = judge.SkillDraft
	InjectedMemory    = judge.InjectedMemory
	ActiveSkill       = judge.ActiveSkill
	JudgeInput        = judge.Input
	ConversationJudge = judge.Interface
	LLMJudge          = judge.LLM
)

// NewLLMJudge delegates to pantheon/extensions/judge.NewLLM.
func NewLLMJudge(llm core.LanguageModel) *LLMJudge { return judge.NewLLM(llm) }
