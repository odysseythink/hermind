package descriptor

func init() {
	Register(Section{
		Key:     "metrics",
		Label:   "Metrics",
		Summary: "Prometheus /metrics HTTP server address. Leave blank to disable.",
		GroupID: "observability",
		Fields: []FieldSpec{
			{
				Name:  "addr",
				Label: "Listen address",
				Help:  `Host:port for the exporter, e.g. ":9100". Empty disables metrics.`,
				Kind:  FieldString,
			},
		},
	})
}
