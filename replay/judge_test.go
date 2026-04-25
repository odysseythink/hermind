package replay

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/benchmark"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// stubAux returns canned JSON strings cycling through the provided list.
type stubAux struct {
	responses []string
	idx       int
	calls     int
}

func (s *stubAux) Name() string { return "stub-aux" }
func (s *stubAux) Complete(_ context.Context, _ *provider.Request) (*provider.Response, error) {
	resp := s.responses[s.idx%len(s.responses)]
	s.idx++
	s.calls++
	return &provider.Response{
		Message:      message.Message{Role: message.RoleAssistant, Content: message.TextContent(resp)},
		FinishReason: "stop",
		Usage:        message.Usage{InputTokens: 10, OutputTokens: 20},
	}, nil
}
func (s *stubAux) Stream(_ context.Context, _ *provider.Request) (provider.Stream, error) {
	return nil, nil
}
func (s *stubAux) ModelInfo(_ string) *provider.ModelInfo                  { return nil }
func (s *stubAux) EstimateTokens(_ string, _ string) (int, error)          { return 0, nil }
func (s *stubAux) Available() bool                                          { return true }

func writeRunRecords(t *testing.T, dir, preset string, recs []benchmark.RunRecord) {
	t.Helper()
	presetDir := filepath.Join(dir, preset)
	require.NoError(t, os.MkdirAll(presetDir, 0o755))
	f, err := os.Create(filepath.Join(presetDir, "records.jsonl"))
	require.NoError(t, err)
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, rec := range recs {
		require.NoError(t, enc.Encode(rec))
	}
}

func writeReplayDataset(t *testing.T, path string, items []ReplayItem) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	enc := json.NewEncoder(f)
	require.NoError(t, enc.Encode(map[string]any{
		"__meta": map[string]any{"kind": "replay", "mode": "cold", "count": len(items)},
	}))
	for _, item := range items {
		require.NoError(t, enc.Encode(item))
	}
}

func TestJudgeAll_NoneIsNoop(t *testing.T) {
	dir := t.TempDir()
	writeRunRecords(t, dir, "primary", []benchmark.RunRecord{
		{InputID: "replay_1", Reply: "current reply"},
	})
	writeReplayDataset(t, filepath.Join(dir, "dataset.jsonl"), []ReplayItem{
		{ID: "replay_1", Message: "hi", Baseline: "old reply"},
	})

	aux := &stubAux{responses: []string{`{"winner":"current","reason":"x"}`}}
	require.NoError(t, JudgeAll(context.Background(), dir, ModeNone, aux))
	require.Equal(t, 0, aux.calls, "ModeNone must skip aux entirely")

	_, err := os.Stat(filepath.Join(dir, "pairwise.jsonl"))
	require.True(t, os.IsNotExist(err))
}

func TestJudgeAll_PairwiseConsensus(t *testing.T) {
	dir := t.TempDir()
	writeRunRecords(t, dir, "primary", []benchmark.RunRecord{
		{InputID: "replay_1", Reply: "current reply"},
	})
	writeReplayDataset(t, filepath.Join(dir, "dataset.jsonl"), []ReplayItem{
		{ID: "replay_1", Message: "hi", Baseline: "old reply"},
	})

	// Forward (A=current, B=baseline) → "A" means current wins.
	// Backward (A=baseline, B=current) → "B" means current wins.
	// Both agree current wins.
	aux := &stubAux{responses: []string{
		`{"winner":"A","reason":"forward A wins"}`,
		`{"winner":"B","reason":"backward B wins"}`,
	}}
	require.NoError(t, JudgeAll(context.Background(), dir, ModePairwise, aux))
	require.Equal(t, 2, aux.calls)

	pairwisePath := filepath.Join(dir, "pairwise.jsonl")
	data, err := os.ReadFile(pairwisePath)
	require.NoError(t, err)
	var v ReplayPairwiseVerdict
	require.NoError(t, json.Unmarshal(data, &v))
	require.Equal(t, "replay_1", v.InputID)
	require.Equal(t, "current", v.Winner)
	require.True(t, v.SwapAgreement)
}

func TestJudgeAll_PairwiseDisagreementIsTie(t *testing.T) {
	dir := t.TempDir()
	writeRunRecords(t, dir, "primary", []benchmark.RunRecord{
		{InputID: "replay_1", Reply: "current"},
	})
	writeReplayDataset(t, filepath.Join(dir, "dataset.jsonl"), []ReplayItem{
		{ID: "replay_1", Message: "hi", Baseline: "old"},
	})

	// Forward "A" wins (current). Backward "A" wins (baseline). Disagreement → tie.
	aux := &stubAux{responses: []string{
		`{"winner":"A","reason":"current is better"}`,
		`{"winner":"A","reason":"baseline is better"}`,
	}}
	require.NoError(t, JudgeAll(context.Background(), dir, ModePairwise, aux))

	data, err := os.ReadFile(filepath.Join(dir, "pairwise.jsonl"))
	require.NoError(t, err)
	var v ReplayPairwiseVerdict
	require.NoError(t, json.Unmarshal(data, &v))
	require.Equal(t, "tie", v.Winner)
	require.False(t, v.SwapAgreement)
}

func TestJudgeAll_RubricPairwiseProducesBothFiles(t *testing.T) {
	dir := t.TempDir()
	writeRunRecords(t, dir, "primary", []benchmark.RunRecord{
		{InputID: "replay_1", Reply: "current"},
	})
	writeReplayDataset(t, filepath.Join(dir, "dataset.jsonl"), []ReplayItem{
		{ID: "replay_1", Message: "hi", Baseline: "old"},
	})

	aux := &stubAux{responses: []string{
		`{"winner":"A","reason":"f"}`,                                                          // pairwise forward
		`{"winner":"B","reason":"b"}`,                                                          // pairwise backward
		`{"semantic_match":8,"style_match":7,"correctness_a":9,"helpfulness":8,"reason":"r"}`, // rubric
	}}
	require.NoError(t, JudgeAll(context.Background(), dir, ModeRubricPairwise, aux))
	require.Equal(t, 3, aux.calls)

	require.FileExists(t, filepath.Join(dir, "pairwise.jsonl"))
	require.FileExists(t, filepath.Join(dir, "rubric.jsonl"))

	rubricData, err := os.ReadFile(filepath.Join(dir, "rubric.jsonl"))
	require.NoError(t, err)
	var rs ReplayRubricScore
	require.NoError(t, json.Unmarshal(rubricData, &rs))
	require.Equal(t, 8, rs.SemanticMatch)
	require.Equal(t, 7, rs.StyleMatch)
	require.Equal(t, 9, rs.CorrectnessA)
	require.Equal(t, 8, rs.Helpfulness)
}

func TestJudgeAll_MalformedAuxJSONLogsErrorContinues(t *testing.T) {
	dir := t.TempDir()
	writeRunRecords(t, dir, "primary", []benchmark.RunRecord{
		{InputID: "replay_1", Reply: "current"},
	})
	writeReplayDataset(t, filepath.Join(dir, "dataset.jsonl"), []ReplayItem{
		{ID: "replay_1", Message: "hi", Baseline: "old"},
	})

	aux := &stubAux{responses: []string{"this is not JSON", "still not JSON"}}
	require.NoError(t, JudgeAll(context.Background(), dir, ModePairwise, aux))

	data, err := os.ReadFile(filepath.Join(dir, "pairwise.jsonl"))
	require.NoError(t, err)
	var v ReplayPairwiseVerdict
	require.NoError(t, json.Unmarshal(data, &v))
	require.NotEmpty(t, v.Error, "judge errors must populate Error field")
}

func TestJudgeAll_UnknownModeError(t *testing.T) {
	err := JudgeAll(context.Background(), t.TempDir(), Mode("bogus"), &stubAux{})
	require.Error(t, err)
}
