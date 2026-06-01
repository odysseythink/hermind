# Context Compression Engine — 设计文档

> 日期：2026-05-28  
> 目标：将 Hermes Agent 的四阶段上下文压缩管道完整引入 Hermind，覆盖常规聊天和 Agent 双路径。

---

## 1. 背景与现状

### 1.1 Hermind 当前问题

- **常规聊天路径**（`ChatService.Stream/Complete`）：无任何上下文压缩，仅通过 `OpenAiHistory`（默认20条消息数）截断历史。
- **Agent 路径**（`agent/runtime.go`）：依赖 Pantheon 内置 `compression/compressor.go`，功能极简（固定保留头3条 + 尾20条 + LLM 中间摘要），无 token 预算计算、无工具输出清洗、无迭代更新。
- **配置层面**：无任何压缩相关配置项。

### 1.2 Hermes Agent 优秀设计

`context_compressor.py`（1583行）实现了完整的四阶段管道：

1. **Phase 1 — 工具输出清洗**：MD5去重、图片剥离、JSON安全截断、长度上限截断（零LLM成本）
2. **Phase 2 — Token预算管理**：按模型最大context计算可用预算，划分head/middle/tail，保护工具调用对不拆散，锚定用户最后消息
3. **Phase 3 — LLM结构化摘要**：强制模板输出（关键事实/用户意图/已完成操作/待决问题），支持迭代更新
4. **Phase 4 — 消息组装**：角色冲突避免、孤儿tool_call修复、合法性校验

---

## 2. 架构设计

### 2.1 模块位置

```
backend/internal/contextmanager/
├── config.go               # 压缩配置
├── tokenizer.go            # Token估算（增强版）
├── model_registry.go       # 模型context长度映射
├── pruner.go               # Phase 1: 工具输出清洗
├── boundary.go             # Phase 2: Token预算 & 边界保护
├── summarizer.go           # Phase 3: LLM结构化摘要
├── assembler.go            # Phase 4: 消息组装
├── fallback.go             # 降级策略
├── pipeline.go             # 四阶段编排器
└── *_test.go               # 各阶段单元测试 + 集成测试
```

### 2.2 数据流

```
原始 []core.Message
    ↓
[Pipeline.Compress(ctx, messages, systemPrompt)]
    ↓
┌────────────────────────────────────────────┐
│ Phase 1: Pruner                            │
│ • MD5去重重复工具结果                        │
│ • 剥离图片Base64 → [图片:已省略]              │
│ • JSON工具结果安全截断（保留schema）          │
│ • 单条工具结果长度上限截断                   │
└────────────────────────────────────────────┘
    ↓
┌────────────────────────────────────────────┐
│ Phase 2: Boundary Protector                │
│ • 查询模型最大context长度                   │
│ • 可用预算 = 上限×比例 − 系统提示 − 输入预留 − 缓冲 │
│ • 划分 head + middle + tail                │
│ • 工具调用对不拆散                         │
│ • 用户最后一条消息锚定在tail                │
└────────────────────────────────────────────┘
    ↓
┌────────────────────────────────────────────┐
│ Phase 3: Summarizer（LLM调用）              │
│ • middle部分结构化序列化                    │
│ • 强制模板摘要提示                          │
│ • 支持迭代更新（复用上轮摘要）              │
│ • 解析模板输出                              │
└────────────────────────────────────────────┘
    ↓
┌────────────────────────────────────────────┐
│ Phase 4: Assembler                         │
│ • head + summary + tail 组合               │
│ • 修复角色冲突（相邻同角色）                │
│ • 修复孤儿tool_call                        │
│ • 最终合法性校验                            │
└────────────────────────────────────────────┘
    ↓
压缩后的 []core.Message
```

### 2.3 集成点

| 路径 | 集成位置 | 方式 |
|------|---------|------|
| 常规聊天 | `ChatService.Stream()` / `Complete()` | 调用 `llmProv.Stream()` 前，先执行 `pipeline.Compress()` |
| Agent | `agent/runtime.go` / `session.go` | 替换 Pantheon 内置 compressor，在 Agent 循环中注入 Pipeline |

---

## 3. 配置设计

### 3.1 新增环境变量

```go
type CompressionConfig struct {
    Enabled           bool    `env:"CONTEXT_COMPRESSION_ENABLED" envDefault:"true"`
    TokenBudgetRatio  float64 `env:"CONTEXT_COMPRESSION_TOKEN_BUDGET_RATIO" envDefault:"0.7"`
    AuxModelProvider  string  `env:"CONTEXT_COMPRESSION_MODEL_PROVIDER" envDefault:""`  // 空=使用主模型
    AuxModelName      string  `env:"CONTEXT_COMPRESSION_MODEL_NAME" envDefault:""`      // 空=使用主模型
    PruneToolsEnabled bool    `env:"CONTEXT_COMPRESSION_PRUNE_TOOLS" envDefault:"true"`
    SummarizeEnabled  bool    `env:"CONTEXT_COMPRESSION_SUMMARIZE" envDefault:"true"`
    HeadKeepCount     int     `env:"CONTEXT_COMPRESSION_HEAD_KEEP" envDefault:"3"`
    TailKeepCount     int     `env:"CONTEXT_COMPRESSION_TAIL_KEEP" envDefault:"6"`
    MaxToolResultLen  int     `env:"CONTEXT_COMPRESSION_MAX_TOOL_RESULT_LEN" envDefault:"4000"`
    SummaryMaxTokens  int     `env:"CONTEXT_COMPRESSION_SUMMARY_MAX_TOKENS" envDefault:"1024"`
}
```

### 3.2 模型注册表

`model_registry.go` 内置约100个主流模型的 context 长度映射，同时读取 Hermind config 中的 provider token limit 作为 fallback。

---

## 4. 接口设计

```go
// Pipeline 是唯一直接对外暴露的入口
type Pipeline struct { /* ... */ }

func NewPipeline(cfg CompressionConfig, auxLLM core.LanguageModel) *Pipeline

func (p *Pipeline) Compress(ctx context.Context, history []core.Message, systemPrompt string) ([]core.Message, error)
```

各 Phase 内部接口见代码实现，不对外暴露。

---

## 5. 关键算法

### 5.1 Phase 1 — 工具输出清洗

- 遍历所有 `ToolResultPart`，内容计算 MD5，重复替换为 `[重复结果，同上]`
- `ImagePart` 替换为 `[图片内容已省略]`
- JSON 内容且长度 > MaxToolResultLen：保留键名，截断数组/长字符串值
- 非 JSON 长文本：保留前/后各 MaxToolResultLen/2，中间替换为 `...[省略N字符]...`

### 5.2 Phase 2 — Token 预算划分

```
模型上限 = ModelRegistry.Lookup(modelName) 或 Config.ProviderTokenLimit
可用预算 = 模型上限 * TokenBudgetRatio − Estimate(systemPrompt) − Estimate(input) − safetyBuffer(200)

tailToken = Sum(Estimate(tail候选))
若 tailToken >= 可用预算 * 0.4: 减少 tail 保留消息数

headToken = Sum(Estimate(head候选))
middleBudget = 可用预算 − headToken − tailToken
```

### 5.3 Phase 3 — 摘要提示模板

```
你是一个对话摘要助手。请将以下对话历史压缩为摘要，严格按模板输出。

[对话历史]
{{messages}}

[输出模板]
## 关键事实
- <bullet points>

## 用户意图演变
- <bullet points>

## 已完成的操作
- <bullet points>

## 待决问题
- <bullet points>

[约束]
- 仅输出模板内容，不要添加任何其他文字
- 若历史涉及文件/代码，保留文件路径和关键代码片段
- 若历史涉及工具调用，保留工具名称和关键参数
```

### 5.4 Phase 4 — 组装校验

- 相邻同角色消息：中间插入空 `user` 消息分隔
- 孤儿 tool_call：补充 `ToolResultPart{Content: "[调用结果已在摘要中]"}`
- 最终 token 估算超预算：递归截断 head 部分

---

## 6. 错误处理与降级

| 场景 | 处理 |
|------|------|
| Pipeline 未启用 | 直接返回原 history |
| Phase 1 失败 | 跳过清洗，继续 Phase 2 |
| Phase 2 失败 | Fallback 到消息数截断 |
| Phase 3 LLM 失败/超时 | 尝试使用上次 summary；无则 fallback |
| Phase 3 输出解析失败 | 重试1次；仍失败则 fallback |
| Phase 4 组装失败 | Fallback |
| 任何 panic | recover → fallback |

**Fallback 行为：** 保留最近 `min(len(history), TailKeepCount*2)` 条消息。

---

## 7. 测试策略

### 7.1 单元测试

| 测试文件 | 覆盖 |
|---------|------|
| `pruner_test.go` | MD5去重、图片剥离、JSON截断、长文本截断 |
| `boundary_test.go` | 模型查询、预算计算、划分逻辑、工具对对齐、用户锚定 |
| `summarizer_test.go` | 提示构建、模板解析、迭代更新、重试逻辑 |
| `assembler_test.go` | 角色冲突修复、孤儿tool_call修复、最终校验 |
| `model_registry_test.go` | 已知模型、未知模型fallback、config fallback |

### 7.2 集成测试

- `pipeline_test.go` — 端到端测试（mock LLM）
- `contextmanager_integration_test.go` — 与 ChatService / Agent Runtime 集成

---

## 8. 迁移计划

1. **Phase 1：核心引擎** — 创建 `contextmanager/` 全部文件，实现 + 单元测试通过（不修改现有调用点）
2. **Phase 2：常规聊天集成** — `ChatService` 注入 Pipeline，在 `Stream/Complete` 中调用 `Compress()`
3. **Phase 3：Agent 集成** — 替换 Pantheon 内置 compressor，在 Agent 循环中注入 Pipeline
4. **Phase 4：配置 & 文档** — `config.go` 添加配置，`main.go` 初始化，更新 `AGENTS.md`

---

## 9. 设计决策记录

| 决策 | 理由 |
|------|------|
| 复用 Pantheon `core.Message` | 避免引入新消息格式，双路径统一适配 |
| 辅助LLM通过 Provider 注册表获取 | 复用现有基础设施，支持任意模型 |
| 内置模型注册表 + config fallback | 无需外部依赖，渐进式覆盖 |
| 每阶段独立失败降级 | 最大化可用性，避免单点故障导致整个压缩失效 |
| 强制模板输出 | 确保摘要结构稳定，便于下游解析和迭代更新 |
