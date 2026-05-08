package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDefaults_RestoresMissingSkill(t *testing.T) {
	home := t.TempDir()
	if err := EnsureDefaults(home); err != nil {
		t.Fatalf("EnsureDefaults: %v", err)
	}

	skillFile := filepath.Join(home, "chart-generation", "SKILL.md")
	data, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("expected chart-generation/SKILL.md to be restored: %v", err)
	}

	s, err := parseSkillBytes(skillFile, data)
	if err != nil {
		t.Fatalf("restored SKILL.md is not parseable: %v", err)
	}
	if s.Name != "chart-generation" {
		t.Errorf("name = %q, want chart-generation", s.Name)
	}
}

func TestEnsureDefaults_DoesNotOverwriteExisting(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "chart-generation")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	custom := []byte("---\nname: chart-generation\ndescription: customized\n---\n\nuser content\n")
	skillFile := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(skillFile, custom, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureDefaults(home); err != nil {
		t.Fatalf("EnsureDefaults: %v", err)
	}

	got, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(custom) {
		t.Errorf("user customization was overwritten")
	}
}

func TestEnsureDefaults_Idempotent(t *testing.T) {
	home := t.TempDir()
	if err := EnsureDefaults(home); err != nil {
		t.Fatalf("first call: %v", err)
	}
	skillFile := filepath.Join(home, "chart-generation", "SKILL.md")
	first, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatal(err)
	}
	mtime1, _ := os.Stat(skillFile)

	if err := EnsureDefaults(home); err != nil {
		t.Fatalf("second call: %v", err)
	}
	second, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatal(err)
	}
	mtime2, _ := os.Stat(skillFile)

	if string(first) != string(second) {
		t.Errorf("content changed between calls")
	}
	if !mtime1.ModTime().Equal(mtime2.ModTime()) {
		t.Errorf("mtime changed — second call rewrote the file")
	}
}
