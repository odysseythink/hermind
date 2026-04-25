package presence

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// stubSource lets tests build a Composite from canned votes.
type stubSource struct {
	name string
	vote Vote
}

func (s stubSource) Name() string             { return s.name }
func (s stubSource) Vote(_ time.Time) Vote    { return s.vote }

func TestComposite_ZeroSourcesNotAvailable(t *testing.T) {
	c := NewComposite()
	require.False(t, c.Available(time.Now()))
}

func TestComposite_AllUnknownNotAvailable(t *testing.T) {
	c := NewComposite(
		stubSource{name: "a", vote: Unknown},
		stubSource{name: "b", vote: Unknown},
	)
	require.False(t, c.Available(time.Now()), "all Unknown ⇒ fail-closed, not available")
}

func TestComposite_AtLeastOneAbsentAvailable(t *testing.T) {
	c := NewComposite(
		stubSource{name: "a", vote: Unknown},
		stubSource{name: "b", vote: Absent},
	)
	require.True(t, c.Available(time.Now()))
}

func TestComposite_AnyPresentVetoes(t *testing.T) {
	// Present + Absent + Unknown should still be NOT available.
	c := NewComposite(
		stubSource{name: "a", vote: Present},
		stubSource{name: "b", vote: Absent},
		stubSource{name: "c", vote: Unknown},
	)
	require.False(t, c.Available(time.Now()))
}

func TestComposite_SourcesPreservesOrder(t *testing.T) {
	c := NewComposite(
		stubSource{name: "a", vote: Unknown},
		stubSource{name: "b", vote: Absent},
		stubSource{name: "c", vote: Present},
	)
	out := c.Sources(time.Now())
	require.Len(t, out, 3)
	require.Equal(t, "a", out[0].Name)
	require.Equal(t, "b", out[1].Name)
	require.Equal(t, "c", out[2].Name)
	require.Equal(t, "Unknown", out[0].Vote.String())
	require.Equal(t, "Absent", out[1].Vote.String())
	require.Equal(t, "Present", out[2].Vote.String())
}
