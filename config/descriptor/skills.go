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
		Summary: "Enable or disable installed skills. The list reflects what's in <instance>/skills/.",
		GroupID: "skills",
		Shape:   ShapeMap,
		Fields: []FieldSpec{
			{
				Name:  "disabled",
				Label: "Disabled skills",
				Help:  "Skills listed here never activate. Names mirror entries under $HERMIND_HOME/skills.",
				Kind:  FieldMultiSelect,
			},
			{
				Name:    "auto_extract",
				Label:   "Auto-extract skills",
				Help:    "After each conversation, ask the LLM to extract reusable skill snippets and save them to the instance's skills/ directory. Default off.",
				Kind:    FieldBool,
				Default: false,
			},
			{
				Name:    "inject_count",
				Label:   "Skills injected per turn",
				Help:    "Maximum dynamically retrieved skills injected into the system prompt each turn. 0 disables retrieval. Default 3.",
				Kind:    FieldInt,
				Default: 3,
			},
			{
				Name:    "generation_half_life",
				Label:   "Generation half-life",
				Help:    "Memory reinforcement signals decay by half every N skill generations. Default 5. Set to 0 to disable decay (signals always weighted as 1.0).",
				Kind:    FieldInt,
				Default: 5,
			},
		},
	})
}
