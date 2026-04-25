package replay

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/storage"
	sqlitestore "github.com/odysseythink/hermind/storage/sqlite"
)

func newReplayTestStore(t *testing.T) storage.Storage {
	t.Helper()
	dir := t.TempDir()
	st, err := sqlitestore.Open(filepath.Join(dir, "state.db"))
	require.NoError(t, err)
	require.NoError(t, st.Migrate())
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func appendUserAssistantPair(t *testing.T, st storage.Storage, user, assistant string, ts time.Time) {
	t.Helper()
	require.NoError(t, st.AppendMessage(context.Background(), &storage.StoredMessage{
		Role: "user", Content: user, Timestamp: ts,
	}))
	require.NoError(t, st.AppendMessage(context.Background(), &storage.StoredMessage{
		Role: "assistant", Content: assistant, Timestamp: ts.Add(time.Second),
	}))
}

func TestGenerate_ColdMode(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	ts := time.Now()
	appendUserAssistantPair(t, st, "hi", "hello!", ts)
	appendUserAssistantPair(t, st, "what time?", "3pm", ts.Add(time.Minute))

	out := filepath.Join(t.TempDir(), "dataset.jsonl")
	cfg := GenerateConfig{Mode: "cold", OutPath: out}
	require.NoError(t, Generate(ctx, st, cfg))

	data, err := os.ReadFile(out)
	require.NoError(t, err)

	lines := splitJSONLines(data)
	require.GreaterOrEqual(t, len(lines), 3, "1 meta + 2 items")

	var meta map[string]any
	require.NoError(t, json.Unmarshal(lines[0], &meta))
	metaInner, _ := meta["__meta"].(map[string]any)
	require.Equal(t, "replay", metaInner["kind"])
	require.Equal(t, "cold", metaInner["mode"])

	var item1 ReplayItem
	require.NoError(t, json.Unmarshal(lines[1], &item1))
	require.Equal(t, "hi", item1.Message)
	require.Equal(t, "hello!", item1.Baseline)
	require.Empty(t, item1.History)
}

func TestGenerate_ContextualMode(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	ts := time.Now()
	appendUserAssistantPair(t, st, "hi", "hello!", ts)
	appendUserAssistantPair(t, st, "what time?", "3pm", ts.Add(time.Minute))

	out := filepath.Join(t.TempDir(), "dataset.jsonl")
	cfg := GenerateConfig{Mode: "contextual", HistoryCap: 20, OutPath: out}
	require.NoError(t, Generate(ctx, st, cfg))

	lines := splitJSONLines(mustReadFile(t, out))
	require.GreaterOrEqual(t, len(lines), 3)

	var item1 ReplayItem
	require.NoError(t, json.Unmarshal(lines[1], &item1))
	require.Empty(t, item1.History)

	var item2 ReplayItem
	require.NoError(t, json.Unmarshal(lines[2], &item2))
	require.Len(t, item2.History, 2)
	require.Equal(t, "hi", item2.History[0].Content.Text())
	require.Equal(t, "hello!", item2.History[1].Content.Text())
}

func TestGenerate_SkipsOrphanUserMessage(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	ts := time.Now()
	appendUserAssistantPair(t, st, "hi", "hello!", ts)
	require.NoError(t, st.AppendMessage(ctx, &storage.StoredMessage{
		Role: "user", Content: "lonely", Timestamp: ts.Add(time.Minute),
	}))

	out := filepath.Join(t.TempDir(), "dataset.jsonl")
	require.NoError(t, Generate(ctx, st, GenerateConfig{Mode: "cold", OutPath: out}))

	lines := splitJSONLines(mustReadFile(t, out))
	require.Equal(t, 2, len(lines))

	var meta map[string]any
	require.NoError(t, json.Unmarshal(lines[0], &meta))
	metaInner, _ := meta["__meta"].(map[string]any)
	require.EqualValues(t, 1, metaInner["skipped_orphans"])
}

func TestLoadDataset_RejectsSyntheticDataset(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "synthetic.jsonl")
	require.NoError(t, os.WriteFile(tmp, []byte(`{"__meta":{"kind":"synthetic","count":1}}
{"id":"a","message":"hi"}
`), 0o644))

	_, err := LoadDataset(tmp)
	require.Error(t, err, "replay loader must reject synthetic datasets")
}

func TestLoadDataset_AcceptsReplayDataset(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "replay.jsonl")
	require.NoError(t, os.WriteFile(tmp, []byte(`{"__meta":{"kind":"replay","mode":"cold","count":1}}
{"id":"replay_1","message":"hi","baseline":"hello!"}
`), 0o644))

	items, err := LoadDataset(tmp)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "replay_1", items[0].GetID())
	require.Equal(t, "hello!", items[0].GetBaseline())
}

func splitJSONLines(data []byte) [][]byte {
	var out [][]byte
	for _, line := range splitBytes(data, '\n') {
		if len(line) > 0 {
			out = append(out, line)
		}
	}
	return out
}

func splitBytes(data []byte, sep byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == sep {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func TestGenerate_PreservesToolCallsInHistory(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	ts := time.Now()

	// First user message + tool-using assistant reply.
	require.NoError(t, st.AppendMessage(ctx, &storage.StoredMessage{
		Role: "user", Content: "what's the weather in SF?", Timestamp: ts,
	}))
	toolCallsJSON := []byte(`[{"id":"toolu_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"SF\"}"}}]`)
	require.NoError(t, st.AppendMessage(ctx, &storage.StoredMessage{
		Role: "assistant", Content: "checking", ToolCalls: toolCallsJSON,
		Timestamp: ts.Add(time.Second),
	}))
	// Tool-result + final user/assistant pair.
	require.NoError(t, st.AppendMessage(ctx, &storage.StoredMessage{
		Role: "tool", Content: "Sunny, 70F", ToolCallID: "toolu_1",
		Timestamp: ts.Add(2 * time.Second),
	}))
	require.NoError(t, st.AppendMessage(ctx, &storage.StoredMessage{
		Role: "assistant", Content: "It's sunny in SF, 70F.",
		Timestamp: ts.Add(3 * time.Second),
	}))
	require.NoError(t, st.AppendMessage(ctx, &storage.StoredMessage{
		Role: "user", Content: "thanks!", Timestamp: ts.Add(4 * time.Second),
	}))
	require.NoError(t, st.AppendMessage(ctx, &storage.StoredMessage{
		Role: "assistant", Content: "you're welcome", Timestamp: ts.Add(5 * time.Second),
	}))

	out := filepath.Join(t.TempDir(), "dataset.jsonl")
	cfg := GenerateConfig{Mode: "contextual", HistoryCap: 20, OutPath: out}
	require.NoError(t, Generate(ctx, st, cfg))

	lines := splitJSONLines(mustReadFile(t, out))
	// 1 meta + 2 items (first user has no preceding history; "thanks!" should have history)
	require.GreaterOrEqual(t, len(lines), 3)

	// The "thanks!" item is the second one. Its history must include the
	// tool-using assistant message with its ToolCalls preserved.
	var thanksItem ReplayItem
	require.NoError(t, json.Unmarshal(lines[2], &thanksItem))
	require.Equal(t, "thanks!", thanksItem.Message)
	require.NotEmpty(t, thanksItem.History, "contextual mode must include preceding history")

	// Find the assistant message with the tool_use in History.
	foundToolCall := false
	for _, m := range thanksItem.History {
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			foundToolCall = true
			require.Equal(t, "toolu_1", m.ToolCalls[0].ID, "ToolCall ID must round-trip")
			require.Equal(t, "get_weather", m.ToolCalls[0].Function.Name, "ToolCall function name must round-trip")
			break
		}
	}
	require.True(t, foundToolCall, "assistant message with tool_calls must be preserved in history; got: %+v", thanksItem.History)
}
