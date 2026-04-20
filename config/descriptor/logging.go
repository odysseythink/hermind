package descriptor

func init() {
	Register(Section{
		Key:     "logging",
		Label:   "Logging",
		Summary: "slog output level for the hermind process.",
		GroupID: "observability",
		Fields: []FieldSpec{
			{
				Name:     "level",
				Label:    "Level",
				Help:     "Minimum log level emitted to stderr.",
				Kind:     FieldEnum,
				Required: false,
				Default:  "info",
				Enum:     []string{"debug", "info", "warn", "error"},
			},
		},
	})
}
