package benchmark

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PresetRunner executes one input against one preset and returns the record.
// Callers build this closure; typically it wires a fresh agent.Engine per
// call against a temp sqlite file.
type PresetRunner func(ctx context.Context, userMessage string) (*RunRecord, error)

// RunConfig parameterizes Run.
type RunConfig struct {
	DatasetPath string
	OutDir      string
	Presets     map[string]PresetRunner
}

// Run executes every (preset × input) combination, writing
// <OutDir>/<preset>/records.jsonl. Already-written (preset, input_id)
// pairs are skipped to support resume.
func Run(ctx context.Context, cfg RunConfig) error {
	items, err := loadDataset(cfg.DatasetPath)
	if err != nil {
		return err
	}
	for presetName, runner := range cfg.Presets {
		presetDir := filepath.Join(cfg.OutDir, presetName)
		if err := os.MkdirAll(presetDir, 0o755); err != nil {
			return fmt.Errorf("benchmark: mkdir preset: %w", err)
		}
		recPath := filepath.Join(presetDir, "records.jsonl")
		done, err := readDoneIDs(recPath)
		if err != nil {
			return err
		}
		f, err := os.OpenFile(recPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("benchmark: open records: %w", err)
		}
		enc := json.NewEncoder(f)
		for _, item := range items {
			if _, ok := done[item.ID]; ok {
				continue
			}
			rec, err := runner(ctx, item.Message)
			if err != nil {
				rec = &RunRecord{Error: err.Error()}
			}
			rec.PresetName = presetName
			rec.InputID = item.ID
			if err := enc.Encode(rec); err != nil {
				f.Close()
				return fmt.Errorf("benchmark: encode: %w", err)
			}
		}
		f.Close()
	}
	return nil
}

func loadDataset(path string) ([]InputItem, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("benchmark: open dataset: %w", err)
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 1<<20), 1<<20)
	var items []InputItem
	for s.Scan() {
		line := s.Text()
		if strings.Contains(line, `"__meta"`) {
			continue
		}
		var it InputItem
		if err := json.Unmarshal([]byte(line), &it); err != nil {
			continue
		}
		if it.ID == "" {
			continue
		}
		items = append(items, it)
	}
	return items, s.Err()
}

func readDoneIDs(path string) (map[string]struct{}, error) {
	done := map[string]struct{}{}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return done, nil
		}
		return nil, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 1<<20), 1<<20)
	for s.Scan() {
		var rec RunRecord
		if err := json.Unmarshal(s.Bytes(), &rec); err != nil {
			continue
		}
		if rec.InputID != "" {
			done[rec.InputID] = struct{}{}
		}
	}
	return done, s.Err()
}
