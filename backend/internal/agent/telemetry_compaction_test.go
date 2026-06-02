package agent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogCompactionFinished(t *testing.T) {
	logger := &mockEventLogger{}
	logCompactionFinished(logger, intPtr(7), 42, "chat", 1000, 400, false)

	// Async — wait briefly
	time.Sleep(100 * time.Millisecond)

	events := logger.Events()
	require.Len(t, events, 1)
	ev := events[0]
	assert.Equal(t, "compaction_finished", ev.event)
	assert.Equal(t, 42, ev.metadata["workspace_id"])
	assert.Equal(t, "chat", ev.metadata["path"])
	assert.Equal(t, 1000, ev.metadata["before_tokens"])
	assert.Equal(t, 400, ev.metadata["after_tokens"])
	assert.Equal(t, 60.0, ev.metadata["saved_pct"])
	assert.Equal(t, false, ev.metadata["fallback_used"])
}

func TestLogCompactionFinished_NilLogger(t *testing.T) {
	// Should not panic
	logCompactionFinished(nil, nil, 1, "agent", 0, 0, false)
}

func intPtr(i int) *int { return &i }
