# Static Native-Tool-Calling Provider Whitelist (PR-AR-4)

**Date**: 2026-05-27
**Status**: Adopted
**Context**: Pantheon v0.0.9 doesn't expose a capability flag for "supports native tool calling". Node uses a per-provider class check; we mirror that as a Go-side static map.

## Whitelist
openai, anthropic, groq, ollama, mistral, google, deepseek, openrouter

## Rationale
Conservative initial set — only providers whose tool-calling has been validated against pantheon's openai-compatible adapter. Other providers fall back to `@agent`-prefix-only.

## Maintenance
Update map in `internal/agent/native_tool_calling.go` as new providers are confirmed. Optional follow-up: probe pantheon's provider for a runtime capability discovery method.
