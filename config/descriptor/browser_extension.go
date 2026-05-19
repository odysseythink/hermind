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
			{
				Name:  "api_key",
				Label: "API key",
				Help:  "Authentication key shared between the backend and the browser extension. The extension sends this in the X-Extension-Key header.",
				Kind:  FieldSecret,
			},
		},
	})
}
