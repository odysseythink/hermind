package acp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"sync"
	"time"
)

// Event is one ACP server-sent event.
type Event struct {
	Type      string    `json:"type"`
	SessionID string    `json:"session_id,omitempty"`
	Data      string    `json:"data,omitempty"`
	Time      time.Time `json:"time"`
}

// EventBus is a tiny pub/sub used by the server to push events to
// one or more SSE subscribers. Every subscriber has its own buffered
// channel; slow subscribers drop old events rather than blocking the
// publisher.
type EventBus struct {
	mu          sync.Mutex
	subscribers []chan Event
}

func NewEventBus() *EventBus { return &EventBus{} }

// Subscribe returns a buffered channel that will receive every
// subsequent published event. Call Unsubscribe to release it.
func (b *EventBus) Subscribe() chan Event {
	ch := make(chan Event, 16)
	b.mu.Lock()
	b.subscribers = append(b.subscribers, ch)
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes and closes a previously returned channel.
func (b *EventBus) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := b.subscribers[:0]
	for _, s := range b.subscribers {
		if s == ch {
			close(s)
			continue
		}
		out = append(out, s)
	}
	b.subscribers = out
}

// Publish broadcasts an event to every subscriber. Non-blocking:
// events are dropped for subscribers whose channel is full.
func (b *EventBus) Publish(ev Event) {
	if ev.Time.IsZero() {
		ev.Time = time.Now().UTC()
	}
	b.mu.Lock()
	subs := append([]chan Event{}, b.subscribers...)
	b.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
			// drop
		}
	}
}

// Notifier emits session/update JSON-RPC notification frames onto an
// io.Writer (typically the ACP stdio transport's stdout). It is safe
// for concurrent use — writes are serialized on an internal mutex so
// frames never interleave with other JSON-RPC frames sharing the same
// writer. Callers that already own a stdout lock can inject it via
// NewNotifier's sharedMu parameter.
type Notifier struct {
	mu  *sync.Mutex
	out io.Writer
}

// NewNotifier wraps w. If sharedMu is non-nil the notifier uses it
// instead of allocating a fresh lock — this is how a transport shares
// its outgoing lock so responses and notifications never interleave.
func NewNotifier(w io.Writer, sharedMu *sync.Mutex) *Notifier {
	if sharedMu == nil {
		sharedMu = &sync.Mutex{}
	}
	return &Notifier{mu: sharedMu, out: w}
}

// AgentMessageChunk emits a streaming text chunk from the assistant.
func (n *Notifier) AgentMessageChunk(sessionID, text string) {
	n.send(sessionID, map[string]any{
		"sessionUpdate":     "agent_message_chunk",
		"agentMessageChunk": map[string]string{"text": text},
	})
}

// AgentThoughtChunk emits a "thinking" text chunk distinct from
// visible message content.
func (n *Notifier) AgentThoughtChunk(sessionID, text string) {
	n.send(sessionID, map[string]any{
		"sessionUpdate":     "agent_thought_chunk",
		"agentThoughtChunk": map[string]string{"text": text},
	})
}

// ToolCallStart emits a tool_call_start update and returns the
// generated tool call ID. Callers pass it into ToolCallUpdate when the
// tool completes.
func (n *Notifier) ToolCallStart(sessionID, toolName, kind, rawInput string) (string, error) {
	id, err := makeToolCallID()
	if err != nil {
		return "", err
	}
	n.send(sessionID, map[string]any{
		"sessionUpdate": "tool_call_start",
		"toolCallId":    id,
		"kind":          kind,
		"title":         toolName,
		"rawInput":      rawInput,
	})
	return id, nil
}

// ToolCallUpdate emits a tool_call_update when a tool finishes (or
// fails). status is usually "completed" or "failed".
func (n *Notifier) ToolCallUpdate(sessionID, toolCallID, status, rawOutput string) {
	n.send(sessionID, map[string]any{
		"sessionUpdate": "tool_call_update",
		"toolCallId":    toolCallID,
		"status":        status,
		"rawOutput":     rawOutput,
	})
}

// AvailableCommands emits the list of slash commands the agent
// exposes (e.g. /reset, /compact). Sent once per session after
// new/load/resume.
func (n *Notifier) AvailableCommands(sessionID string, commands []string) {
	cmdObjs := make([]map[string]string, 0, len(commands))
	for _, c := range commands {
		cmdObjs = append(cmdObjs, map[string]string{"name": c})
	}
	n.send(sessionID, map[string]any{
		"sessionUpdate":     "available_commands_update",
		"availableCommands": cmdObjs,
	})
}

func (n *Notifier) send(sessionID string, update map[string]any) {
	frame := map[string]any{
		"jsonrpc": "2.0",
		"method":  "session/update",
		"params": map[string]any{
			"sessionId": sessionID,
			"update":    update,
		},
	}
	buf, err := json.Marshal(frame)
	if err != nil {
		return
	}
	buf = append(buf, '\n')
	n.mu.Lock()
	defer n.mu.Unlock()
	_, _ = n.out.Write(buf)
}

// makeToolCallID returns "tc-" + 12 hex chars, matching the Python
// helper's shape so existing client test expectations line up.
func makeToolCallID() (string, error) {
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "tc-" + hex.EncodeToString(buf[:]), nil
}
