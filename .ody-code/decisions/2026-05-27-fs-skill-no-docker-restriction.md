# Filesystem Agent — No Docker-Only Restriction

**Date**: 2026-05-27
**Status**: Adopted
**Context**: Node's `filesystem-agent.isToolAvailable()` returns true only when `NODE_ENV=development` or `HERMIND_RUNTIME=docker`. We don't replicate this.

**Decision**: Go skill is enabled in any deployment, gated by `cfg.AgentFilesystemEnabled` (default true) and a strict `safeJoin` sandbox under `cfg.AgentFilesystemRoot`.

**Rationale**: The safeJoin guard (absolute-path + symlink resolution + prefix check) is the real security boundary. The docker-only check is belt-and-suspenders. Admins who want the Node behaviour can set `AGENT_FILESYSTEM_ENABLED=false`.
