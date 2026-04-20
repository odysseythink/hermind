package descriptor

// The Agent section exposes the two top-level scalars on config.AgentConfig.
// config.AgentConfig.Compression is a nested struct and is deferred until
// the descriptor model supports nested sections; for now it is only
// editable via the CLI.
func init() {
	Register(Section{
		Key:     "agent",
		Label:   "Agent",
		Summary: "Engine turn limit and gateway request budget.",
		GroupID: "runtime",
		Fields: []FieldSpec{
			{
				Name:     "max_turns",
				Label:    "Max turns",
				Help:     "Maximum model turns per user request before the engine bails out.",
				Kind:     FieldInt,
				Required: true,
				Default:  90,
			},
			{
				Name:    "gateway_timeout",
				Label:   "Gateway timeout (seconds)",
				Help:    "Seconds a gateway request may run before being cancelled. 0 uses the gateway default.",
				Kind:    FieldInt,
				Default: 1800,
			},
		},
	})
}
