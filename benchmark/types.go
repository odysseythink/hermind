// Package benchmark runs local A/B evaluation of hermind config presets
// against a synthetic dataset using LLM-as-judge scoring and pairwise
// preference (with position-swap consensus).
package benchmark

import (
	"time"

	"github.com/odysseythink/hermind/message"
)

// InputItem is one row of the benchmark dataset.
type InputItem struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Message  string `json:"message"`
}

// DatasetMeta is the first-line metadata record in dataset.jsonl.
type DatasetMeta struct {
	Seed        int64     `json:"seed"`
	Model       string    `json:"generator_model"`
	GeneratedAt time.Time `json:"generated_at"`
	Count       int       `json:"count"`
}

// InjectedSnapshot is the minimal view of a memory injected during a run,
// persisted in RunRecord for later judging.
type InjectedSnapshot struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

// RunRecord is one (preset, input) execution result persisted to JSONL.
type RunRecord struct {
	PresetName       string             `json:"preset_name"`
	InputID          string             `json:"input_id"`
	Reply            string             `json:"reply"`
	InjectedMemories []InjectedSnapshot `json:"injected_memories"`
	InjectedSkills   []string           `json:"injected_skills"`
	Iterations       int                `json:"iterations"`
	Usage            message.Usage      `json:"usage"`
	Error            string             `json:"error,omitempty"`
}

// RubricScore is one judgment per (preset, input).
type RubricScore struct {
	PresetName      string `json:"preset_name"`
	InputID         string `json:"input_id"`
	Correctness     int    `json:"correctness"`
	MemoryRelevance int    `json:"memory_relevance"`
	SkillApplied    int    `json:"skill_applied"`
	Overall         int    `json:"overall"`
	Reason          string `json:"reason"`
	Error           string `json:"error,omitempty"`
}

// PairwiseVerdict is one pairwise comparison with position-swap consensus.
type PairwiseVerdict struct {
	InputID   string `json:"input_id"`
	PresetA   string `json:"preset_a"`
	PresetB   string `json:"preset_b"`
	WinnerAB  string `json:"winner_ab"` // "a" | "b" | "tie"
	WinnerBA  string `json:"winner_ba"`
	Consensus string `json:"consensus"` // "a" | "b" | "tie"
	Reason    string `json:"reason"`
}
