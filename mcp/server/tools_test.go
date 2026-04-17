package server

import "testing"

func TestBuiltinTools_Count(t *testing.T) {
	got := BuiltinTools()
	if len(got) != 10 {
		t.Errorf("expected 10 tools, got %d", len(got))
	}
}

func TestBuiltinTools_Names(t *testing.T) {
	want := map[string]bool{
		"conversations_list":    false,
		"conversation_get":      false,
		"messages_read":         false,
		"messages_send":         false,
		"attachments_fetch":     false,
		"events_poll":           false,
		"events_wait":           false,
		"permissions_list_open": false,
		"permissions_respond":   false,
		"channels_list":         false,
	}
	for _, tl := range BuiltinTools() {
		if _, ok := want[tl.Name]; !ok {
			t.Errorf("unexpected tool %q", tl.Name)
		}
		want[tl.Name] = true
	}
	for n, seen := range want {
		if !seen {
			t.Errorf("missing tool %q", n)
		}
	}
}
