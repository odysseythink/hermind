package descriptor

// FallbackProviders mirrors config.Config.FallbackProviders ([]config.ProviderConfig).
// Every element conforms to the same 4-field schema as the primary Providers
// section (see providers.go) — the only structural difference is ordering:
// the runtime tries each fallback in list order.
func init() {
	Register(Section{
		Key:     "fallback_providers",
		Label:   "Fallback Providers",
		Summary: "Ordered list of providers tried in turn when the primary fails.",
		GroupID: "models",
		Shape:   ShapeList,
		Fields: []FieldSpec{
			{
				Name:     "provider",
				Label:    "Provider type",
				Kind:     FieldEnum,
				Required: true,
				Enum:     SupportedProviders,
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
				Label: "Model",
				Help:  "Optional — provider-qualified id used when this fallback is active.",
				Kind:  FieldString,
			},
		},
	})
}
