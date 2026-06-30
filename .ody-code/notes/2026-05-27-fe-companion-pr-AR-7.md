# Frontend Companion Brief — PR-AR-7 OAuth Agent Skills

## Overview

PR-AR-7 adds three new agent skills to the Go server: **gmail-agent**, **google-calendar-agent**, and **outlook-agent**. The frontend needs admin settings pages for each so admins can configure the skills.

## Gmail Settings Page

**Route suggestion**: `/admin/agent-skills/gmail`

**Fields**:
- `deploymentId` — text input, the Google Apps Script deployment ID
- `apiKey` — password input, a secret chosen by the admin to protect the Apps Script endpoint
- `enabled` — toggle (controls whether the config is written to `gmail_agent_config` SystemSetting)

**Storage**: POST to `/api/system/setting` with key `gmail_agent_config` and value:
```json
{"deploymentId":"...","apiKey":"..."}
```

**Validation**: Both fields required when enabled. Deployment ID should look like a Google Apps Script ID (alphanumeric with dashes/underscores).

## Google Calendar Settings Page

**Route suggestion**: `/admin/agent-skills/google-calendar`

**Fields**: Same shape as Gmail — `deploymentId`, `apiKey`, `enabled`.

**Storage**: POST to `/api/system/setting` with key `google_calendar_agent_config`.

## Outlook Settings Page

**Route suggestion**: `/admin/agent-skills/outlook`

**Fields**:
- `clientId` — text input, the Azure app registration client ID
- `clientSecret` — password input, the Azure app client secret
- `tenant` — optional text input (default "common")
- `enabled` — toggle

**Storage**: POST to `/api/system/setting` with key `outlook_agent_config` and value:
```json
{"clientId":"...","clientSecret":"...","tenant":"common"}
```

**Connect flow**: After saving config, show a "Connect Outlook" button that opens:
```
GET /api/oauth/outlook/authorize?return_to=<current-page-url>
```
in a popup or new tab. The callback redirects back to `return_to`.

**Status indicator**: Poll `GET /api/oauth/outlook/status` to show:
- Connected (green) + expiry date
- Disconnected (gray) + "Connect" button

**Disconnect**: POST to `/api/oauth/outlook/disconnect`.

## Single-User Mode Gate

All three skills are **disabled in multi-user mode**. The frontend should:
- Show a banner on each settings page when `MULTI_USER_MODE=true`: "OAuth agent skills are only available in single-user mode."
- Gray out / disable the forms

## Approval Gate UX

Destructive actions (send_email, create_draft, etc.) trigger the existing tool approval WebSocket flow. No new UI needed — reuse the existing approval dialog.

## Out of Scope

- Apps Script template deployment is manual (admin follows README)
- No auto-discovery of Apps Script deployments
- No OAuth for Google (Gmail/Calendar use Apps Script, not direct OAuth)
