# Pantheon `native` Provider — Deferred

**Date**: 2026-05-27
**Status**: Adopted
**Context**: pantheon 提供 `native` provider（本地 GGUF + llama.cpp 集成）。我们不在 v3-A 接入。

**Reason**: 需要本地 llama.cpp shared library，影响 docker 镜像体积 + 跨平台编译（Mac arm64 / Linux amd64 / Windows）。

**Mitigation**: 用户用 `ollama` 或 `lmstudio` provider 间接达成"本地 LLM"目标。
