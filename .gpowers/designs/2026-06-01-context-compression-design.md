# Context Compression — Design Document（重新评估版）

> **功能**: 将上下文压缩能力引入 Hermind 的 Agent 与 Regular Chat 两条路径
> **日期**: 2026-06-01
> **状态**: Approved (design sections), pending Deep audit gate
> **方案**: 复用 Pantheon `agent/compression` 引擎 + hermind 薄接入 + 上游补缺口
> **审计强度**: Deep
> **取代**: `2026-05-28-context-compression-design.md`（旧方案提议在 hermind 重新移植整套引擎，经重新评估判定为重复造轮子，作废）

---

## 0. 重新评估的核心结论 `[C:USER]`

旧报告（`2026-05-31-hermes-agent-highlight-features-report.md`）断言「Pantheon 内置 Compressor 过于简单（固定 head=3/tail=20，bullet-point）」。**该前提已过时。**

通读 `/Users/ranwei/workspace/go_work/pantheon/agent/compression/`（11 个源文件，~2572 行）后确认：**Pantheon 已把 Hermes `context_compressor.py` 的引擎约 90% 移植为 Go**，包含完整 5 阶段流水线、抗抖动、冷却、脱敏、工具剪枝、13-section 结构化模板、迭代更新、focus topic、孤儿 tool 对修复。

因此本设计**不重新移植引擎**，只做四件事：**接入（两条路径）+ 校准（喂 context length）+ 持久化（thread_compactions 表）+ 上游补缺口（贡献给 Pantheon）**。

---

## 1. 目标与非目标

### 1.1 In（V1 范围）`[C:USER]`

- **Agent 路径**：创建 Pantheon Agent 时加 `WithContextEngine`，并调用 `UpdateModel` 校准阈值
- **Regular Chat 路径**：用 `llmProv.LanguageModel()` 作辅助模型，直接以库形式调用压缩引擎，替换 `buildChatHistory` 朴素「最近 20 条」截断
- **持久化**：新表 `thread_compactions` 存摘要 + `WorkspaceChat.Include=false` 软删被压缩行
- **校准**：内嵌 model→context-length 映射表 + workspace 级覆盖
- **脱敏调优**：去掉 bare-hex 与 email 正则，避免误杀 git SHA / 邮件等合法内容
- **开关**：全局 `SystemSetting "context_compress_enabled"`（默认 OFF / opt-in）+ per-workspace 覆盖
- **上游 Pantheon 补缺口（本轮做）**：per-tool 摘要模板、摘要输入截断、fallback 模型重试、summary-prefix 结束标记、`PreviousSummary()`/`SetPreviousSummary()`/`LastFallbackUsed()` accessor、agent loop 回喂 `UpdateFromResponse`
- **`/compress <topic>` 手动压缩端点**（详见 §18.1）`[C:USER]`
- **跨 thread handoff**：`WorkspaceThread.ParentThreadID` + 创建子 thread 时种子拷贝父摘要 + Pantheon MemoryProvider（详见 §18.2）`[C:USER]`
- **实时 usage 校准**：捕获 `LLMChunk.Usage` / agent step usage 回喂引擎，替代纯静态估算（详见 §18.3）`[C:USER]`
- **per-workspace 覆盖前端 UI**：工作区设置页压缩开关/阈值/ctxLen 覆盖表单（详见 §18.4）`[C:USER]`
- **失败冷却 600s 第三级**（上游 `state.go`，详见 §18.5）`[C:USER]`

### 1.2 Out（V2+）`[C:DEFERRED]`

经用户确认，原延后项全部并入 V1（见 §18）。当前 V2+ 仅剩：
- 子代理（subagent）并行隔离 —— 与本压缩功能正交，独立设计
- 压缩摘要的多代历史浏览 UI —— `thread_compactions` 已存多行，前端浏览器 V2

### 1.3 非目标

- 不在 hermind 新建独立压缩引擎包（旧方案做法，已作废）
- 不修改 Pantheon 的 5 阶段算法主干（只加 accessor 与补缺口）

---

## 2. 上游对比表 `[C:UPSTREAM]`

**关键论断**：差距集中在「接入 / 校准 / 持久化」，引擎算法本身基本齐备。

| 能力 | Hermes `context_compressor.py` | Pantheon Go `agent/compression` | hermind 动作 |
|---|---|---|---|
| 5 阶段流水线 | ✅ `compress()` | ✅ `compressInternal` (`compressor.go`) | 复用 |
| Phase1 MD5 去重 | ✅ | ✅ `prune.go` | 复用 |
| Phase1 图片剥离 | ✅ → 占位文本 | ✅ `[image: previously shared image]` | 复用 |
| Phase1 JSON 安全截断 | ✅ | ✅ `truncateJSONArgs` | 复用 |
| Phase1 per-tool 单行摘要 | ✅ `_summarize_tool_result` | ⚠️ 仅通用 `[tool_result id: N chars]` | **上游补** |
| Phase2 head 保护 | ✅ | ✅ `ProtectFirstN` | 复用 |
| Phase2 token 预算尾保护 (1.5× 软顶, min3) | ✅ | ✅ `determineBoundaries` | 复用 |
| Phase2 工具对对齐 | ✅ | ✅ `alignToToolPairBoundaries` | 复用 |
| Phase2 最新用户消息锚定 (#10896) | ✅ | ✅ (`boundaries.go` user-msg 循环) | 复用 |
| Phase3 13-section 结构化模板 | ✅ | ✅ `summary.go` | 复用 |
| Phase3 迭代更新 | ✅ | ✅ (含 maxLength 门控) | 复用，但**摘要仅内存** |
| Phase3 focus topic | ✅ | ✅ 参数已存在 | 复用（V1 不暴露） |
| Phase3 摘要输入截断 (6000 字符/msg) | ✅ `_serialize_for_summary` | ❌ `renderTranscript` 全量拼接 | **上游补** |
| Phase4 角色冲突规避 | ✅ | ⚠️ 仅查 tail[0]==assistant | 复用（够用） |
| Phase4 SUMMARY_PREFIX + 结束标记 | ✅ | ⚠️ const 定义但 `assemble` 未用 | **上游补** |
| Phase4 静态 fallback | ✅ | ✅ `buildStaticFallbackSummary` | 复用 |
| Phase4 孤儿 tool 对修复 + stub | ✅ | ✅ `sanitize.go` | 复用 |
| 抗抖动 (<10%, 2 次) | ✅ | ✅ `state.go` | 复用 |
| 失败冷却 | ✅ 30/60/600 | ⚠️ 30/60（无 600 级） | 复用（够用，600 可后续） |
| fallback 模型重试 | ✅ | ❌ 空实现（"continue to Level 2"） | **上游补** |
| 阈值校准 `UpdateModel` | ✅ `model_metadata` | ✅ 有 API 但 **agent loop 从不调用** | **hermind 调用 + 喂 ctxLen** |
| model→ctx 长度 | ✅ 77KB 表 | ❌ 需调用方传入 | **内嵌映射表** |
| 摘要持久化 | ✅ handoff summary | ❌ `previousSummary` 仅内存 | **新表 + accessor** |
| 图片 token 估算 | 1600 | 256 (`helpers.go`) | 复用（差异可接受） |
| 两条 chat 路径接入 | n/a | n/a | **本设计核心** |

---

## 3. 架构与数据流 `[C:INFERRED]`

**关键论断**：两条路径共享同一 Pantheon 引擎，但「消费方式」不同 —— Agent 路径由 Pantheon 经 `WithContextEngine` 在每 step 自动调用；Chat 路径由 hermind 自己以库形式调用并落库。

### 3.1 新增 / 改动包

```
backend/internal/agent/compression/        # hermind 侧薄接入层（不含引擎算法）
├── model_metadata.go                       # model→context-length 映射 + 查找
├── factory.go                              # 构造 compression.DefaultCompressor + 注入配置/脱敏/校准
├── persistence.go                          # thread_compactions 读写 + 摘要回填/取出
├── redact_patterns.go                      # 调优后的脱敏正则集
└── *_test.go

backend/internal/models/thread_compaction.go   # 新模型
backend/internal/services/chat_service.go       # 改：buildChatHistory + Stream/Complete 接入
backend/internal/agent/{handler,session,runtime}.go  # 改：pAgent.New 加 WithContextEngine

# 上游（独立 PR 到 pantheon 仓库）
pantheon/agent/compression/{compressor,summary,prune,assemble,state}.go  # 补缺口
```

### 3.2 Agent 路径数据流（WebSocket）

```
HandleWS → newSession
  ├─ persistence.LoadLatestSummary(ws.ID, threadID) → prevSummary, upToChatID
  ├─ compressor := factory.NewForAgent(cfg{Threshold:0.50}, lm)   # lm 已是 core.LanguageModel
  │     ├─ compressor.UpdateModel(modelName, ctxLen)              # ← 校准（关键）
  │     └─ compressor.SetPreviousSummary(prevSummary)             # ← 回填（上游新增 accessor）
  └─ sess.pAgent = pantheonAgent.New(lm,
        WithRegistry(reg), WithMaxSteps(10),
        WithContextEngine(compressor))                            # ← 接入（关键）

每个 agent step（Pantheon 内部 agent.go:197 / stream.go:73）:
  contextEngine.CompressMessages(ctx, messages, "")
    → 触发时 5 阶段流水线 → messages 被替换为压缩版

step 完成后（hermind session 侧钩子）:
  if compressor.PreviousSummary() 变化:
     persistence.SaveSummary(ws.ID, threadID, summary, beforeTok, afterTok, fallbackUsed, upToChatID)
     ws.SendEvent("context.compressed", {before, after, savedPct, fallbackUsed})
     if fallbackUsed: ws.SendEvent 显式警告
```

### 3.3 Regular Chat 路径数据流（HTTP）

```
ChatService.Stream()/Complete() → buildRAGContext (chat_service.go:177/:276)
  → buildChatHistory(ws.ID, threadID, limit)  (chat_service.go:48 调用 / :306 定义)
      ├─ comp := persistence.LoadLatestSummary(ws.ID, threadID)
      ├─ query WorkspaceChat WHERE include=true
      │       AND (comp==nil OR id > comp.UpToChatID)            # 只读摘要之后的行
      ├─ history = [若 comp!=nil: 摘要合成消息] + 展开的 chat 行
      └─ return history
  → 【新增压缩判定】
      if compressEnabled(ws) && estimateTokens(history) > 0.75*ctxLen:
          compressor := factory.NewForChat(cfg{Threshold:0.75}, s.llmProv.LanguageModel())
          compressor.UpdateModel(modelName, ctxLen)
          compressor.SetPreviousSummary(comp.Summary)
          compressed := compressor.CompressMessages(ctx, history, "")
          newSummary := compressor.PreviousSummary()
          persistence.SaveSummary(...)                            # 落库
          db.Model(WorkspaceChat).Where(id <= boundaryChatID).Update(include=false)  # 软删
          history = compressed
          mlog.Info("chat compaction: %d→%d tokens", before, after)
  → llmProv.Stream(history, systemPrompt)  (chat_service.go:200)
```

---

## 4. 数据模型 `[C:USER]`

**关键论断**：`(workspace_id, thread_id)` 唯一定位「最新一份摘要」；`thread_id=nil` 覆盖默认工作区会话（无 thread）；`up_to_chat_id` 让 `buildChatHistory` 增量地只读摘要之后的行。

```go
// internal/models/thread_compaction.go
type ThreadCompaction struct {
    ID           int       `gorm:"primaryKey;autoIncrement" json:"id"`
    WorkspaceID  int       `gorm:"index:idx_ws_thread,priority:1" json:"workspaceId"`
    ThreadID     *int      `gorm:"index:idx_ws_thread,priority:2" json:"threadId"` // nil=默认工作区会话
    Summary      string    `json:"summary"`        // 已脱敏的 13-section 结构化摘要
    UpToChatID   int       `json:"upToChatId"`     // 本次压缩覆盖到的最后一条 WorkspaceChat.ID（增量基准）
    BeforeTokens int       `json:"beforeTokens"`
    AfterTokens  int       `json:"afterTokens"`
    FallbackUsed bool      `json:"fallbackUsed"`   // true → 触发用户可见警告
    CreatedAt    time.Time `json:"createdAt"`
}
```

- AutoMigrate 注册到 `internal/services/db.go`（与现有 `&models.SystemSetting{}` 等并列）
- 查询「最新摘要」：`WHERE workspace_id=? AND thread_id <=> ? ORDER BY id DESC LIMIT 1`
- 复用 `WorkspaceChat.Include`（已存在 `gorm:"default:true"`）作软删标志，`buildChatHistory` 已按 `include = ?` 过滤（`chat_service.go:308`），无需改过滤条件，仅需新增「id > up_to_chat_id」与摘要合成行

**WorkspaceThread 新增字段（支撑 §18.2 跨 thread handoff）**：

```go
// internal/models/workspace_thread.go — 新增字段
ParentThreadID *int `gorm:"index" json:"parentThreadId"` // nil=根 thread；非 nil=从该父 thread fork，创建时种子拷贝父摘要
```

**Workspace 新增字段（支撑 §5 开关 / §6 覆盖 / §18.4 UI）**：

```go
// internal/models/workspace.go — 新增可空字段（nil 回落全局/默认）
CompressEnabled    *bool    `json:"compressEnabled"`    // per-workspace 开关覆盖
CompressThreshold  *float64 `json:"compressThreshold"`  // per-workspace 阈值覆盖（nil 用分路径默认）
CompressContextLen *int     `json:"compressContextLen"` // per-workspace ctxLen 覆盖（喂 ContextLengthFor）
```

---

## 5. 配置（分路径默认值）`[C:USER]`

**关键论断**：Agent 路径累积大量 tool 消息，应更早触发（0.50）；Chat 路径较轻，更晚触发（0.75）。其余默认沿用 `compression.DefaultCompressionConfig()`。

```go
// factory.go
func baseConfig() compression.CompressionConfig {
    return compression.CompressionConfig{
        Enabled: true, ProtectFirstN: 3, ProtectLast: 20,
        AntiThrashEnabled: true, AntiThrashThreshold: 0.10, AntiThrashMaxConsecutive: 2,
        CooldownEnabled: true, CooldownBase: 30 * time.Second, CooldownMax: 60 * time.Second,
        RedactionEnabled: true, ToolPruningEnabled: true, IterativeUpdateEnabled: true,
    }.WithDefaults()
}

func NewForAgent(lm core.LanguageModel) *compression.DefaultCompressor {
    cfg := baseConfig(); cfg.Threshold = 0.50            // Agent: 更早触发
    return compression.NewDefaultCompressor(cfg, lm)
}
func NewForChat(lm core.LanguageModel) *compression.DefaultCompressor {
    cfg := baseConfig(); cfg.Threshold = 0.75            // Chat: 更晚触发
    return compression.NewDefaultCompressor(cfg, lm)
}
```

| 字段 | Agent | Chat | 说明 |
|---|---|---|---|
| `Threshold` | **0.50** | **0.75** | 唯一分路径差异 |
| `ProtectFirstN` | 3 | 3 | head 保护 |
| `ProtectLast` | 20 | 20 | 尾保护消息数下限基准 |
| `RedactionEnabled` | true | true | 用调优正则集（§7） |

全局开关：`SystemSetting "context_compress_enabled"`，默认 `"false"`（opt-in）。
per-workspace 覆盖：`Workspace` 新增可空字段 `CompressEnabled *bool`，nil 时回落全局。

---

## 6. 内嵌模型映射 `[C:INFERRED]`

**关键论断**：hermind 全仓库无任何 `ContextLength` 来源（grep 确认）；校准依赖一份薄映射表，未命中回落保守默认 8192。

```go
// internal/agent/compression/model_metadata.go
var modelContextLength = map[string]int{
    "gpt-4o":             128000, "gpt-4o-mini": 128000,
    "gpt-4-turbo":        128000, "gpt-4":       8192,
    "gpt-3.5-turbo":      16385,
    "claude-3-5-sonnet":  200000, "claude-3-5-haiku": 200000,
    "claude-3-opus":      200000, "claude-3-sonnet":  200000, "claude-3-haiku": 200000,
    "gemini-1.5-pro":     1000000, "gemini-1.5-flash": 1000000,
    "gemini-2.0-flash":   1000000,
    "llama3":             8192,  "llama-3.1": 128000,
    "qwen2":              32768, "qwen2.5":   32768,
    "deepseek-chat":      64000, "deepseek-coder": 64000,
    "mixtral-8x7b":       32768,
}
const defaultContextLength = 8192

// 查找优先级：workspace 覆盖 → 精确匹配 → 最长前缀匹配 → default
func ContextLengthFor(model string, wsOverride *int) int {
    if wsOverride != nil && *wsOverride > 0 { return *wsOverride }
    if v, ok := modelContextLength[model]; ok { return v }
    best := 0; bestLen := defaultContextLength
    for k, v := range modelContextLength {
        if strings.HasPrefix(model, k) && len(k) > best { best = len(k); bestLen = v }
    }
    return bestLen
}
```

---

## 7. 脱敏调优 `[C:USER]`

**关键论断**：Pantheon `redact.DefaultPatterns` 含 `\b[a-f0-9]{32,64}\b`（误杀 git commit SHA/MD5）与 email 正则（误杀合法邮箱），对会讨论代码的助手破坏性大。改用自定义集，保留真正的密钥规则。

```go
// internal/agent/compression/redact_patterns.go
var CompactionRedactPatterns = []*regexp.Regexp{
    regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9\-._~+/=]{16,}`),
    regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
    regexp.MustCompile(`sk-(?:ant-|proj-|live-)?[A-Za-z0-9_-]{20,}`),
    regexp.MustCompile(`sk-or-v1-[A-Za-z0-9]{32,}`),
    regexp.MustCompile(`(?i)(password|api_key|apikey|token)\s*[:=]\s*["']?[^\s"']{8,}`),
    // 去掉: \b[a-f0-9]{32,64}\b  —— 误杀 git SHA(40 hex)/MD5
    // 去掉: email 正则           —— 误杀合法邮箱
}
```

**接入方式**：Pantheon `prune.go`/`summary.go` 当前硬调用 `redact.String`（默认集）。两种选择：
- **(选定)** 上游让脱敏正则集可注入：给 `CompressionConfig` 加 `RedactPatterns []*regexp.Regexp`（nil 时用 `DefaultPatterns`），`redact.String` 调用改为 `redact.With(cfg.RedactPatterns)`。hermind 注入 `CompactionRedactPatterns`。此项并入 §10 上游清单。

---

## 8. 降级与错误分层 `[C:USER]`

**关键论断**：失败不静默丢弃上下文；插入静态 fallback 告知模型，并在 Agent 路径向用户显式警告。

| 错误类 | 引擎处理（已存在） | 用户感知（hermind 新增） |
|---|---|---|
| 摘要 LLM 失败（404/503/超时/JSON 错误） | `generateSummaryWithFallback` → fallback 模型（上游补）→ 仍失败则 `buildStaticFallbackSummary`（抽取最后 user 消息 + tool 列表）+ `enterCooldown` | Agent: `ws.SendEvent` 显式警告「部分上下文压缩失败」；Chat: `mlog.Warning` |
| 冷却中（30s/60s） | `ShouldCompress` 返回 false → 跳过 | 无 |
| 抗抖动（连续 2 次 <10%） | `ShouldCompress` 返回 false → 跳过 | 无（可后续提示 /new） |
| 孤儿 tool_call/result | `sanitizeToolPairs` 删孤儿 result + 为孤儿 call 注入 stub result | 无 |
| 阈值未校准（ctxLen=0） | `ShouldCompress` 退化为 true | hermind 通过 `UpdateModel` 保证不发生 |

`fallbackUsed` 标志经 `compressor.PreviousSummary()` 旁路无法获取 —— 需上游 `LastFallbackUsed() bool` accessor（并入 §10）。

---

## 9. 可观测 `[C:USER]`

- **Agent 路径**：`ws.SendEvent("context.compressed", {before, after, savedPct, fallbackUsed})`；`fallbackUsed=true` 时额外发警告事件
- **Chat 路径**：`mlog.Info("chat compaction: %d→%d tokens (saved %.0f%%)", before, after, pct)`
- **Telemetry**：`compaction_finished{path: "agent"|"chat", before_tokens, after_tokens, fallback_used}`（接入现有 `internal/agent/telemetry.go` 模式）

---

## 10. 上游 Pantheon 改动清单（独立 PR）`[C:USER]`

**关键论断**：以下均为引擎真实缺口，因 hermind 本就 vendor Pantheon，直接上游补齐使两仓库一起变好。

1. **Accessor**：`DefaultCompressor.PreviousSummary() string`、`SetPreviousSummary(string)`、`LastFallbackUsed() bool`（state.go）
7. **agent loop 回喂 usage**：`agent.go`/`stream.go` 每 step 后 `if a.contextEngine != nil { a.contextEngine.UpdateFromResponse(resp.Usage) }`，使 `ShouldCompress` 用真实 prompt_tokens（支撑 §18.3）
8. **600s 冷却第三级**：`state.go` `enterCooldown` 的 `CooldownMax` 提至 600s 并按 `ineffectiveCount` 分级 30/60/600（支撑 §18.5）
2. **per-tool 摘要模板**：`summarizeToolResult` 按 tool name 分支（terminal/browser_navigate/create_files/web_scraping/session_search + 通用 fallback）（prune.go）
3. **摘要输入截断**：`renderTranscript` 加 per-message 6000 字符截断（头 4000 + 尾 1500），tool args 1500（helpers.go / summary.go）
4. **fallback 模型重试**：`generateSummaryWithFallback` 实现 Level 1（用 `cfg.FallbackModel` 构造实例重试）（state.go）
5. **summary-prefix + 结束标记**：`assemble` 真正使用 `summaryPrefix` const，并在 summary 落为 role=user 时追加「--- END OF CONTEXT SUMMARY — respond to the message below, not the summary above ---」（assemble.go）
6. **可注入脱敏集**：`CompressionConfig.RedactPatterns []*regexp.Regexp`，`redact.String` → `redact.With`（config.go / prune.go / summary.go）

> 若上游 PR 短期无法合并：hermind 临时 fork 或 `replace` 指令指向本地分支；§10 改动不阻塞 §3–§9 接入主体（接入可先用现状引擎，缺口逐项补）。

---

## 11. 关键算法伪码

### 11.1 `buildChatHistory` 改造（chat_service.go:306）`[C:INFERRED]`

```go
func (s *ChatService) buildChatHistory(ctx, wsID int, threadID *int, limit int) ([]core.Message, error) {
    comp := s.compactionStore.LoadLatest(ctx, wsID, threadID) // nil if none

    q := s.db.Where("workspace_id = ? AND include = ?", wsID, true)
    if threadID != nil { q = q.Where("thread_id = ?", *threadID) } else { q = q.Where("thread_id IS NULL") }
    if comp != nil { q = q.Where("id > ?", comp.UpToChatID) }      // 仅读摘要之后的行

    var chats []models.WorkspaceChat
    if err := q.Order("id DESC").Limit(limit).Find(&chats).Error; err != nil { return nil, err }

    history := make([]core.Message, 0, len(chats)*2+1)
    if comp != nil {                                              // 摘要作为 head 合成消息
        history = append(history, core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT,
            "[Compressed summary of earlier conversation]\n"+comp.Summary))
    }
    for i := len(chats) - 1; i >= 0; i-- {
        history = append(history, core.NewTextMessage(core.MESSAGE_ROLE_USER, chats[i].Prompt))
        history = append(history, core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT, chats[i].Response))
    }
    return history, nil
}
```

### 11.2 Chat 路径压缩判定（buildRAGContext 内，chat_service.go:48 之后）`[C:INFERRED]`

```go
if s.compressEnabled(ctx, ws) {
    ctxLen := compression.ContextLengthFor(modelName, ws.CompressContextLen)
    if estimateTokens(history) > int(0.75*float64(ctxLen)) {
        comp := factory.NewForChat(s.llmProv.LanguageModel())
        comp.UpdateModel(modelName, ctxLen)
        if prev := s.compactionStore.LoadLatest(ctx, ws.ID, threadID); prev != nil {
            comp.SetPreviousSummary(prev.Summary)
        }
        before := estimateTokens(history)
        compressed, err := comp.CompressMessages(ctx, history, "")
        if err == nil {
            after := estimateTokens(compressed)
            boundaryChatID := latestChatID(ws.ID, threadID)        // 当前最后一条
            s.compactionStore.Save(ctx, models.ThreadCompaction{
                WorkspaceID: ws.ID, ThreadID: threadID,
                Summary: comp.PreviousSummary(), UpToChatID: boundaryChatID,
                BeforeTokens: before, AfterTokens: after, FallbackUsed: comp.LastFallbackUsed(),
            })
            s.db.Model(&models.WorkspaceChat{}).
                Where("workspace_id = ? AND thread_id <=> ? AND id <= ?", ws.ID, threadID, boundaryChatID).
                Update("include", false)
            history = compressed
            mlog.Info("chat compaction: %d→%d tokens", before, after)
        }
    }
}
```

### 11.3 Agent 路径接入（handler.go:97 / session.go:116 / runtime.go:182）`[C:INFERRED]`

```go
// 三处 pAgent.New 统一改造（以 handler.go:97 为例）
opts := []pantheonAgent.Option{
    pantheonAgent.WithRegistry(reg),
    pantheonAgent.WithMaxSteps(10),
}
if r.deps.CompressEnabled(ctx, &ws) {
    comp := factory.NewForAgent(lm)
    comp.UpdateModel(modelName, compression.ContextLengthFor(modelName, ws.CompressContextLen))
    if prev := r.deps.CompactionStore.LoadLatest(ctx, ws.ID, threadID); prev != nil {
        comp.SetPreviousSummary(prev.Summary)
    }
    sess.compressor = comp                                   // 保存引用供 step 后持久化
    opts = append(opts, pantheonAgent.WithContextEngine(comp))
}
sess.pAgent = pantheonAgent.New(lm, opts...)
```

---

## 12. API 端点 `[C:USER]`

- **新增 `POST /api/workspaces/:slug/compress`**（详见 §18.1）：手动触发压缩 + 可选 focus topic
- WS 新增**事件类型**（非端点）：`context.compressed`（§9），复用现有 `ws.SendEvent` 机制
- `PUT /api/workspaces/:slug`（已存在）扩展：`UpdateWorkspaceRequest` 加压缩覆盖字段（§18.4）

---

## 18. V1 扩展项设计（原延后项，经确认并入）

### 18.1 `/compress <topic>` 手动压缩端点 `[C:USER]`

**路由**：`POST /api/workspaces/:slug/compress`
**请求体**：`{ "threadId": int|null, "topic": string|"" }`
**响应**：

| 状态码 | body | 触发条件 |
|---|---|---|
| 200 | `{before, after, savedPct, summary, fallbackUsed}` | 压缩成功 |
| 409 | `{error:"nothing to compress"}` | 历史过短（≤ head+3+1）或抗抖动/冷却中跳过 |
| 503 | `{error:"summary model unavailable"}` | 辅助模型不可用且静态 fallback 也判定无意义 |

**逻辑**（handler → chat_service 复用 §11.2 压缩块）：
```go
func (s *ChatService) CompressNow(ctx, ws, threadID *int, topic string) (CompactionResult, error) {
    history := s.buildChatHistory(ctx, ws.ID, threadID, largeLimit) // 取全量（不截断）
    if len(history) <= minForCompress { return _, ErrNothingToCompress }
    comp := factory.NewForChat(s.llmProv.LanguageModel())
    comp.UpdateModel(modelName, ctxLen)
    if prev := s.compactionStore.LoadLatest(ctx, ws.ID, threadID); prev != nil { comp.SetPreviousSummary(prev.Summary) }
    compressed, err := comp.CompressMessages(ctx, history, topic) // ← topic 透传 focusTopic
    ... 落库 + 软删（同 §11.2）...
    return CompactionResult{Before, After, SavedPct, comp.PreviousSummary(), comp.LastFallbackUsed()}, nil
}
```
**关键论断**：手动端点与自动压缩共用同一引擎调用，唯一差异是 `topic` 透传与「不看阈值、强制压缩」。

### 18.2 跨 thread handoff `[C:USER]`

**机制**：两层。
1. **服务层种子拷贝（V1 主路径）**：`ThreadService.Create`（thread_service.go:34）接收 `ParentThreadID`。若非 nil，读父 thread 最新 `ThreadCompaction`，为子 thread 写一条种子 `ThreadCompaction{ThreadID: child, Summary: parent.Summary, UpToChatID: 0}`。子 thread 首次 `buildChatHistory`/agent session 即带父摘要。
2. **Pantheon MemoryProvider（agent 内部 session 切换）**：实现 hermind `compactionMemoryProvider`，经 `WithMemoryProviders` 注册：
```go
type compactionMemoryProvider struct{ store *CompactionStore; wsID int }
func (p *compactionMemoryProvider) OnPreCompress(msgs []core.Message) ([]core.Message, error) { return msgs, nil }
func (p *compactionMemoryProvider) OnSessionSwitch(newSessionID, parentSessionID string) error {
    parent := p.store.LoadLatestBySession(parentSessionID)
    if parent != nil { p.store.SeedForSession(newSessionID, parent.Summary) }
    return nil
}
```
**关键论断**：hermind 当前无 agent 内部 subagent 切换，所以 V1 的实际生效路径是「服务层种子拷贝」；MemoryProvider 是为未来 subagent 预埋的 Pantheon 级钩子，注册即可，不阻塞 V1。
**dto 改动**：`dto.CreateThreadRequest` 加 `ParentThreadID *int`。

### 18.3 实时 usage 校准 `[C:USER]`

**关键论断**：用真实 `prompt_tokens` 替代字符估算，使阈值判定更准。

- **Agent 路径**：依赖 §10 上游改动 #7 —— Pantheon agent loop 每 step 后 `contextEngine.UpdateFromResponse(resp.Usage)`。hermind 侧零改动（接入引擎即自动受益）。
- **Chat 路径**：`ChatService.Stream` 消费 `LLMChunk` 时捕获末个 chunk 的 `Usage *core.Usage`（llm.go:17 已暴露），存入轻量 per-(ws,thread) 缓存（内存 map + 可选落 `thread_compactions.last_prompt_tokens` 字段）。下一轮 `buildRAGContext` 的压缩判定优先用 `lastPromptTokens` 而非 `estimateTokens(history)`。
- **降级**：无 usage（首轮 / provider 不返回）时回落字符估算。

**字段微调**：`ThreadCompaction` 加 `LastPromptTokens int`（可选，用于跨请求传递真实 usage）。

### 18.4 per-workspace 覆盖前端 UI `[C:USER]`

- **后端**：`dto.UpdateWorkspaceRequest` 加 `CompressEnabled *bool`、`CompressThreshold *float64`、`CompressContextLen *int`；`WorkspaceService.Update`（workspace_service.go:85）按现有 `OpenAiHistory` 模式写入 `updates` map。
- **前端**：工作区设置页新增「上下文压缩」分区：
  - 开关（三态：跟随全局 / 强制开 / 强制关 → 映射 `*bool`）
  - 阈值滑块（0.3–0.95，空=分路径默认）
  - ctxLen 覆盖输入框（空=用内嵌映射表）
- **生效优先级**（§5/§6 已定）：workspace 覆盖 → 全局 SystemSetting → 分路径默认。

### 18.5 失败冷却 600s 第三级 `[C:USER]`

上游 §10 改动 #8。`state.go` `enterCooldown`：
```go
// 现状：CooldownBase=30s, CooldownMax=60s
// 改为分级：ineffectiveCount 0→30s, 1→60s, ≥2→600s（CooldownMax 提至 600s）
cooldowns := []time.Duration{30*time.Second, 60*time.Second, 600*time.Second}
idx := min(c.state.ineffectiveCount, len(cooldowns)-1)
c.state.summaryCooldownUntil = time.Now().Add(cooldowns[idx])
```
**关键论断**：贴齐 Hermes 的 30/60/600 三级语义；无可用 provider 等持续性故障最终落到 600s，避免高频无效重试。

---

## 13. 风险册

| # | 风险 | 可能性 | 影响 | 缓解 |
|---|---|---|---|---|
| 1 | Chat 路径软删（Include=false）后用户在 UI 看不到历史消息 | 中 | 中 | 软删仅影响**喂给 LLM 的 history**；UI 列表查询不加 include 过滤即可保留显示。需确认前端 chat 列表查询未隐式依赖 include |
| 2 | `UpToChatID` 与并发新消息竞态（压缩期间用户又发消息） | 低 | 中 | `boundaryChatID` 取压缩开始时的最后一条 id；软删 `id <= boundaryChatID`，新消息 id 更大不受影响 |
| 3 | 上游 PR 未合并阻塞接入 | 中 | 中 | §10 改动与 §3–§9 解耦；可先用现状引擎接入，缺口逐项补；必要时 `go.mod replace` 指向本地分支 |
| 4 | 模型映射未命中 → ctxLen=8192 偏小 → 过早压缩 | 中 | 低 | 保守默认安全；workspace 覆盖兜底；映射表持续补充 |
| 5 | thread_id=nil 的默认工作区会话摘要串话（多用户） | 低 | 高 | 查询始终带 `workspace_id`；MULTI_USER 下默认会话本就 workspace 隔离 |
| 6 | 脱敏调优后密钥仍可能入摘要（去了 bare-hex） | 低 | 中 | 保留 key/token/bearer/AWS 显式规则；bare-hex 误杀代价 > 漏检收益（已与用户确认权衡） |
| 7 | Agent 路径每 step 都跑 CompressMessages 增加延迟 | 中 | 低 | `ShouldCompress` 在阈值内直接返回原历史（已校准后成本极低，仅一次 token 估算） |
| 8 | V1 范围因并入 5 项扩展近翻倍，工期与上游 PR 耦合加深 | 中 | 中 | §18 各项相互解耦，可按 §11 接入主体 → §18.1/18.4（hermind 独立）→ §18.2/18.3/18.5（含上游）顺序增量交付；MemoryProvider 注册即可（V1 无 subagent，不阻塞） |
| 9 | `compactionMemoryProvider` V1 实际不触发（无 subagent），形成预埋未用代码 | 中 | 低 | 仅注册 + 服务层种子拷贝承担 V1 handoff；provider 由单测覆盖 OnSessionSwitch 行为，避免死代码 |

---

## 14. Done 标准

- [ ] `thread_compaction.go` 模型 + AutoMigrate 注册（`db.go`）
- [ ] `model_metadata.go` / `factory.go` / `persistence.go` / `redact_patterns.go` 实现 + 单测
- [ ] Agent 三处 `pAgent.New` 加 `WithContextEngine`（条件开启）
- [ ] Chat 路径 `buildChatHistory` + 压缩判定改造
- [ ] `SystemSetting "context_compress_enabled"` 默认值写入 `db.go` defaults
- [ ] `Workspace.CompressEnabled *bool` + `CompressContextLen *int` 字段
- [ ] 上游 Pantheon §10 **八项**改动（独立 PR）：accessor、per-tool 模板、输入截断、fallback 模型、summary-prefix、可注入脱敏集、`UpdateFromResponse` 回喂、600s 三级冷却
- [ ] WS 事件 `context.compressed` + telemetry `compaction_finished`
- [ ] §18.1 `POST /api/workspaces/:slug/compress` handler + `CompressNow` service
- [ ] §18.2 `WorkspaceThread.ParentThreadID` + `CreateThreadRequest.ParentThreadID` + 种子拷贝 + `compactionMemoryProvider` 注册
- [ ] §18.3 Chat 路径捕获 `LLMChunk.Usage` → `lastPromptTokens` 缓存 → 喂压缩判定
- [ ] §18.4 `UpdateWorkspaceRequest` 三字段 + WorkspaceService.Update + 前端工作区设置「上下文压缩」分区
- [ ] **测试命令全绿**：`cd backend && go test ./internal/agent/compression/... ./internal/services/... ./internal/agent/...`
- [ ] **手测**：长 Agent 会话（>50% ctx）观察日志 `context compression triggered` + WS 收到 `context.compressed`；重连后摘要从 DB 回填（日志确认 `SetPreviousSummary`）
- [ ] **手测**：长 Regular Chat（>75% ctx）观察 `chat compaction: N→M tokens` + UI 历史仍完整显示

---

## 15. 测试计划（断言级）

| 测试文件 | 断言 |
|---|---|
| `model_metadata_test.go` | 精确命中 `claude-3-5-sonnet`→200000；前缀 `gpt-4o-2024-xx`→128000；未命中→8192；wsOverride 优先 |
| `redact_patterns_test.go` | git SHA(40 hex) **不被**脱敏；email **不被**脱敏；`sk-ant-xxx`/`Bearer xxx`/`api_key=xxx` **被**脱敏 |
| `persistence_test.go` | Save→LoadLatest 取回最新；thread_id=nil 与具体 thread 互不串；同 (ws,thread) 多次 Save 取 id 最大 |
| `factory_test.go` | NewForAgent Threshold==0.50；NewForChat==0.75；其余默认一致 |
| `chat_service_compaction_test.go` | history 超 0.75*ctx 触发压缩；buildChatHistory 含摘要合成行且仅读 id>UpToChatID；软删后 include=false；UI 查询不受影响 |
| `agent_compaction_test.go` (e2e) | WithContextEngine 后长会话触发；step 后 SaveSummary 落库；重连 newSession 调 SetPreviousSummary；fallback 时 WS 收到警告 |
| 上游 `pantheon/.../compressor_test.go` | PreviousSummary/SetPreviousSummary 往返；per-tool 模板输出含 tool name；renderTranscript 截断 >6000 字符；可注入 RedactPatterns 生效；`UpdateFromResponse(usage)` 后 ShouldCompress 用真实 prompt_tokens；冷却第三级达 600s |
| `compress_endpoint_test.go` (§18.1) | 历史过短→409；正常→200 含 summary；topic 透传到 focusTopic；强制压缩不看阈值 |
| `thread_handoff_test.go` (§18.2) | ParentThreadID 非 nil→子 thread 种子摘要==父最新摘要；UpToChatID==0；根 thread 无种子 |
| `usage_calibration_test.go` (§18.3) | 捕获末 chunk usage→lastPromptTokens；下一轮压缩判定优先用真实 usage；无 usage 回落估算 |
| `workspace_compress_settings_test.go` (§18.4) | 三字段 nil→回落全局/默认；非 nil→覆盖生效；优先级 workspace>全局>分路径默认 |

---

## 16. Assumptions & Unverified Items

| # | 假设 | 置信度 | 影响 if 错 | 如何验证 |
|---|---|---|---|---|
| 1 | Pantheon `WithContextEngine` 在每 step 自动调 `CompressMessages`，hermind 无需手动触发 Agent 压缩 | High | 若需手动触发则 Agent 接入方式变 | 已读 `agent.go:197`/`stream.go:73` 确认 |
| 2 | `s.llmProv.LanguageModel()` 返回的 core.LanguageModel 可直接用作摘要辅助模型 | High | Chat 路径辅助模型来源需改 | 已读 `llm.go:23` 接口含 `LanguageModel()`；建议实现时跑一次真实摘要 |
| 3 | 前端 chat 历史列表查询**不**隐式依赖 `include` 过滤，软删不影响 UI 显示 | Medium | 软删会让用户 UI 丢历史（风险册#1） | 实现前 grep 前端/handler 中 chat 列表查询是否带 `include` |
| 4 | thread_compactions 的 `thread_id <=> ?`（NULL-safe equal）在目标 SQLite/Postgres 均可用 | Medium | thread_id=nil 查询需改写为 IS NULL 分支 | 实现时按现有 `buildChatHistory` 的 `thread_id IS NULL` 分支写法即可 |
| 5 | 上游 §10 六项改动可独立 PR 且不破坏 Pantheon 现有调用方 | Medium | 需 fork/replace 兜底 | 改动均为新增/可选参数，向后兼容；提交 PR 跑 pantheon 全测 |
| 6 | modelName 在接入点可获得（Agent: lm/ws 配置；Chat: ws 配置） | Medium | 校准取不到模型名 | 实现时从 `ws` 模型配置或 `buildLanguageModel` 入参取 |
| 7 | 默认 contextLength=8192 对未知模型是安全保守值 | High | 偏小→过早压缩（可接受，风险#4） | 标准下限假设 |
| 8 | `ChatService.Stream` 能从 `LLMChunk.Usage` 拿到末轮真实 usage（§18.3） | High | usage 校准回落字符估算 | 已读 llm.go:17/112 确认字段存在 |
| 9 | Pantheon agent loop 加 `UpdateFromResponse` 调用不破坏现有行为（§10#7） | High | agent 路径 usage 校准失效，回落估算 | resp.Usage 已存在(agent.go:288)，仅多一次无副作用调用 |
| 10 | 加 `WorkspaceThread.ParentThreadID` 不影响现有 thread 查询/迁移（§18.2） | Medium | 迁移需处理既有行（默认 nil 安全） | 可空字段 + AutoMigrate；既有行默认 NULL |

---

## 17. Open Questions / Resolved Decisions

**已解决（Resolved）**：
1. 核心策略 = 复用 Pantheon 引擎 + 薄接入（非重新移植）`[C:USER]`
2. 覆盖范围 = Agent + Regular Chat 两条路径 `[C:USER]`
3. 摘要持久化 = 新表 `thread_compactions` `[C:USER]`
4. 脱敏 = 调优（去 bare-hex + email）`[C:USER]`
5. context-length 来源 = 内嵌静态映射 + workspace 覆盖 `[C:USER]`
6. 引擎缺口 = 上游贡献给 Pantheon（本轮做）`[C:USER]`
7. 开关 = 全局 SystemSetting + per-workspace 覆盖 `[C:USER]`
8. 默认态 = OFF（opt-in）`[C:USER]`
9. 触发阈值 = Agent 0.50 / Chat 0.75 `[C:USER]`
10. 可观测 = WS 事件 + mlog + telemetry 全量 `[C:USER]`
11. 失败降级 = 静态 fallback + 用户显式警告 `[C:USER]`
12. Agent 摘要回填机制 = 上游加 accessor `[C:USER]`
13. V1 范围扩展 = `/compress` 端点 + 跨 thread handoff + 实时 usage 校准 + per-workspace 前端 UI + 600s 冷却三级，全部并入 V1（§18）`[C:USER]`

**延后（Deferred）**：subagent 并行隔离（与压缩正交）、压缩摘要多代历史浏览 UI。

---

*设计日期: 2026-06-01 · 方法: gpowers brainstorming (Deep audit) · 取代 2026-05-28 旧设计*
