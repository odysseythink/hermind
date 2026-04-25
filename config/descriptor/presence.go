package descriptor

// Presence mirrors config.PresenceConfig. http_idle_absent_after_seconds
// is the top-level scalar; sleep_window.* are dotted-path fields on the
// nested presence.SleepWindowConfig. start/end/timezone are gated by
// sleep_window.enabled per the existing memory-section pattern (the
// frontend's isVisible() routes visible_when.field through getPath, so
// dotted paths in predicates Just Work).
func init() {
	sleepGate := &Predicate{Field: "sleep_window.enabled", Equals: true}
	Register(Section{
		Key:     "presence",
		Label:   "User presence",
		Summary: "Three-state presence framework that gates background workers (idle consolidator, future schedulers).",
		GroupID: "memory",
		Shape:   ShapeMap,
		Fields: []FieldSpec{
			{
				Name:    "http_idle_absent_after_seconds",
				Label:   "HTTP-idle quiet window (seconds)",
				Help:    "Time without inbound HTTP requests before the user is voted Absent. Default 300 (5 min). 0 disables the signal.",
				Kind:    FieldInt,
				Default: 300,
			},
			{
				Name:    "sleep_window.enabled",
				Label:   "Sleep window enabled",
				Help:    "When on, votes Absent during the configured local-clock hours. Never votes Present (insomnia/night-shift case).",
				Kind:    FieldBool,
				Default: false,
			},
			{
				Name:        "sleep_window.start",
				Label:       "Sleep window start (HH:MM)",
				Help:        "Local-clock start time, 24-hour format. Cross-midnight windows are supported (e.g., 22:00 → 06:00).",
				Kind:        FieldString,
				Default:     "22:00",
				VisibleWhen: sleepGate,
			},
			{
				Name:        "sleep_window.end",
				Label:       "Sleep window end (HH:MM)",
				Help:        "Local-clock end time, 24-hour format.",
				Kind:        FieldString,
				Default:     "06:00",
				VisibleWhen: sleepGate,
			},
			{
				Name:        "sleep_window.timezone",
				Label:       "Timezone (IANA name)",
				Help:        "IANA timezone name, e.g. America/Los_Angeles. Empty = process local timezone.",
				Kind:        FieldString,
				Default:     "",
				VisibleWhen: sleepGate,
			},
		},
	})
}
