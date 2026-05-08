package descriptor

// Proxy mirrors config.ProxyConfig. The endpoint is opt-in: when
// `enabled` is false the /v1/messages route is not mounted, so the
// keep-alive interval is dependent — gated via VisibleWhen.
//
// Toggling `enabled` requires a hermind restart: api/server.go mounts
// the route once at startup inside the `if Config.Proxy.Enabled` guard
// (live re-mounting is not implemented).
func init() {
	Register(Section{
		Key:     "proxy",
		Label:   "Anthropic /v1/messages proxy",
		Summary: "Expose an Anthropic-compatible Messages API endpoint at /v1/messages. Off by default.",
		GroupID: "advanced",
		Shape:   ShapeMap,
		Fields: []FieldSpec{
			{
				Name:    "enabled",
				Label:   "Enable proxy",
				Help:    "Mount the /v1/messages route at the API server root. Restart hermind after toggling.",
				Kind:    FieldBool,
				Default: false,
			},
			{
				Name:        "keep_alive_seconds",
				Label:       "SSE keep-alive interval (seconds)",
				Help:        "Heartbeat ping interval on the streaming response. Default 15. Values <= 0 are clamped to 15.",
				Kind:        FieldInt,
				Default:     15,
				VisibleWhen: &Predicate{Field: "enabled", Equals: true},
			},
		},
	})
}
