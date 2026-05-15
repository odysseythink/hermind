package replay

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/pantheon/benchmark"
	"github.com/odysseythink/pantheon/core"
)

func TestReplayItem_ImplementsItem(t *testing.T) {
	var _ benchmark.Item = ReplayItem{} // compile-time check
	ri := ReplayItem{
		ID:       "replay_42",
		Category: "general",
		Message:  "what time is it?",
		History: []message.HermindMessage{
			{Role: core.MESSAGE_ROLE_USER, Content: core.NewTextContent("hi")},
			{Role: core.MESSAGE_ROLE_ASSISTANT, Content: core.NewTextContent("hello!")},
		},
		Baseline: "It's about 3pm.",
	}
	require.Equal(t, "replay_42", ri.GetID())
	require.Equal(t, "what time is it?", ri.GetMessage())
	require.Equal(t, "general", ri.GetCategory())
	require.Equal(t, "It's about 3pm.", ri.GetBaseline())
	require.Len(t, ri.GetHistory(), 2)
}

func TestModeConstants(t *testing.T) {
	require.Equal(t, Mode("none"), ModeNone)
	require.Equal(t, Mode("pairwise"), ModePairwise)
	require.Equal(t, Mode("rubric+pairwise"), ModeRubricPairwise)
}
