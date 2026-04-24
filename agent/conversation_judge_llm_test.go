package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type judgeStubProvider struct {
	reply string
	err   error
}

func (s *judgeStubProvider) Name() string { return "stub" }
func (s *judgeStubProvider) Complete(_ context.Context, _ *provider.Request) (*provider.Response, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &provider.Response{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: message.TextContent(s.reply),
		},
	}, nil
}
func (s *judgeStubProvider) Stream(_ context.Context, _ *provider.Request) (provider.Stream, error) {
	panic("stub stream")
}
func (s *judgeStubProvider) ModelInfo(model string) *provider.ModelInfo { return nil }
func (s *judgeStubProvider) EstimateTokens(model, text string) (int, error) { return 0, nil }
func (s *judgeStubProvider) Available() bool { return true }

func TestLLMJudgeParsesVerdict(t *testing.T) {
	p := &judgeStubProvider{reply: `{"outcome":"struggle","memories_used":["mc_1"],"skills_to_extract":[{"name":"n","description":"d","body":"b"}],"reasoning":"retried"}`}
	j := NewLLMJudge(p)
	v, err := j.Run(context.Background(), JudgeInput{
		History: []message.Message{{Role: message.RoleUser, Content: message.TextContent("hi")}},
		InjectedMemories: []memprovider.InjectedMemory{{ID: "mc_1", Content: "fact"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "struggle", v.Outcome)
	assert.Equal(t, []string{"mc_1"}, v.MemoriesUsed)
	require.Len(t, v.SkillsToExtract, 1)
	assert.Equal(t, "n", v.SkillsToExtract[0].Name)
}

func TestLLMJudgeHandlesFences(t *testing.T) {
	p := &judgeStubProvider{reply: "```json\n{\"outcome\":\"success\"}\n```"}
	v, err := NewLLMJudge(p).Run(context.Background(), JudgeInput{})
	require.NoError(t, err)
	assert.Equal(t, "success", v.Outcome)
}

func TestLLMJudgeMalformedReturnsUnknown(t *testing.T) {
	p := &judgeStubProvider{reply: "not json"}
	v, err := NewLLMJudge(p).Run(context.Background(), JudgeInput{})
	require.NoError(t, err)
	assert.Equal(t, "unknown", v.Outcome)
}

func TestLLMJudgeProviderErrorReturnsUnknown(t *testing.T) {
	p := &judgeStubProvider{err: errors.New("aux down")}
	v, err := NewLLMJudge(p).Run(context.Background(), JudgeInput{})
	require.NoError(t, err)
	assert.Equal(t, "unknown", v.Outcome)
}

func TestLLMJudgeNilProviderReturnsUnknown(t *testing.T) {
	v, err := NewLLMJudge(nil).Run(context.Background(), JudgeInput{})
	require.NoError(t, err)
	assert.Equal(t, "unknown", v.Outcome)
}
