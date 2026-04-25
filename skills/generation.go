// Package skills — Tracker keeps a content hash + monotonic sequence
// of the skills library, used to decay stale memory feedback signals
// across library evolution.
package skills

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

var _ = context.Context(nil) // placeholder removed in Task 7 once Tracker uses ctx
