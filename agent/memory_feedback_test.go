package agent

import (
	"testing"

	"github.com/odysseythink/hermind/tool/memory/memprovider"
	"github.com/stretchr/testify/assert"
)

func TestSyncMemoryFeedback_Union(t *testing.T) {
	injected := []memprovider.InjectedMemory{
		{ID: "a", Content: "alpha fact"},
		{ID: "b", Content: "beta fact"},
		{ID: "c", Content: "gamma fact"},
		{ID: "d", Content: "delta fact"},
	}
	verdict := &Verdict{MemoriesUsed: []string{"a"}}
	cited := []string{"b"}
	reply := "the user wanted gamma fact which was relevant"

	uses := syncMemoryFeedbackDecide(injected, verdict, cited, reply)
	assert.True(t, uses["a"], "verdict signal")
	assert.True(t, uses["b"], "cite signal")
	assert.True(t, uses["c"], "substring signal")
	assert.False(t, uses["d"], "no signal → neglect")
}

func TestSyncMemoryFeedback_EmptyInputs(t *testing.T) {
	uses := syncMemoryFeedbackDecide(nil, nil, nil, "")
	assert.Empty(t, uses)
}

func TestSyncMemoryFeedback_NilVerdict(t *testing.T) {
	injected := []memprovider.InjectedMemory{{ID: "a", Content: "alpha"}}
	uses := syncMemoryFeedbackDecide(injected, nil, nil, "alpha is here")
	assert.True(t, uses["a"])
}
