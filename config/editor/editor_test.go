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

func TestSetExistingScalar(t *testing.T) {
	doc := mustLoad(t, "testdata/commented.yaml")
	if err := doc.Set("providers.anthropic.api_key", "NEW"); err != nil {
		t.Fatal(err)
	}
	v, ok := doc.Get("providers.anthropic.api_key")
	if !ok || v != "NEW" {
		t.Fatalf("got (%q,%v)", v, ok)
	}
	if err := doc.Save(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(doc.Path())
	if !strings.Contains(string(b), "# top comment") {
		t.Error("top comment lost")
	}
	if !strings.Contains(string(b), "api_key: NEW") {
		t.Errorf("new value missing:\n%s", b)
	}
}

func TestSetCreatesIntermediateMaps(t *testing.T) {
	doc := mustLoad(t, "testdata/commented.yaml")
	if err := doc.Set("agent.compression.threshold", "0.6"); err != nil {
		t.Fatal(err)
	}
	v, ok := doc.Get("agent.compression.threshold")
	if !ok || v != "0.6" {
		t.Fatalf("got (%q,%v)", v, ok)
	}
}

func TestRemove(t *testing.T) {
	doc := mustLoad(t, "testdata/commented.yaml")
	if err := doc.Remove("providers.anthropic.api_key"); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.Get("providers.anthropic.api_key"); ok {
		t.Error("still present")
	}
}

func TestSetBlockAddsNewMapEntry(t *testing.T) {
	doc := mustLoad(t, "testdata/commented.yaml")
	frag := "provider: openai\napi_key: sk-xxx\nmodel: gpt-4o\n"
	if err := doc.SetBlock("providers.openai", frag); err != nil {
		t.Fatal(err)
	}
	if v, _ := doc.Get("providers.openai.model"); v != "gpt-4o" {
		t.Errorf("got %q", v)
	}
}

func TestSaveCreatesParentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "notthere")
	path := filepath.Join(dir, "config.yaml")
	doc, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Set("model", "x"); err != nil {
		t.Fatal(err)
	}
	if err := doc.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}
