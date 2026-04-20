# Memory config smoke flow

**Prereq:** hermind web running (`hermind web`), a test `config.yaml`
without a `memory:` block (or with `memory: {}`).

1. Open `http://127.0.0.1:<port>/web` in a browser.
2. Click the **Memory** group in the sidebar. The main panel shows the
   Memory section with a single **Provider** dropdown — no other fields.
3. Pick **honcho** from the provider dropdown. Four new fields appear:
   base URL, API key (secret), workspace, peer.
4. Fill **API key** = `test-honcho-key` and **workspace** = `demo`.
   Click Save.
5. Reload the page. The workspace input still reads `demo`; the API key
   input renders blank (redaction is working on the GET path).
6. Without retyping the API key, click Save again. Open `config.yaml` on
   disk — `memory.honcho.api_key` is still `test-honcho-key`
   (preservation is working on the PUT path).
7. Change the provider dropdown to **mem0**. The Honcho fields vanish;
   Mem0's `user_id`, `base_url`, `api_key` appear. The prior Honcho
   values remain in `config.yaml` under `memory.honcho` — they're just
   hidden because `visible_when` gates them on the provider discriminator.
8. Change the provider to the blank option at the top of the dropdown.
   All backend-specific fields disappear. Save. On-disk `memory.provider`
   is now empty (or the whole `memory:` block is omitted by `omitempty`).

**Regression watch:**
- Switching provider should NOT clobber the other backend's fields in
  `config.yaml` (keeps partial credentials around).
- Refreshing after saving an API key must show the secret input as
  blanked out, not as the literal key (redaction sanity check).
- Typing a new API key and saving must persist the new value, not the
  prior one (preservation only fills blanks).
