package descriptor

// Skills mirrors config.SkillsConfig's global `disabled` list. Choices
// are not baked into the descriptor: api/handlers_config_schema.go
// populates field.Enum at response time by walking the skills home
// directory, so newly-installed skills show up after a page reload
// without a rebuild.
//
// `platform_disabled` (per-platform override map) is deliberately
// excluded from this section — the UI affordance for editing a nested
// map-of-lists is a separate design and not blocking the "skills group
// no longer empty" goal.
func init() {
	Register(Section{
		Key:     "skills",
		Label:   "Skills",
		Summary: "Enable or disable skills across every platform. Unchecked = enabled.",
		GroupID: "skills",
		Shape:   ShapeMap,
		Fields: []FieldSpec{
			{
				Name:  "disabled",
				Label: "Disabled skills",
				Help:  "Skills listed here never activate. Check a skill to disable it globally. Install skills into $HERMIND_HOME/skills to make them appear.",
				Kind:  FieldMultiSelect,
			},
		},
	})
}
