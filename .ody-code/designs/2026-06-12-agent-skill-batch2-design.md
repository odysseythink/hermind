# Agent Skill System — Batch 2 智能审查增强设计

> **审计级别**: Deep  
> **批次**: Batch 2（LLM Review Fork + Umbrella Building + Dry-run Mode + Report Generation + Auxiliary Model Config）  
> **方案**: B — 完整 Review Fork  
> **日期**: 2026-06-11

---

## Scope In/Out

### In

| # | 功能 | 范围 | 来源 |
|---|------|------|------|
| 1 | **LLM Review Fork** | 创建独立 Review Agent 会话，使用 auxiliary 模型，最多 8 轮迭代审查 agent-created skills | [C:USER] |
| 2 | **Umbrella Building** | LLM 识别相似技能集群，创建 umbrella skill，原技能标记 archived + `absorbed_into` | [C:USER] |
| 3 | **Dry-run Mode** | 通过系统设置 `agent_skill_curator_dry_run=true` 触发，生成报告但不实际修改技能 | [C:USER] |
| 4 | **Report Generation** | 每次 Curator run 生成 `CuratorReport` 数据库记录 + `run.json` + `REPORT.md`，含完整 LLM 推理 | [C:USER] |
| 5 | **Auxiliary Curator Model Config** | 系统设置 `curator_llm_provider` / `curator_llm_model` / `curator_llm_api_key` | [C:USER] |
| 6 | **LLM Review 安全等级** | 系统设置 `curator_safety_level`（high/medium/low），默认 medium | [C:USER] |

### Out

| # | 功能 | 延后理由 |
|---|------|---------|
| 1 | 前端 Curator UI（状态/报告查看） | 前端改动独立，Batch 3 或独立任务 | [C:DEFERRED] |
| 2 | 自动首次运行 | 用户要求默认关闭，首次需手动触发 | [C:USER] |
| 3 | GEPA 自进化 | 独立仓库，非运行时内置 | [C:DEFERRED] |
| 4 | Cron job reference rewrite | 合并后更新 cron 引用，属于边缘 case | [C:DEFERRED] |
| 5 | 多 workspace 并行 Review | 先串行，后续优化 | [C:DEFERRED] |

---

## Architecture

```
Curator Worker (workers/skill_curator.go)
├── readConfig() — enabled/provider/model/dry-run/safety-level
├── if not enabled or first-run: return
├── BackupService.Snapshot(wsID) [Batch 1]
├── Deterministic transitions [Batch 1]
│   └── active→stale→archived (respect Pinned)
├── LLMReviewService.Review(ctx, wsID, opts)
│   ├── buildReviewAgent() — auxiliary model + restricted toolset
│   ├── iteration loop (max 8)
│   │   ├── skills_list → identify candidates
│   │   ├── skill_view → read content
│   │   ├── skill_manage (patch/consolidate/create) → execute decisions
│   │   └── classify: keep/patch/consolidate/archive
│   └── return decisions + reasoning
├── UmbrellaBuilder.Build(ctx, candidates)
│   └── cluster → create umbrella → archive absorbed skills
└── CuratorReportService.Save(ctx, report)
    ├── CuratorReport DB record
    ├── run.json (machine-readable)
    └── REPORT.md (human-readable)
```

---

## Parts

| # | File | Scope | Status |
|---|------|-------|--------|
| 1 | `agent-skill-batch2-design/core.md` | Data Models + Service Interfaces | **done** |
| 2 | `agent-skill-batch2-design/agent-fork.md` | Review Agent Runtime + Dry-run + Toolset Restriction | **done** |
| 3 | `agent-skill-batch2-design/orchestration.md` | Curator Integration + Algorithms + Tests | **done** |

---

## Data Models

新增与扩展的数据结构详见 `core.md` §1 与 `orchestration.md` §3.6。

| 模型/字段 | 位置 | 说明 | 来源 |
|---|---|---|---|
| `CuratorReport` | `backend/internal/models/curator_report.go` [新增] | 每次 curator run 的元数据、decisions JSON、reasoning、cost、status | [C:USER] |
| `CuratorReviewDecision` | 同文件 | 单个决策：skill、action、target、reason、applied、dryRun | [C:USER] |
| `AgentSkill.AbsorbedInto` | `backend/internal/models/agent_skill.go` [扩展] | 被 umbrella 吸收后记录目标 umbrella 名称 | [C:USER] |
| `AgentSkill.WriteOrigin` | 已存在（Batch 1） | umbrella 创建时设为 `"curator"` | [C:USER] |
| `dto.UpdateAgentSkillRequest.AbsorbedInto` | `backend/internal/dto/agent_skill.go` [扩展] | 支持通过 Update 写入吸收目标 | [C:INFERRED] |
| `dto.CreateAgentSkillRequest.WriteOrigin` | 已存在（Batch 1） | 创建 umbrella 时传入 `"curator"` | [C:USER] |

---

## Algorithms

非平凡算法分布在各 part 文件中：

| # | 算法 | 位置 | 核心职责 |
|---|---|---|---|
| 1 | `reviewRuntime.RunReview` | `agent-fork.md` §4.1 | 构建 auxiliary 模型、受限注册表、运行 Review Session、收集 tool calls |
| 2 | `buildReviewLanguageModel` | `agent-fork.md` §4.2 | 把 `curator_llm_*` 设置映射到现有 LLM 工厂 |
| 3 | `BuildReviewRegistry` | `agent-fork.md` §4.3 | 组装只读 skills_list / skill_view / rag-memory + skill_manage |
| 4 | `DryRunWrapper.Wrap` | `agent-fork.md` §4.4 | 拦截 mutating tool call，生成模拟决策 |
| 5 | `IsMutatingAction` | `agent-fork.md` §4.5 | allow-list 判断 tool + action 是否会产生副作用 |
| 6 | `RecordWrapper.Wrap` | `agent-fork.md` §4.6 | 在真实 handler 前后记录调用 |
| 7 | `LLMReviewService.Review` | `orchestration.md` §4.1 | 构造 prompt、调用 runtime、解析 final response 与 tool calls |
| 8 | `parseToolCalls` | `orchestration.md` §4.4 | 把 `skill_manage` tool calls 映射为 `CuratorReviewDecision` |
| 9 | `estimateCost` | `orchestration.md` §4.5 | 按 provider/model 估算 USD 成本 |
| 10 | `UmbrellaBuilder.Build` | `orchestration.md` §4.6 | 按 target umbrella 分组，创建 umbrella，归档被吸收 skill |
| 11 | `CuratorReportService.Finalize` / `SaveFiles` | `orchestration.md` §4.9 / §4.10 | 写入 DB 记录并落盘 `run.json` + `REPORT.md` |
| 12 | `SkillCuratorJob.Run` | `orchestration.md` §4.12 | 备份 → transitions → LLM Review → Umbrella → Report 编排 |

---

## Error Handling

跨组件错误处理详见 `agent-fork.md` §6 与 `orchestration.md` §6。关键模式：

| 层级 | 失败场景 | 处理策略 |
|---|---|---|
| Review Agent Runtime | 模型不可用 / 工具返回错误 / 达到最大迭代 | 按 safety level：high 中止并返回错误；medium/low 记录并返回部分结果 |
| LLMReviewService | final response 非 JSON / incomplete | 回退到原始文本作为 reasoning；incomplete 标记写入 summary |
| UmbrellaBuilder | 目标 skill 被 Pin / umbrella 名称冲突 | 跳过该 skill；冲突时追加序号或跳过 |
| CuratorReportService | 存储目录不可写 | 记录 error；报告 DB 记录仍存在 |
| SkillCuratorJob | 任意阶段失败 | high：回滚快照；medium：保留已应用变更并记录失败；low：忽略并继续 |

---

## Assumptions & Unverified Items

> 以下汇总自 `core.md`、`agent-fork.md`、`orchestration.md`。Deep 审计要求对每条 [C:INFERRED] 逐项确认。

| # | Assumption | Confidence | Impact if wrong | How to verify | 来源 |
|---|-----------|------------|-----------------|---------------|------|
| 1 | `conversation.New` 的 `WithMaxRounds` 选项可有效限制 Review Agent 轮数 | 高 | 轮数限制失效，可能跑出过长时间 | 单元测试验证 max iterations | agent-fork.md |
| 2 | `core.Response.Usage.TotalTokens` 在各 provider 下均被填充 | 中 | TokenUsage / 成本估算不准确 | 检查 Pantheon 源码或真实 provider 测试 | agent-fork.md |
| 3 | 包装 `tool.Entry.Handler` 不会影响 Pantheon 的工具分发与 schema 校验 | 高 | 工具调用失败或参数错位 | 单元测试 registry 分发 | agent-fork.md |
| 4 | `skill_manage` schema 增加可选 `reason` 字段不会破坏现有调用方 | 中 | 旧客户端若对额外字段报错则失败 | JSON schema 标记 optional；跑 `agent_skills_test.go` | agent-fork.md |
| 5 | Review Agent 在 prompt 约束下能输出可解析的 final JSON | 中 | `Reasoning` 字段需回退到原始文本 | 提示工程 + 集成测试 | agent-fork.md |
| 6 | `rag-memory/search` 不修改向量库状态，可作为只读工具 | 高 | dry-run 会引入未预期的写入 | 代码审查；`rag_memory.go:46-76` 仅查询 | agent-fork.md |
| 7 | `AgentSkillManager.GetBySlug` 返回 `Pinned` 字段且缺失 skill 返回 error | 高 | Dry-run 校验失效 | 已在 `agent_skills.go` 使用 | agent-fork.md |
| 8 | `AgentSkill` 可以增加 `AbsorbedInto` 字段而不破坏现有查询/API | 高 | 迁移失败或 JSON 序列化异常 | GORM AutoMigrate + 现有测试 | orchestration.md |
| 9 | `dto.UpdateAgentSkillRequest` 增加 `AbsorbedInto` 后，service Update 会将其写入 DB | 高 | umbrella 归档不记录吸收目标 | 检查 `AgentSkillService.Update` 实现 | orchestration.md |
| 10 | `skillSvc.List(ctx, wsID, false)` 返回 active + stale（不含 archived） | 高 | Review 候选集错误包含已归档 skill | 已读 `agent_skill_service.go:63` | orchestration.md |
| 11 | `BackupService.Restore` 能完整回滚 Review Agent 造成的 skill 变更 | 高 | high safety 回滚后仍有残留 | 已有 Batch 1 测试 | orchestration.md |
| 12 | 将 `skill_manage(create)` 统一映射为 `consolidate` 不会误标普通新建 | 中 | report 中 action 分类错误 | prompt 限制 review agent 只在 umbrella 场景调用 create | orchestration.md |
| 13 | 价格表覆盖主要 provider/model；缺失时 fallback 到 gpt-4o-mini | 中 | 成本估算偏高/偏低 | 维护 pricingMap；允许用户后续贡献 | orchestration.md |
| 14 | `<StorageDir>/curator-reports` 可写且不会暴露到 web root | 高 | 报告文件泄露 | 使用 0750/0640；静态文件路由仅服务 frontend/dist | orchestration.md |
| 15 | `CuratorReport.DecisionsJSON` 由 GORM hook 与 `Decisions` 互转 | 高 | DB 不保存 decisions | 在 model hook 中实现（见 core.md §3.1） | orchestration.md |
| 16 | `agent_skill_curator_enabled` 默认 false 的改动不会破坏现有期望默认开启的测试 | 中 | 现有 curator 测试失败 | 更新 `skill_curator_test.go` 中显式设置 enabled | orchestration.md |

---

## Risk Register

| # | Risk | Likelihood | Impact | Mitigation |
|---|------|-----------|--------|-----------|
| 1 | Review agent in live mode deletes useful skills | 中 | 数据丢失 | Pin 保护 + Batch 1 自动快照 + deterministic transitions 先行 |
| 2 | Dry-run wrapper fails to block a mutating action due to action name mismatch | 低 | 未预期 DB 变更 | 单元测试每个 mutating action；使用 allow-list |
| 3 | Auxiliary model provider misconfigured causes curator job to fail silently | 中 | 无报告 | 返回 error；report status=failed；fallback 主模型 |
| 4 | Large workspace leads to high review cost | 中 | API 费用高 | 限制 max_iterations；候选集可先按 category 分批（未来优化） |
| 5 | Tool-call recordings capture sensitive content in report | 低 | PII/skill 内容泄露 | 报告目录与 storage 同权限；不对外提供 HTTP 访问 |
| 6 | Safety=high restore snapshot fails, leaving bad changes | 低 | 错误变更残留 | restore 失败记 error；用户可手动从 skill-backups 恢复 |
| 7 | Umbrella name collides with existing skill | 中 | 创建失败或覆盖 | 先 GetBySlug 检查；冲突时追加序号或跳过 |
| 8 | `AbsorbedInto` field missing breaks umbrella traceability | 中 | 报告不完整 | 本设计已明确补充该字段；实现时优先迁移 |

---

## Self-Review

### 审查策略

重点审查 3 个“错了最贵”的决策：
1. `parseToolCalls` 中 `skill_manage(create)` → `consolidate` 的映射。
2. `IsMutatingAction` 的 allow-list 是否漏掉 mutating action。
3. `estimateCost` 的 fallback 是否会让本地/低成本模型被高估。

### 昂贵决策的 adversarial 输入

**1. create → consolidate 映射**
- 输入 A：Review agent 调用 `skill_manage(create, name="api-client-pattern", reason="Create umbrella for HTTP client skills")`，且后续有 `delete(absorbed_into="api-client-pattern")`。
  - 期望：create 决策 action=consolidate，delete 决策 action=archive。
- 输入 B：Review agent 误调用 `skill_manage(create, name="standalone-new-skill")` 且无后续 delete。
  - 期望：仍被标记为 consolidate。这是已知限制；prompt 明确禁止此类 create。
- 输入 C：Agent 使用 `edit` 而非 `patch`。
  - 期望：映射为 patch 决策。符合设计。

**2. IsMutatingAction allow-list**
- 输入 A：`skill_manage(action="patch")` → mutating=true。
- 输入 B：`skill_manage(action="Delete")`（大写）→ schema enum 限制为小写，实际不会进入 wrapper；若出现则视为非 mutating，但 inner handler 会返回 error。
- 输入 C：`rag-memory(action="store")` → mutating=true，wrapper 拒绝。

**3. estimateCost fallback**
- 输入 A：provider="ollama", model="llama3" → map 命中 "ollama/default"，cost=0。
- 输入 B：provider="togetherai", model="unknown" → 无命中，fallback "openai/gpt-4o-mini"，cost 偏高但安全。
- 输入 C：tokenUsage=0 → cost=0，无除零。

### 四镜头审查

- **Security**：工具集限制排除了 MCP/AgentFlow/文件系统；dry-run 使用 allow-list 拦截 mutating action；报告目录使用 0750/0640 且不在 web root；Pin 检查防止误改；`reason` 字段为可选字符串，无代码注入风险。未发现新增 secrets 泄露路径。
- **Test**：Part 2/3 的测试计划覆盖了 restricted registry、dry-run wrapper、max iterations、token usage、parsing decisions、umbrella creation、report files、safety rollback、default disabled。每个核心行为都有 must-pass 与 must-reject case。
- **Ops**：RunID 使用 `YYYYMMDDHHMMSS`，同一 workspace 秒级冲突概率低（cron 日常）；high safety 提供 snapshot restore；cost 估算使用固定价格表；report 目录按 workspace/runID 隔离。
- **Integration**：依赖的数据源/钩子均已验证存在：`BackupManager.Restore`（`skill_backup_service.go`）、`ProvenanceRecorder.Record`（`provenance_service.go`）、`AgentSkillManager` 全接口（`agent_skill_service.go:59-78`）、`SystemService.GetSetting`（`system_service.go:21`）、`Session` 与 `conversation.WithMaxRounds`（`session.go:113`）。设计落地位置为用户命名的 `backend/internal/` 与 `backend/cmd/server/main.go`，无静默转移目标。
- **Scope**：本批次仍是单一 coherent 设计（Batch 2 curator enhancement），未膨胀为多个独立子项目。Umbrella/Report/Dry-run 都服务于同一 curator 工作流。

### 修复项

- 发现 `AgentSkill` 模型缺少 `AbsorbedInto` 字段（core.md 未覆盖），已在 `orchestration.md` §3.6 补充，并纳入假设表 #8/#9。
- 发现 `CuratorReport` 的 `DecisionsJSON` hook 需要实现，已在 `core.md` §3.1 与假设表 #15 标注。

---

## User Final Approval

- **状态**: 待审批
- **审计级别**: Deep
- **审批时间**: —
