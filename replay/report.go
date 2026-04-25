package replay

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RenderOptions controls report rendering.
type RenderOptions struct {
	// OutPath is the markdown output path. The JSON output uses the
	// same directory with the name "report.json".
	OutPath string
	// Mode is the judge mode used. Determines what tables appear.
	Mode Mode
	// Full includes all items in the report; default (false) shows
	// only regressions in pairwise / rubric+pairwise modes. In
	// ModeNone, all items are always included.
	Full bool
}

// reportItem is the per-item view bundled into report.json.
type reportItem struct {
	InputID  string                 `json:"input_id"`
	UserMsg  string                 `json:"user_message"`
	History  int                    `json:"history_count"`
	Baseline string                 `json:"baseline"`
	Current  string                 `json:"current"`
	Pairwise *ReplayPairwiseVerdict `json:"pairwise,omitempty"`
	Rubric   *ReplayRubricScore     `json:"rubric,omitempty"`
}

// reportJSON is the JSON shape written to report.json.
type reportJSON struct {
	Mode    string         `json:"mode"`
	Items   []reportItem   `json:"items"`
	Summary map[string]any `json:"summary,omitempty"`
}

// Render writes a markdown report and a sibling report.json.
// It reads dataset.jsonl, records from all presets, and optional
// pairwise.jsonl and rubric.jsonl, then writes:
// - report.md: human-readable markdown with optional regression filter
// - report.json: machine-readable JSON with all items (unfiltered)
func Render(ctx context.Context, runDir string, opts RenderOptions) error {
	items, err := LoadDataset(filepath.Join(runDir, "dataset.jsonl"))
	if err != nil {
		return err
	}
	itemByID := make(map[string]ReplayItem, len(items))
	for _, it := range items {
		ri, _ := it.(ReplayItem)
		itemByID[ri.ID] = ri
	}
	records, err := loadAllRecords(runDir)
	if err != nil {
		return err
	}

	var pairwiseByID map[string]ReplayPairwiseVerdict
	if opts.Mode == ModePairwise || opts.Mode == ModeRubricPairwise {
		pairwiseByID, _ = loadPairwise(filepath.Join(runDir, "pairwise.jsonl"))
	}
	var rubricByID map[string]ReplayRubricScore
	if opts.Mode == ModeRubricPairwise {
		rubricByID, _ = loadRubric(filepath.Join(runDir, "rubric.jsonl"))
	}

	var rows []reportItem
	verdictCounts := map[string]int{"current": 0, "baseline": 0, "tie": 0}
	rubricSums := map[string]int{
		"semantic_match": 0, "style_match": 0, "correctness_a": 0, "helpfulness": 0,
	}
	rubricCount := 0

	for presetName, recs := range records {
		_ = presetName
		for _, rec := range recs {
			ri, ok := itemByID[rec.InputID]
			if !ok {
				continue
			}
			row := reportItem{
				InputID:  rec.InputID,
				UserMsg:  ri.Message,
				History:  len(ri.History),
				Baseline: ri.Baseline,
				Current:  rec.Reply,
			}
			if v, ok := pairwiseByID[rec.InputID]; ok {
				row.Pairwise = &v
				verdictCounts[v.Winner]++
			}
			if rs, ok := rubricByID[rec.InputID]; ok {
				row.Rubric = &rs
				rubricSums["semantic_match"] += rs.SemanticMatch
				rubricSums["style_match"] += rs.StyleMatch
				rubricSums["correctness_a"] += rs.CorrectnessA
				rubricSums["helpfulness"] += rs.Helpfulness
				rubricCount++
			}
			rows = append(rows, row)
		}
	}

	displayRows := rows
	if !opts.Full && (opts.Mode == ModePairwise || opts.Mode == ModeRubricPairwise) {
		displayRows = displayRows[:0]
		for _, r := range rows {
			if r.Pairwise != nil && r.Pairwise.Winner == "baseline" {
				displayRows = append(displayRows, r)
			}
		}
	}

	var md strings.Builder
	fmt.Fprintf(&md, "# Replay Report\n\n")
	fmt.Fprintf(&md, "Mode: `%s`\n", string(opts.Mode))
	fmt.Fprintf(&md, "Filter: %s\n\n", filterLabel(opts))

	if opts.Mode != ModeNone {
		fmt.Fprintf(&md, "## Summary\n\n")
		fmt.Fprintf(&md, "| Verdict | Count |\n|---|---|\n")
		fmt.Fprintf(&md, "| current improved | %d |\n", verdictCounts["current"])
		fmt.Fprintf(&md, "| baseline better (regression) | %d |\n", verdictCounts["baseline"])
		fmt.Fprintf(&md, "| tie | %d |\n\n", verdictCounts["tie"])
	}

	if opts.Mode == ModeRubricPairwise && rubricCount > 0 {
		fmt.Fprintf(&md, "## Rubric averages\n\n")
		fmt.Fprintf(&md, "| Dimension | Avg |\n|---|---|\n")
		for _, dim := range []string{"semantic_match", "style_match", "correctness_a", "helpfulness"} {
			avg := float64(rubricSums[dim]) / float64(rubricCount)
			fmt.Fprintf(&md, "| %s | %.1f |\n", dim, avg)
		}
		md.WriteString("\n")
	}

	fmt.Fprintf(&md, "## Items\n\n")
	for _, r := range displayRows {
		fmt.Fprintf(&md, "### %s", r.InputID)
		if r.Pairwise != nil {
			switch r.Pairwise.Winner {
			case "baseline":
				fmt.Fprintf(&md, " (regression ⚠️)")
			case "current":
				fmt.Fprintf(&md, " (improved)")
			case "tie":
				fmt.Fprintf(&md, " (tie)")
			}
		}
		md.WriteString("\n\n")
		fmt.Fprintf(&md, "**User**: %s\n\n", r.UserMsg)
		if r.History > 0 {
			fmt.Fprintf(&md, "**History**: %d preceding messages\n\n", r.History)
		}
		fmt.Fprintf(&md, "**Baseline reply**:\n> %s\n\n", strings.ReplaceAll(r.Baseline, "\n", "\n> "))
		fmt.Fprintf(&md, "**Current reply**:\n> %s\n\n", strings.ReplaceAll(r.Current, "\n", "\n> "))
		if r.Rubric != nil {
			fmt.Fprintf(&md, "**Rubric**: semantic=%d style=%d correctness_a=%d helpfulness=%d\n\n",
				r.Rubric.SemanticMatch, r.Rubric.StyleMatch, r.Rubric.CorrectnessA, r.Rubric.Helpfulness)
		}
		md.WriteString("---\n\n")
	}

	if err := os.WriteFile(opts.OutPath, []byte(md.String()), 0o644); err != nil {
		return fmt.Errorf("replay: write report.md: %w", err)
	}

	jsonPath := filepath.Join(filepath.Dir(opts.OutPath), "report.json")
	jsonOut := reportJSON{
		Mode:  string(opts.Mode),
		Items: rows,
		Summary: map[string]any{
			"verdicts":     verdictCounts,
			"rubric_count": rubricCount,
		},
	}
	jsonData, err := json.MarshalIndent(jsonOut, "", "  ")
	if err != nil {
		return fmt.Errorf("replay: marshal report.json: %w", err)
	}
	if err := os.WriteFile(jsonPath, jsonData, 0o644); err != nil {
		return fmt.Errorf("replay: write report.json: %w", err)
	}
	return nil
}

func filterLabel(opts RenderOptions) string {
	if opts.Full || opts.Mode == ModeNone {
		return "all items"
	}
	return "regressions only"
}

func loadPairwise(path string) (map[string]ReplayPairwiseVerdict, error) {
	out := map[string]ReplayPairwiseVerdict{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	for {
		var v ReplayPairwiseVerdict
		if err := dec.Decode(&v); err != nil {
			break
		}
		out[v.InputID] = v
	}
	return out, nil
}

func loadRubric(path string) (map[string]ReplayRubricScore, error) {
	out := map[string]ReplayRubricScore{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	for {
		var v ReplayRubricScore
		if err := dec.Decode(&v); err != nil {
			break
		}
		out[v.InputID] = v
	}
	return out, nil
}
