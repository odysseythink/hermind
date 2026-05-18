package descriptor

// Tools mirrors config.ToolsConfig's global `disabled` list. Choices
// are not baked into the descriptor: api/handlers_config_schema.go
// populates field.Enum at response time by walking the available tools,
// so newly-added tools show up after a page reload without a rebuild.
func init() {
	Register(Section{
		Key:     "tools",
		Label:   "Tools",
		Summary: "Enable or disable system tools. Disabled tools are hidden from the LLM.",
		GroupID: "skills",
		Shape:   ShapeMap,
		Fields: []FieldSpec{
			{
				Name:  "disabled",
				Label: "Disabled tools",
				Help:  "Tools listed here are not exposed to the LLM.",
				Kind:  FieldMultiSelect,
			},
		},
	})
}
