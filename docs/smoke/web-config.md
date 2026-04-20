# Manual smoke test — hermind web config UI

Run before cutting a release if you touched anything in `web/` or the
`/api/platforms/*` endpoints. Expected total time: 5 minutes.

## 0. Prep

```bash
# fresh home for an isolated hermind
mkdir -p /tmp/hermind-smoke/.hermind
cat > /tmp/hermind-smoke/.hermind/config.yaml <<'EOF'
model: claude-sonnet-4-5-20250929
providers:
  anthropic:
    provider: anthropic
    api_key: test-placeholder
storage:
  driver: sqlite
  sqlite_path: /tmp/hermind-smoke/.hermind/hermind.db
EOF

# launch hermind web
HOME=/tmp/hermind-smoke ./bin/hermind web --addr=127.0.0.1:9119 --no-browser
```

Note the URL printed on stdout (includes the `?t=<token>` query).
Open it in a browser.

## 1. Empty-state shell

- [ ] TopBar shows `⬡ hermind` on the left, a grey status chip ("All
  saved") on the right.
- [ ] Sidebar is empty with "No instances configured." italic.
- [ ] Main pane shows the "No instance selected" empty card.
- [ ] Footer has both Save / Save & Apply disabled.

## 2. New instance

- [ ] Click **+ New instance**. A modal opens; key input focused.
- [ ] Type `ACP_bot` as key — the Create button proceeds past click
      but the key field shows a red "lowercase letters, digits,
      underscore" error.
- [ ] Change key to `tg_main`; pick **Telegram Bot (telegram)** from
      the dropdown; click **Create**.
- [ ] Dialog closes. Sidebar shows `tg_main` with an amber dirty dot.
- [ ] Main pane shows the Telegram editor with one Bot Token field
      (empty). Enabled checkbox is checked.

## 3. Edit + save

- [ ] TopBar shows "1 unsaved change", Save buttons enabled.
- [ ] Type `bogus-token` into the Bot Token field.
- [ ] Click **Save**. Footer flashes green "Saved." and Sidebar dirty
      dot clears.

## 4. Reveal

- [ ] Click **Show** next to the Bot Token field. Value becomes
      `bogus-token` (visible).
- [ ] Click **Hide**. Value re-masks (still in the React state, just
      masked visually).
- [ ] Edit the field — the button becomes disabled with a "Save
      changes before revealing…" tooltip.

## 5. Test connection

- [ ] Click **Test connection**. Response: red dot + "probe failed:
      status 404…" (the bogus token isn't a real Telegram bot).
- [ ] Edit the Bot Token field; Test button disables with "Save
      changes first…" tooltip. Save, then click Test — same 404
      error (still bogus).

## 6. Outbound-webhook 501 warn

- [ ] Back in the sidebar, click **+ New instance**. Pick **Slack
      (Incoming Webhook)**; key `slack_out`. Create.
- [ ] Type `https://example.com` into the Webhook URL field; Save.
- [ ] Click **Test connection**. Response: grey dot + "no probe for
      this platform type" (501 mapped to warn chip).

## 7. Apply

- [ ] Click **Save & Apply**. TopBar dot pulses briefly. Footer
      flashes green "Applied."
- [ ] Check hermind stdout — you should see log lines like
      `gateway: starting platform platform=telegram`.

## 8. Toggle + delete

- [ ] Uncheck **Enabled** on `slack_out`. Sidebar item dims, gains
      an "off" badge. Save & Apply.
- [ ] Click **Delete instance** on `slack_out`. Confirm the browser
      dialog. Sidebar loses the entry; Editor returns to the
      no-selection card.

## 9. Hash persistence

- [ ] Select `tg_main`. URL should be `…/#tg_main`.
- [ ] Hit refresh. The page comes back with `tg_main` re-selected.
- [ ] Visit `…/#nonexistent` — page loads empty-state (the key
      doesn't exist so nothing is selected).

## Cleanup

```bash
rm -rf /tmp/hermind-smoke
```

## Stage 1 · Shell rewrite

- Sidebar shows all seven groups: Models, Gateway, Memory, Skills, Runtime, Advanced, Observability.
- Gateway is expanded on first load; all others are collapsed.
- Clicking a non-Gateway group label shows a "Coming soon — stage N" panel with a read-only summary and an "Edit via CLI" note.
- Legacy deep link: visiting `/#feishu-bot-main` (with `feishu-bot-main` configured) auto-rewrites the URL to `/#gateway/feishu-bot-main` and selects that instance.
- Unknown legacy hashes fall back to the empty state.
- The TopBar Save button is disabled when clean and shows `Save · N changes` when dirty.
- There is no global `Save and Apply` button.
- Gateway panel has its own `Apply` button in the breadcrumb row.
- Apply is disabled while gateway slice is dirty; tooltip reads "Save first, then apply".
- Reload preserves the expanded/collapsed state of each group (localStorage key `hermind.shell.expandedGroups`).
- Empty state (no hash, no saved selection) shows a 7-card landing grid; clicking a card opens that group.

## Stage 2 · Schema infrastructure (Storage)

- Visiting `#runtime/storage` renders the Storage editor: a Driver enum select, plus either a SQLite path field (driver=sqlite) or a Postgres URL field (driver=postgres).
- Changing the Driver value swaps which secondary field is visible; the hidden field's value is not submitted until re-shown.
- The Postgres URL field is a secret: the Show button is disabled with tooltip "Reveal not supported for this field (stage 2)".
- `GET /api/config/schema` returns a `sections` array that includes `storage` with its three fields and two visible_when predicates.
- `GET /api/config` with `storage.driver = postgres` returns `postgres_url` blanked.
- `PUT /api/config` with `storage.postgres_url = ""` preserves the stored URL (round-trip mirror of platform-secret behavior).
- Editing any storage field marks the Runtime group dirty (sidebar dot + TopBar `Save · N changes`). Save flushes to disk; the YAML reflects the new driver and the appropriate path/URL field.
- Routing: the Runtime group in the sidebar lists one entry — Storage. Other non-gateway groups show "Coming soon — stage N" inside their collapsible rows.

## Stage 3 · Simple sections (Logging, Metrics, Tracing, Agent, Terminal)

- Sidebar now shows three sub-entries inside Runtime (Storage, Agent, Terminal) and three inside Observability (Logging, Metrics, Tracing). Each is clickable and routes to its own editor.
- `GET /api/config/schema` returns six sections (storage + the five new ones) sorted by key.
- **Logging:** `#observability/logging` — Level enum defaults to `info`. Change to `debug`, Save, and confirm `config.yaml` contains `level: debug` directly under a `logging:` block.
- **Metrics:** `#observability/metrics` — Listen address is a plain string. `:9100` round-trips.
- **Tracing:** `#observability/tracing` — Enabled toggle gates the File field. Flipping enabled off hides File; the YAML still round-trips the stored `file` because nothing was blanked on the backend (non-secret).
- **Agent:** `#runtime/agent` — Two int inputs (`max_turns`, `gateway_timeout`). Compression is not editable here (CLI-only); see the deferral note atop `config/descriptor/agent.go`.
- **Terminal:** `#runtime/terminal` — Backend enum (local, docker, ssh, modal, daytona, singularity) gates per-backend fields. `modal_token` and `daytona_token` are `FieldSecret`; GET blanks them, PUT preserves them when the submitted value is empty (same round-trip behavior as `storage.postgres_url`). `docker_volumes` is intentionally absent — edit it in `config.yaml` until list-field support lands.
