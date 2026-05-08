// Package presence answers a single question for background workers:
// "is the user effectively away right now, so deferable work can run?"
//
// Each Source contributes a three-state Vote (Unknown / Absent / Present).
// A Composite ANDs together: any Present vetoes; at least one Absent
// with no Present unlocks; all-Unknown fails closed (no positive
// evidence of absence).
package presence

import "time"

// Vote is a three-state opinion on user presence.
type Vote int

const (
	// Unknown means the source has no opinion (signal disabled, ambiguous,
	// or not enough data).
	Unknown Vote = iota
	// Absent means the source confidently asserts the user is not at the
	// keyboard.
	Absent
	// Present means the source confidently asserts the user IS at the
	// keyboard. Any Present vetoes the Composite regardless of other
	// votes.
	Present
)

// String renders Vote for diagnostics and JSON serialization.
func (v Vote) String() string {
	switch v {
	case Absent:
		return "Absent"
	case Present:
		return "Present"
	default:
		return "Unknown"
	}
}

// Source contributes one signal to the Composite.
type Source interface {
	// Name is a stable diagnostic identifier (e.g. "http_idle").
	Name() string
	// Vote returns the source's current opinion at `now`.
	Vote(now time.Time) Vote
}

// Provider is what background consumers call to ask "may I run now?".
type Provider interface {
	// Available reports whether deferable work may run at `now`.
	Available(now time.Time) bool
	// Sources returns one snapshot per registered source for diagnostics.
	Sources(now time.Time) []SourceVote
}

// SourceVote is a single source's name + current vote, returned by
// Provider.Sources for /api/memory/health.
type SourceVote struct {
	Name string
	Vote Vote
}

// Composite ORs Absent votes while letting any Present veto.
// All-Unknown fails closed.
type Composite struct {
	sources []Source
}

// NewComposite constructs a Composite from a stable-ordered list of
// sources. Order is preserved by Sources().
func NewComposite(sources ...Source) *Composite {
	return &Composite{sources: sources}
}

// Available implements Provider.
func (c *Composite) Available(now time.Time) bool {
	hasAbsent := false
	for _, s := range c.sources {
		switch s.Vote(now) {
		case Present:
			return false
		case Absent:
			hasAbsent = true
		}
	}
	return hasAbsent
}

// Sources implements Provider.
func (c *Composite) Sources(now time.Time) []SourceVote {
	out := make([]SourceVote, 0, len(c.sources))
	for _, s := range c.sources {
		out = append(out, SourceVote{Name: s.Name(), Vote: s.Vote(now)})
	}
	return out
}
