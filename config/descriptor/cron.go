package descriptor

// Cron mirrors config.CronConfig. YAML layout: cron.jobs: [...].
// Each job is {name, schedule, prompt, model} — uniform, no discriminator.
// Subkey="jobs" tells the API redact/preserve pipeline and the UI to
// unwrap one extra YAML layer; NoDiscriminator opts out of the
// "exactly one FieldEnum named provider" ShapeList invariant.
func init() {
	Register(Section{
		Key:             "cron",
		Label:           "Cron jobs",
		Summary:         "Scheduled prompts that run on a recurring schedule.",
		GroupID:         "advanced",
		Shape:           ShapeList,
		Subkey:          "jobs",
		NoDiscriminator: true,
		Fields: []FieldSpec{
			{Name: "name", Label: "Name", Kind: FieldString, Required: true,
				Help: "Stable identifier for this job."},
			{Name: "schedule", Label: "Schedule", Kind: FieldString, Required: true,
				Help: `Cron expression or "every 5m" / "every 1h" shorthand.`},
			{Name: "prompt", Label: "Prompt", Kind: FieldString, Required: true,
				Help: "Prompt text to send. For long prompts, edit YAML directly."},
			{Name: "model", Label: "Model override", Kind: FieldString,
				Help: "Leave blank to use the default model.",
				DatalistSource: &DatalistSource{Section: "providers", Field: "model"}},
		},
	})
}
