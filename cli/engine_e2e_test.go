package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/agent"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider/anthropic"
	"github.com/odysseythink/hermind/storage/sqlite"
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/hermind/tool/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEngineE2E_ToolRoundTrip verifies the full stack: a mocked
// Anthropic server emits a tool_use event, the engine dispatches the
// tool via the registry, the result is fed back to the LLM, and the
// LLM's final text answer is returned. Renamed from the old
// TestEndToEndToolLoop when the TUI was deleted in Phase 3.
// sessionrun.Run's tool_call/tool_result publishing has its own unit
// test in api/sessionrun/runner_test.go.
func TestEngineE2E_ToolRoundTrip(t *testing.T) {
	// Prepare a file the mocked LLM will "request" via read_file
	dir := t.TempDir()
	testFilePath := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(testFilePath, []byte("hi from tool"), 0o644))

	// Mock Anthropic server: turn 1 returns tool_use, turn 2 returns text
	turn := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		turn++
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)

		var events []string
		switch turn {
		case 1:
			// tool_use response
			events = []string{
				"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_01\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-opus-4-6\",\"content\":[],\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\n",
				"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"tool_e2e\",\"name\":\"read_file\",\"input\":{}}}\n\n",
				"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"path\\\":\\\"" + testFilePath + "\\\"}\"}}\n\n",
				"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n",
				"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"},\"usage\":{\"output_tokens\":5}}\n\n",
				"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
			}
		case 2:
			// Final text response
			events = []string{
				"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_02\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-opus-4-6\",\"content\":[],\"usage\":{\"input_tokens\":25,\"output_tokens\":0}}}\n\n",
				"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Got it.\"}}\n\n",
				"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":3}}\n\n",
				"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
			}
		default:
			t.Fatalf("unexpected turn %d", turn)
		}

		for _, e := range events {
			_, _ = io.WriteString(w, e)
			flusher.Flush()
		}
		_, _ = io.ReadAll(r.Body) // drain request body
	}))
	defer srv.Close()

	// Build provider
	p, err := anthropic.New(config.ProviderConfig{
		Provider: "anthropic",
		APIKey:   "test",
		BaseURL:  srv.URL,
		Model:    "claude-opus-4-6",
	})
	require.NoError(t, err)

	// Fresh storage + tool registry
	store, err := sqlite.Open(filepath.Join(dir, "state.db"))
	require.NoError(t, err)
	require.NoError(t, store.Migrate())
	defer store.Close()

	reg := tool.NewRegistry()
	file.RegisterAll(reg)

	// Run the engine
	engine := agent.NewEngineWithTools(p, store, reg, config.AgentConfig{MaxTurns: 10}, "cli")
	result, err := engine.RunConversation(context.Background(), &agent.RunOptions{
		UserMessage: "read " + testFilePath,
		Model:       "claude-opus-4-6",
	})
	require.NoError(t, err)

	// 2 iterations: tool_use then final text
	assert.Equal(t, 2, result.Iterations)
	assert.Equal(t, "Got it.", result.Response.Content.Text())

	// Verify the tool was actually called — the tool_result message should
	// be the 3rd message (user, assistant_tool_use, user_tool_result, assistant_text)
	require.Len(t, result.Messages, 4)
	toolResultMsg := result.Messages[2]
	assert.Equal(t, message.RoleUser, toolResultMsg.Role)
	require.False(t, toolResultMsg.Content.IsText())
	blocks := toolResultMsg.Content.Blocks()
	require.Len(t, blocks, 1)
	assert.Equal(t, "tool_result", blocks[0].Type)
	assert.Equal(t, "tool_e2e", blocks[0].ToolUseID)
	// The tool result should contain the file content
	var toolResultData map[string]any
	require.NoError(t, json.Unmarshal([]byte(blocks[0].ToolResult), &toolResultData))
	assert.Equal(t, "hi from tool", toolResultData["content"])
}
