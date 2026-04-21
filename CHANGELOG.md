# Changelog

## Unreleased

### Added

- **Frontend i18n (中 / EN)**: Web UI now supports English and Simplified Chinese via `react-i18next`. Language toggle in TopBar (right of status); default follows `navigator.language`, manual choice persisted in `localStorage`. Descriptor labels/help from the backend are overlaid by per-locale JSON (`web/src/locales/{en,zh-CN}/descriptors.json`) with fallback to the backend English literal. CI guards translation completeness via a Go-generated fixture (`api/fixture_gen_test.go`, build tag `fixture`) plus a vitest completeness test.

### Known limitations

- Platform descriptor labels (`/api/platforms/schema`) are not yet covered by the fixture; platform-specific field labels fall back to backend English.
- Enum option values in dropdowns render as the raw canonical string (e.g. `browserbase`); only the field **label** is translated. Full enum-option translation is a follow-up.
- `CHANGELOG.md` and server-side error messages remain English.

### Breaking

- **Feishu platform (`feishu`) switched from one-way bot webhook to
  self-built app over long-connection.** The `webhook_url` option is
  removed. Replace it with `app_id`, `app_secret`, `domain`, and
  optionally `encrypt_key` / `default_chat_id`. Recreate your Feishu bot
  as a self-built app in the Open Platform console (see
  `docs/smoke/feishu-app.md`). On startup, any `feishu` instance still
  carrying `webhook_url` without `app_id` will fail with a migration
  error.
