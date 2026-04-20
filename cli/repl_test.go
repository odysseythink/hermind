// cli/repl_test.go
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/agent"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/anthropic"
	"github.com/odysseythink/hermind/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEndToEndSingleTurn proves the full stack works: user message →
// anthropic (mock) → stream → storage → ConversationResult.
func TestEndToEndSingleTurn(t *testing.T) {
	// Mock Anthropic server that returns a single-event SSE stream
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/messages", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), `"stream":true`)

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)

		events := []string{
			"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_01\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-opus-4-6\",\"content\":[],\"usage\":{\"input_tokens\":8,\"output_tokens\":0}}}\n\n",
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi!\"}}\n\n",
			"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2}}\n\n",
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
		}
		for _, e := range events {
			_, _ = io.WriteString(w, e)
			flusher.Flush()
		}
	}))
	defer srv.Close()

	// Build provider pointing at the mock server
	p, err := anthropic.New(config.ProviderConfig{
		Provider: "anthropic",
		APIKey:   "test",
		BaseURL:  srv.URL,
		Model:    "claude-opus-4-6",
	})
	require.NoError(t, err)

	// Fresh SQLite store in a temp dir
	dir := t.TempDir()
	store, err := sqlite.Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	require.NoError(t, store.Migrate())
	defer store.Close()

	// Capture streaming output
	var streamed bytes.Buffer
	engine := agent.NewEngine(p, store, config.AgentConfig{MaxTurns: 10}, "cli")
	engine.SetStreamDeltaCallback(func(d *provider.StreamDelta) {
		if d != nil {
			streamed.WriteString(d.Content)
		}
	})

	result, err := engine.RunConversation(context.Background(), &agent.RunOptions{
		UserMessage: "hi",
		SessionID:   "e2e-session",
		Model:       "claude-opus-4-6",
	})
	require.NoError(t, err)

	// Verify the response
	assert.Equal(t, "Hi!", result.Response.Content.Text())
	assert.Equal(t, "Hi!", streamed.String())
	assert.Equal(t, 8, result.Usage.InputTokens)
	assert.Equal(t, 2, result.Usage.OutputTokens)

	// Verify both messages were persisted
	msgs, err := store.GetMessages(context.Background(), "e2e-session", 10, 0)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "assistant", msgs[1].Role)

	// Verify session usage was updated
	sess, err := store.GetSession(context.Background(), "e2e-session")
	require.NoError(t, err)
	assert.Equal(t, 8, sess.Usage.InputTokens)
	assert.Equal(t, 2, sess.Usage.OutputTokens)

	// Verify content is JSON-encoded message.Content
	var userContent message.Content
	require.NoError(t, json.Unmarshal([]byte(msgs[0].Content), &userContent))
	assert.Equal(t, "hi", userContent.Text())

	// Silence unused-import warnings in test files
	_ = os.Stdout
	_ = strings.TrimSpace
}

func TestREPL_RespectsDisabledSkills(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERMIND_HOME", dir)

	// Seed two skills.
	for _, n := range []string{"keep", "drop"} {
		p := filepath.Join(dir, "skills", "cat", n)
		_ = os.MkdirAll(p, 0o755)
		body := "---\nname: " + n + "\ndescription: t\n---\nbody"
		_ = os.WriteFile(filepath.Join(p, "SKILL.md"), []byte(body), 0o644)
	}
	// Seed config disabling "drop".
	cfgPath := filepath.Join(dir, "config.yaml")
	cfgYAML := []byte("model: test\nskills:\n  disabled: [drop]\n")
	_ = os.WriteFile(cfgPath, cfgYAML, 0o644)

	app, err := NewApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	reg, _ := loadSkills(app)
	active := reg.Active()
	if len(active) != 1 || active[0].Name != "keep" {
		t.Errorf("active = %v, want [keep]", active)
	}
}
