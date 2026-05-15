// Package replay re-runs real historical user turns from state.db
// against the current configuration and compares the new replies to
// the historical baselines.
//
// Replay is a strict sibling of the benchmark package: the benchmark
// runner drives both flows, but replay owns its own dataset format
// (with optional history + baseline) and its own judge mode tree
// (pairwise / none / rubric+pairwise) tuned for current-vs-baseline
// comparison.
package replay

import (
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/pantheon/benchmark"
)

// ReplayItem is one row of a replay dataset. It implements
// benchmark.Item so it flows through benchmark.Run unchanged.
type ReplayItem struct {
	ID       string `json:"id"`
	Category string `json:"category,omitempty"`
	Message  string `json:"message"`
	// History is the conversation context preceding the target turn.
	// Empty for "cold" mode (target message replayed in isolation);
	// non-empty for "contextual" mode (target turn replayed with full
	// preceding history).
	History []message.HermindMessage `json:"history,omitempty"`
	// Baseline is the historical assistant reply that immediately
	// followed Message in state.db. Required for replay items.
	Baseline string `json:"baseline"`
}

// Compile-time interface satisfaction.
var _ benchmark.Item = ReplayItem{}

// GetID implements benchmark.Item.
func (r ReplayItem) GetID() string { return r.ID }

// GetMessage implements benchmark.Item.
func (r ReplayItem) GetMessage() string { return r.Message }

// GetCategory implements benchmark.Item.
func (r ReplayItem) GetCategory() string { return r.Category }

// GetBaseline implements benchmark.Item.
func (r ReplayItem) GetBaseline() string { return r.Baseline }

// GetHistory implements benchmark.Item.
func (r ReplayItem) GetHistory() []message.HermindMessage { return r.History }

// Mode selects the replay judge strategy.
type Mode string

const (
	// ModeNone skips the judge entirely. The report shows side-by-side
	// current vs baseline replies but no automated verdict.
	ModeNone Mode = "none"
	// ModePairwise runs aux × 2 per item with position-swap consensus.
	ModePairwise Mode = "pairwise"
	// ModeRubricPairwise runs ModePairwise plus a parallel rubric pass
	// (one extra aux call per item) scoring multiple dimensions.
	ModeRubricPairwise Mode = "rubric+pairwise"
)

// ReplayPairwiseVerdict is one pairwise comparison between the current
// reply and the historical baseline.
type ReplayPairwiseVerdict struct {
	PresetName     string `json:"preset_name"`
	InputID        string `json:"input_id"`
	Winner         string `json:"winner"`         // "current" | "baseline" | "tie"
	SwapAgreement  bool   `json:"swap_agreement"` // both swap directions agreed
	ReasonForward  string `json:"reason_forward"`
	ReasonBackward string `json:"reason_backward"`
	Error          string `json:"error,omitempty"`
}

// ReplayRubricScore is one multi-dimension rubric judgment.
type ReplayRubricScore struct {
	PresetName    string `json:"preset_name"`
	InputID       string `json:"input_id"`
	SemanticMatch int    `json:"semantic_match"`
	StyleMatch    int    `json:"style_match"`
	CorrectnessA  int    `json:"correctness_a"`
	Helpfulness   int    `json:"helpfulness"`
	Reason        string `json:"reason"`
	Error         string `json:"error,omitempty"`
}
