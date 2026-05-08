package descriptor

// Browser mirrors config.BrowserConfig. The provider field is a FieldEnum
// discriminator: when blank, no browser provider is configured (matches
// yaml omitempty). Each backend's sub-fields are gated by VisibleWhen so
// only the active backend renders.
//
// Dotted field names like "browserbase.api_key" rely on the dotted-path
// infrastructure in ConfigSection.tsx, state.ts (edit/config-field
// reducer), and api/handlers_config.go (walkPath helper).
// TODO(env-override-badges): BROWSERBASE_API_KEY / BROWSERBASE_PROJECT_ID
// override the YAML values at runtime. The UI currently shows YAML values
// only. Future plan: render a badge indicating "env overrides this".
func init() {
	gate := func(backend string) *Predicate {
		return &Predicate{Field: "provider", Equals: backend}
	}
	Register(Section{
		Key:     "browser",
		Label:   "Browser",
		Summary: "Browser automation provider. Leave blank for no browser integration.",
		GroupID: "advanced",
		Shape:   ShapeMap,
		Fields: []FieldSpec{
			{
				Name:  "provider",
				Label: "Provider",
				Help:  "Browser automation backend. Leave blank to disable.",
				Kind:  FieldEnum,
				Enum:  []string{"browserbase", "camofox"},
			},

			// Browserbase (cloud)
			{Name: "browserbase.base_url", Label: "Browserbase base URL",
				Kind: FieldString, VisibleWhen: gate("browserbase")},
			{Name: "browserbase.api_key", Label: "Browserbase API key",
				Kind: FieldSecret, VisibleWhen: gate("browserbase"),
				Help: "Env var BROWSERBASE_API_KEY overrides this value at runtime."},
			{Name: "browserbase.project_id", Label: "Browserbase project ID",
				Kind: FieldString, VisibleWhen: gate("browserbase"),
				Help: "Env var BROWSERBASE_PROJECT_ID overrides this value at runtime."},
			{Name: "browserbase.keep_alive", Label: "Keep session alive",
				Kind: FieldBool, VisibleWhen: gate("browserbase")},
			{Name: "browserbase.proxies", Label: "Enable Browserbase proxies",
				Kind: FieldBool, VisibleWhen: gate("browserbase")},

			// Camofox (local)
			{Name: "camofox.base_url", Label: "Camofox base URL",
				Kind: FieldString, VisibleWhen: gate("camofox"),
				Help: "Defaults to http://localhost:9377 when blank."},
			{Name: "camofox.managed_persistence", Label: "Managed persistence",
				Kind: FieldBool, VisibleWhen: gate("camofox")},
		},
	})
}
