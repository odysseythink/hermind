package trajectory

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestEpisode_JSONShapeMatchesTinker(t *testing.T) {
	ep := Episode{
		EpisodeID: "ep-1",
		Meta: Meta{
			Environment: "web-research",
			ConfigID:    "run-v1",
			Model:       "anthropic/claude-opus-4-6",
			StartedAt:   1700000000,
		},
		Steps: []Step{
			{From: "user", Value: "what is 2+2?"},
			{From: "assistant", Value: "4"},
		},
		EpisodeReward: 1.0,
	}
	data, err := json.Marshal(ep)
	if err != nil {
		t.Fatal(err)
	}
	// Tinker expects "from"/"value" pairs, "episode_id", "steps",
	// "episode_reward", and a "meta" block.
	for _, want := range []string{
		`"episode_id":"ep-1"`,
		`"from":"user"`,
		`"value":"what is 2+2?"`,
		`"episode_reward":1`,
		`"environment":"web-research"`,
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %s in %s", want, data)
		}
	}
}

func TestEpisode_EmptyStepsAllowed(t *testing.T) {
	ep := Episode{EpisodeID: "empty"}
	data, err := json.Marshal(ep)
	if err != nil {
		t.Fatal(err)
	}
	// Empty steps must render as [] not null, so the trainer's
	// iteration does not choke.
	if !strings.Contains(string(data), `"steps":[]`) {
		t.Errorf("steps not empty array: %s", data)
	}
}

func TestToJSONLRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	ep := Episode{
		EpisodeID: "rt",
		Meta:      Meta{Environment: "env", Model: "m", StartedAt: 1},
		Steps:     []Step{{From: "user", Value: "hi"}},
	}
	if err := ToJSONL(&buf, ep); err != nil {
		t.Fatal(err)
	}
	line := bytes.TrimRight(buf.Bytes(), "\n")
	got, err := FromJSONL(line)
	if err != nil {
		t.Fatal(err)
	}
	if got.EpisodeID != "rt" || len(got.Steps) != 1 || got.Steps[0].Value != "hi" {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
}

func TestReadAllJSONL(t *testing.T) {
	var buf bytes.Buffer
	for i := 0; i < 3; i++ {
		if err := ToJSONL(&buf, Episode{EpisodeID: string(rune('a' + i))}); err != nil {
			t.Fatal(err)
		}
	}
	eps, err := ReadAllJSONL(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 3 {
		t.Fatalf("got %d episodes", len(eps))
	}
}
