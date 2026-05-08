package descriptor

import "github.com/odysseythink/hermind/provider/factory"

// Providers mirrors config.Config.Providers (map[string]config.ProviderConfig).
// Each instance conforms to the same 4-field schema regardless of provider
// type — unlike gateway.platforms where each type has distinct fields.
func init() {
	Register(Section{
		Key:     "providers",
		Label:   "Providers",
		Summary: "LLM providers available to Default Model, Auxiliary, and fallback.",
		GroupID: "models",
		Shape:   ShapeKeyedMap,
		Fields: []FieldSpec{
			{
				Name:     "provider",
				Label:    "Provider type",
				Kind:     FieldEnum,
				Required: true,
				Enum:     factory.Types(),
			},
			{
				Name:  "base_url",
				Label: "Base URL",
				Kind:  FieldString,
			},
			{
				Name:     "api_key",
				Label:    "API key",
				Kind:     FieldSecret,
				Required: true,
			},
			{
				Name:  "model",
				Label: "Default model for this provider",
				Help:  "Optional — provider-qualified id used when a request doesn't pin a specific model.",
				Kind:  FieldString,
			},
		},
	})
}
