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

func TestGet(t *testing.T) {
	doc := mustLoad(t, "testdata/commented.yaml")
	cases := []struct {
		path string
		want string
		ok   bool
	}{
		{"model", "anthropic/claude-opus-4-6", true},
		{"providers.anthropic.api_key", "abc", true},
		{"missing", "", false},
		{"providers.anthropic.missing", "", false},
	}
	for _, tc := range cases {
		got, ok := doc.Get(tc.path)
		if ok != tc.ok || got != tc.want {
			t.Errorf("Get(%q) = (%q, %v); want (%q, %v)", tc.path, got, ok, tc.want, tc.ok)
		}
	}
}

func mustLoad(t *testing.T, src string) *Doc {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
