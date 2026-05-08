package descriptor

func init() {
	Register(Section{
		Key:     "storage",
		Label:   "Storage",
		Summary: "Where hermind keeps conversation history and agent state.",
		GroupID: "runtime",
		Fields: []FieldSpec{
			{
				Name:     "driver",
				Label:    "Driver",
				Help:     "Storage backend to use.",
				Kind:     FieldEnum,
				Required: true,
				Default:  "sqlite",
				Enum:     []string{"sqlite", "postgres"},
			},
			{
				Name:        "sqlite_path",
				Label:       "SQLite path",
				Help:        "Filesystem path to the SQLite database file.",
				Kind:        FieldString,
				VisibleWhen: &Predicate{Field: "driver", Equals: "sqlite"},
			},
			{
				Name:        "postgres_url",
				Label:       "Postgres URL",
				Help:        "postgres://user:pass@host/db connection string.",
				Kind:        FieldSecret,
				VisibleWhen: &Predicate{Field: "driver", Equals: "postgres"},
			},
		},
	})
}
