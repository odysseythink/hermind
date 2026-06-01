# Context Compression Engine — Design Document

> **功能**: 将 Hermes Agent 的上下文压缩引擎引入 Hermind
> **日期**: 2026-05-28
> **状态**: Approved
> **方案**: A（统一 ContextCompressionService + 适配 Pantheon 接口）

---

## 1. 背景与目标

### 1.1 问题

Hermind 当前没有上下文压缩机制：
- **Regular Chat**：仅通过 `Workspace.OpenAiHistory`（默认 20 条消息）限制历史长度，无 token 预算意识，长会话直接撞上下文上限
- **Agent Chat**：Pantheon 内置 `compression.Compressor` 但过于简单（固定 head=3/tail=20，bullet-point 摘要），且 Hermind 当前未启用

### 1.2 目标

引入 Hermes Agent 的**四阶段上下文压缩引擎**，覆盖 Regular Chat 和 Agent Chat 两条路径：
- Phase 1：工具输出预剪枝（零 LLM 成本）
- Phase 2：边界划定（Token 预算制尾保护）
- Phase 3：结构化 LLM 摘要（迭代更新）
- Phase 4：组装与完整性修复（角色冲突 + tool 对修复）

### 1.3 非目标

- 不实现 `/compress <topic>` 用户交互（Phase 1 预留 `focusTopic` 字段，后续可扩展）
- 不实现模型元数据的自动更新（内嵌静态映射表，手动维护）
- 不修改 Pantheon SDK 本身（通过适配器模式集成）

---

## 2. 架构概览

### 2.1 新增包

```
backend/internal/agent/compression/
├── compressor.go              # 主入口：HermesCompressor
├── config.go                  # CompressionConfig
├── phase1_pruner.go           # 工具输出预剪枝
├── phase2_boundary.go         # 边界划定
├── phase3_summarizer.go       # 结构化 LLM 摘要
├── phase4_assembler.go        # 组装与完整性修复
├── model_metadata.go          # 模型上下文长度映射
├── token_estimator.go         # Token 估算
├── redactor.go                # 敏感信息脱敏
├── prompts/
│   └── summary_template.txt   # 结构化摘要模板
└── *_test.go
```

### 2.2 组件职责

| 组件 | 职责 | 依赖 |
|------|------|------|
| `HermesCompressor` | 编排四阶段流水线，维护 per-session 状态，实现 Pantheon `compression.Compressor` 接口 | 全部 |
| `ToolResultPruner` | Phase 1：MD5 去重、图片剥离、JSON 安全截断、单行摘要 | `TokenEstimator` |
| `BoundaryAnalyzer` | Phase 2：Head 保护、Token 预算尾保护、边界对齐、用户消息锚定 | `TokenEstimator`, `ModelMetadata` |
| `Summarizer` | Phase 3：结构化摘要生成、迭代更新、脱敏、失败冷却 | auxiliary `core.LanguageModel` |
| `Assembler` | Phase 4：角色选择、SUMMARY_PREFIX、合并到尾部、tool 对修复 | — |
| `ModelMetadata` | 模型名 → 上下文长度映射 + Workspace 覆盖 | — |
| `TokenEstimator` | 多模态消息 token 估算（文本 + 图片 + tool 参数） | — |
| `Redactor` | 敏感信息脱敏（API keys, tokens, passwords） | — |

### 2.3 系统集成

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Regular Chat (HTTP API)                          │
│  ChatService.Stream() / Complete()                                      │
│    ├─ buildRAGContext() → buildChatHistory()                            │
│    ├─ 【新增】ContextCompression.CompressIfNeeded()                     │
│    │   (load previous_summary from DB if threadID)                      │
│    └─ llmProv.Stream(messages, systemPrompt)                            │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                        Agent Chat (WebSocket)                           │
│  Runtime.RunAgentDirectly() / HandleWS()                                │
│    ├─ newSession()                                                      │
│    ├─ buildCompressor(ws, lm) → HermesCompressor{per-session state}    │
│    ├─ pantheonAgent.New(lm,                                            │
│    │     WithRegistry(reg),                                             │
│    │     WithMaxSteps(10),                                              │
│    │     WithCompressor(compressor))  ← 每模型调用前自动触发           │
│    └─ conv.Start() → Agent RunStream()                                  │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 3. 核心压缩引擎 — 四阶段流水线

### 3.1 Phase 1：工具输出预剪枝（`phase1_pruner.go`）

**输入**：`[]core.Message`，尾部 token 预算，最小尾部消息数
**输出**：剪枝后的 `[]core.Message`，剪枝计数

**处理逻辑**（从后向前保护尾部）：

| 策略 | 触发条件 | 处理方式 |
|------|---------|---------|
| MD5 去重 | `ToolResultPart` 文本内容长度 > 200，且与后续消息内容 MD5 相同 | 替换为 `"[Duplicate tool output — same content as a more recent call]"` |
| 图片剥离 | 消息包含 `ImagePart` | `ImagePart` 替换为 `TextPart{"[screenshot removed to save context]"}` |
| JSON 安全截断 | `ToolCallPart.Arguments` 长度 > 500 | 解析 JSON，递归截断长字符串叶子节点，保证 JSON 合法性 |
| 信息性单行摘要 | 旧 `ToolResultPart` 文本内容长度 > 200 | 按工具名生成摘要 |

**工具摘要映射表**（覆盖 Hermind 当前工具集）：

| 工具名 | 摘要格式 |
|--------|---------|
| `browser_navigate` / `click` / `snapshot` | `[browser_navigate] {url} ({len} chars)` |
| `create_files` | `[create_files] wrote {path} ({lines} lines)` |
| `web_scraping` | `[web_scraping] {url} ({len} chars result)` |
| `session_search` | `[session_search] query='{q}' ({count} matches)` |
| `terminal` | `[terminal] ran \`{cmd}\` -> exit {code}, {n} lines output` |
| （通用 fallback） | `[{name}] {args_preview} ({len} chars result)` |

### 3.2 Phase 2：边界划定（`phase2_boundary.go`）

**算法**：

1. **Head 保护**：`headEnd = (system ? 1 : 0) + protectHeadCount`
2. **Tail Token 预算**（从尾部反向累加）：
   - `minTail = max(3, len(messages) - headEnd - 1)`
   - `softCeiling = tailTokenBudget * 1.5`
   - 从后向前遍历，累加 `EstimateTokens(msg)`
   - 当 `accumulated > softCeiling` 且已保护消息数 ≥ `minTail` 时停止
3. **边界对齐 — 向后调整**（`_alignBoundaryBackward`）：
   - 如果 `tailStart` 落在 tool result 群组中间，向后拉直到包含完整 `assistant(tool_calls) + tool(results)` 群组
4. **用户消息锚定**（`_ensureLastUserMessageInTail`）：
   - 如果最新的 user 消息不在 Tail 中，将 `tailStart` 向前拉以包含它（修复 Hermes Bug #10896）

**Token 估算**（`token_estimator.go`）：

```go
func (e *TokenEstimator) EstimateMessage(msg core.Message) int {
    tokens := 10  // role + metadata 开销
    for _, part := range msg.Content {
        switch p := part.(type) {
        case core.TextPart:
            tokens += estimateTextTokens(p.Text)  // (len + 3) / 4
        case core.ImagePart:
            tokens += 1600  // 统一按 1600 token 估算
        case core.ToolCallPart:
            tokens += estimateTextTokens(p.Arguments) / 4
        case core.ToolResultPart:
            for _, rp := range p.Content {
                if tp, ok := rp.(core.TextPart); ok {
                    tokens += estimateTextTokens(tp.Text)
                }
            }
        }
    }
    return tokens
}
```

### 3.3 Phase 3：结构化 LLM 摘要（`phase3_summarizer.go`）

**摘要预算**：

```go
func (s *Summarizer) ComputeBudget(turns []core.Message, contextLength int) int {
    contentTokens := estimateMessagesTokens(turns)
    budget := int(float64(contentTokens) * summaryRatio)
    maxTokens := min(int(float64(contextLength)*0.05), summaryTokensCeiling)
    return max(minSummaryTokens, min(budget, maxTokens))
}
```

**默认值**：
- `minSummaryTokens = 2000`
- `summaryRatio = 0.20`
- `summaryTokensCeiling = 12000`

**结构化模板**（12 sections）：

```
## Active Task
[逐字引用用户最新未完成任务]

## Goal
[用户整体目标]

## Constraints & Preferences
[用户偏好、编码风格、约束]

## Completed Actions
[编号列表：动作 — 结果 [tool: name]]

## Active State
[当前工作状态]

## In Progress
[正在进行中的工作]

## Blocked
[未解决的阻塞/错误]

## Key Decisions
[关键决策及原因]

## Resolved Questions
[已回答的问题及答案]

## Pending User Asks
[未回答的用户问题]

## Relevant Files
[相关文件列表及备注]

## Remaining Work
[剩余工作]

## Critical Context
[必须保留的具体值、错误信息、配置]
```

**迭代更新**：
- 如果 `previousSummary != ""`：Prompt 包含 `PREVIOUS SUMMARY` + `NEW TURNS TO INCORPORATE`，要求增量更新（保留已有信息，追加新动作）
- 如果 `focusTopic != ""`：追加 FOCUS TOPIC 指令，优先保留相关内容（60-70% 预算）

**输入截断**（ summarizer 的上下文窗口保护）：
- 单条消息总字符上限：`6000`
- 保留头部：`4000` 字符
- 保留尾部：`1500` 字符
- Tool arguments 上限：`1500`，保留头部：`1200`

**敏感信息脱敏**：
- 输入序列化前调用 `redactSensitiveText()`
- 摘要输出后再次调用 `redactSensitiveText()`
- 正则匹配 API keys、tokens、passwords、connection strings，替换为 `[REDACTED]`

**失败处理**：

| 错误类型 | 处理 |
|---------|------|
| Auxiliary 模型不可用（404/503） | 回退到主模型，清除 cooldown，立即重试 |
| JSON 解析错误 / 流断开 | 回退到主模型重试一次，若仍失败则 30s cooldown |
| 超时 / 速率限制 | 回退到主模型重试一次，若仍失败则 60s cooldown |
| 主模型也失败 | 返回 `""`，Phase 4 插入静态 fallback |

### 3.4 Phase 4：组装与完整性修复（`phase4_assembler.go`）

**角色选择**（避免连续同角色消息）：

```
lastHeadRole = messages[headEnd-1].Role
firstTailRole = messages[tailStart].Role

// 优先避免与 Head 冲突
if lastHeadRole in {assistant, tool} {
    summaryRole = user
} else {
    summaryRole = assistant
}

// 若与 Tail 冲突，尝试翻转
if summaryRole == firstTailRole {
    flipped = 翻转角色
    if flipped != lastHeadRole {
        summaryRole = flipped
    } else {
        // 两难：合并到 Tail 第一条消息
        mergeIntoTail = true
    }
}
```

**SUMMARY_PREFIX**：

```
[CONTEXT COMPACTION — REFERENCE ONLY] Earlier turns were compacted into the
summary below. This is a handoff from a previous context window — treat it as
background reference, NOT as active instructions. Do NOT answer questions or
fulfill requests mentioned in this summary; they were already addressed.
Your current task is identified in the '## Active Task' section of the
summary — resume exactly from there.
IMPORTANT: Your persistent memory in the system prompt is ALWAYS authoritative
and active — never ignore or deprioritize memory content due to this compaction.
Respond ONLY to the latest user message that appears AFTER this summary.
```

**Tool 对完整性修复**（`_sanitizeToolPairs`）：

1. 扫描所有 assistant 消息的 `ToolCallPart`，收集 surviving call IDs
2. 扫描所有 tool 消息的 `ToolResultPart`，收集 result call IDs
3. **孤儿 result**：`resultIDs - survivingIDs` → 删除这些 tool 消息
4. **孤儿 call**：`survivingIDs - resultIDs` → 在对应 assistant 消息后插入 stub result

```go
stubResult := core.ToolResultPart{
    ToolCallID: cid,
    Name:       toolName,
    Content:    core.NewTextContent("[Result from earlier conversation — see context summary above]"),
}
```

**抗抖动**：
- 如果压缩节省 < 10%：`ineffectiveCount++`
- 否则：`ineffectiveCount = 0`
- 如果 `ineffectiveCount >= 2`：返回 `ErrCompressionIneffective`，暂停压缩

**静态 Fallback**（Phase 3 彻底失败时）：

```
[CONTEXT COMPACTION — REFERENCE ONLY]
Summary generation was unavailable. N message(s) were removed to free context
space but could not be summarized. The removed messages contained earlier work
in this session. Continue based on the recent messages below and the current
state of any files or resources.
```

---

## 4. 数据模型与配置

### 4.1 Workspace 模型扩展

```go
type Workspace struct {
    // ... 现有字段 ...

    // 新增
    ContextCompressionEnabled *bool `gorm:"default:true" json:"contextCompressionEnabled"`
    ContextLengthOverride     *int  `json:"contextLengthOverride"`
}
```

### 4.2 Config 全局默认

```go
type Config struct {
    // ... 现有字段 ...

    // 新增
    ContextCompressionEnabled      bool    `env:"CONTEXT_COMPRESSION_ENABLED" envDefault:"true"`
    ContextCompressionThresholdPct float64 `env:"CONTEXT_COMPRESSION_THRESHOLD_PCT" envDefault:"0.50"`
    ContextCompressionSummaryRatio float64 `env:"CONTEXT_COMPRESSION_SUMMARY_RATIO" envDefault:"0.20"`
    ContextCompressionProtectHead  int     `env:"CONTEXT_COMPRESSION_PROTECT_HEAD" envDefault:"3"`
    ContextCompressionProtectTail  int     `env:"CONTEXT_COMPRESSION_PROTECT_TAIL" envDefault:"20"`
    ContextCompressionAuxModel     string  `env:"CONTEXT_COMPRESSION_AUX_MODEL" envDefault:""`
}
```

### 4.3 Thread 级状态持久化

```go
type ThreadContextSummary struct {
    ID               int       `gorm:"primaryKey;autoIncrement"`
    ThreadID         int       `gorm:"uniqueIndex"`
    SummaryText      string    `gorm:"type:text"`
    CompressionCount int
    LastCompressedAt time.Time
    CreatedAt        time.Time
    UpdatedAt        time.Time
}
```

- **用途**：Regular Chat 的请求-响应模式需要跨 HTTP 请求保持 `previous_summary`
- **清理**：Thread 删除时级联删除；后台 cron 清理 90 天未更新记录

---

## 5. 模型上下文长度映射

### 5.1 内嵌映射表（`model_metadata.go`）

覆盖主流模型（30+ 条目），格式：

```go
var builtInModelContextLengths = map[string]int{
    "gpt-4o":           128_000,
    "gpt-4o-mini":      128_000,
    "gpt-4-turbo":      128_000,
    "gpt-4":             8_192,
    "claude-3-5-sonnet": 200_000,
    "claude-3-5-haiku":  200_000,
    "claude-3-opus":     200_000,
    "gemini-1.5-pro":  1_000_000,
    "gemini-1.5-flash":1_000_000,
    // ... 等
}
```

### 5.2 解析逻辑

```go
func (m *ModelMetadata) GetContextLength(modelID string, override *int) int {
    if override != nil && *override > 0 {
        return *override
    }
    if len, ok := builtInModelContextLengths[modelID]; ok {
        return len
    }
    // 模糊匹配（前缀匹配）
    for prefix, len := range builtInModelContextLengths {
        if strings.HasPrefix(modelID, prefix) {
            return len
        }
    }
    return defaultContextLength  // 8_192
}
```

---

## 6. 两条路径的集成细节

### 6.1 Agent Chat 路径

**集成点**：`backend/internal/agent/session.go` 的 `newSession()`

```go
func newSession(...) *Session {
    // ... 现有代码 ...

    compressor := buildCompressor(ws, lm, cfg, sysSvc)

    s.pAgent = pantheonAgent.New(lm,
        pantheonAgent.WithRegistry(reg),
        pantheonAgent.WithMaxSteps(10),
        pantheonAgent.WithCompressor(compressor),
    )
    // ...
}
```

**关键机制**：
- Pantheon `RunStream()` 在每次模型调用前自动调用 `Compress()`
- `HermesCompressor.Compress()` 内部先做 `ShouldCompress(tokenCount)` 判断，未达阈值直接 pass-through
- Token 估算在 Compress 内部完成（遍历 history），成本极低

### 6.2 Regular Chat 路径

**集成点**：`backend/internal/services/chat_service.go` 的 `buildRAGContext()`

```go
func (s *ChatService) buildRAGContext(...) (string, []any, []core.Message, error) {
    history, err = s.buildChatHistory(ctx, ws.ID, threadID, historyLimit)
    if err != nil {
        return "", nil, nil, err
    }

    if s.compressionSvc != nil && s.compressionSvc.IsEnabled(ws) {
        var prevSummary string
        if threadID != nil {
            prevSummary, _ = s.compressionSvc.LoadThreadSummary(ctx, *threadID)
        }

        compressed, summary, err := s.compressionSvc.Compress(ctx, ws, history, prevSummary, lm)
        if err == nil && summary != "" && threadID != nil {
            _ = s.compressionSvc.SaveThreadSummary(ctx, *threadID, summary)
        }
        history = compressed
    }

    // ... 后续代码不变 ...
}
```

**注意**：Regular Chat 的消息只有 user/assistant 文本对（无 tool calls），Phase 1 和 Phase 4 实际无操作。压缩引擎自适应处理。

### 6.3 Auxiliary 模型构建

```go
func buildAuxiliaryLanguageModel(
    ws *models.Workspace,
    cfg *config.Config,
    settings map[string]string,
) (core.LanguageModel, error) {
    auxModel := pick("ContextCompressionAuxModel", settings, cfg.ContextCompressionAuxModel)
    if auxModel == "" {
        return nil, nil  // 使用主模型
    }
    return buildLanguageModelWithOverride(ws, settings, cfg, auxModel)
}
```

---

## 7. 错误处理与降级策略

| 场景 | 行为 | 用户感知 |
|------|------|---------|
| 压缩未触发（token < threshold） | 原样返回历史 | 无 |
| Phase 1 剪枝成功，节省足够空间 | 可跳过 Phase 3（后续优化） | 无 |
| Phase 3 摘要成功 | 返回压缩历史 + 摘要 | 无 |
| Phase 3 失败，fallback 到主模型成功 | 返回压缩历史 + 摘要 | 日志警告 |
| Phase 3 彻底失败 | 插入静态 fallback | 模型会看到提示 |
| 连续 2 次压缩节省 < 10% | 暂停压缩，返回原历史 | 无 |
| 进入 cooldown（30s/60s） | 返回原历史 | 无 |

---

## 8. 测试策略

| 层级 | 内容 | 文件 |
|------|------|------|
| **单元测试** | Token 估算器（各种内容类型） | `token_estimator_test.go` |
| | 工具剪枝（MD5 去重、图片剥离、JSON 截断） | `phase1_pruner_test.go` |
| | 边界划定（token 预算、边界对齐、用户锚定） | `phase2_boundary_test.go` |
| | 组装器（角色选择、tool 对修复） | `phase4_assembler_test.go` |
| | 模型元数据（模型名解析、Workspace 覆盖） | `model_metadata_test.go` |
| | 脱敏器（各种敏感模式） | `redactor_test.go` |
| **集成测试** | 完整四阶段流水线端到端 | `compressor_test.go` |
| | 与 ChatService 集成（Regular Chat） | `chat_service_test.go`（补充） |
| | 与 Session 集成（Agent Chat） | `session_agent_test.go`（补充） |

---

## 9. 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| Phase 3 LLM 摘要增加 API 成本 | 中 | Phase 1/2 先执行零成本剪枝；仅在高 token 时触发摘要；使用 auxiliary 廉价模型 |
| 模型上下文长度映射不完整 | 低 | 模糊前缀匹配 + Workspace 覆盖 + 保守默认值 8K |
| 压缩后摘要质量差导致模型行为异常 | 中 | 结构化模板强制输出；静态 fallback 明确告知模型上下文被截断；迭代更新保留信息 |
| Thread 级状态持久化增加 DB 写入 | 低 | 仅在压缩触发时写入；轻量表结构；后台清理旧记录 |
| 引入新包增加编译时间 | 低 | 包内模块化设计，按需引用 |

---

## 10. 附录

### 10.1 Hermes 参考文件

| 文件 | 大小 | 核心职责 |
|------|------|---------|
| `agent/context_compressor.py` | 213KB | 上下文压缩引擎完整实现 |
| `agent/model_metadata.py` | 77KB | 模型元数据与上下文长度解析 |
| `agent/context_engine.py` | 8KB | 上下文引擎抽象基类 |

### 10.2 Hermind 修改文件清单

| 文件 | 修改类型 |
|------|---------|
| `backend/internal/agent/compression/*.go` | 新增 |
| `backend/internal/agent/compression/prompts/summary_template.txt` | 新增 |
| `backend/internal/agent/session.go` | 修改（注入 compressor） |
| `backend/internal/agent/llm_factory.go` | 修改（auxiliary 模型构建） |
| `backend/internal/services/chat_service.go` | 修改（Regular Chat 集成） |
| `backend/internal/models/workspace.go` | 修改（新增字段） |
| `backend/internal/models/thread_context_summary.go` | 新增 |
| `backend/internal/config/config.go` | 修改（新增配置） |
| `backend/cmd/server/main.go` | 修改（初始化 compression service） |
