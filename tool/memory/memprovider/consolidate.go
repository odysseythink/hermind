package memprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool/embedding"
)

// ConsolidateOptions tunes the consolidation pass.
type ConsolidateOptions struct {
	// MemType restricts the pass to memories of the given type. Empty
	// means "all types" (each type is scanned independently so duplicates
	// only collapse within the same type).
	MemType string
	// ScanLimit caps how many memories are inspected per type. Default 500.
	ScanLimit int
	// JaccardThreshold above which two memories are considered duplicates
	// on token-set overlap. Default 0.75.
	JaccardThreshold float64
	// CosineThreshold above which two memories are considered duplicates
	// on embedding similarity. Default 0.92. Only applied when both
	// memories have vectors.
	CosineThreshold float64
	// DecayAfter, when > 0, archives active memories older than this age.
	// Default 0 (no decay). Only applied to memories whose MemType is
	// "episodic" — semantic facts and preferences don't decay on age.
	DecayAfter time.Duration
}

func (o *ConsolidateOptions) fill() {
	if o.ScanLimit <= 0 {
		o.ScanLimit = 500
	}
	if o.JaccardThreshold <= 0 {
		o.JaccardThreshold = 0.75
	}
	if o.CosineThreshold <= 0 {
		o.CosineThreshold = 0.92
	}
}

// ConsolidateReport summarizes what one consolidation pass did.
type ConsolidateReport struct {
	Scanned    int
	Superseded int
	Archived   int
}

// Consolidate scans memories of the given type (or all known types when
// empty) and marks near-duplicate older memories as superseded by newer
// ones. Optionally archives memories older than DecayAfter when they are
// episodic.
//
// The pass is deliberately O(n²) over ScanLimit — ScanLimit defaults to
// 500 which keeps the worst case at 250k comparisons. Callers should run
// this from a post-session hook or a cron, not mid-conversation.
func Consolidate(ctx context.Context, store storage.Storage, opts *ConsolidateOptions) (*ConsolidateReport, error) {
	if store == nil {
		return nil, fmt.Errorf("consolidate: storage is required")
	}
	if opts == nil {
		opts = &ConsolidateOptions{}
	}
	opts.fill()

	report := &ConsolidateReport{}
	types := []string{opts.MemType}
	if opts.MemType == "" {
		types = []string{"episodic", "semantic", "preference", ""}
	}
	now := time.Now().UTC()

	for _, t := range types {
		mems, err := store.ListMemoriesByType(ctx, t, opts.ScanLimit)
		if err != nil {
			return report, fmt.Errorf("consolidate: list %q: %w", t, err)
		}
		report.Scanned += len(mems)

		// ListMemoriesByType returns newest first. Walk from oldest toward
		// newest so each older memory gets supersededBy set to the newest
		// duplicate seen (the first entry we'd encounter going back).
		decoded := decodeVectors(mems)
		for i := len(mems) - 1; i >= 0; i-- {
			older := mems[i]
			if older.Status != "" && older.Status != storage.MemoryStatusActive {
				continue
			}
			for j := i - 1; j >= 0; j-- {
				newer := mems[j]
				if newer.Status != "" && newer.Status != storage.MemoryStatusActive {
					continue
				}
				if isDuplicate(older, newer, decoded[i], decoded[j], opts) {
					if err := store.MarkMemorySuperseded(ctx, older.ID, newer.ID); err == nil {
						report.Superseded++
					}
					break
				}
			}
		}

		if opts.DecayAfter > 0 && t == "episodic" {
			cutoff := now.Add(-opts.DecayAfter)
			for _, m := range mems {
				if m.CreatedAt.Before(cutoff) && (m.Status == "" || m.Status == storage.MemoryStatusActive) {
					mm := *m
					mm.Status = storage.MemoryStatusArchived
					mm.UpdatedAt = now
					if err := store.SaveMemory(ctx, &mm); err == nil {
						report.Archived++
					}
				}
			}
		}
	}
	data, _ := json.Marshal(map[string]any{
		"scanned":    report.Scanned,
		"superseded": report.Superseded,
		"archived":   report.Archived,
	})
	_ = store.AppendMemoryEvent(ctx, time.Now().UTC(), "memory.consolidated", data)
	return report, nil
}

func decodeVectors(mems []*storage.Memory) [][]float32 {
	out := make([][]float32, len(mems))
	for i, m := range mems {
		if len(m.Vector) == 0 {
			continue
		}
		v, err := embedding.DecodeVector(m.Vector)
		if err != nil {
			continue
		}
		out[i] = v
	}
	return out
}

func isDuplicate(a, b *storage.Memory, va, vb []float32, opts *ConsolidateOptions) bool {
	if strings.TrimSpace(a.Content) == strings.TrimSpace(b.Content) {
		return true
	}
	if len(va) > 0 && len(vb) > 0 {
		if float64(embedding.CosineSimilarity(va, vb)) >= opts.CosineThreshold {
			return true
		}
	}
	return jaccardTokens(a.Content, b.Content) >= opts.JaccardThreshold
}

func jaccardTokens(a, b string) float64 {
	aset := toTokenSet(a)
	bset := toTokenSet(b)
	if len(aset) == 0 || len(bset) == 0 {
		return 0
	}
	inter := 0
	small, large := aset, bset
	if len(aset) > len(bset) {
		small, large = bset, aset
	}
	for k := range small {
		if _, ok := large[k]; ok {
			inter++
		}
	}
	union := len(aset) + len(bset) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func toTokenSet(s string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, tok := range strings.Fields(strings.ToLower(s)) {
		tok = strings.Trim(tok, ".,;:!?\"'()[]{}`")
		if len(tok) < 3 {
			continue
		}
		out[tok] = struct{}{}
	}
	return out
}
