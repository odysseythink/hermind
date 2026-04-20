package descriptor

func init() {
	Register(Section{
		Key:     "model",
		Label:   "Default model",
		Summary: "Model used when a request doesn't pin one explicitly.",
		GroupID: "models",
		Shape:   ShapeScalar,
		Fields: []FieldSpec{
			{
				Name:     "model",
				Label:    "Model",
				Help:     "Provider-qualified id, e.g. anthropic/claude-opus-4-7.",
				Kind:     FieldString,
				Required: true,
				DatalistSource: &DatalistSource{
					Section: "providers",
					Field:   "model",
				},
			},
		},
	})
}
