# Hermind Memory Layer — 智能记忆中间层设计文档

> **Status**: Approved  
> **Date**: 2026-05-20  
> **Source**: EverOS 亮点功能引入方案（方案 A：智能记忆中间层）

---

## 1. 背景与目标

### 1.1 问题陈述

Hermind 当前的记忆系统存在以下缺口：

1. **记忆扁平化**：`memory_save` / `memory_search` 将所有记忆视为无差别的文本块，缺乏结构化分类。
2. **检索简单**：依赖单次 FTS5 / 向量搜索，没有根据查询复杂度自适应调整检索深度的能力。
3. **无生命周期管理**：对话开始前不会自动预加载用户画像，对话结束后不会自动结构化提取新记忆。
4. **技能提取稀疏**：`SkillsEvolver` 分析完整 history 生成 skill，成本高且命中率低。

### 1.2 设计目标

借鉴 EverOS 的 EverCore 记忆系统，在 Hermind 中引入一个**可选的智能记忆中间层**（`agent/memorylayer`），实现：

- **记忆分类学（Taxonomy）**：6 类结构化记忆类型
- **Agentic 多轮检索**：LLM 引导的迭代召回
- **生命周期钩子（Lifecycle Hooks）**：对话启动/结束的自动注入与提取
- **Taxonomy-Driven Skill 提取**：从分类记忆中高效生成 SKILL.md

**核心约束**：零侵入现有架构，关闭时行为完全不变。

---

## 2. 总体架构

### 2.1 组件位置

```
┌─────────────────────────────────────────────────────────────┐
│                    API / CLI / Gateway                       │
│                      (现有，无改动)                           │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              agent/memorylayer (新增)                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │ Taxonomy     │  │ Agentic      │  │ Lifecycle        │  │
│  │ Extractor    │  │ Retriever    │  │ Hooks            │  │
│  │ (LLM分类)     │  │ (多轮检索)    │  │ (启动/结束注入)   │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ Skill Extractor (技能自动提取)                        │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
      ┌──────────┐   ┌──────────┐   ┌──────────────┐
      │ MetaClaw │   │ Honcho   │   │ Built-in     │
      │ (本地LLM) │   │ (云端)    │   │ SQLite FTS5  │
      └──────────┘   └──────────┘   └──────────────┘
```

### 2.2 核心原则

1. **可选包装**：`memorylayer` 是现有 `memprovider.Provider` / `Recaller` 之上的可选增强层。
2. **接口兼容**：不修改现有 `Provider` / `Recaller` / `tool.Registry` 接口。
3. **引擎层通用化**：所有增强功能对所有 memory provider 生效（MetaClaw、Honcho、Mem0、SQLite 等）。
4. **配置驱动**：通过 `config.AgentConfig.MemoryLayer` 完全控制开关和行为参数。

---

## 3. 记忆分类学（Taxonomy）

### 3.1 记忆类型定义

在现有 `MetaClaw` 的 `episodic / semantic / preference` 基础上，扩展到 **6 类记忆**：

| 类型 | 英文名 | 说明 | 注入策略 |
|---|---|---|---|
| 核心记忆 | `core` | 用户明确要求的"永远记住"（如"我是左撇子"） | 每次对话强制注入，受 `core_max_count` 限制 |
| 用户画像 | `profile` | 推断出的用户属性、习惯、偏好 | 按需检索注入，受 synergy budget 限制 |
| 原子事实 | `fact` | 离散知识点（如"项目的 API 密钥存在 .env"） | 按需检索注入 |
| 对话片段 | `episode` | 完整或压缩的对话回合 | 按需检索注入 |
| 技能模板 | `skill` | 从成功任务中提取的可复用模式 | 存入 skills 目录，同时保留记忆引用 |
| 前瞻记忆 | `foresight` | 用户的计划、待办、预测性信息 | 高优先级检索，启动时可选强制注入 |

### 3.2 存储层扩展

现有 `storage.Memory` 结构已有 `MemType string`，扩展合法取值并增加索引：

```go
// storage/storage.go — 已有字段兼容
type Memory struct {
    ID        string
    Content   string
    MemType   string  // 新增枚举: core, profile, fact, episode, skill, foresight
    Category  string  // 保留用于向后兼容
    Embedding []float32
    CreatedAt time.Time
    UpdatedAt time.Time
    Source    string  // "manual" | "extracted" | "synced"
    TurnID    string  // 关联的对话回合
}
```

**向后兼容映射**：
- `episodic` → `episode`
- `semantic` → `fact`
- `preference` → `profile`

### 3.3 自动分类提取器

新增 `agent/memorylayer/extractor.go`：

```go
type Extractor struct {
    provider provider.Provider
    config   TaxonomyConfig
}

func (ex *Extractor) Extract(ctx context.Context, userMsg, assistantMsg string) ([]TypedMemory, error) {
    // LLM 调用：从对话中提取结构化的记忆列表
    // 提示词模板包含完整的 6 类定义和示例
}

type TypedMemory struct {
    Type        string    // core | profile | fact | episode | skill | foresight
    Content     string
    Context     string    // 提取时的上下文摘要
    Confidence  float64   // 0.0 ~ 1.0
    Embedding   []float32 // 可选
}
```

提取示例输出：

```json
[
  {"type": "profile", "content": "用户偏好使用 TypeScript 而非 JavaScript", "confidence": 0.92},
  {"type": "fact",    "content": "项目使用 pnpm 作为包管理器", "confidence": 0.88},
  {"type": "skill",   "content": "修复 Vite 构建时，先检查 public/ 目录路径是否正确映射", "confidence": 0.85}
]
```

---

## 4. Agentic 多轮检索

### 4.1 核心流程

在现有 `Recaller.Recall()` 之上，包装 `AgenticRetriever`：

```
用户输入 ──► AgenticRetriever.Retrieve(userMsg, limit)
                │
                ├── Round 1: 底层 Recaller.Recall(userMsg, limit*2)
                │            → 候选记忆 M1
                │
                ├── Sufficiency Check: LLM 判断 M1 是否足够回答用户问题
                │            → "sufficient" → 直接返回 M1[:limit]
                │
                └── "insufficient":
                     ├── Query Expansion: LLM 生成 N 个子查询
                     ├── Round 2: 对每个子查询调用 Recaller.Recall(subQ, limit/2)
                     │            → M2, M3, ...
                     ├── Fusion: 合并 M1 + M2 + ...，去重，按原始相关度重排
                     └── 返回 Top K
```

### 4.2 Sufficiency Check 提示词

```
你是一个记忆质量评估器。用户的问题是：
"{userMsg}"

已召回的相关记忆：
{memory_list}

请判断：这些记忆是否足够回答用户的问题？
- 如果足够，回答 "SUFFICIENT"
- 如果不够，说明缺失了哪些信息，并以 JSON 输出 2-3 个补充查询
```

### 4.3 成本配置

```go
type AgenticConfig struct {
    Enabled           bool   // 总开关
    MaxRounds         int    // 最大检索轮次（默认 2）
    ExpansionQueries  int    // 每轮生成的子查询数（默认 3）
    SufficiencyModel  string // sufficiency check 用的模型（空=使用当前对话模型）
    CostBudgetTokens  int    // 每轮对话最多消耗的额外 token（默认 2000）
}
```

### 4.4 实现位置

新增 `agent/memorylayer/retriever.go`：

```go
type AgenticRetriever struct {
    base     memprovider.Recaller
    provider provider.Provider
    config   AgenticConfig
}

func (ar *AgenticRetriever) Retrieve(ctx context.Context, userMsg string, limit int) ([]memprovider.InjectedMemory, error) {
    // Round 1
    candidates, _ := ar.base.Recall(ctx, userMsg, limit*2)
    
    // Sufficiency check
    if sufficient, _ := ar.checkSufficiency(ctx, userMsg, candidates); sufficient {
        return candidates[:limit], nil
    }
    
    // Query expansion + Round 2
    queries, _ := ar.expandQueries(ctx, userMsg, candidates)
    for _, q := range queries {
        extra, _ := ar.base.Recall(ctx, q, limit/2)
        candidates = merge(candidates, extra)
    }
    
    return rerank(candidates, userMsg)[:limit], nil
}
```

**引擎集成**：在 `api/server.go` 中，若 `AgenticConfig.Enabled`，将 `eng.SetActiveMemoriesProvider()` 的 callback 包装为 `AgenticRetriever.Retrieve`。`AgenticRetriever` 内部持有底层 `Recaller`，对其召回结果进行多轮增强后返回。

### 4.5 与 Synergy Budget 的交互

`applySynergyBudget(activeSkills, memContents, e.synergy)` 从头部截断记忆注入长度。Agentic 检索返回的记忆已按综合得分排序，截断逻辑无需改动。

---

## 5. 生命周期钩子（Lifecycle Hooks）

### 5.1 钩子定义

新增 `agent/memorylayer/lifecycle.go`：

```go
type LifecycleHooks struct {
    OnConversationStart func(ctx context.Context, engine *agent.Engine) error
    OnBeforePrompt      func(ctx context.Context, opts *agent.PromptOptions) error
    OnTurnComplete      func(ctx context.Context, turn TurnSnapshot) error
    OnConversationEnd   func(ctx context.Context, result *ConversationResult) error
}

type TurnSnapshot struct {
    Iteration int
    History   []message.HermindMessage
    UserMsg   string
    AssistantReply string
}
```

### 5.2 各阶段行为

#### OnConversationStart — 预加载

```go
func (lh *LifecycleHooks) onConversationStart(ctx context.Context, eng *agent.Engine) {
    // 1. 加载 core memories（强制注入）
    coreMems, _ := eng.Storage().SearchMemories(ctx, "", 
        &storage.MemorySearchOptions{MemTypes: []string{"core"}, Limit: coreMaxCount})
    
    // 2. 加载最近的高优先级 foresight
    foresight, _ := eng.Storage().SearchMemories(ctx, "", 
        &storage.MemorySearchOptions{MemTypes: []string{"foresight"}, Limit: foresightMaxCount})
    
    // 3. 标记为 pinned，绕过 synergy budget
    // （通过 engine 新增字段或 callback 机制传递，具体实现见 engine.go 改动）
    eng.SetPinnedMemories(append(coreMems, foresight...))
}
```

#### OnBeforePrompt — 动态增强

```go
func (lh *LifecycleHooks) onBeforePrompt(ctx context.Context, opts *agent.PromptOptions) {
    // 将 pinned memories 插入 ActiveMemories 最前面
    // pinned memories 渲染为独立的 ## Pinned memories 区块
    opts.ActiveMemories = prependPinned(opts.ActiveMemories, lh.pinned)
}
```

#### OnTurnComplete — 中期维护

```go
func (lh *LifecycleHooks) onTurnComplete(ctx context.Context, turn TurnSnapshot) {
    if turn.Iteration%lh.config.WorkingSummaryInterval == 0 {
        summary, _ := lh.generateWorkingSummary(ctx, turn.History)
        lh.saveWorkingSummary(ctx, summary)
    }
}
```

#### OnConversationEnd — 后处理

```go
func (lh *LifecycleHooks) onConversationEnd(ctx context.Context, result *ConversationResult) {
    // 1. 结构化提取
    extracted, _ := lh.extractor.ExtractConversation(ctx, result.Messages)
    for _, mem := range extracted {
        lh.storage.SaveMemory(ctx, mem)
    }
    
    // 2. Skill 候选检测
    for _, mem := range extracted {
        if mem.MemType == "skill" {
            lh.skillExtractor.ProposeSkill(ctx, mem)
        }
    }
    
    // 3. Session 统计
    lh.storage.AppendMemoryEvent(ctx, time.Now(), "session.end", map[string]any{
        "turns": result.Iterations,
        "new_memories": len(extracted),
    })
}
```

### 5.3 与现有机制的协同

| 现有机制 | 生命周期钩子 | 关系 |
|---|---|---|
| `Recaller.Recall` | `OnBeforePrompt` | **前置增强**：先注入 pinned，再调用 Recall |
| `SyncTurn` | `OnConversationEnd` | **并行**：SyncTurn 同步原始文本，钩子做结构化提取 |
| `SkillsEvolver.Extract` | `OnConversationEnd` | **协同**：Evolver 分析完整 history，钩子从 extracted memories 加速 skill 候选 |
| `MetaClaw.working_summary` | `OnTurnComplete` | **统一提升**：将 MetaClaw 特有逻辑引擎层通用化 |

---

## 6. 技能自动提取增强

### 6.1 设计

引入 `agent/memorylayer/skill_extractor.go`，与现有 `skills.Evolver` **协同**：

```
Conversation End
    │
    ├── Taxonomy Extractor 提取所有类型记忆
    │       └── 发现 skill 类型记忆
    │
    ├── Skill Extractor 处理 skill 记忆
    │       ├── 去重：与现有 skills 目录对比相似度
    │       ├── 增强：用 LLM 将记忆片段扩展为完整 SKILL.md
    │       └── 评估：Confidence > threshold 才写入
    │
    └── 写入 ./.hermind/skills/auto-{date}-{slug}.md
```

### 6.2 Skill 记忆格式

```json
{
  "type": "skill",
  "content": "修复 Vite 构建时 public/ 目录路径 404 的问题",
  "context": "Hermind 前端项目使用 Vite，Go 后端通过 /ui/* 提供静态文件",
  "applicability": ["vite", "static-files", "go-backend"],
  "confidence": 0.85
}
```

### 6.3 Skill Extractor 核心逻辑

```go
type SkillExtractor struct {
    skillsDir      string
    similarity     float64  // 默认 0.85
    threshold      float64  // 默认 0.70
    maxSkills      int      // 默认 50
    evictionPolicy string   // "oldest" | "lowest_confidence"
}

func (se *SkillExtractor) ProposeSkill(ctx context.Context, mem TypedMemory) error {
    // 1. 与现有 skills 去重
    existing, _ := loadExistingSkills(se.skillsDir)
    for _, skill := range existing {
        if cosineSim(mem.Embedding, skill.Embedding) > se.similarity {
            return nil
        }
    }
    
    // 2. 扩展为 SKILL.md
    skillMarkdown, _ := se.expandToSkillMarkdown(ctx, mem)
    
    // 3. 阈值过滤
    if mem.Confidence < se.threshold {
        return nil
    }
    
    // 4. 容量检查 + 淘汰
    if countAutoSkills(se.skillsDir) >= se.maxSkills {
        se.evictOldestOrLowest(se.skillsDir, se.evictionPolicy)
    }
    
    // 5. 写入
    filename := fmt.Sprintf("auto-%s-%s.md", time.Now().Format("20060102"), slugify(mem.Content))
    return os.WriteFile(filepath.Join(se.skillsDir, filename), []byte(skillMarkdown), 0644)
}
```

### 6.4 与现有 Evolver 的协同

| 场景 | 处理方式 |
|---|---|
| Skill Extractor 生成新 skill | 直接写入 `skills/` 目录，Evolver 下一轮可见 |
| Evolver 从 history 提取 skill | 正常流程，不受 Skill Extractor 影响 |
| 两者同时提出相似 skill | 后写入的覆盖先写入的（文件命名含 timestamp）|
| 用户手动编辑 auto- skill | 保留编辑，Skill Extractor 通过相似度检测跳过 |

---

## 7. 配置接口

### 7.1 完整 YAML 配置

```yaml
# config.yaml
memory_layer:
  enabled: true
  
  taxonomy:
    enabled: true
    types: [core, profile, fact, episode, skill, foresight]
    extract_on_sync: true
    extract_prompt: ""           # 可选：自定义提取提示词模板路径
  
  agentic:
    enabled: true
    max_rounds: 2
    expansion_queries: 3
    sufficiency_model: ""        # 空=使用当前对话模型
    cost_budget_tokens: 2000
  
  lifecycle:
    inject_core_on_start: true
    inject_foresight_on_start: true
    core_max_count: 10
    foresight_max_count: 3
    working_summary_interval: 5
    extract_on_end: true
  
  skill_extraction:
    enabled: true
    threshold: 0.7
    dedup_similarity: 0.85
    max_auto_skills: 50
    eviction_policy: "oldest"    # oldest | lowest_confidence
```

### 7.2 Go 配置结构

```go
// config/config.go
type MemoryLayerConfig struct {
    Enabled   bool                 `yaml:"enabled"`
    Taxonomy  TaxonomyConfig       `yaml:"taxonomy"`
    Agentic   AgenticConfig        `yaml:"agentic"`
    Lifecycle LifecycleConfig      `yaml:"lifecycle"`
    SkillExtraction SkillExtractionConfig `yaml:"skill_extraction"`
}
```

### 7.3 向后兼容性

| 场景 | 行为 |
|---|---|
| `memory_layer.enabled: false`（或缺失） | hermind 完全按现有逻辑运行 |
| 旧版 MetaClaw 配置存在 | 两者独立运行，`memory_layer` 包装 MetaClaw 时保留 MetaClaw 原有行为 |
| 无外部 memprovider | `memory_layer` 正常工作，Agentic 检索退化为纯本地 FTS5 |

---

## 8. 测试策略

### 8.1 单元测试

| 测试文件 | 内容 |
|---|---|
| `agent/memorylayer/extractor_test.go` | 模拟 LLM 输出，验证 6 类记忆分类正确性 |
| `agent/memorylayer/retriever_test.go` | mock Recaller，验证多轮检索 fusion 逻辑 |
| `agent/memorylayer/lifecycle_test.go` | mock Engine，验证 4 个钩子触发时机 |
| `agent/memorylayer/skill_extractor_test.go` | mock 文件系统，验证去重、阈值、写入逻辑 |

### 8.2 集成测试

- 启动临时 SQLite + mock LLM provider
- 运行完整对话回合，验证：
  1. core memory 是否正确注入 system prompt
  2. Agentic 检索是否在 sufficiency 不足时触发第二轮
  3. 对话结束后是否正确提取新记忆并保存

### 8.3 基准测试

对比开启/关闭 `memory_layer` 时的：
- 对话完成延迟（增加 < 500ms 为可接受）
- 额外 token 消耗（单轮 < 2000 tokens）
- 记忆召回准确率（用合成数据集测试）

---

## 9. 风险与缓解

| 风险 | 缓解措施 |
|---|---|
| LLM 提取成本过高 | `cost_budget_tokens` 硬限制 + `sufficiency_model` 降级选项 |
| core/foresight 注入过长 | `core_max_count` / `foresight_max_count` 硬上限 |
| 自动 skill 质量差 | `threshold` + `dedup_similarity` 过滤，低质量不写入 |
| 与 MetaClaw 功能重叠 | `memory_layer` 为引擎层通用包装，MetaClaw 作为 provider 继续存在 |
| 多轮检索延迟高 | `max_rounds=2` 限制，sufficiency 命中时直接返回 |

---

## 10. 里程碑

| 里程碑 | 内容 | 预估工时 |
|---|---|---|
| M1 | Taxonomy Extractor + 存储层扩展 | 2 天 |
| M2 | Agentic Retriever | 2 天 |
| M3 | Lifecycle Hooks（4 阶段） | 2 天 |
| M4 | Skill Extractor | 1 天 |
| M5 | 配置集成 + 单元测试 | 1 天 |
| M6 | 集成测试 + 性能调优 | 1 天 |
| **总计** | | **~9 天** |

---

## 11. 附录：文件变更清单

### 新增文件

```
agent/memorylayer/
├── layer.go              # 主入口：MemoryLayer 结构体，初始化/关闭
├── extractor.go          # Taxonomy Extractor
├── extractor_test.go
├── retriever.go          # Agentic Retriever
├── retriever_test.go
├── lifecycle.go          # Lifecycle Hooks
├── lifecycle_test.go
├── skill_extractor.go    # Skill Extractor
├── skill_extractor_test.go
└── prompts/
    ├── taxonomy_extract.txt    # 分类提取提示词模板
    ├── sufficiency_check.txt   # Sufficiency Check 提示词模板
    ├── query_expansion.txt     # Query Expansion 提示词模板
    └── skill_expand.txt        # Skill 扩展提示词模板
```

### 修改文件

```
storage/storage.go              # Memory 结构扩展 MemType 枚举
storage/sqlite/memory.go        # SearchMemories 支持 MemTypes 过滤
agent/engine.go                 # 新增 PinnedMemories / LifecycleHooks 字段
agent/conversation.go           # 在 4 个生命周期点调用 hooks
agent/prompt.go                 # renderPinnedMemories + 注入位置
api/server.go                   # 条件性包装 Recaller 为 AgenticRetriever
config/config.go                # 新增 MemoryLayerConfig
cli/engine_deps.go              # 条件性初始化 MemoryLayer
```

---

*Design approved by user on 2026-05-20. Ready for implementation planning.*
