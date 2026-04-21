# Changelog

## Unreleased

### Breaking

- **Feishu platform (`feishu`) switched from one-way bot webhook to
  self-built app over long-connection.** The `webhook_url` option is
  removed. Replace it with `app_id`, `app_secret`, `domain`, and
  optionally `encrypt_key` / `default_chat_id`. Recreate your Feishu bot
  as a self-built app in the Open Platform console (see
  `docs/smoke/feishu-app.md`). On startup, any `feishu` instance still
  carrying `webhook_url` without `app_id` will fail with a migration
  error.
