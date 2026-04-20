// Package trajectory defines the data model hermind emits for RL
// training. The JSON shape matches what Tinker + Atropos consume,
// so Python trainers can read hermind episodes without a translator.
//
// The package is intentionally small and has no dependencies on the
// rest of hermind (beyond stdlib). Callers elsewhere (for example
// rl/collector) convert batch-specific types into these structures
// and then push them through a Sink implementation.
package trajectory

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// Episode is one complete agent interaction. The field ordering and
// JSON tags are load-bearing: the Python trainer keys off "episode_id",
// "meta", "steps" and "episode_reward".
type Episode struct {
	EpisodeID     string  `json:"episode_id"`
	Meta          Meta    `json:"meta"`
	Steps         []Step  `json:"steps"`
	EpisodeReward float64 `json:"episode_reward"`
}

// Meta holds episode-wide metadata.
type Meta struct {
	Environment string                 `json:"environment"`
	ConfigID    string                 `json:"config_id,omitempty"`
	Model       string                 `json:"model"`
	StartedAt   int64                  `json:"started_at"`         // unix seconds
	EndedAt     int64                  `json:"ended_at,omitempty"` // unix seconds
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// Step is a single message turn inside an episode. The from/value shape
// is the Tinker-native format.
type Step struct {
	From       string  `json:"from"`  // "user" | "assistant" | "tool" | "system"
	Value      string  `json:"value"` // free-form text
	ToolName   string  `json:"tool_name,omitempty"`
	ToolCallID string  `json:"tool_call_id,omitempty"`
	Reward     float64 `json:"reward,omitempty"`
	Tokens     int     `json:"tokens,omitempty"`
}

// episodeAlias is a type alias used to implement MarshalJSON without
// triggering an infinite recursion when the JSON encoder dispatches
// back through Episode.
type episodeAlias Episode

// MarshalJSON ensures a nil Steps slice renders as [] rather than null
// so downstream trainers can iterate without special-casing empty
// episodes.
func (e Episode) MarshalJSON() ([]byte, error) {
	if e.Steps == nil {
		e.Steps = []Step{}
	}
	return json.Marshal(episodeAlias(e))
}

// ToJSONL encodes ep as a single JSONL line (payload + trailing "\n")
// to w. It's a thin wrapper over json.Marshal that callers use when
// they own the writer (for example the FileSink).
func ToJSONL(w io.Writer, ep Episode) error {
	data, err := json.Marshal(ep)
	if err != nil {
		return fmt.Errorf("trajectory: marshal: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("trajectory: write: %w", err)
	}
	if _, err := w.Write([]byte{'\n'}); err != nil {
		return fmt.Errorf("trajectory: write newline: %w", err)
	}
	return nil
}

// FromJSONL parses one JSONL line (without trailing newline) into an
// Episode. It exists so tests and downstream tools (e.g. replay
// utilities) share a single decoder path.
func FromJSONL(line []byte) (Episode, error) {
	var ep Episode
	if err := json.Unmarshal(line, &ep); err != nil {
		return Episode{}, fmt.Errorf("trajectory: unmarshal: %w", err)
	}
	return ep, nil
}

// ReadAllJSONL streams every episode out of r. It is convenience for
// tests; callers with large files should use bufio.Scanner + FromJSONL
// directly to control memory.
func ReadAllJSONL(r io.Reader) ([]Episode, error) {
	var out []Episode
	scan := bufio.NewScanner(r)
	// Allow large episode lines — default 64k is tight for realistic runs.
	scan.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scan.Scan() {
		ep, err := FromJSONL(scan.Bytes())
		if err != nil {
			return nil, err
		}
		out = append(out, ep)
	}
	return out, scan.Err()
}
