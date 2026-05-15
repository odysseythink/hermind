package replay

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/odysseythink/pantheon/benchmark"
)

func setupReportDir(t *testing.T, mode Mode) string {
	t.Helper()
	dir := t.TempDir()
	writeReplayDataset(t, filepath.Join(dir, "dataset.jsonl"), []ReplayItem{
		{ID: "replay_1", Message: "what time?", Baseline: "3pm"},
		{ID: "replay_2", Message: "weather?", Baseline: "sunny"},
	})
	writeRunRecords(t, dir, "primary", []benchmark.RunRecord{
		{InputID: "replay_1", Reply: "around 3 PM"},
		{InputID: "replay_2", Reply: "raining cats and dogs"},
	})

	if mode == ModePairwise || mode == ModeRubricPairwise {
		f, _ := os.Create(filepath.Join(dir, "pairwise.jsonl"))
		enc := json.NewEncoder(f)
		_ = enc.Encode(ReplayPairwiseVerdict{
			PresetName: "primary", InputID: "replay_1", Winner: "current", SwapAgreement: true,
		})
		_ = enc.Encode(ReplayPairwiseVerdict{
			PresetName: "primary", InputID: "replay_2", Winner: "baseline", SwapAgreement: true,
		})
		f.Close()
	}
	if mode == ModeRubricPairwise {
		f, _ := os.Create(filepath.Join(dir, "rubric.jsonl"))
		enc := json.NewEncoder(f)
		_ = enc.Encode(ReplayRubricScore{
			PresetName: "primary", InputID: "replay_1",
			SemanticMatch: 9, StyleMatch: 7, CorrectnessA: 8, Helpfulness: 9,
		})
		_ = enc.Encode(ReplayRubricScore{
			PresetName: "primary", InputID: "replay_2",
			SemanticMatch: 4, StyleMatch: 5, CorrectnessA: 3, Helpfulness: 4,
		})
		f.Close()
	}
	return dir
}

func TestRender_NoneShowsAllItems(t *testing.T) {
	dir := setupReportDir(t, ModeNone)
	mdPath := filepath.Join(dir, "report.md")
	require.NoError(t, Render(context.Background(), dir, RenderOptions{
		OutPath: mdPath,
		Mode:    ModeNone,
	}))
	content, err := os.ReadFile(mdPath)
	require.NoError(t, err)
	body := string(content)
	require.Contains(t, body, "replay_1")
	require.Contains(t, body, "replay_2")
	require.Contains(t, body, "around 3 PM")
	require.Contains(t, body, "raining cats and dogs")
}

func TestRender_PairwiseDefaultRegressionsOnly(t *testing.T) {
	dir := setupReportDir(t, ModePairwise)
	mdPath := filepath.Join(dir, "report.md")
	require.NoError(t, Render(context.Background(), dir, RenderOptions{
		OutPath: mdPath,
		Mode:    ModePairwise,
	}))
	body := string(mustReadFile(t, mdPath))
	require.NotContains(t, body, "### replay_1", "improved item should be filtered out by default")
	require.Contains(t, body, "### replay_2", "regression must be shown")
}

func TestRender_PairwiseFullShowsAll(t *testing.T) {
	dir := setupReportDir(t, ModePairwise)
	mdPath := filepath.Join(dir, "report.md")
	require.NoError(t, Render(context.Background(), dir, RenderOptions{
		OutPath: mdPath,
		Mode:    ModePairwise,
		Full:    true,
	}))
	body := string(mustReadFile(t, mdPath))
	require.Contains(t, body, "### replay_1")
	require.Contains(t, body, "### replay_2")
}

func TestRender_RubricAveragesIncluded(t *testing.T) {
	dir := setupReportDir(t, ModeRubricPairwise)
	mdPath := filepath.Join(dir, "report.md")
	require.NoError(t, Render(context.Background(), dir, RenderOptions{
		OutPath: mdPath,
		Mode:    ModeRubricPairwise,
		Full:    true,
	}))
	body := string(mustReadFile(t, mdPath))
	require.Contains(t, body, "Rubric averages")
	// Average of 9 and 4 = 6.5
	require.True(t, strings.Contains(body, "6.5") || strings.Contains(body, "6.50"),
		"semantic_match avg = 6.5; got body: %s", body)
}

func TestRender_AlsoWritesJSON(t *testing.T) {
	dir := setupReportDir(t, ModePairwise)
	require.NoError(t, Render(context.Background(), dir, RenderOptions{
		OutPath: filepath.Join(dir, "report.md"),
		Mode:    ModePairwise,
		Full:    true,
	}))
	jsonPath := filepath.Join(dir, "report.json")
	require.FileExists(t, jsonPath)
	data, err := os.ReadFile(jsonPath)
	require.NoError(t, err)
	require.True(t, json.Valid(data), "report.json must be valid JSON")
}
