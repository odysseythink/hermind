package skills

import (
	"github.com/odysseythink/hermind/tool"
	pskills "github.com/odysseythink/pantheon/extensions/skills"
)

// InjectActive registers a synthetic tool per active skill into reg.
// Delegates to pantheon/extensions/skills.
func InjectActive(reg *tool.Registry, skillsReg *Registry) {
	pskills.InjectActive(reg, skillsReg)
}
