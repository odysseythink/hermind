package descriptor

// Memory mirrors config.MemoryConfig. The provider field is a FieldEnum
// discriminator: when blank, no external memory is configured (matches
// the yaml "omitempty" semantics on MemoryConfig.Provider). Each non-
// Holographic backend has sub-fields gated by VisibleWhen so only the
// active backend's inputs render.
//
// Dotted field names like "honcho.api_key" require the dotted-path
// infrastructure in ConfigSection.tsx, state.ts (edit/config-field
// reducer), and api/handlers_config.go (walkPath helper).
func init() {
	gate := func(backend string) *Predicate {
		return &Predicate{Field: "provider", Equals: backend}
	}
	Register(Section{
		Key:     "memory",
		Label:   "Memory",
		Summary: "Optional external long-term memory provider. Leave blank for no external memory.",
		GroupID: "memory",
		Shape:   ShapeMap,
		Fields: []FieldSpec{
			{
				Name:  "provider",
				Label: "Provider",
				Help:  "External memory backend. Leave blank for no external memory.",
				Kind:  FieldEnum,
				Enum: []string{
					"", "honcho", "mem0", "supermemory", "hindsight",
					"retaindb", "openviking", "byterover", "holographic",
				},
			},

			// Honcho
			{Name: "honcho.base_url", Label: "Honcho base URL",
				Kind: FieldString, VisibleWhen: gate("honcho")},
			{Name: "honcho.api_key", Label: "Honcho API key",
				Kind: FieldSecret, VisibleWhen: gate("honcho")},
			{Name: "honcho.workspace", Label: "Honcho workspace",
				Kind: FieldString, VisibleWhen: gate("honcho")},
			{Name: "honcho.peer", Label: "Honcho peer",
				Kind: FieldString, VisibleWhen: gate("honcho")},

			// Mem0
			{Name: "mem0.base_url", Label: "Mem0 base URL",
				Kind: FieldString, VisibleWhen: gate("mem0")},
			{Name: "mem0.api_key", Label: "Mem0 API key",
				Kind: FieldSecret, VisibleWhen: gate("mem0")},
			{Name: "mem0.user_id", Label: "Mem0 user ID",
				Kind: FieldString, VisibleWhen: gate("mem0")},

			// Supermemory
			{Name: "supermemory.base_url", Label: "Supermemory base URL",
				Kind: FieldString, VisibleWhen: gate("supermemory")},
			{Name: "supermemory.api_key", Label: "Supermemory API key",
				Kind: FieldSecret, VisibleWhen: gate("supermemory")},
			{Name: "supermemory.user_id", Label: "Supermemory user ID",
				Kind: FieldString, VisibleWhen: gate("supermemory")},

			// Hindsight
			{Name: "hindsight.base_url", Label: "Hindsight base URL",
				Kind: FieldString, VisibleWhen: gate("hindsight")},
			{Name: "hindsight.api_key", Label: "Hindsight API key",
				Kind: FieldSecret, VisibleWhen: gate("hindsight")},
			{Name: "hindsight.bank_id", Label: "Hindsight bank ID",
				Kind: FieldString, VisibleWhen: gate("hindsight")},
			{Name: "hindsight.budget", Label: "Hindsight budget",
				Kind: FieldEnum, Enum: []string{"low", "mid", "high"},
				VisibleWhen: gate("hindsight")},

			// RetainDB
			{Name: "retaindb.base_url", Label: "RetainDB base URL",
				Kind: FieldString, VisibleWhen: gate("retaindb")},
			{Name: "retaindb.api_key", Label: "RetainDB API key",
				Kind: FieldSecret, VisibleWhen: gate("retaindb")},
			{Name: "retaindb.project", Label: "RetainDB project",
				Kind: FieldString, VisibleWhen: gate("retaindb")},
			{Name: "retaindb.user_id", Label: "RetainDB user ID",
				Kind: FieldString, VisibleWhen: gate("retaindb")},

			// OpenViking
			{Name: "openviking.endpoint", Label: "OpenViking endpoint",
				Kind: FieldString, VisibleWhen: gate("openviking")},
			{Name: "openviking.api_key", Label: "OpenViking API key",
				Kind: FieldSecret, VisibleWhen: gate("openviking")},

			// Byterover (local CLI wrapper, no api_key)
			{Name: "byterover.brv_path", Label: "Byterover brv CLI path",
				Kind: FieldString, VisibleWhen: gate("byterover")},
			{Name: "byterover.cwd", Label: "Byterover working directory",
				Kind: FieldString, VisibleWhen: gate("byterover")},

			// Holographic is a placeholder — no fields.
		},
	})
}
