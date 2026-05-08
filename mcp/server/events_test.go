package server

import (
	"context"
	"testing"
	"time"
)

func TestEventBridge_Poll_EmitsNewMessages(t *testing.T) {
	bridge := NewEventBridge(nil, 10*time.Millisecond)

	bridge.push(Event{Cursor: 1, Kind: "message", SessionKey: "s1"})
	bridge.push(Event{Cursor: 2, Kind: "message", SessionKey: "s1"})

	got, next := bridge.Poll(0, "", 10)
	if len(got) != 2 {
		t.Fatalf("got %d events", len(got))
	}
	if next != 2 {
		t.Errorf("next cursor = %d, want 2", next)
	}
}

func TestEventBridge_Poll_FiltersBySession(t *testing.T) {
	bridge := NewEventBridge(nil, 10*time.Millisecond)
	bridge.push(Event{Cursor: 1, Kind: "message", SessionKey: "s1"})
	bridge.push(Event{Cursor: 2, Kind: "message", SessionKey: "s2"})

	got, _ := bridge.Poll(0, "s1", 10)
	if len(got) != 1 || got[0].SessionKey != "s1" {
		t.Errorf("filter failed: %+v", got)
	}
}

func TestEventBridge_Wait_ReturnsOnPush(t *testing.T) {
	bridge := NewEventBridge(nil, 10*time.Millisecond)
	ctx := context.Background()

	doneCh := make(chan Event, 1)
	go func() {
		ev, _ := bridge.Wait(ctx, 0, "", 500*time.Millisecond)
		if ev != nil {
			doneCh <- *ev
		}
	}()

	time.Sleep(20 * time.Millisecond)
	bridge.push(Event{Cursor: 1, Kind: "message"})

	select {
	case got := <-doneCh:
		if got.Cursor != 1 {
			t.Errorf("got %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wait did not return")
	}
}

func TestPermissionQueue_Roundtrip(t *testing.T) {
	q := NewPermissionQueue()
	id := q.Open(PermissionRequest{Command: "rm", Kind: "execute"})
	open := q.ListOpen()
	if len(open) != 1 || open[0].ID != id {
		t.Errorf("list = %+v", open)
	}
	if !q.Respond(id, "allow-once") {
		t.Error("respond should succeed")
	}
	if q.Respond(id, "allow-once") {
		t.Error("second respond should fail")
	}
}
