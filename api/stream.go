package api

// Event type constants used in StreamEvent.Type.
const (
	EventTypeMessageChunk = "message_chunk"
	EventTypeToolCall     = "tool_call"
	EventTypeToolResult   = "tool_result"
	EventTypeDone         = "done"
	EventTypeError        = "error"
)
