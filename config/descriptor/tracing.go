package descriptor

func init() {
	Register(Section{
		Key:     "tracing",
		Label:   "Tracing",
		Summary: "Stdlib-based tracing emitted to a JSON-lines sink.",
		GroupID: "observability",
		Fields: []FieldSpec{
			{
				Name:    "enabled",
				Label:   "Enabled",
				Help:    "Turn tracing on.",
				Kind:    FieldBool,
				Default: false,
			},
			{
				Name:        "file",
				Label:       "File",
				Help:        "Path to the JSON-lines trace file. Leave blank for stderr.",
				Kind:        FieldString,
				VisibleWhen: &Predicate{Field: "enabled", Equals: true},
			},
		},
	})
}
