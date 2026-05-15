package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/replay"
	"github.com/odysseythink/hermind/storage"
	sqlitestore "github.com/odysseythink/hermind/storage/sqlite"
	"github.com/odysseythink/pantheon/benchmark"
)

func TestReplayLifecycle_Smoke(t *testing.T) {
	tmp := t.TempDir()

	// 1. Open a state.db, append a few user/assistant pairs.
	statePath := filepath.Join(tmp, "state.db")
	store, err := sqlitestore.Open(statePath)
	require.NoError(t, err)
	defer store.Close()
	require.NoError(t, store.Migrate())

	ctx := context.Background()
	ts := time.Now()
	for i, pair := range [][2]string{
		{"hi", "hello!"},
		{"what time?", "3pm"},
		{"weather?", "sunny"},
	} {
		require.NoError(t, store.AppendMessage(ctx, &storage.StoredMessage{
			Role: "user", Content: pair[0],
			Timestamp: ts.Add(time.Duration(i) * time.Minute),
		}))
		require.NoError(t, store.AppendMessage(ctx, &storage.StoredMessage{
			Role: "assistant", Content: pair[1],
			Timestamp: ts.Add(time.Duration(i)*time.Minute + time.Second),
		}))
	}

	// 2. Generate replay dataset (cold mode).
	datasetPath := filepath.Join(tmp, "dataset.jsonl")
	require.NoError(t, replay.Generate(ctx, store, replay.GenerateConfig{
		Mode: "cold", OutPath: datasetPath,
	}))

	// 3. Run benchmark.Run with a stub closure that echoes the message.
	runDir := filepath.Join(tmp, "runs")
	require.NoError(t, os.MkdirAll(runDir, 0o755))

	// Copy dataset to runDir (Render expects it there)
	datasetContent, err := os.ReadFile(datasetPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(runDir, "dataset.jsonl"), datasetContent, 0o644))

	echoRunner := func(_ context.Context, item benchmark.Item) (*benchmark.RunRecord, error) {
		return &benchmark.RunRecord{Reply: "current: " + item.GetMessage()}, nil
	}
	require.NoError(t, benchmark.Run(ctx, benchmark.RunConfig{
		DatasetPath: datasetPath,
		OutDir:      runDir,
		Presets:     map[string]benchmark.PresetRunner{"primary": echoRunner},
		LoaderFn:    replay.LoadDataset,
	}))

	// 4. Verify records were written.
	recPath := filepath.Join(runDir, "primary", "records.jsonl")
	data, err := os.ReadFile(recPath)
	require.NoError(t, err)

	dec := json.NewDecoder(strings.NewReader(string(data)))
	count := 0
	for {
		var rec benchmark.RunRecord
		if err := dec.Decode(&rec); err != nil {
			break
		}
		require.Contains(t, rec.Reply, "current:")
		count++
	}
	require.Equal(t, 3, count, "expected 3 records (3 user turns)")

	// 5. Render in ModeNone.
	mdPath := filepath.Join(runDir, "report.md")
	require.NoError(t, replay.Render(ctx, runDir, replay.RenderOptions{
		OutPath: mdPath,
		Mode:    replay.ModeNone,
	}))
	require.FileExists(t, mdPath)
	require.FileExists(t, filepath.Join(runDir, "report.json"))
}
