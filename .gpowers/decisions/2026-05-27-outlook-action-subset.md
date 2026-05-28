# Outlook Agent — v1 Action Subset (5 of 20+)

**Date**: 2026-05-27
**Status**: Adopted
**Context**: Node exposes ~20 outlook actions (reply, mark_read, move_to_*, multiple draft helpers). The agent loop only needs a minimal surface.

**Decision**: v1 ships `search`, `read_thread`, `read_message`, `create_draft`, `send_email`. Other actions defer to a follow-up PR. LLM can simulate `reply_to_thread` via `read_thread` then `send_email` with the captured subject/recipients.
