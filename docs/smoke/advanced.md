# Advanced group web UI smoke flow

Covers: `config/descriptor/browser.go`, `config/descriptor/mcp.go`,
`config/descriptor/cron.go`, and the corresponding web panels
(`BrowserPanel`, `McpPanel`, `CronPanel`).

## Prerequisites

- Go binary built from current branch: `go build -o /tmp/hermind ./cmd/hermind`
- A clean config directory at `/tmp/hermind-advanced-smoke`
- A seeded `config.yaml` (see Setup below)

## Setup

Create `/tmp/hermind-advanced-smoke/config.yaml`:

```yaml
browser:
  provider: browserbase
  browserbase:
    api_key: secret-test-key
    project_id: proj-123
    base_url: https://api.browserbase.com
mcp:
  servers:
    smoketest:
      command: /bin/echo
      enabled: true
cron:
  jobs:
    - name: hourly-ping
      schedule: every 1h
      prompt: What time is it?
```

Start hermind:

```
go run ./cmd/hermind --config /tmp/hermind-advanced-smoke/config.yaml &
```

## Browser section

1. Open `http://127.0.0.1:9119/#advanced`. Click the **Browser** row in the
   sidebar.
   Expected: provider dropdown visible with 3 options (blank, browserbase,
   camofox); `browserbase` is selected; `api_key` field is redacted (shows
   empty); `base_url` shows `https://api.browserbase.com`; `project_id`
   shows `proj-123`.

2. Change `base_url` to `https://api.browserbase.com/v2`, save, then reload
   the page.
   Expected: new value persists; `api_key` is still redacted (empty input).

3. Switch provider dropdown to `camofox`.
   Expected: all five browserbase-specific fields hide; `camofox.base_url`
   and `camofox.managed_persistence` fields appear.

4. Switch provider dropdown to blank (empty).
   Expected: all sub-fields hide; only the provider dropdown is visible.

## MCP section

5. Expand **MCP servers** in the sidebar.
   Expected: `smoketest` row visible with subtitle `/bin/echo`; no dirty dot.

6. Click `smoketest`.
   Expected: `command` field = `/bin/echo`; enabled toggle is on.

7. Click **Add MCP server**.
   Expected: a dialog appears asking for a name only.

8. Type `Bad Key` in the name field.
   Expected: format-error message shown; Create button is disabled.

9. Clear the field, type `probe_srv`, click **Create**.
   Expected: dialog closes; panel opens for `probe_srv` showing empty fields.

10. Fill `command` = `/bin/true`, toggle Enabled on, save.
    Reload the page.
    Expected: both `smoketest` and `probe_srv` are present in the sidebar;
    `probe_srv` has command `/bin/true` and enabled on.

## Cron section

11. Expand **Cron jobs** in the sidebar.
    Expected: `#1 hourly-ping` row with subtitle `every 1h`; no dirty dot.

12. Click **Add cron job**.
    Expected: no dialog; a new blank row `#2` appears in the sidebar and the
    editor opens immediately to its fields (name, schedule, prompt).

13. Fill name = `test-job`, schedule = `every 5m`, prompt = `hello`, save.
    Reload the page.
    Expected: two jobs visible — `#1 hourly-ping` and `#2 test-job`.

14. Click `#1 hourly-ping`, then click **Move down**.
    Expected: sidebar now shows `#1 test-job`, `#2 hourly-ping`.

15. Save, reload the page.
    Expected: order persists — `#1 test-job`, `#2 hourly-ping`.

16. Select `#1 test-job`, click **Delete**.
    Expected: only `#1 hourly-ping` remains.

## Cleanup

```
kill %1 && rm -rf /tmp/hermind-advanced-smoke
```

## Failure modes

- Advanced tab still shows "coming soon": `groups.ts` not updated or the
  webroot bundle was not rebuilt + committed.
- Browser fields don't appear: descriptor `browser` not registered or
  `ShapeMap` dispatch key mismatch.
- MCP Add dialog accepts spaces in name: `NewMcpServerDialog` validation
  regex not applied.
- Cron add opens a dialog instead of inserting inline: cron descriptor uses
  `ShapeList` (no subkey dialog) — verify `NoDiscriminator` + `Subkey` flags.
- Order change does not persist after reload: move-up/move-down mutation did
  not write through `PUT /api/config`; check list reorder handler.
