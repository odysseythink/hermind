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
