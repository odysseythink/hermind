package replay

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/odysseythink/hermind/benchmark"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// JudgeAll runs the configured judge mode over the records in runDir.
// runDir must contain:
//   - dataset.jsonl (replay format)
//   - <preset>/records.jsonl for each preset that ran
//
// Outputs:
//   - pairwise.jsonl when mode == "pairwise" or "rubric+pairwise"
//   - rubric.jsonl   when mode == "rubric+pairwise"
//   - none mode is a no-op (the report stage handles side-by-side rendering)
func JudgeAll(ctx context.Context, runDir string, mode Mode, aux provider.Provider) error {
	switch mode {
	case ModeNone:
		return nil
	case ModePairwise:
		return judgePairwise(ctx, runDir, aux)
	case ModeRubricPairwise:
		if err := judgePairwise(ctx, runDir, aux); err != nil {
			return err
		}
		return judgeRubric(ctx, runDir, aux)
	default:
		return fmt.Errorf("replay: unknown mode %q (want none | pairwise | rubric+pairwise)", string(mode))
	}
}

func judgePairwise(ctx context.Context, runDir string, aux provider.Provider) error {
	items, err := LoadDataset(filepath.Join(runDir, "dataset.jsonl"))
	if err != nil {
		return err
	}
	itemByID := make(map[string]ReplayItem, len(items))
	for _, it := range items {
		ri, _ := it.(ReplayItem)
		itemByID[ri.ID] = ri
	}

	out, err := os.Create(filepath.Join(runDir, "pairwise.jsonl"))
	if err != nil {
		return fmt.Errorf("replay: create pairwise.jsonl: %w", err)
	}
	defer out.Close()
	enc := json.NewEncoder(out)

	presetRecords, err := loadAllRecords(runDir)
	if err != nil {
		return err
	}
	for presetName, recs := range presetRecords {
		for _, rec := range recs {
			ri, ok := itemByID[rec.InputID]
			if !ok {
				continue
			}
			v := pairwiseOnce(ctx, aux, ri, rec, presetName)
			if err := enc.Encode(v); err != nil {
				return fmt.Errorf("replay: encode pairwise: %w", err)
			}
		}
	}
	return nil
}

func pairwiseOnce(ctx context.Context, aux provider.Provider, ri ReplayItem, rec benchmark.RunRecord, presetName string) ReplayPairwiseVerdict {
	v := ReplayPairwiseVerdict{
		PresetName: presetName,
		InputID:    rec.InputID,
	}

	// Forward: A = current, B = baseline.
	winnerF, reasonF, errF := callPairwiseAux(ctx, aux, ri, rec.Reply, ri.Baseline)
	v.ReasonForward = reasonF
	if errF != nil {
		v.Error = errF.Error()
		v.Winner = "tie"
		return v
	}

	// Backward: A = baseline, B = current.
	winnerB, reasonB, errB := callPairwiseAux(ctx, aux, ri, ri.Baseline, rec.Reply)
	v.ReasonBackward = reasonB
	if errB != nil {
		v.Error = errB.Error()
		v.Winner = "tie"
		return v
	}

	// Resolve consensus.
	// Forward: "A" → current; "B" → baseline; "tie" → tie.
	// Backward: "A" → baseline; "B" → current; "tie" → tie.
	currentForward := winnerF == "A"
	currentBackward := winnerB == "B"
	switch {
	case currentForward && currentBackward:
		v.Winner = "current"
		v.SwapAgreement = true
	case !currentForward && !currentBackward && winnerF != "tie" && winnerB != "tie":
		v.Winner = "baseline"
		v.SwapAgreement = true
	case winnerF == "tie" && winnerB == "tie":
		v.Winner = "tie"
		v.SwapAgreement = true
	default:
		v.Winner = "tie"
		v.SwapAgreement = false
	}
	return v
}

func callPairwiseAux(ctx context.Context, aux provider.Provider, ri ReplayItem, replyA, replyB string) (winner, reason string, err error) {
	systemPrompt := "You are a regression judge. Given the user message and two candidate replies, pick which better answers the user.\n\nOutput JSON only: {\"winner\": \"A\" | \"B\" | \"tie\", \"reason\": \"...\"}"
	userPrompt := fmt.Sprintf(
		"User message: %s\n\nReply A: %s\n\nReply B: %s",
		ri.Message, replyA, replyB,
	)
	resp, err := aux.Complete(ctx, &provider.Request{
		SystemPrompt: systemPrompt,
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent(userPrompt)},
		},
		MaxTokens: 256,
	})
	if err != nil {
		return "", "", err
	}
	raw := strings.TrimSpace(resp.Message.Content.Text())
	var v struct {
		Winner string `json:"winner"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return "", "", fmt.Errorf("malformed aux JSON: %w; raw=%q", err, raw)
	}
	return v.Winner, v.Reason, nil
}

func judgeRubric(ctx context.Context, runDir string, aux provider.Provider) error {
	items, err := LoadDataset(filepath.Join(runDir, "dataset.jsonl"))
	if err != nil {
		return err
	}
	itemByID := make(map[string]ReplayItem, len(items))
	for _, it := range items {
		ri, _ := it.(ReplayItem)
		itemByID[ri.ID] = ri
	}

	out, err := os.Create(filepath.Join(runDir, "rubric.jsonl"))
	if err != nil {
		return fmt.Errorf("replay: create rubric.jsonl: %w", err)
	}
	defer out.Close()
	enc := json.NewEncoder(out)

	presetRecords, err := loadAllRecords(runDir)
	if err != nil {
		return err
	}
	for presetName, recs := range presetRecords {
		for _, rec := range recs {
			ri, ok := itemByID[rec.InputID]
			if !ok {
				continue
			}
			score := rubricOnce(ctx, aux, ri, rec, presetName)
			if err := enc.Encode(score); err != nil {
				return fmt.Errorf("replay: encode rubric: %w", err)
			}
		}
	}
	return nil
}

func rubricOnce(ctx context.Context, aux provider.Provider, ri ReplayItem, rec benchmark.RunRecord, presetName string) ReplayRubricScore {
	score := ReplayRubricScore{
		PresetName: presetName,
		InputID:    rec.InputID,
	}
	systemPrompt := "You are a regression judge. Score the current reply A relative to baseline reply B on four dimensions (0-10 each).\n\nOutput JSON only: {\"semantic_match\":N, \"style_match\":N, \"correctness_a\":N, \"helpfulness\":N, \"reason\":\"...\"}"
	userPrompt := fmt.Sprintf(
		"User message: %s\n\nReply A (current): %s\n\nReply B (baseline): %s",
		ri.Message, rec.Reply, ri.Baseline,
	)
	resp, err := aux.Complete(ctx, &provider.Request{
		SystemPrompt: systemPrompt,
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent(userPrompt)},
		},
		MaxTokens: 256,
	})
	if err != nil {
		score.Error = err.Error()
		return score
	}
	raw := strings.TrimSpace(resp.Message.Content.Text())
	var parsed struct {
		SemanticMatch int    `json:"semantic_match"`
		StyleMatch    int    `json:"style_match"`
		CorrectnessA  int    `json:"correctness_a"`
		Helpfulness   int    `json:"helpfulness"`
		Reason        string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		score.Error = fmt.Sprintf("malformed rubric JSON: %v; raw=%q", err, raw)
		return score
	}
	score.SemanticMatch = parsed.SemanticMatch
	score.StyleMatch = parsed.StyleMatch
	score.CorrectnessA = parsed.CorrectnessA
	score.Helpfulness = parsed.Helpfulness
	score.Reason = parsed.Reason
	return score
}

func loadAllRecords(runDir string) (map[string][]benchmark.RunRecord, error) {
	entries, err := os.ReadDir(runDir)
	if err != nil {
		return nil, fmt.Errorf("replay: read runDir: %w", err)
	}
	out := make(map[string][]benchmark.RunRecord)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		recPath := filepath.Join(runDir, e.Name(), "records.jsonl")
		data, err := os.ReadFile(recPath)
		if err != nil {
			continue
		}
		var recs []benchmark.RunRecord
		dec := json.NewDecoder(strings.NewReader(string(data)))
		for {
			var rec benchmark.RunRecord
			if err := dec.Decode(&rec); err != nil {
				break
			}
			recs = append(recs, rec)
		}
		out[e.Name()] = recs
	}
	return out, nil
}
