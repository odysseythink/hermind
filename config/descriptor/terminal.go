package descriptor

// Terminal mirrors config.TerminalConfig. docker_volumes is []string and
// has no scalar-field representation; it is left CLI-only until the
// descriptor model supports list fields.
func init() {
	Register(Section{
		Key:     "terminal",
		Label:   "Terminal",
		Summary: "Shell-exec backend used by the agent's bash tool.",
		GroupID: "runtime",
		Fields: []FieldSpec{
			{
				Name:     "backend",
				Label:    "Backend",
				Help:     "Which execution backend to run shell commands through.",
				Kind:     FieldEnum,
				Required: true,
				Default:  "local",
				Enum:     []string{"local", "docker", "ssh", "modal", "daytona", "singularity"},
			},

			// Shared across every backend.
			{
				Name:  "cwd",
				Label: "Working directory",
				Help:  "Absolute path used as cwd for each command. Empty = backend default.",
				Kind:  FieldString,
			},
			{
				Name:  "timeout",
				Label: "Default timeout (seconds)",
				Help:  "Command timeout; 0 means use the backend default.",
				Kind:  FieldInt,
			},

			// Docker backend.
			{
				Name:        "docker_image",
				Label:       "Docker image",
				Help:        "Image name passed to `docker run`.",
				Kind:        FieldString,
				VisibleWhen: &Predicate{Field: "backend", Equals: "docker"},
			},

			// SSH backend.
			{
				Name:        "ssh_host",
				Label:       "SSH host",
				Help:        "Hostname or IP of the target machine.",
				Kind:        FieldString,
				VisibleWhen: &Predicate{Field: "backend", Equals: "ssh"},
			},
			{
				Name:        "ssh_user",
				Label:       "SSH user",
				Kind:        FieldString,
				VisibleWhen: &Predicate{Field: "backend", Equals: "ssh"},
			},
			{
				Name:        "ssh_key",
				Label:       "SSH key path",
				Help:        "Filesystem path to the private key file.",
				Kind:        FieldString,
				VisibleWhen: &Predicate{Field: "backend", Equals: "ssh"},
			},

			// Modal backend.
			{
				Name:        "modal_base_url",
				Label:       "Modal base URL",
				Kind:        FieldString,
				VisibleWhen: &Predicate{Field: "backend", Equals: "modal"},
			},
			{
				Name:        "modal_token",
				Label:       "Modal token",
				Kind:        FieldSecret,
				VisibleWhen: &Predicate{Field: "backend", Equals: "modal"},
			},

			// Daytona backend.
			{
				Name:        "daytona_base_url",
				Label:       "Daytona base URL",
				Kind:        FieldString,
				VisibleWhen: &Predicate{Field: "backend", Equals: "daytona"},
			},
			{
				Name:        "daytona_token",
				Label:       "Daytona token",
				Kind:        FieldSecret,
				VisibleWhen: &Predicate{Field: "backend", Equals: "daytona"},
			},

			// Singularity backend.
			{
				Name:        "singularity_image",
				Label:       "Singularity image",
				Help:        "Path to the .sif image file.",
				Kind:        FieldString,
				VisibleWhen: &Predicate{Field: "backend", Equals: "singularity"},
			},
		},
	})
}
