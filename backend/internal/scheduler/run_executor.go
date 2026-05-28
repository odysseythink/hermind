package scheduler

import (
	"context"
	"errors"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
)

// ErrRuntimeNotWired is returned by RuntimeAgentRunner.RunUntil until the
// adapter is bound to the real agent.Runtime.
var ErrRuntimeNotWired = errors.New("RuntimeAgentRunner.RunOnce not yet wired to agent.Runtime")

// RuntimeAgentRunner adapts the existing agent.Runtime to AgentRunner.
//
// Auto-approve: this runner registers a workspace-less invocation and runs the
// prompt against the agent runtime with all tool approvals auto-granted (each
// approval logged to EventLog under "scheduled_job_tool_auto_approved").
//
// WIRING NOTE: The agent runtime's public surface (agent/runtime.go,
// agent/handler.go) requires reading before replacing the stub below. The
// contract (AgentRunner.RunOnce) is locked; only the body changes.
type RuntimeAgentRunner struct {
	rt       any // *agent.Runtime — typed as any to avoid import cycle until wired
	eventLog *services.EventLogService
}

func NewRuntimeAgentRunner(rt any, eventLog *services.EventLogService) *RuntimeAgentRunner {
	return &RuntimeAgentRunner{rt: rt, eventLog: eventLog}
}

func (r *RuntimeAgentRunner) RunOnce(ctx context.Context, job *models.ScheduledJob) (*AgentRunResult, error) {
	// TODO: wire to agent.Runtime.RunHeadless or equivalent once that surface
	// is read and extended. Until then the scheduler tests use a fakeRunner.
	return nil, ErrRuntimeNotWired
}
