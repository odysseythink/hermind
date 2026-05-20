package file

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePath_Allowed(t *testing.T) {
	tmp := t.TempDir()
	allowed := []string{tmp}
	f := filepath.Join(tmp, "test.txt")
	os.WriteFile(f, []byte("hello"), 0o644)

	if err := validatePath(f, allowed); err != nil {
		t.Fatalf("expected allowed, got: %v", err)
	}
}

func TestValidatePath_TraversalRejected(t *testing.T) {
	tmp := t.TempDir()
	allowed := []string{tmp}
	if err := validatePath(filepath.Join(tmp, "..", "etc", "passwd"), allowed); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}

func TestValidatePath_OutsideAllowed(t *testing.T) {
	tmp := t.TempDir()
	other := t.TempDir()
	allowed := []string{tmp}
	f := filepath.Join(other, "test.txt")
	os.WriteFile(f, []byte("hello"), 0o644)

	if err := validatePath(f, allowed); err == nil {
		t.Fatal("expected outside allowed to be rejected")
	}
}

func TestValidatePath_EmptyAllowed(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "test.txt")
	os.WriteFile(f, []byte("hello"), 0o644)

	if err := validatePath(f, nil); err == nil {
		t.Fatal("expected empty allowed to be rejected")
	}
}

func TestValidatePath_SymlinkEscape(t *testing.T) {
	tmp := t.TempDir()
	allowed := []string{tmp}
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.txt")
	os.WriteFile(outsideFile, []byte("secret"), 0o644)

	link := filepath.Join(tmp, "link.txt")
	if err := os.Symlink(outsideFile, link); err != nil {
		t.Skip("cannot create symlinks on this platform: ", err)
	}

	if err := validatePath(link, allowed); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestValidatePath_DeepNonExistent(t *testing.T) {
	tmp := t.TempDir()
	allowed := []string{tmp}
	deep := filepath.Join(tmp, "a", "b", "c", "new.txt")
	if err := validatePath(deep, allowed); err != nil {
		t.Fatalf("expected deep path under allowed to pass, got: %v", err)
	}
}

func TestGetAllowedDirs(t *testing.T) {
	cfg := map[string]any{"allowed_directories": "/home/user\n/tmp\n\n"}
	got := getAllowedDirs(cfg)
	want := []string{"/home/user", "/tmp"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
