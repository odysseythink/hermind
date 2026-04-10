// cli/ui/messages.go
package ui

import (
	"github.com/nousresearch/hermes-agent/agent"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
)

// Tea message types posted from the Engine goroutine back to the bubbletea Program.
// The Update handler reads these and mutates the Model accordingly.

// streamDeltaMsg is posted for each streaming content chunk from the LLM.
type streamDeltaMsg struct {
	Delta *provider.StreamDelta
}

// toolStartMsg is posted when a tool begins executing.
type toolStartMsg struct {
	Call message.ContentBlock // type="tool_use"
}

// toolResultMsg is posted when a tool finishes executing.
type toolResultMsg struct {
	Call   message.ContentBlock // the original tool_use block
	Result string               // JSON result (possibly truncated for display)
}

// convDoneMsg is posted when a conversation turn completes.
// Carries the full result so the Model can update totals.
type convDoneMsg struct {
	Result *agent.ConversationResult
}

// convErrorMsg is posted if the Engine returns an error.
type convErrorMsg struct {
	Err error
}

// tickMsg is a periodic tick used for the streaming cursor animation.
// Sent by a tea.Tick command; Update handles it by toggling the cursor state.
type tickMsg struct{}

// quitMsg signals the program should exit cleanly (e.g. from /exit).
type quitMsg struct{}
