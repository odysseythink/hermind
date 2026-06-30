# Agent Session 30-Minute Hard Cap (PR-AR-5)

**Date**: 2026-05-27
**Status**: Adopted
**Context**: Long-running agent sessions can accumulate cost (LLM tokens, tool calls, memory). A hard cap prevents runaway sessions.

## Decision
Default `AGENT_SESSION_MAX_DURATION=30m`. Configurable via env. Empty/zero = 30m fallback.

## Rationale
- **Node parity**: The Node `aibitat` runtime has an implicit 30-minute timeout via the Express server keepalive; we make it explicit.
- **Cost control**: At ~$0.01-0.10 per LLM call, a 10-step agent loop every 30s costs ~$0.20-2.00 per hour. 30m is a $0.10-1.00 ceiling per session.
- **User expectation**: ChatGPT sessions rarely last >30m of active use. Users expect a fresh context for truly long tasks.
- **Operational safety**: Prevents zombie sessions if the client disconnects without closing the WS.

## Consequences
- Users with legitimate long-running tasks (e.g., multi-file code generation) may need to restart the agent.
- The FE should show a friendly "Session timed out" message so users understand why the connection dropped.
