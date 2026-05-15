// Package skills wraps the pantheon skill loader with hermind-instance
// path defaults and hermind-specific extensions (retriever, evolver,
// inject — see other files in this package).
package skills

import (
	"os"
	"path/filepath"

	pskills "github.com/odysseythink/pantheon/extensions/skills"
)

// Type re-exports so existing call sites don't break.
type (
	Skill     = pskills.Skill
	LoadError = pskills.LoadError
)

// ParseSkillFile delegates to pantheon.
func ParseSkillFile(path string) (*Skill, error) { return pskills.ParseSkillFile(path) }

// DefaultHome returns the default skills home directory under the
// current hermind instance. Honors $HERMIND_HOME; falls back to
// ./.hermind/skills if cwd resolution fails.
func DefaultHome() string {
	if v := os.Getenv("HERMIND_HOME"); v != "" {
		return filepath.Join(v, "skills")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ".hermind/skills"
	}
	return filepath.Join(cwd, ".hermind", "skills")
}
