package skills

import (
	"context"
	"encoding/json"
	"time"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/extensions/judge"
	pskills "github.com/odysseythink/pantheon/extensions/skills"
)

// Evolver wraps the pantheon evolver with hermind-specific storage
// and tracker hooks.
type Evolver struct {
	*pskills.Evolver
	storage storage.Storage
	tracker *Tracker
}

// NewEvolver constructs an Evolver.
func NewEvolver(llm core.LanguageModel, skillDir string) *Evolver {
	ev := &Evolver{Evolver: pskills.NewEvolver(llm, skillDir)}
	ev.Evolver.OnExtracted = ev.onExtracted
	return ev
}

// SetStorage optionally wires a storage.Storage so extraction events
// are surfaced via /api/memory/report.
func (ev *Evolver) SetStorage(s storage.Storage) {
	ev.storage = s
}

// WithTracker attaches a Tracker that will be Refreshed after a
// successful skill write. Returns the Evolver for fluent chaining.
func (ev *Evolver) WithTracker(t *Tracker) *Evolver {
	ev.tracker = t
	return ev
}

func (ev *Evolver) onExtracted(filename, reason string) {
	if ev.storage != nil {
		data, _ := json.Marshal(map[string]any{
			"filename": filename,
			"reason":   reason,
		})
		_ = ev.storage.AppendMemoryEvent(context.Background(), time.Now().UTC(), "skill.extracted", data)
	}
	if ev.tracker != nil {
		if _, err := ev.tracker.Refresh(context.Background()); err != nil {
			mlog.Warning("skills.tracker refresh after Extract failed", mlog.String("err", err.Error()))
		}
	}
}

// Extract analyses the conversation history and persists skills.
func (ev *Evolver) Extract(ctx context.Context, turns []message.HermindMessage, verdict *judge.Verdict) error {
	coreTurns := make([]core.Message, len(turns))
	for i, t := range turns {
		coreTurns[i] = message.ToPantheon(t)
	}
	return ev.Evolver.Extract(ctx, coreTurns, verdict)
}
