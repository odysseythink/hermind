# Decision: rag-memory `store` action deferred to future PR

**Context:** Node's rag-memory skill has both `search` and `store` actions. `store` embeds content and writes to a workspace-scoped vector namespace (`memory-<workspaceID>`).

**Decision:** PR-AR-3 ships `search` fully. `store` returns a deferred-status JSON payload so the agent loop continues uninterrupted.

**Rationale:** Implementing `store` requires either:
1. Extending `VectorService` with `Upsert(ctx, namespace, vec, text, metadata)` — touches vectordb abstraction layer
2. Or using the existing embedding + vector write pipeline — complex end-to-end wiring

Both options expand PR-AR-3 scope beyond the 10-12h estimate. `store` is lower priority than the 4 default skills + MCP/Flow projection.

**Planned resolution:** PR-AR-3.1 or PR-AR-6 (whichever lands first) will implement `store` by extending `VectorService`.
