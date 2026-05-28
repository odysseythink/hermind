package scheduler

import (
	"context"

	"github.com/odysseythink/hermind/backend/internal/models"
)

// AgentRunResult carries the result of an agent run for persistence.
type AgentRunResult struct {
	Text      string         `json:"text"`
	Thoughts  []any          `json:"thoughts,omitempty"`
	ToolCalls []any          `json:"toolCalls,omitempty"`
	Outputs   []any          `json:"outputs,omitempty"`
	Metrics   map[string]any `json:"metrics,omitempty"`
	Duration  int64          `json:"duration"` // milliseconds
}

// AgentRunner abstracts the agent invocation so scheduler is testable without
// the real Runtime. Implementations must respect ctx.Done() for cancellation
// (kill semantics).
//
// Auto-approve: when called via the scheduler the implementation MUST treat
// all internal tool-approval requests as approved, recording each one to the
// EventLog under "scheduled_job_tool_auto_approved" for audit.
type AgentRunner interface {
	RunOnce(ctx context.Context, job *models.ScheduledJob) (*AgentRunResult, error)
}
