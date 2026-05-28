// Package agent implements the @agent runtime for go-server.
//
// v1 main-line is complete (PR-AR-7). Features:
//   - WebSocket upgrade with CORS origin allowlist
//   - Workspace agent invocation DB model with temp-token auth
//   - Pantheon conversation + agent wiring with tool registry
//   - Per-session tool approval gate (MCP + Flow + destructive default skills)
//   - Global + per-session auto-approve toggles
//   - Total session timeout (config-driven hard cap)
//   - Telemetry events (chat_started / chat_sent / chat_terminated)
//   - Graceful shutdown drains pending approvals
//   - sql-agent (list_databases / list_tables / get_schema / query)
//   - filesystem-agent (list_dir / read_file / write_file / edit_file / move_file / copy_file / search_files / get_info / create_dir)
//   - create-files-agent (txt / md — docx/pdf/pptx/xlsx deferred to PR-AR-6.1)
//   - gmail-agent (12 actions via Apps Script bridge, single-user only)
//   - google-calendar-agent (8 actions via Apps Script bridge, single-user only)
//   - outlook-agent (5 actions via Microsoft Graph OAuth, single-user only)
//
// Outlook OAuth smoke procedure:
//  1. Set OUTLOOK env vars + register OAuth app at portal.azure.com
//  2. SetSetting "outlook_agent_config" with clientId/clientSecret
//  3. GET /api/oauth/outlook/authorize?return_to=http://localhost:3001/dashboard
//  4. Approve in Microsoft consent
//  5. Land back at /dashboard; outlook-agent now appears in tool list
//  6. @agent send an email to me@example.com saying "test"
//  7. Approve in WS UI; mail sent
//
// Future PRs: per-user skill whitelist (Phase 2), binary file formats (PR-AR-6.1), multi-user OAuth.
package agent
