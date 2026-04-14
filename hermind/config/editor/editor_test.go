package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSaveRoundTripPreservesComments(t *testing.T) {
	src, err := os.ReadFile("testdata/commented.yaml")
	if err != nil { t.Fatal(err) }

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, src, 0o644); err != nil { t.Fatal(err) }

	doc, err := Load(path)
	if err != nil { t.Fatalf("Load: %v", err) }
	if err := doc.Save(); err != nil { t.Fatalf("Save: %v", err) }

	out, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	got := string(out)
	for _, want := range []string{"# top comment", "# inline", "anthropic/claude-opus-4-6", "api_key: abc"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in saved output:\n%s", want, got)
		}
	}
}

func TestLoadMissingFileReturnsEmptyDoc(t *testing.T) {
	doc, err := Load(filepath.Join(t.TempDir(), "absent.yaml"))
	if err != nil { t.Fatalf("Load: %v", err) }
	if doc == nil { t.Fatal("Load returned nil doc") }
	if doc.Path() == "" { t.Error("Path() empty") }
}
