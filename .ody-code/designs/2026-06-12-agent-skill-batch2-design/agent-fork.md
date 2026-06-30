# Batch 2 — Part 2: Review Agent Runtime + Dry-run + Toolset Restriction

> 范围：LLM Review Agent 的独立运行时会话、受限工具注册表、Dry-run 拦截层，以及它们与 `LLMReviewService` / `skill_manage` 的集成点。

---

## 1. Scope In/Out

### 1.1 In

| # | 功能 | 说明 | 来源 |
|---|------|------|------|
| 1 | **ReviewAgentRuntime** | 在 `backend/internal/agent/review_agent.go` 中实现，复用现有 `Session` 但使用 auxiliary 模型、最大 8 轮、受限工具集 | [C:USER] |
| 2 | **受限工具集** | Review 注册表仅包含 `skills_list`、`skill_view`、`skill_manage`、只读 `rag-memory/search`；不含 MCP、AgentFlow、文件系统、邮件/日历等 | [C:USER] |
| 3 | **Dry-run 拦截** | `DryRun=true` 时，`skill_manage` 的所有写操作与 `rag-memory/store` 只模拟、不修改 DB/文件系统，并生成结构化决策记录 | [C:USER] |
| 4 | **Tool-call 记录** | 记录 Review Agent 每次调用的 tool name、args、result，用于后续生成 `CuratorReport` | [C:USER] |
| 5 | **Review 系统提示** | 在 `backend/internal/agent/system_prompt.go` 新增 curator review prompt，定义审查标准、输出 JSON 格式、停止标记 | [C:USER] |
| 6 | **Auxiliary 模型构造** | 根据 `curator_llm_provider` / `curator_llm_model` / `curator_llm_api_key` 构建 `core.LanguageModel`，缺省回退到主模型 | [C:USER] |

### 1.2 Out

| # | 功能 | 延后理由 |
|---|------|---------|
| 1 | **Umbrella 聚类算法** | 属于 `orchestration.md`（Part 3）的编排逻辑 |
| 2 | **CuratorReport 文件落盘** | 属于 Part 3 的报告服务 |
| 3 | **Safety-level 完整回滚/继续策略** | high/medium/low 在编排层统一处理，Part 2 仅把 safety 传入并用于运行时级失败判断 |
| 4 | **前端 Curator 状态/报告 UI** | 前端独立，Batch 3 或单独任务 |

---

## 2. Architecture & Data Flow

```
LLMReviewService.Review(ctx, wsID, opts)
        │
        ▼
mapReviewOptionsToSettings(opts)  ──► map[string]string
        │
        ▼
ReviewAgentRuntime.RunReview(ctx, wsID, prompt, modelSettings)
        │
        ├── load workspace
        ├── buildReviewLanguageModel(modelSettings, cfg) ──► core.LanguageModel
        ├── resolveCuratorReviewPrompt(ws) ──► systemPrompt
        ├── newReviewAccumulator()
        ├── buildReviewRegistry(deps, dryRun, accum, safety) ──► *tool.Registry
        │       ├─ skills_list   (real + recorder)
        │       ├─ skill_view    (real + recorder)
        │       ├─ rag-memory    (search-only + recorder)
        │       └─ skill_manage  (dry-run wrapper + recorder)
        │
        └─ newReviewSession(ctx, ws, lm, systemPrompt, reg, maxIter)
                │
                ▼
              Session.Run(prompt)
                │
                ▼
              ReviewAgentResult { FinalResponse, ToolCalls, TokenUsage }
```

数据变化：
- **Dry-run**：`skill_manage` 调用不修改 `agent_skills` / `agent_skill_files`，只向 `accumulator` 追加 `ReviewAgentToolCall`。
- **Live**：`skill_manage` 正常执行，并通过 `ProvenanceSvc` 记录变更来源（Batch 1）。
- **返回值**：`FinalResponse` 是 LLM 最终输出的 JSON 字符串（含 `reasoning` 与 `review_complete` 标记），`ToolCalls` 供 LLMReviewService 解析为 `CuratorReviewDecision`。

---

## 3. Interfaces & Types

### 3.1 复用接口（定义见 `core.md`）

- `ReviewAgentRuntime`：见 `core.md` §2.4
- `ReviewAgentResult`：见 `core.md` §2.4
- `ReviewAgentToolCall`：见 `core.md` §2.4

### 3.2 Review Agent 运行时实现

文件：`backend/internal/agent/review_agent.go` [新增] [C:USER]

```go
type reviewRuntime struct {
    cfg      *config.Config
    db       *gorm.DB
    skillSvc services.AgentSkillManager
    vs       tools.VectorSearcher
    sysSvc   *services.SystemService
}

// NewReviewAgentRuntime 创建 Review Agent 运行时。
// 它只依赖配置、DB、技能管理、向量搜索和系统设置，不依赖完整 agent.Runtime。
func NewReviewAgentRuntime(
    cfg *config.Config,
    db *gorm.DB,
    skillSvc services.AgentSkillManager,
    vs tools.VectorSearcher,
    sysSvc *services.SystemService,
) ReviewAgentRuntime
```

### 3.3 运行状态收集器

文件：`backend/internal/agent/review_agent.go` [同文件] [C:INFERRED]

```go
type reviewAccumulator struct {
    calls         []ReviewAgentToolCall
    finalResponse string
    tokenUsage    int
}

func (a *reviewAccumulator) record(toolName string, args json.RawMessage, result string)
func (a *reviewAccumulator) captureResponse(resp *core.Response)
```

### 3.4 模型使用量包装器

文件：`backend/internal/agent/review_agent.go` [同文件] [C:INFERRED]

```go
type usageCaptureLM struct {
    inner core.LanguageModel
    accum *reviewAccumulator
}

func (m *usageCaptureLM) Generate(ctx context.Context, req *core.Request) (*core.Response, error)
func (m *usageCaptureLM) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error)
```

### 3.5 Review 工具注册表构造器

文件：`backend/internal/agent/review_registry.go` [新增] [C:USER]

```go
// ReviewRegistryDeps 是构建 Review 工具集所需的最小依赖。
type ReviewRegistryDeps struct {
    Workspace     *models.Workspace
    SkillSvc      services.AgentSkillManager
    VectorSearch  tools.VectorSearcher
    ProvenanceSvc services.ProvenanceRecorder
}

// BuildReviewRegistry 构建受限工具注册表。
func BuildReviewRegistry(
    deps ReviewRegistryDeps,
    dryRun bool,
    accum *reviewAccumulator,
) *tool.Registry
```

### 3.6 Dry-run 拦截层

文件：`backend/internal/agent/tools/review_dryrun.go` [新增] [C:USER]

```go
// DryRunWrapper 把 mutating tool 替换为模拟执行。
type DryRunWrapper struct {
    skillSvc services.AgentSkillManager
    accum    *reviewAccumulator // 通过闭包引用，记录模拟决策
}

// Wrap 返回被 dry-run 包裹的 tool.Entry。
func (w *DryRunWrapper) Wrap(entry *tool.Entry) *tool.Entry

// IsMutatingAction 判断某个 tool + action 是否会产生副作用。
func IsMutatingAction(toolName, action string) bool
```

### 3.7 Tool-call 记录器

文件：`backend/internal/agent/tools/review_recorder.go` [新增] [C:USER]

```go
// RecordWrapper 在真实 handler 执行前后记录调用。
type RecordWrapper struct {
    accum *reviewAccumulator
}

func (w *RecordWrapper) Wrap(entry *tool.Entry) *tool.Entry
```

---

## 4. Algorithms

### 4.1 `reviewRuntime.RunReview`

```
function RunReview(ctx, workspaceID, prompt, modelSettings):
    ws := loadWorkspace(ctx, workspaceID)
    if ws not found:
        return error

    maxIter := parseInt(modelSettings["curator_max_iterations"], 8)
    dryRun  := modelSettings["curator_dry_run"] == "true"
    safety  := modelSettings["curator_safety_level"]  // default "medium"

    lm, err := buildReviewLanguageModel(modelSettings, cfg)
    if err != nil:
        return error

    systemPrompt := resolveCuratorReviewPrompt(ws)
    accum := new(reviewAccumulator)
    instrumentedLM := &usageCaptureLM{inner: lm, accum: accum}

    regDeps := ReviewRegistryDeps{
        Workspace:     ws,
        SkillSvc:      skillSvc,
        VectorSearch:  vs,
        ProvenanceSvc: nil,  // dry-run 不记录来源；live 模式在 skill_manage 内部自行使用 ProvenanceSvc
    }
    reg := BuildReviewRegistry(regDeps, dryRun, accum)

    session := newReviewSession(ctx, ws, instrumentedLM, systemPrompt, reg, maxIter)
    runErr := session.Run(prompt)

    result := &ReviewAgentResult{
        FinalResponse: accum.finalResponse,
        ToolCalls:     accum.calls,
        TokenUsage:    accum.tokenUsage,
    }

    if runErr != nil:
        if safety == "high":
            return nil, runErr
        // medium/low: 记录错误，返回已收集的部分结果
        logWarning("review runtime error (safety=%s): %v", safety, runErr)

    return result, nil
```

### 4.2 `buildReviewLanguageModel`

```
function buildReviewLanguageModel(settings, cfg):
    mapped := map[string]string{
        "LLMProvider": settings["curator_llm_provider"] or cfg.LLMProvider,
    }

    provider := mapped["LLMProvider"]
    mapped["OpenAiModelPref"] = settings["curator_llm_model"] or cfg.LLMModel
    mapped["OllamaLLMModelPref"] = settings["curator_llm_model"] or cfg.LLMModel

    apiKey := settings["curator_llm_api_key"]
    if apiKey != "":
        mapped["LLMApiKey"] = apiKey
        mapped["OpenAiKey"] = apiKey
    else:
        mapped["LLMApiKey"] = cfg.LLMApiKey
        mapped["OpenAiKey"] = cfg.OpenAiKey

    return buildLanguageModel(ws, mapped, cfg)   // 复用 llm_factory.go 已有工厂
```

### 4.3 `BuildReviewRegistry`

```
function BuildReviewRegistry(deps, dryRun, accum):
    reg := tool.NewRegistry()
    tc := &tools.ToolContext{
        Workspace:     deps.Workspace,
        VectorSearchSvc: deps.VectorSearch,
        AgentSkillSvc: deps.SkillSvc,
        ProvenanceSvc: nil,
        Approval:      nil,  // Review Agent 不需要人工审批
        Settings:      map[string]string{},
    }

    recorder := &RecordWrapper{accum: accum}

    // 只读 skill 列表与查看
    reg.Register(recorder.Wrap(NewSkillsListSkill(tc, deps.SkillSvc)))
    reg.Register(recorder.Wrap(NewSkillViewSkill(tc, deps.SkillSvc)))

    // 只读 RAG memory
    reg.Register(recorder.Wrap(NewReviewRAGMemorySkill(tc)))

    // skill_manage：live 直接执行；dry-run 加模拟层
    manage := NewSkillManageSkill(tc, deps.SkillSvc, deps.ProvenanceSvc)
    if dryRun:
        dry := &DryRunWrapper{skillSvc: deps.SkillSvc, accum: accum}
        manage = dry.Wrap(manage)
    reg.Register(recorder.Wrap(manage))

    return reg
```

### 4.4 `DryRunWrapper.Wrap` / 模拟执行

```
function Wrap(entry):
    inner := entry.Handler
    entry.Handler := func(ctx, raw):
        args := parseSkillManageArgs(raw)
        if args == nil or !IsMutatingAction(entry.Name, args.Action):
            // 非 mutating action（理论上 skill_manage 全是）直接放行
            return inner(ctx, raw)

        // 验证目标存在、未被 Pin
        skill, err := skillSvc.GetBySlug(ctx, wsID, slugifyForLookup(args.Name))
        if err != nil and args.Action != "create":
            return tool.Error("dry-run: skill not found")
        if skill != nil and skill.Pinned:
            return tool.Error("dry-run: skill is pinned")

        record := map[string]any{
            "success":    true,
            "dry_run":    true,
            "tool":       entry.Name,
            "action":     args.Action,
            "skill_name": args.Name,
            "skill_slug": slugifyForLookup(args.Name),
            "reason":     args.Reason,
        }
        if args.Action == "delete":
            record["absorbed_into"] = args.AbsorbedInto
        if args.Action in {"patch", "edit", "write_file"}:
            record["content_preview"] = truncate(args.NewString or args.Content or args.FileContent, 500)

        // 向 accumulator 记录一次模拟决策
        accum.record(entry.Name, raw, tool.Result(record))
        return tool.Result(record), nil
```

### 4.5 `IsMutatingAction`

```
function IsMutatingAction(toolName, action):
    if toolName == "skill_manage":
        return action in {"create", "edit", "patch", "delete", "write_file", "remove_file"}
    if toolName == "rag-memory":
        return action == "store"
    return false
```

### 4.6 `RecordWrapper.Wrap`

```
function Wrap(entry):
    inner := entry.Handler
    entry.Handler := func(ctx, raw):
        result, err := inner(ctx, raw)
        // err 是 Go 错误时才记录失败；tool.Error 返回的是正常字符串结果
        if err != nil:
            accum.record(entry.Name, raw, "ERROR: " + err.Error())
        else:
            accum.record(entry.Name, raw, result)
        return result, err
```

### 4.7 `newReviewSession`

```
function newReviewSession(parentCtx, ws, lm, systemPrompt, reg, maxIter):
    ctx, cancel := context.WithCancel(parentCtx)
    s := &Session{...}   // 与 newSession 相同字段初始化
    s.conv := conversation.New(conversation.WithMaxRounds(maxIter + 2))
    s.conv.RegisterParticipant(userNoopModel)   // USER 占位，仅用于启动
    s.initAgent(lm, reg)                        // 注册 @agent
    return s
```

### 4.8 `resolveCuratorReviewPrompt`

Prompt 模板（关键指令）：

```
You are a skill curator for workspace {{ws.name}} (slug: {{ws.slug}}).
Your job is to review agent-created procedural-memory skills and decide:
- keep: skill is useful and well-structured
- patch: minor improvements needed (use skill_manage patch)
- consolidate: a group of similar skills should be merged into a new umbrella skill
- archive: skill is redundant, stale, or low quality

Rules:
- Only review skills where created_by == "agent" and pinned == false.
- Use skills_list and skill_view to inspect skills. Use rag-memory search for context.
- To consolidate: create an umbrella skill with skill_manage(create), then delete absorbed skills with skill_manage(delete, absorbed_into="umbrella-name").
- For every mutating action, include a short "reason" argument explaining why.
- NEVER call tools outside the skill-management set.
- When finished, output exactly this JSON and nothing else:
  {"reasoning": "<summary of review and decisions>", "review_complete": true}
```

---

## 5. Call-Site Integration

### 5.1 新增文件：`backend/internal/agent/review_agent.go`

实现 `NewReviewAgentRuntime` 与 `RunReview`。

### 5.2 修改：`backend/internal/agent/session.go`（约第 113 行）

现有 `newSession` 硬编码 `conversation.WithMaxRounds(defaultMaxRounds)`。为 Review Agent 新增构造函数：

```go
// newReviewSession 创建用于 Curator Review 的只会话。
func newReviewSession(
    parentCtx context.Context,
    ws *models.Workspace,
    lm core.LanguageModel,
    systemPrompt string,
    reg *tool.Registry,
    maxRounds int,
) *Session { ... }
```

### 5.3 修改：`backend/internal/agent/system_prompt.go`

新增：

```go
func resolveCuratorReviewPrompt(ws *models.Workspace) string
```

### 5.4 修改：`backend/internal/agent/tools/agent_skills.go`（约第 39-50 行）

在 `skillManageArgs` 和 JSON schema 中增加可选字段 `reason`：

```go
type skillManageArgs struct {
    ...
    Reason string `json:"reason,omitempty"`
}
```

schema 中 `reason` 字段为可选字符串，供 Review Agent 说明每个决策的意图。

### 5.5 新增文件：`backend/internal/agent/tools/review_dryrun.go` 与 `review_recorder.go`

实现 `DryRunWrapper` 与 `RecordWrapper`。

### 5.6 调用方：`backend/internal/services/llm_review_service.go`

```go
func (s *llmReviewService) Review(ctx context.Context, workspaceID int, opts ReviewOptions) (*ReviewResult, error) {
    settings := map[string]string{
        "curator_llm_provider": opts.ModelProvider,
        "curator_llm_model":    opts.ModelID,
        "curator_llm_api_key":  opts.APIKey,
        "curator_dry_run":      strconv.FormatBool(opts.DryRun),
        "curator_max_iterations": strconv.Itoa(opts.MaxIterations),
        "curator_safety_level": opts.SafetyLevel,
    }
    prompt := buildReviewPrompt(...)   // Part 3 定义
    agentResult, err := s.runtime.RunReview(ctx, workspaceID, prompt, settings)
    ...
}
```

### 5.7 初始化：`backend/cmd/server/main.go`

```go
reviewRuntime := agent.NewReviewAgentRuntime(cfg, db, agentSkillSvc, vectorSearchSvc, sysSvc)
llmReviewSvc := services.NewLLMReviewService(db, agentSkillSvc, backupSvc, reviewRuntime, provenanceSvc)
```

---

## 6. Error Handling & Degradation

| 错误类别 | 即时处理 | 降级路径 | 恢复条件 |
|---|---|---|---|
| Curator 模型未配置 / key 缺失 | 返回 error；orchestration 标记 report `failed` | 若 `curator_llm_provider` 为空，回退到 `cfg.LLMProvider` [C:INFERRED] | 用户配置 curator 模型或依赖回退 |
| 模型输出非 JSON / 缺少 `review_complete` | 将原始文本作为 reasoning；orchestration 推断 `incomplete=true` | 仍从 `ToolCalls` 解析已执行决策 | 提示词优化 / 重试 |
| 工具返回错误字符串（如 skill 不存在） | 把错误返回给 LLM，由 LLM 决定下一步 | medium/low 继续；high 在运行时级 Go error 时中止 | LLM 重试或跳过 |
| 达到最大迭代次数 | 停止 conversation；返回已收集结果 | orchestration 根据 safety 决定是否应用部分决策 | 用户增大 `curator_max_iterations` |
| Dry-run 拦截到 Pin 保护 skill | 返回 error 字符串；不生成决策 | Agent 选择其他候选 | 用户手动 unpin |
| 上下文取消 | 中止；返回 error | report 标记 failed | 下次 cron/manual 重试 |

---

## 7. Test Plan

新增/修改测试文件：

- `backend/internal/agent/review_agent_test.go`
- `backend/internal/agent/tools/review_dryrun_test.go`
- `backend/internal/agent/tools/review_registry_test.go`

| # | 用例 | 断言 |
|---|------|------|
| 1 | `TestReviewRegistry_OnlyAllowedTools` | registry 中工具名集合 `==` `{skills_list, skill_view, skill_manage, rag-memory}`；无 MCP/flow/文件系统工具 |
| 2 | `TestReviewRAGMemory_StoreRejected` | `rag-memory(action="store")` 返回 error 或空结果；`action="search"` 可正常调用 |
| 3 | `TestDryRunWrapper_BlocksCreate` | dry-run 下调用 `skill_manage(create)` 后，DB 中不存在该 skill；返回 JSON 包含 `dry_run:true` |
| 4 | `TestDryRunWrapper_RecordsDeleteDecision` | dry-run 下调用 `skill_manage(delete, absorbed_into="umbrella")` 后，accumulator 中 `ReviewAgentToolCall.Result` 包含 `action:delete` 与 `absorbed_into:umbrella` |
| 5 | `TestDryRunWrapper_PinProtection` | 对 `Pinned=true` 的 skill 调用 `skill_manage(patch/delete)` 返回错误，且不生成决策 |
| 6 | `TestReviewRuntime_MaxIterations` | mock LM 每轮都请求 `skills_list`；断言调用次数不超过 `maxIterations`；`FinalResponse` 不含 `review_complete` |
| 7 | `TestReviewRuntime_TokenUsageCapture` | mock LM 返回 `core.Response{Usage: core.Usage{TotalTokens: 42}}`；断言 `ReviewAgentResult.TokenUsage == 42` |
| 8 | `TestSkillManageReasonField` | schema 接受带 `reason` 的请求；handler 在 dry-run 结果中保留 `reason` |
| 9 | `TestReviewLanguageModel_FallsBackToMain` | `curator_llm_provider` 为空时，构造的模型使用 `cfg.LLMProvider` / `cfg.LLMModel` |

**Done criteria**：

```bash
cd backend && go test -tags=fts5 ./internal/agent/...
```

---

## 8. Assumptions & Unverified Items

| # | 假设 | 置信度 | 错误影响 | 验证方式 |
|---|------|--------|----------|----------|
| 1 | `conversation.New` 的 `WithMaxRounds` 选项可有效限制 Review Agent 轮数 | 高 | 轮数限制失效，可能跑出过长时间 | 已在 `session.go:113` 使用；增加单元测试验证 |
| 2 | `core.Response.Usage.TotalTokens` 在各 provider 下均被填充 | 中 | TokenUsage / 成本估算不准确 | 检查 Pantheon 源码或运行真实 provider 测试 |
| 3 | 包装 `tool.Entry.Handler` 不会影响 Pantheon 的工具分发与 schema 校验 | 高 | 工具调用失败或参数错位 | 单元测试 registry 分发 |
| 4 | `skill_manage` schema 增加可选 `reason` 字段不会破坏现有调用方 | 中 | 旧客户端若对额外字段报错则失败 | JSON schema 标记为 optional；跑 `agent_skills_test.go` |
| 5 | Review Agent 在 prompt 约束下能输出可解析的 final JSON | 中 | `Reasoning` 字段需回退到原始文本 | 提示工程 + 集成测试 |
| 6 | `rag-memory/search` 不修改向量库状态，可作为只读工具 | 高 | dry-run 会引入未预期的写入 | 代码审查；`rag_memory.go:46-76` 仅查询 |
| 7 | `AgentSkillManager.GetBySlug` 返回 `Pinned` 字段且缺失 skill 返回 error | 高 | Dry-run 校验失效 | 已在 `agent_skills.go` 使用 |

---

## 9. Risk Register

| # | 风险 | 可能性 | 影响 | 缓解措施 |
|---|------|--------|------|----------|
| 1 | Live 模式下 Review Agent 误删有用 skill | 中 | 数据丢失 | Pin 保护 + Batch 1 自动快照 + deterministic transitions 先行 |
| 2 | Dry-run 因 action 名称匹配错误漏过写操作 | 低 | 未预期 DB 变更 | 使用显式 allow-list；单元测试覆盖所有 mutating action |
| 3 | Auxiliary 模型配置错误导致 curator 静默失败 | 中 | 无报告 | 返回 error 并设置 report `failed`；缺省回退主模型 |
| 4 | 大 workspace 技能过多导致 Review 成本过高 | 中 | API 费用高 | 限制 `maxIterations`；可选在 Part 3 增加候选集大小上限 |
| 5 | Tool-call 记录中捕获 skill 内容造成敏感信息泄露 | 低 | PII/业务逻辑进入报告 | 报告文件与 DB 同权限管理；不向外发送 |

---

## 10. Local Notes

- **与 `core.md` 的关系**：本文件不修改 `ReviewAgentRuntime` / `ReviewAgentResult` 的接口签名；所有扩展（如 incomplete 推断）通过 `FinalResponse` JSON 内容或 orchestration 层计算完成。若实现时发现必须扩展返回类型，再回到 `core.md` 同步。
- **与 Batch 1 的关系**：`skill_manage` 的 Pin 检查、Provenance 记录、`AbsorbedInto` 字段均已存在，直接复用。
- **辅助模型回退**：当 `curator_llm_provider` 为空时，通过 `buildReviewLanguageModel` 映射到主模型设置，保证未配置时仍可用（但用户要求默认关闭 curator，首次手动触发）。
