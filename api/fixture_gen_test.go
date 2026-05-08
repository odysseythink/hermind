//go:build fixture

package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestDumpSchemaFixture writes a canonical JSON snapshot of the descriptor
// registry (using the same DTO shape that GET /api/config/schema returns)
// to web/src/i18n/__fixtures__/config-schema.json so the frontend
// completeness test can verify every section/field has a translation
// entry.
//
// Run: go test -tags fixture ./api -run TestDumpSchemaFixture
func TestDumpSchemaFixture(t *testing.T) {
	out := filepath.Join("..", "web", "src", "i18n", "__fixtures__", "config-schema.json")
	resp := BuildConfigSchema()
	data, err := json.MarshalIndent(resp.Sections, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(out, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
