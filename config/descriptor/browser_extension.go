package descriptor

func init() {
	Register(Section{
		Key:     "browser_extension",
		Label:   "Browser extension",
		Summary: "Browser extension integration settings for browser_control and browser_extension_read tools.",
		GroupID: "advanced",
		Shape:   ShapeMap,
		Fields: []FieldSpec{
			{
				Name:  "enabled",
				Label: "Enabled",
				Help:  "Enable the browser extension integration. When disabled, browser extension tools are not registered.",
				Kind:  FieldBool,
			},

		},
	})
}
