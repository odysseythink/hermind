package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// defaultsFS holds the system-default skills that ship with the binary.
// Each subdirectory under defaults/ is one skill; all files inside are
// materialized verbatim under <home>/<name>/ when missing.
//
//go:embed all:defaults
var defaultsFS embed.FS

// EnsureDefaults materializes any embedded default skill that is missing
// from the on-disk skills home. It never overwrites existing files —
// users may have customized a default, and the disable mechanism lives
// in config (skills.disabled), not in deletion.
//
// A skill is considered "present" if its <home>/<name>/SKILL.md exists.
// If a user deletes other files inside an otherwise-present skill, those
// are not restored; restore the whole skill by deleting its directory.
func EnsureDefaults(home string) error {
	entries, err := fs.ReadDir(defaultsFS, "defaults")
	if err != nil {
		return fmt.Errorf("skills: read embedded defaults: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if _, err := os.Stat(filepath.Join(home, name, "SKILL.md")); err == nil {
			continue
		}
		if err := materializeDefault(name, home); err != nil {
			return fmt.Errorf("skills: restore default %q: %w", name, err)
		}
	}
	return nil
}

func materializeDefault(name, home string) error {
	srcRoot := filepath.ToSlash(filepath.Join("defaults", name))
	return fs.WalkDir(defaultsFS, srcRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, p)
		if err != nil {
			return err
		}
		dst := filepath.Join(home, name, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := fs.ReadFile(defaultsFS, p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
}
