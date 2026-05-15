// Package skills — Tracker keeps a content hash + monotonic sequence
// of the skills library, used to decay stale memory feedback signals
// across library evolution.
package skills

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/mlog"
)

// computeLibraryHash returns the SHA-256 of the sorted (filename,
// SHA-256(content)) tuples for every *.md file directly under dir.
// Subdirectories and non-.md files are ignored. A missing directory
// returns the well-defined "empty" hash (sha256 over zero tokens).
func computeLibraryHash(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyHash(), nil
		}
		return "", fmt.Errorf("skills: read library dir: %w", err)
	}

	type tok struct{ name, contentHash string }
	tokens := make([]tok, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("skills: read %s: %w", path, err)
		}
		sum := sha256.Sum256(data)
		tokens = append(tokens, tok{name: e.Name(), contentHash: hex.EncodeToString(sum[:])})
	}
	sort.Slice(tokens, func(i, j int) bool { return tokens[i].name < tokens[j].name })

	final := sha256.New()
	for _, t := range tokens {
		final.Write([]byte(t.name))
		final.Write([]byte{0}) // delimiter so {a, bb} ≠ {ab, b}
		final.Write([]byte(t.contentHash))
		final.Write([]byte{0})
	}
	return hex.EncodeToString(final.Sum(nil)), nil
}

func emptyHash() string {
	sum := sha256.Sum256(nil)
	return hex.EncodeToString(sum[:])
}

func shortHash(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:12]
}

// Tracker watches the skills directory and maintains a (hash, seq)
// state row in storage so the memory ranker can decay stale
// reinforcement signals across library evolution.
type Tracker struct {
	store    storage.Storage
	skillDir string
}

// NewTracker constructs a Tracker. The store must already be migrated;
// the skillDir need not exist (computeLibraryHash treats missing dir
// as empty).
func NewTracker(store storage.Storage, skillDir string) *Tracker {
	return &Tracker{store: store, skillDir: skillDir}
}

// Refresh recomputes the library hash and persists it. Returns true if
// a real bump happened (hash changed). On any error the state is left
// untouched and bumped=false.
func (t *Tracker) Refresh(ctx context.Context) (bool, error) {
	h, err := computeLibraryHash(t.skillDir)
	if err != nil {
		return false, err
	}
	oldHash, oldSeq, newSeq, bumped, err := t.store.SetSkillsGeneration(ctx, h)
	if err != nil {
		return false, err
	}
	if !bumped {
		return false, nil
	}

	data, _ := json.Marshal(map[string]any{
		"old_seq":  oldSeq,
		"new_seq":  newSeq,
		"old_hash": oldHash,
		"new_hash": h,
	})
	_ = t.store.AppendMemoryEvent(ctx, time.Now().UTC(), "skills.generation_bumped", data)

	mlog.Info("skills.generation_bumped",
		mlog.Int64("old_seq", oldSeq),
		mlog.Int64("new_seq", newSeq),
		mlog.String("old_hash", shortHash(oldHash)),
		mlog.String("new_hash", shortHash(h)),
	)
	return true, nil
}

// Current returns the persisted (hash, seq, updated_at).
func (t *Tracker) Current(ctx context.Context) (*storage.SkillsGeneration, error) {
	return t.store.GetSkillsGeneration(ctx)
}
