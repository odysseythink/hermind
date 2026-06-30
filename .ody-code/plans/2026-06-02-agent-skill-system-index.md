# Agent Skill System 增强 — 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Hermes Agent 技能系统的核心亮点能力移植到 Hermind 现有骨架上，分 3 个 Phase 增量交付。

**Architecture:** 保留 Hermind 的 DB 中心模型、REST API、WebSocket Agent Runtime 和审批系统，逐步添加动态预处理、条件可见性、平台隔离、两层缓存、LLM Curator 审查能力。

**Tech Stack:** Go 1.26, Gin, GORM (SQLite/PostgreSQL), Pantheon SDK, `github.com/hashicorp/golang-lru`

---

## File Structure

### 现有文件（修改）

| 文件 | 责任 | 变更阶段 |
|------|------|---------|
| `backend/internal/models/agent_skill.go` | AgentSkill 模型扩展（新增 6 个 JSON text 列 + UsageSidecar） | Phase 1 |
| `backend/internal/models/workspace.go` | 无变化（仅引用） | — |
| `backend/internal/services/agent_skill_service.go` | 扩展：Parse* 辅助方法、条件过滤查询、PreprocessContent、导入方法、缓存读写 | Phase 1/2 |
| `backend/internal/agent/system_prompt.go` | 重写 resolveSystemPrompt，添加增强型技能提示组装 | Phase 1 |
| `backend/internal/agent/handler.go` | 修改 buildSessionRegistry：传递 availableTools/availableToolsets | Phase 1 |
| `backend/internal/agent/tools/builder.go` | 添加 AvailableTools()/AvailableToolsets() 导出方法 | Phase 1 |
| `backend/internal/agent/tools/skill_view.go` | 重写 skill_view 处理函数：添加预处理管道 + 缓存 | Phase 2 |
| `backend/internal/agent/tools/skills_list.go` | 无变化（仅作为索引） | — |
| `backend/internal/handlers/agent_skills.go` | 扩展：新增 import 和 curator report 端点 | Phase 2/3 |
| `backend/cmd/server/main.go` | 无变化（已有 CuratorJob 注册） | — |
| `backend/internal/workers/skill_curator.go` | 扩展：传递 CuratorService 依赖 | Phase 3 |

### 新增文件

| 文件 | 责任 | 阶段 |
|------|------|------|
| `backend/internal/models/curator_audit_log.go` | CuratorAuditLog GORM 模型 | Phase 3 |
| `backend/internal/services/skill_preprocess.go` | 动态预处理管道（模板变量、内联 shell、配置注入） | Phase 2 |
| `backend/internal/services/skill_cache.go` | 两层缓存（进程 LRU + 磁盘快照） | Phase 2 |
| `backend/internal/services/skill_platform.go` | 平台过滤逻辑（runtime.GOOS 匹配） | Phase 1 |
| `backend/internal/services/skill_filter.go` | 条件可见性过滤（requires_tools/toolsets） | Phase 1 |
| `backend/internal/services/skill_import.go` | 外部技能目录导入 + Prompt Injection Guard | Phase 2 |
| `backend/internal/services/curator_service.go` | Curator 主服务（时间驱动 + LLM 审查 + 报告） | Phase 3 |
| `backend/internal/services/curator_llm.go` | LLM 审查具体逻辑（Prompt 构建 + 解析 + 归并） | Phase 3 |
| `backend/internal/services/curator_report.go` | Curator 报告生成（JSON + Markdown） | Phase 3 |
| `backend/internal/services/curator_audit.go` | 审计日志写入 | Phase 3 |
| `backend/internal/services/skill_frontmatter.go` | Frontmatter YAML 解析 + 字段提取 | Phase 1 |
| `backend/internal/services/skill_config.go` | 技能配置变量解析和值解析 | Phase 2 |

---

## Dependency Overview

```
Phase 1 (foundation — must complete first)
  ├── Task 1: Model extension (AgentSkill new columns)
  ├── Task 2: Frontmatter parser + field extractor
  ├── Task 3: Platform filtering (skill_platform.go)
  ├── Task 4: Conditional filtering (skill_filter.go)
  ├── Task 5: Enhanced system prompt builder
  ├── Task 6: Handler integration (availableTools/toolsets plumbing)
  └── Task 7: Phase 1 integration tests

Phase 2 (preprocessing + caching + import)
  ├── Task 8: Skill preprocess pipeline (template vars, config injection)
  ├── Task 9: Inline shell execution (with safety guards)
  ├── Task 10: Two-layer cache (LRU + disk snapshot)
  ├── Task 11: External skill directory import
  ├── Task 12: Prompt Injection Guard scanner
  ├── Task 13: skill_view tool integration (preprocess + cache)
  ├── Task 14: REST API for import
  └── Task 15: Phase 2 integration tests

Phase 3 (curator intelligence)
  ├── Task 16: CuratorService scaffolding + time-driven transitions
  ├── Task 17: UsageSidecar persistence (JSON column read/write)
  ├── Task 18: CuratorAuditLog model + write path
  ├── Task 19: LLM review prompt builder
  ├── Task 20: LLM review response parser
  ├── Task 21: Three-signal reconciliation (absorbed → consolidations → heuristic)
  ├── Task 22: Curator report generation (JSON + Markdown)
  ├── Task 23: Worker integration (wire CuratorService into SkillCuratorJob)
  ├── Task 24: REST API for curator report
  └── Task 25: Phase 3 integration tests
```

**跨阶段阻塞点：**
- Phase 2 依赖 Phase 1 的模型扩展（新列必须存在才能读写）
- Phase 3 依赖 Phase 2 的 UsageSidecar（Curator 需要读取使用统计）
- Phase 3 的 LLM 审查依赖 Pantheon auxiliary client（需确认接口可用）

---

## Risks & Open Questions

| 风险 | 影响 | 缓解 |
|------|------|------|
| Pantheon auxiliary 客户端不支持 `Complete()` 调用 | Phase 3 LLM 审查不可用 | Phase 3 Task 19 先做 spike：验证 `llm_factory.go` 中 auxiliary client 接口；若不可用，设计 typed shim |
| GORM AutoMigrate 对新增 JSON text 列的默认值在 PostgreSQL 下表现不同 | Phase 1 迁移失败 | Task 1 包含两种 DB 的迁移验证（SQLite + PostgreSQL） |
| 增强型系统提示增加 token 消耗 | 长会话成本上升 | 条件过滤（Phase 1 Task 4）已减少注入技能数量；fallback_for 仅排序不过滤是保守策略 |
| Curator LLM 审查消耗额外 API 成本 | 企业用户不满 | 默认开启但可开关；使用廉价模型（gpt-4o-mini）；失败降级不阻塞 |
| 内联 shell 即使有关闭开关仍可能被恶意利用 | 安全事件 | 双层防护：全局开关默认关 + 仅 admin 可开；shell 执行在独立进程中，cwd 受限 |

---

## Spec Coverage

| Spec 章节 | 实施阶段 | 状态 |
|-----------|---------|------|
| §2 数据模型扩展 | Phase 1 Task 1 | covered |
| §4 Phase 1 详细设计（平台过滤、条件过滤、系统提示） | Phase 1 Task 3-7 | covered |
| §5 Phase 2 详细设计（预处理、缓存、导入） | Phase 2 Task 8-15 | covered |
| §6 Phase 3 详细设计（Curator、LLM、审计） | Phase 3 Task 16-25 | covered |
| §7 安全设计 | Phase 1-3 各 Task 中内嵌 | covered |
| §8 测试计划 | 每 Phase 最后 Task 为集成测试 | covered |
| §9 假设 | 在 Risk 表中跟踪 | covered |

---

## Parts (generate one per invocation, in order)

> ▶ To generate the next `pending` part: run `/compact`, then re-invoke the **`/writing-plans`** slash command. Do NOT type "continue" — it skips the rule reload and batch-generates everything.

| # | File | Scope | Status |
|---|---|---|---|
| 1 | `2026-06-02-agent-skill-system-phase1.md` | Phase 1: 模型扩展 + 系统提示增强 + 平台隔离 + 条件过滤 | done |
| 2 | `2026-06-02-agent-skill-system-phase2.md` | Phase 2: 动态预处理 + 缓存 + 外部目录导入 | done |
| 3 | `2026-06-02-agent-skill-system-phase3.md` | Phase 3: Curator LLM 审查 + 侧车数据 + 审计日志 | done |

---

## Self-Review (Index Level)

- [x] 1. spec-coverage table: 所有 § 已映射到具体 Phase/Tasks
- [x] 2. placeholder scan: index 中无 TODO/TBD
- [x] 3. no phantom tasks: index 只列文件和依赖概览，无空任务
- [x] 4. dependency soundness: Phase 1 → Phase 2 → Phase 3 顺序正确
- [x] 5. caller & build soundness: 共享签名变更集中在 Phase 1 Task 1/2（模型扩展），后续任务只添加方法不改变现有签名
- [x] 6. test-the-risk: 每 Phase 有独立集成测试任务
- [x] 7. type consistency: 模型扩展一次性完成，后续使用相同字段名
