# Tool Approval: Global + Session Toggles Only (PR-AR-5)

**Date**: 2026-05-27
**Status**: Adopted
**Context**: The Node implementation has per-tool and per-user whitelists (`AgentSkillWhitelist` DB table + UI). We needed a v1 back port.

## Decision
Ship only two toggles for PR-AR-5:
1. **Global system setting** `agent_tool_auto_approve` — applies to all sessions
2. **Per-session client frame** `setAutoApprove` — applies to one WS session

Defer per-tool / per-user whitelist to Phase 2.

## Rationale
- **Scope control**: PR-AR-5 is already the largest PR in the v1 series (5 tasks, ~800 lines). Adding a DB migration + CRUD UI would push it past the 10h estimate.
- **UX parity**: The Node UI for `AgentSkillWhitelist` is complex (skill matrix × user grid). A simple on/off switch covers 80% of use cases.
- **Migration path**: When Phase 2 lands, the existing toggles become defaults. The approval wrap in `Builder.addWithApproval` already checks both layers; adding a third (whitelist) is a one-line change.

## Consequences
- Users who want fine-grained control must wait for Phase 2.
- The `AgentSkillWhitelist` table is not created in v1; Phase 2 will need a migration.
