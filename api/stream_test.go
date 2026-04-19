package api

import (
	"encoding/json"
	"testing"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

func TestEncodeStreamEvent_TokenShape(t *testing.T) {
	ev := StreamEvent{
		Type:      EventTypeToken,
		SessionID: "sess-1",
		Data:      &provider.StreamDelta{Content: "hi"},
	}
	data, err := encodeStreamEvent(ev)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var round struct {
		Type      string          `json:"type"`
		SessionID string          `json:"session_id"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if round.Type != EventTypeToken {
		t.Errorf("type = %q", round.Type)
	}
	if round.SessionID != "sess-1" {
		t.Errorf("session_id = %q", round.SessionID)
	}
	if !contains(string(round.Data), `"hi"`) {
		t.Errorf("data = %s", round.Data)
	}
}

// fakeEngine satisfies EngineHook so BridgeEngineToHub can be tested
// without importing the agent package.
type fakeEngine struct {
	onDelta      func(*provider.StreamDelta)
	onToolStart  func(message.ContentBlock)
	onToolResult func(message.ContentBlock, string)
}

func (f *fakeEngine) SetStreamDeltaCallback(fn func(*provider.StreamDelta)) {
	f.onDelta = fn
}
func (f *fakeEngine) SetToolStartCallback(fn func(message.ContentBlock)) {
	f.onToolStart = fn
}
func (f *fakeEngine) SetToolResultCallback(fn func(message.ContentBlock, string)) {
	f.onToolResult = fn
}

func TestBridgeEngineToHub_ForwardsDeltas(t *testing.T) {
	hub := NewMemoryStreamHub()
	eng := &fakeEngine{}
	BridgeEngineToHub(eng, hub, "sess-a")

	if eng.onDelta == nil || eng.onToolStart == nil || eng.onToolResult == nil {
		t.Fatal("bridge did not install all callbacks")
	}

	// Subscribe first, then fire a delta and assert we receive it.
	ch, _ := hub.Subscribe(t.Context(), "sess-a")
	eng.onDelta(&provider.StreamDelta{Content: "chunk"})

	select {
	case ev := <-ch:
		if ev.Type != EventTypeToken {
			t.Errorf("type = %q, want %q", ev.Type, EventTypeToken)
		}
		if ev.SessionID != "sess-a" {
			t.Errorf("session_id = %q", ev.SessionID)
		}
		d, ok := ev.Data.(*provider.StreamDelta)
		if !ok || d.Content != "chunk" {
			t.Errorf("data = %#v", ev.Data)
		}
	default:
		t.Fatal("no event published")
	}
}

func TestBridgeEngineToHub_NilSafe(t *testing.T) {
	// Must not panic.
	BridgeEngineToHub(nil, nil, "")
	BridgeEngineToHub(&fakeEngine{}, nil, "x")
	BridgeEngineToHub(&fakeEngine{}, NewMemoryStreamHub(), "")
}
