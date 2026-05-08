package storage

import "testing"

func TestMemoryTypeConstants(t *testing.T) {
	cases := map[string]string{
		"episodic":         MemTypeEpisodic,
		"semantic":         MemTypeSemantic,
		"preference":       MemTypePreference,
		"project_state":    MemTypeProjectState,
		"procedural_obs":   MemTypeProceduralObservation,
		"working_summary":  MemTypeWorkingSummary,
	}
	for want, got := range cases {
		if want != got {
			t.Errorf("expected %q, got %q", want, got)
		}
	}
}
