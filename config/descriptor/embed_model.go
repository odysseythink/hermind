package descriptor

func init() {
	Register(Section{
		Key:     "embed_model",
		Label:   "Embedding model",
		Summary: "Model used for text embeddings (hybrid search, topic shift detection, skill retrieval).",
		GroupID: "models",
		Shape:   ShapeScalar,
		Fields: []FieldSpec{
			{
				Name:    "embed_model",
				Label:   "Embedding model",
				Help:    "Provider-qualified id for the embedding model, e.g. openai/text-embedding-3-small. Only used when the active provider supports embeddings. WARNING: changing this after memories already exist may cause dimension mismatches in the vector store.",
				Kind:    FieldString,
				Default: "text-embedding-3-small",
			},
		},
	})
}
