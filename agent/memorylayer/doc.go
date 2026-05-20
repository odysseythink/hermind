// Package memorylayer wraps the existing memprovider.Recaller with
// Hybrid Retrieval (BM25 + Vector + RRF), an LLM Reranker, and a
// MemCell-lite boundary-triggered taxonomy extractor.
//
// All components are decorators: the underlying Provider / Recaller
// interface stays unchanged, and any external memory provider continues
// to work (downgraded to single-source mode without RRF when it cannot
// expose pure-BM25 / pure-vector ranking).
//
// See .gpowers/designs/2026-05-20-memory-layer-design.md for the
// design rationale and Phase 1 scope.
package memorylayer
