# v3-C — AgentSkillWhitelist Coexists with Global Auto-Approve Toggle

**Date**: 2026-05-28  
**Status**: Adopted

PR-AR-5's global `agent_tool_auto_approve` SystemSetting is **not removed**. The two mechanisms are combined with OR semantics: either enabled means the tool passes. Whitelist is the finer-grained capability. A future deprecation path may remove the global toggle, but this PR does not touch it.
