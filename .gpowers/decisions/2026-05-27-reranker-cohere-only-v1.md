# Reranker — Cohere Only for v1

**Date**: 2026-05-27
**Status**: Adopted
**Context**: pantheon v0.0.9 中实现 rerank 的 provider 有 `openai`（OpenAI-compatible 格式，含 Cohere v2 / Jina 自适应）和 `native`（本地模型）。v3-A 选择通过 `openaicompat.Client` 直接调用 Cohere API。

**Decision**: v3-A 只接 Cohere（通过 `openaicompat.Client` 直接调用 `/v2/rerank`）；其它 reranker provider（"none"/"noop"/""）走 NoopReranker。

**Future**: pantheon 后续如增加专用 jina / voyage reranker provider，新增 case 即可。
