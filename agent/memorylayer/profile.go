package memorylayer

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/core"
)

//go:embed prompts/profile_update.txt
var profileUpdatePromptTemplate string

type ProfileConfig struct {
	Enabled       bool
	Timeout       time.Duration // default 6s
	MaxSections   int           // max sections rendered to the LLM (default 24)
	DefaultUserID string        // single-user installs may leave UserID empty on Turn; we fill it.
}

func (c *ProfileConfig) fill() {
	if c.Timeout <= 0 {
		c.Timeout = 6 * time.Second
	}
	if c.MaxSections <= 0 {
		c.MaxSections = 24
	}
	if c.DefaultUserID == "" {
		c.DefaultUserID = "default"
	}
}

type ProfileUpdater struct {
	store storage.Storage
	llm   core.LanguageModel
	cfg   ProfileConfig
}

func NewProfileUpdater(store storage.Storage, llm core.LanguageModel, cfg ProfileConfig) *ProfileUpdater {
	cfg.fill()
	return &ProfileUpdater{store: store, llm: llm, cfg: cfg}
}

// Apply runs one update cycle for a boundary. Best-effort — all errors
// log and return so the caller never blocks on this.
func (p *ProfileUpdater) Apply(ctx context.Context, b *Boundary) {
	if p == nil || !p.cfg.Enabled || p.llm == nil || b == nil || len(b.Turns) == 0 {
		return
	}
	userID := p.cfg.DefaultUserID

	cur, err := p.store.GetProfile(ctx, userID)
	if err != nil && err != storage.ErrNotFound {
		mlog.Warning("profile: GetProfile failed", mlog.String("err", err.Error()))
		return
	}

	callCtx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()

	prompt := renderProfilePrompt(cur, b, p.cfg.MaxSections)
	resp, err := p.llm.Generate(callCtx, &core.Request{
		SystemPrompt: "You maintain a structured user profile from conversations.",
		Messages: []core.Message{{
			Role:    core.MESSAGE_ROLE_USER,
			Content: []core.ContentParter{core.TextPart{Text: prompt}},
		}},
	})
	if err != nil {
		mlog.Warning("profile: LLM call failed", mlog.String("err", err.Error()))
		return
	}
	delta := parseProfileDelta(extractText(resp), cur, userID)
	if delta == nil || (len(delta.Adds)+len(delta.Updates)+len(delta.Deletes)) == 0 {
		return
	}
	version, err := p.store.SaveProfileDelta(ctx, delta)
	if err != nil {
		mlog.Warning("profile: SaveProfileDelta failed", mlog.String("err", err.Error()))
		return
	}
	data, _ := json.Marshal(map[string]any{
		"user_id": userID,
		"version": version,
		"adds":    len(delta.Adds),
		"updates": len(delta.Updates),
		"deletes": len(delta.Deletes),
		"reason":  b.Reason,
	})
	_ = p.store.AppendMemoryEvent(ctx, time.Now().UTC(), "profile.updated", data)
}

// renderProfilePrompt assigns s1..sN to each existing section, in the
// same order Get returned them. The corresponding map is rebuilt by
// parseProfileDelta when resolving update/delete IDs.
func renderProfilePrompt(cur *storage.Profile, b *Boundary, maxSections int) string {
	var sb strings.Builder
	if cur != nil && len(cur.Sections) > 0 {
		for i, sec := range cur.Sections {
			if i >= maxSections {
				break
			}
			fmt.Fprintf(&sb, "s%d | kind=%s | key=%s | value=%q | confidence=%.2f\n",
				i+1, sec.Kind, sec.Key, sec.Value, sec.Confidence)
		}
	} else {
		sb.WriteString("(empty)\n")
	}
	var turns strings.Builder
	for _, t := range b.Turns {
		fmt.Fprintf(&turns, "[turn %d]\nuser: %s\nassistant: %s\n\n",
			t.ID, t.UserMsg, t.Assistant)
	}
	p := strings.ReplaceAll(profileUpdatePromptTemplate, "{{CURRENT_SECTIONS}}", sb.String())
	return strings.ReplaceAll(p, "{{TURNS}}", turns.String())
}

type rawDelta struct {
	Adds    []rawSection `json:"adds"`
	Updates []rawSection `json:"updates"`
	Deletes []struct {
		ID string `json:"id"`
	} `json:"deletes"`
}

type rawSection struct {
	ID          string  `json:"id"`           // present on updates
	Kind        string  `json:"kind"`
	Key         string  `json:"key"`
	Value       string  `json:"value"`
	Evidence    string  `json:"evidence"`
	SourceTurns []int64 `json:"source_turns"`
	Confidence  float64 `json:"confidence"`
}

func parseProfileDelta(text string, cur *storage.Profile, userID string) *storage.ProfileDelta {
	text = strings.TrimSpace(text)
	if i := strings.Index(text, "{"); i >= 0 {
		text = text[i:]
	}
	if j := strings.LastIndex(text, "}"); j >= 0 {
		text = text[:j+1]
	}
	var raw rawDelta
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil
	}

	idToRef := make(map[string]storage.ProfileSectionRef)
	if cur != nil {
		for i, sec := range cur.Sections {
			idToRef[fmt.Sprintf("s%d", i+1)] = storage.ProfileSectionRef{
				UserID: userID, Kind: sec.Kind, Key: sec.Key,
			}
		}
	}
	out := &storage.ProfileDelta{UserID: userID}
	for _, a := range raw.Adds {
		if isValidKind(a.Kind) && a.Key != "" && a.Value != "" {
			out.Adds = append(out.Adds, toSection(userID, a))
		}
	}
	for _, u := range raw.Updates {
		// If the LLM gave us a short id, use the existing key/kind. Else
		// fall back to the (kind, key) the LLM emitted.
		if ref, ok := idToRef[u.ID]; ok {
			sec := toSection(userID, u)
			sec.Kind, sec.Key = ref.Kind, ref.Key
			out.Updates = append(out.Updates, sec)
		} else if isValidKind(u.Kind) && u.Key != "" {
			out.Updates = append(out.Updates, toSection(userID, u))
		}
	}
	for _, d := range raw.Deletes {
		if ref, ok := idToRef[d.ID]; ok {
			out.Deletes = append(out.Deletes, ref)
		}
	}
	return out
}

func isValidKind(k string) bool { return k == "explicit" || k == "implicit" }

func toSection(userID string, r rawSection) storage.ProfileSection {
	return storage.ProfileSection{
		UserID:      userID,
		Kind:        r.Kind,
		Key:         r.Key,
		Value:       r.Value,
		Evidence:    r.Evidence,
		SourceTurns: r.SourceTurns,
		Confidence:  r.Confidence,
	}
}
