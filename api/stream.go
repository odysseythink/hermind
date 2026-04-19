package api

import (
	"encoding/json"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// Common stream event type constants. The StreamHub is schemaless on
// StreamEvent.Type; these constants are the vocabulary the web UI
// parses. Publishers (the agent bridge below, or the parallel
// WebSocket agent) should use them so the client only has to branch
// on a small closed set of strings.
const (
	// EventTypeToken is an incremental LLM content delta. Data is the
	// *provider.StreamDelta, which JSON-marshals to the same shape the
	// provider emitted.
	EventTypeToken = "token"
	// EventTypeToolCall fires when the LLM asks to invoke a tool. Data
	// is the message.ContentBlock describing the call.
	EventTypeToolCall = "tool_call"
	// EventTypeToolResult fires after the tool finishes. Data is a
	// {call, result} object.
	EventTypeToolResult = "tool_result"
	// EventTypeStatus is a coarse lifecycle event ("started",
	// "turn_complete", "error"). Data is a string.
	EventTypeStatus = "status"
)

// encodeStreamEvent produces the JSON frame a WebSocket or SSE client
// receives. The shape matches the on-the-wire format used by the
// REST DTOs: every event is a self-describing object with a "type",
// "session_id" and optional "data" payload.
func encodeStreamEvent(ev StreamEvent) ([]byte, error) {
	return json.Marshal(ev)
}

// EngineHook is the minimum surface the agent.Engine exposes that the
// stream bridge needs. It matches the three SetXxxCallback setters on
// agent.Engine. Declared as an interface here so cli/web.go can wire
// the bridge without introducing a circular import between api/ and
// agent/ (api/ stays dependency-light and the bridge just needs the
// setter shape).
type EngineHook interface {
	SetStreamDeltaCallback(fn func(delta *provider.StreamDelta))
	SetToolStartCallback(fn func(call message.ContentBlock))
	SetToolResultCallback(fn func(call message.ContentBlock, result string))
}

// BridgeEngineToHub installs callbacks on the given engine that
// publish every streaming delta, tool start, and tool result into the
// StreamHub under sessionID. The agent.Engine only supports single-use
// per conversation, so a fresh bridge is installed each time a new
// Engine is constructed (in cli/web.go or the gateway).
//
// The caller remains responsible for publishing coarse lifecycle
// events ("started", "turn_complete") around the actual conversation
// call; the bridge only mirrors what the engine itself surfaces.
func BridgeEngineToHub(engine EngineHook, hub StreamHub, sessionID string) {
	if engine == nil || hub == nil || sessionID == "" {
		return
	}
	engine.SetStreamDeltaCallback(func(delta *provider.StreamDelta) {
		hub.Publish(StreamEvent{
			Type:      EventTypeToken,
			SessionID: sessionID,
			Data:      delta,
		})
	})
	engine.SetToolStartCallback(func(call message.ContentBlock) {
		hub.Publish(StreamEvent{
			Type:      EventTypeToolCall,
			SessionID: sessionID,
			Data:      call,
		})
	})
	engine.SetToolResultCallback(func(call message.ContentBlock, result string) {
		hub.Publish(StreamEvent{
			Type:      EventTypeToolResult,
			SessionID: sessionID,
			Data: map[string]any{
				"call":   call,
				"result": result,
			},
		})
	})
}
