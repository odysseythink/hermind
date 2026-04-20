package descriptor

import "github.com/odysseythink/hermind/provider/factory"

// Auxiliary mirrors config.AuxiliaryConfig. All four fields are optional —
// leaving every one blank is a valid state that means "reuse the main
// provider" (see provider/auxiliary.go).
func init() {
	Register(Section{
		Key:     "auxiliary",
		Label:   "Auxiliary provider",
		Summary: "Secondary provider for compression, vision, and background tasks. Leave all fields blank to reuse the main provider.",
		GroupID: "runtime",
		Fields: []FieldSpec{
			{
				Name:  "provider",
				Label: "Provider",
				Help:  "Provider factory. Leave blank to reuse the main provider.",
				Kind:  FieldEnum,
				Enum:  factory.Types(),
			},
			{
				Name:  "base_url",
				Label: "Base URL",
				Kind:  FieldString,
			},
			{
				Name:  "api_key",
				Label: "API key",
				Kind:  FieldSecret,
			},
			{
				Name:  "model",
				Label: "Model",
				Help:  "Provider-qualified id; optional — falls back to the main provider's default.",
				Kind:  FieldString,
			},
		},
	})
}
