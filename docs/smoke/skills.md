# Skills web UI smoke flow

Covers: `config/descriptor/skills.go`, `api/handlers_config_schema.go`
skills enrichment, `web/src/components/fields/MultiSelectField.tsx`.

## Prerequisites

- Go binary built from current branch: `go build -o /tmp/hermind ./cmd/hermind`
- A clean `HERMIND_HOME` pointed at `/tmp/hermind-skills-smoke`
- Two seeded skills under `$HERMIND_HOME/skills/demo/{alpha,beta}/SKILL.md`
- A minimal `config.yaml` containing at least `model: anthropic/claude-opus-4-6`

## Steps

1. Start hermind:
   ```
   /tmp/hermind web --addr 127.0.0.1:9119 --no-browser
   ```

2. In a second shell, dump the schema and confirm skills is present:
   ```
   curl -s http://127.0.0.1:9119/api/config/schema \
     -H "Authorization: Bearer $(cat $HERMIND_HOME/token)" \
     | jq '.sections[] | select(.key == "skills")'
   ```
   Expected: JSON with a single `disabled` field of `kind: "multiselect"`, and `enum: ["alpha", "beta"]`.

3. In a browser, navigate to `http://127.0.0.1:9119/ui/`. Click the **Skills** tab in the sidebar.
   Expected: two checkboxes — `alpha`, `beta` — both unchecked, with the help text "Skills listed here never activate…" visible.

4. Check `alpha`, trigger the Apply workflow (click Apply or let autosave fire, matching the rest of the UI's behavior), then reload the page.
   Expected: `alpha` is still checked after reload.

5. On disk:
   ```
   grep -A2 '^skills:' $HERMIND_HOME/config.yaml
   ```
   Expected:
   ```
   skills:
     disabled:
       - alpha
   ```

6. Uncheck `alpha`, apply, reload. Expected: the `skills:` block disappears from `config.yaml` (because `Disabled []string` carries `yaml:"disabled,omitempty"` and the outer struct is likewise `omitempty`).

## Failure modes

- Skills tab still shows "coming soon": `groups.ts` not updated or the
  webroot bundle was not rebuilt + committed.
- Checkbox list is empty: the schema response's `enum` array is empty —
  either `HERMIND_HOME/skills/` is empty or `handleConfigSchema`'s
  post-pass did not run (descriptor key mismatch, most likely).
- Apply-then-reload reverts: `PUT /api/config` did not round-trip the
  `disabled` array; verify `config.SaveToPath` wrote the file and the
  UI's GET rehydration reads the same key.
