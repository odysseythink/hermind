# v3-C — Multi-User OAuth Enabled

**Date**: 2026-05-28  
**Status**: Adopted

PR-AR-7 hard-rejected multi-user mode in the `CheckFn` of all three OAuth skills (`gmail-agent`, `google-calendar-agent`, `outlook-agent`). The underlying `TokenStore.UserID` unique index already supports per-user storage. This PR removes the multi-user short-circuit. `client_id`/`client_secret` remain globally shared (admin configures once); tokens are stored per-user as separate rows.
