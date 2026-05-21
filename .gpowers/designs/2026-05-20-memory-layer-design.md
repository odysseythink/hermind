# Hermind Memory Layer — 智能记忆中间层设计文档（v2）

> **Status**: Phase 1 shipped 2026-05-21. Phase 2 shipped 2026-05-22.
>            Phase 3 plan pending.
> **Date**: 2026-05-22
> **Supersedes**: v1 (2026-05-20)
> **Source**: EverOS / EverCore 亮点功能引入方案 — 经客观评估后的修订版

---

## 0. v2 修订纲要

v1 抓住了"分类 + 生命周期 + 技能提取"的脚手架，但漏掉了 EverCore 在 LoCoMo 92.3% 上真正发力的几件事，同时引入了若干非必要抽象。v2 的核心变化：

**补强（高 ROI）**
1. 新增 **Hybrid Retrieval（BM25 + 向量 + RRF）+ 轻量 Reranker** —— Agentic 多轮的真正地基。
2. 新增 **MemCell-lite 边界检测 + Provenance 链路（`ParentTurnID` / `ParentMemID`）** —— 决定下游提取质量与可追溯性。
3. 新增 **Living Profile 作为独立单文档对象**（不是把 profile 当作一个 MemType）。
4. **Foresight 真做成时效对象**（`ExpiresAt` + 过期归档 + 可选提醒事件）。
5. 把现有的 `ReinforcementCount / NeglectCount / LastUsedAt` 接入检索打分。

**剔除（过度设计）**
1. 独立 `SkillExtractor` 并行管线 → 改为向 `skills.Evolver` 发候选事件，单一写入者。
2. 4 段生命周期钩子 → 收敛为 2 段（`OnSessionStart` + `OnTurnComplete`）。
3. `memory_layer.enabled: false` 全零成本回退 → 删除，默认开启 + 做好降级。
4. 6 类 MemType 同表 → 拆为 4 类 MemType（`core/episode/fact/foresight`）+ 独立 Profile 表 + 现有 skills 文件。
5. 删除：`sufficiency_model` 独立模型配置、`eviction_policy` 双策略、`extract_prompt` 自定义路径、`OnBeforePrompt` / `OnConversationEnd` 钩子。
6. Agentic：`max_rounds=2` → `max_extra_rounds=1`，`expansion_queries=3` → `2`。

工时由 v1 的 9 天调整为 **13–17 天**（含真实地基）。

---

## 1. 背景与目标

### 1.1 问题陈述

Hermind 当前记忆系统的缺口：

1. **检索地基薄弱**：`Recaller.Recall` 要么 FTS 要么向量，没有融合，更没有精排；无法对复杂查询给出稳定召回。
2. **派生记忆扁平且无溯源**：`storage.Memory` 没有 `ParentTurnID` 字段，提取出的 fact 与源对话之间断开。
3. **无生命周期管理**：会话开始不预加载画像；turn 完成无统一处理面；working_summary 是 MetaClaw 私有逻辑。
4. **画像缺失**：没有"who is this user"的单文档对象；偏好散落在 `preference` 记忆中，无法增量编辑。
5. **缺时效语义**：无法表达"这件事下周三过期"。
6. **已有信号未利用**：`ReinforcementCount / NeglectCount / LastUsedAt / ReinforcedAtSeq` 在存储层已实现，但 Recall 完全没用上。

### 1.2 设计目标

在 `agent/memorylayer` 引入智能记忆中间层，目标按重要性排序：

- **G1 检索质量**：Hybrid + RRF + Reranker，把单轮召回先做扎实；Agentic 多轮作为上层增益。
- **G2 派生质量**：MemCell-lite 边界检测，让提取作用在完整语义单元上而非碎片 turn。
- **G3 画像与时效**：Living Profile 独立对象 + Foresight 时效字段。
- **G4 生命周期统一**：两段钩子覆盖会话启动与每轮收尾。
- **G5 信号闭环**：把 reinforcement 信号反哺到检索排序。

**约束**：对外接口（`memprovider.Provider` / `Recaller` / `tool.Registry`）保持兼容；MetaClaw 等现有 provider 不强制改造。

---

## 2. 总体架构

```
┌───────────────────────────────────────────────────────────────────────┐
│                      API / CLI / Gateway（无改动）                      │
└───────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌───────────────────────────────────────────────────────────────────────┐
│                        agent/memorylayer（新增）                        │
│                                                                       │
│  ┌──────────────┐  ┌──────────────────────────────────────┐           │
│  │ Lifecycle    │  │ Hybrid Recaller                       │           │
│  │ (2 hooks)    │  │  ├── BM25 (FTS5)                      │           │
│  └──────┬───────┘  │  ├── Vector (existing embedder)       │           │
│         │          │  ├── RRF Fusion                       │           │
│         │          │  └── Signal Boost (reinforcement)     │           │
│         │          └──────────────┬───────────────────────┘           │
│         │                         │                                    │
│         │                         ▼                                    │
│         │          ┌─────────────────────────────┐                     │
│         │          │ LLM Reranker（top-K → top-N）│                     │
│         │          └──────────────┬───────────────┘                     │
│         │                         │                                    │
│         │                         ▼                                    │
│         │          ┌─────────────────────────────────────┐             │
│         │          │ Agentic Wrapper                      │             │
│         │          │  ├── Sufficiency Check               │             │
│         │          │  └── 1 extra round (≤2 sub-queries)  │             │
│         │          └─────────────────────────────────────┘             │
│         │                                                              │
│         ▼                                                              │
│  ┌────────────────────────────────────────────────────────────┐        │
│  │ Boundary-Triggered Extractors                              │        │
│  │  ├── MemCell-lite boundary detector                         │        │
│  │  ├── Taxonomy Extractor (core / episode / fact / foresight)│        │
│  │  ├── Profile Updater (incremental add/update/delete)       │        │
│  │  └── Skill Candidate Emitter → skills.Evolver              │        │
│  └────────────────────────────────────────────────────────────┘        │
└───────────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┼────────────────┬─────────────────┐
              ▼               ▼                ▼                 ▼
       ┌──────────┐   ┌──────────────┐  ┌────────────┐  ┌────────────┐
       │ MetaClaw │   │ External     │  │ SQLite +   │  │ skills/    │
       │ (本地LLM) │   │ memproviders │  │ FTS5 +     │  │ (.md files)│
       │          │   │ (Honcho 等)   │  │ vectors    │  │            │
       └──────────┘   └──────────────┘  └────────────┘  └────────────┘
```

**设计原则**

1. **可选包装**：是 `Recaller` 之上的装饰器，外部接口不变。
2. **引擎层通用化**：所有 provider 受益于 Hybrid + Rerank + Agentic。
3. **单一写入者**：skill 由 `skills.Evolver` 唯一写入；记忆层只发候选事件。
4. **配置驱动**：通过 `config.AgentConfig.MemoryLayer` 控制；**不再有全功能 disable 开关**，但子能力可独立关闭。

---

## 3. 记忆分类学（4 类 MemType + 2 类独立对象）

v1 的 6 类混在一起；v2 按存储形态拆分。

### 3.1 进入 `MemType` 的 4 类（共用 `memories` 表）

| 类型 | 说明 | 检索时注入策略 |
|---|---|---|
| `core` | 用户显式要求永远记住（"我对花生过敏"） | 每次对话强制注入，受 `core_max_tokens` 上限 |
| `episode` | MemCell 派生的对话段叙事（替代 v1 `episodic`） | 按相关度检索注入 |
| `fact` | 原子事实（替代 v1 `semantic`） | 按相关度检索注入 |
| `foresight` | **带时效**的前瞻预测，含 `ExpiresAt` | 未过期者高优先；启动时可选强注入未来 7 天内 |

**向后兼容映射**
- `episodic` → `episode`
- `semantic` → `fact`
- `preference` → 迁移到 Profile 独立对象（见 §6）

### 3.2 独立对象：Profile（独立表）

详见 §6。**不是 `MemType`**，因为它是单文档增量编辑对象，与离散记忆形态不同。

### 3.3 独立对象：Skill（仍住 `skills/` 目录）

记忆层不在 `memories` 表里另开一份 skill 副本；只通过事件通知 `skills.Evolver`，由 Evolver 决定是否写文件。**不引入 `MemType=skill`**。

### 3.4 存储层字段扩展

```go
// storage/types.go — 在现有 Memory 上增量
type Memory struct {
    // ...现有字段保留...
    MemType      string    // 取值: core | episode | fact | foresight（+ 兼容旧值）

    // v2 新增
    ParentTurnID  int64    // 源对话 turn id，0 表示手工或外部同步
    ParentMemID   string   // 源 MemCell id（episode/fact/foresight 派生时）
    ExpiresAt     time.Time// foresight 必填；其他类型零值表示永久
    ClusterID     string   // 主题簇 id（可选，§9）
}
```

新增/调整索引：`idx_memories_mem_type`、`idx_memories_expires_at`、`idx_memories_parent_turn`。

### 3.5 Taxonomy Extractor（按边界触发，不按 turn 触发）

```go
// agent/memorylayer/extractor.go
type TypedMemory struct {
    Type         string    // core | episode | fact | foresight
    Content      string
    Confidence   float64
    ExpiresAt    time.Time // 仅 foresight
    ParentTurnID int64
    ParentMemID  string
}

func (ex *Extractor) ExtractFromBoundary(ctx, cell MemCell) ([]TypedMemory, error)
```

LLM 输出示例：

```json
[
  {"type":"core",     "content":"用户对花生过敏","confidence":0.98},
  {"type":"episode",  "content":"调试 Vite 静态文件 404 的会话","confidence":0.9},
  {"type":"fact",     "content":"项目使用 pnpm","confidence":0.92},
  {"type":"foresight","content":"用户计划周三前交报告","expires_at":"2026-05-27T00:00:00Z","confidence":0.86}
]
```

---

## 4. Hybrid Retrieval + Reranker（地基）

### 4.1 为什么不能跳过这一层

EverCore 的 Agentic 之所以好用，是因为每一轮内部就是 BM25 + dense + RRF；Hermind 现在每一轮拿到的候选质量本身就差，多轮也救不回来。v2 把 Hybrid 当作地基先做。

### 4.2 Hybrid Recaller 实现

```go
// agent/memorylayer/hybrid_recaller.go
type HybridRecaller struct {
    base       memprovider.Recaller   // 兜底 / 外部 provider
    fts        FTS5Recaller           // SQLite FTS5 直查
    vector     VectorRecaller         // 现有 embedder + sqlite_vec
    reranker   Reranker               // §4.3
    signalFn   func(*storage.Memory) float64 // reinforcement 信号
    cfg        HybridConfig
}

func (h *HybridRecaller) Recall(ctx, query string, limit int) ([]InjectedMemory, error) {
    // 1. 并发跑 BM25 与 Vector，各取 top-N（默认 N = limit * 3）
    // 2. RRF 融合：score(d) = Σ 1/(k + rank_m(d))，默认 k=60
    // 3. 信号加权：final = rrf * (1 + α * normalizedReinforcement(d))
    //    - α 默认 0.15；新记忆 reinforcement=0 时不加分
    //    - neglect 高的记忆扣分（penalty）
    // 4. 取 top-K（K = limit * 2）送 Reranker
    // 5. Reranker → top-limit
}
```

**RRF 默认参数**
- `k = 60`（Cormack 经典值）
- `bm25_top_n = vector_top_n = limit * 3`
- `pre_rerank_top_k = limit * 2`

**信号利用**（Hermind 优势，EverCore 没有）
- `ReinforcementCount` ↑ → 轻微加权
- `NeglectCount` 高且 `LastUsedAt` 久远 → 轻微扣分
- 跨 `ReinforcedAtSeq` 的 reinforcement 按 skills 代差衰减（与现有 `skills/generation.go` 协同）

### 4.3 Reranker（轻量 LLM-as-reranker）

```go
type Reranker interface {
    Rerank(ctx context.Context, query string, candidates []Candidate, topN int) ([]Candidate, error)
}

type LLMReranker struct {
    llm   core.LanguageModel
    cfg   RerankerConfig  // BatchSize, Timeout, Concurrency
}
```

- 一次性把 top-K 候选喂给 LLM，让其按相关度返回排序索引（不是逐对打分，避免 O(K²) 调用）。
- 批量 + 并发，参考 EverCore 的 `rerank_service.py`。
- **失败降级**：reranker 报错 → 跳过精排直接返回 RRF top-N，记录 `metadata.reranker_skipped=true`。

### 4.4 与外部 provider 的协同

外部 provider 没有 FTS+vector 双索引时，HybridRecaller 退化为：`base.Recall` 单路 + Reranker。仍然比纯 base 强（多了精排）。

---

## 5. Agentic 多轮检索（建在 §4 之上）

### 5.1 流程

```
userMsg
  ↓
HybridRecaller.Recall（已含 RRF + Rerank）→ M1
  ↓
Sufficiency Shortcut: 若 M1[0].score > shortcut_threshold（默认 0.85），直接返回
  ↓
Sufficiency Check（LLM）
  ├── "SUFFICIENT" → 返回 M1
  └── "INSUFFICIENT":
        LLM 生成 2 个补充子查询（max_extra_rounds=1）
        并发 HybridRecaller.Recall(subQ) → M2, M3
        RRF 跨查询再融合
        Reranker → top-limit
```

### 5.2 配置

```go
type AgenticConfig struct {
    Enabled           bool    // 默认 true
    MaxExtraRounds    int     // 默认 1（v1 是 2，砍）
    ExpansionQueries  int     // 默认 2（v1 是 3，砍）
    ShortcutThreshold float64 // 默认 0.85，跳过 LLM 判断
    PerSessionTokenCap int    // 整个 session 多花的 token 上限，默认 20000
    PerTurnTokenCap    int    // 单 turn 默认 2000
    Timeout           time.Duration // 默认 8s，超时降级单轮
}
```

**降级路径**
- 超时 / LLM 失败 → 返回 §4 的 HybridRecaller 结果，metadata 标 `agentic_fallback=true`。
- 触达 `PerSessionTokenCap` → 当前 session 后续 turn 自动只走 §4。

### 5.3 与 Synergy Budget 的协同

Agentic 返回的列表仍交给 `applySynergyBudget` 处理上下文长度。Pinned core 在 budget 计算前先扣除（§7）。

---

## 6. Living Profile（独立对象）

### 6.1 与 v1 的差别

v1 把 profile 当成 MemType；v2 把它做成 EverCore 的形态：**每个用户一条增量编辑的文档**。

### 6.2 数据模型

```go
// storage/types.go
type Profile struct {
    UserID    string
    Sections  []ProfileSection // explicit_info + implicit_traits
    UpdatedAt time.Time
    Version   int64
}

type ProfileSection struct {
    Kind        string   // "explicit" | "implicit"
    Key         string   // e.g. "diet.restrictions", "style.communication"
    Value       string
    Evidence    string   // 文本佐证
    SourceTurns []int64  // 关联 turn id，用于溯源
    Confidence  float64
    UpdatedAt   time.Time
}
```

新增表：`profiles`（按 user_id 主键），`profile_sections`（外键到 profile）。

### 6.3 增量编辑（EverCore ID 映射策略）

```go
// agent/memorylayer/profile.go
type ProfileUpdater struct {
    storage  storage.Storage
    llm      core.LanguageModel
}

// 输入：当前 profile + 新边界 MemCell；
// 输出：差量操作 [add | update | delete]
func (p *ProfileUpdater) Apply(ctx, userID, MemCell) (ProfileDelta, error)
```

LLM 提示词里把 sections 映射成短 ID（`s1`, `s2`...）避免幻觉。差量产物原子写入。

### 6.4 注入策略

- **OnSessionStart**：整篇 profile 渲染为 `## User Profile` 段落注入（token 上限 `profile_max_tokens`，默认 800）。
- **OnTurnComplete**（边界触发时）：`ProfileUpdater.Apply` 异步执行，不阻塞 turn。

---

## 7. MemCell-lite 边界检测 + Provenance

### 7.1 为什么需要

v1 在 `OnConversationEnd` 提取记忆，但 Hermind 没有明确的 "conversation end" 信号。EverCore 的答案是按语义边界切片。v2 引入最简版本。

### 7.2 边界检测器

```go
// agent/memorylayer/boundary.go
type BoundaryDetector struct {
    cfg BoundaryConfig
    buf []TurnRef  // recent turns awaiting boundary
}

type BoundaryConfig struct {
    HardTokenLimit    int           // 默认 8000，超过强制切
    HardTurnLimit     int           // 默认 20，超过强制切
    IdleGap           time.Duration // 默认 10 分钟无新 turn → 切
    TopicShiftSignal  bool          // 启用低成本主题漂移启发
}
```

**主题漂移启发（无 LLM 成本）**
- 用现有 embedder 计算 buffer 头 vs 末 turn 的 cosine sim；< 0.55 视为漂移。
- 计算只在到达"软阈值"（默认 1500 tokens）后触发。

### 7.3 边界触发链

```
turn complete
    ↓
Detector.Observe(turn)
    ↓
若 IsBoundary():
    ├── 形成 MemCell（in-memory，不入库）
    ├── 异步 dispatch:
    │     ├── Taxonomy Extractor → memories（带 ParentTurnID/ParentMemID）
    │     ├── Profile Updater → profiles
    │     └── Skill Candidate Emitter → skills.Evolver
    └── 写 memory_event "boundary.detected"
```

### 7.4 Provenance 价值

- UI 显示 citation："这条事实来自第 47 轮对话"。
- Source turn 被删除时级联归档派生项。
- Evolver 收到 candidate 时直接定位源对话，无须重读全部 history。

---

## 8. Foresight 真做成时效对象

### 8.1 字段

`Memory.ExpiresAt` 已在 §3.4 加入。Foresight 类必须设置该字段。

### 8.2 行为

- **检索时过滤**：默认排除 `ExpiresAt < now` 的 foresight；除非查询显式要历史。
- **Consolidate 扩展**：`memprovider/consolidate.go` 增加 "expired foresight → archived" 步骤。
- **OnSessionStart 注入**：未来 7 天到期的 foresight 强注入（数量 ≤ `foresight_max_count`，默认 3）。
- **可选：到期前事件**（v2.1 再加）：到期前 N 小时写 `memory_event "foresight.due"`，由 UI 或外部插件消费。

### 8.3 Solo / Group

Hermind 单人场景，照 EverCore 的"solo-only"逻辑全量启用，不做场景区分。

---

## 9. 主题聚类（可选，Tier 2）

参考 EverCore `ClusterManager`：

- 在线 centroid-based，不调 LLM。
- 每条新 fact/episode 用 embedder 向量与现存 cluster centroid 比对，cosine > 0.78 加入该 cluster，否则新建。
- 记入 `Memory.ClusterID`。
- 检索时附加规则："命中的 fact 所属 cluster 的其他成员"作为补充 top-N（受 `cluster_companions_max` 限制）。

**默认关**，配置 `clustering.enabled=true` 开启；不影响地基功能。

---

## 10. 生命周期钩子（2 段）

v1 的 4 段简化为 2 段。

### 10.1 OnSessionStart

```go
func (lh *LifecycleHooks) OnSessionStart(ctx, eng *agent.Engine) error {
    // 1. 加载 core memories（受 core_max_tokens 限制）
    // 2. 加载未过期且 < 7 天的 foresight
    // 3. 加载 Profile（整篇渲染）
    // 4. 写入 engine.pinnedMemories（独立于 synergy budget）
}
```

### 10.2 OnTurnComplete

```go
func (lh *LifecycleHooks) OnTurnComplete(ctx, turn TurnSnapshot) error {
    // 1. Detector.Observe(turn)
    // 2. 若 boundary detected：
    //     ├─ Taxonomy Extractor（异步）
    //     ├─ Profile Updater（异步）
    //     └─ Skill Candidate Emitter（异步）
    // 3. working_summary（按 working_summary_interval 触发，统一从 MetaClaw 抽到这里）
    // 4. AppendMemoryEvent(session.turn)
}
```

**砍掉的 v1 钩子**
- `OnBeforePrompt` → 直接改 `agent/prompt.go` 中 pinned 渲染逻辑。
- `OnConversationEnd` → 由 Detector.Flush（session shutdown 时触发一次）替代，不暴露为公开 hook。

---

## 11. 配置接口

```yaml
memory_layer:
  # 不再有顶层 enabled；子能力可独立关
  hybrid:
    enabled: true             # 砍掉等于回到旧 Recaller，效果差，但保留逃生口
    rrf_k: 60
    bm25_top_n_multiplier: 3
    vector_top_n_multiplier: 3
    pre_rerank_top_k_multiplier: 2
    reinforcement_alpha: 0.15
    neglect_penalty: 0.10

  reranker:
    enabled: true
    batch_size: 20
    timeout_ms: 1500

  agentic:
    enabled: true
    max_extra_rounds: 1
    expansion_queries: 2
    shortcut_threshold: 0.85
    per_turn_token_cap: 2000
    per_session_token_cap: 20000
    timeout_ms: 8000

  taxonomy:
    types: [core, episode, fact, foresight]
    extract_on_boundary: true

  profile:
    enabled: true
    profile_max_tokens: 800

  foresight:
    inject_on_start_days_ahead: 7
    foresight_max_count_on_start: 3

  boundary:
    hard_token_limit: 8000
    hard_turn_limit: 20
    idle_gap_minutes: 10
    topic_shift_enabled: true
    topic_shift_cosine_threshold: 0.55

  lifecycle:
    inject_core_on_start: true
    core_max_tokens: 600
    working_summary_interval: 5

  clustering:
    enabled: false            # Tier 2，默认关
    cluster_companions_max: 3

  # —— v1 删除项（保留注释作为变更记录） ——
  # skill_extraction: 删除（合并入 skills.Evolver）
  # sufficiency_model: 删除（一律用对话模型）
  # eviction_policy: 删除（统一 oldest）
  # extract_prompt path: 删除（提示词硬编码）
```

---

## 12. 存储与代码改动清单

### 新增文件

```
agent/memorylayer/
├── layer.go              # 入口、初始化
├── hybrid_recaller.go    # §4.2
├── reranker.go           # §4.3
├── agentic.go            # §5
├── boundary.go           # §7.2
├── extractor.go          # §3.5
├── profile.go            # §6
├── lifecycle.go          # §10
├── prompts/
│   ├── taxonomy_extract.txt
│   ├── sufficiency_check.txt
│   ├── query_expansion.txt
│   ├── rerank.txt
│   └── profile_update.txt
└── *_test.go
```

### 修改文件

```
storage/types.go              # Memory 增加 ParentTurnID/ParentMemID/ExpiresAt/ClusterID
                              # 新增 Profile / ProfileSection
storage/sqlite/memory.go      # SearchMemories 支持过滤 expires_at / cluster_id / parent_*
storage/sqlite/profile.go     # 新增 profile CRUD
storage/sqlite/migrations/    # 新表 profiles, profile_sections；新增列与索引
agent/engine.go               # pinnedMemories 字段（独立于 synergy budget）
agent/conversation.go         # OnSessionStart / OnTurnComplete 接入
agent/prompt.go               # pinned 段落渲染（## User Profile / ## Core / ## Foresight）
api/server.go                 # 用 HybridRecaller + Agentic 包装现有 Recaller
config/config.go              # MemoryLayerConfig（结构如 §11）
cli/engine_deps.go            # 初始化记忆层组件
tool/memory/memprovider/consolidate.go  # 增加 expired foresight 归档
skills/evolver.go             # 增加 OnSkillCandidate 入口（替代独立 SkillExtractor）
```

---

## 13. 测试与基准

### 13.1 单元测试

| 文件 | 重点 |
|---|---|
| `hybrid_recaller_test.go` | RRF 公式、信号加权、reranker 降级路径 |
| `reranker_test.go` | 批量与并发、超时降级、空候选 |
| `agentic_test.go` | shortcut、insufficient → 子查询、token 上限、fallback |
| `boundary_test.go` | 硬阈值、idle、主题漂移；buffer flush 一致性 |
| `extractor_test.go` | 4 类记忆产出、ParentTurnID 关联 |
| `profile_test.go` | add/update/delete diff 应用、ID 映射、乐观锁 |
| `lifecycle_test.go` | 两段钩子触发、异步任务不阻塞 turn |

### 13.2 集成测试

- 临时 SQLite + mock LLM；跑 50 turn 合成对话，断言：
  1. core / profile / foresight 在 OnSessionStart 后出现在 system prompt。
  2. Hybrid 在 BM25-only 和 Vector-only 两种偏置查询上都比单路高 ≥ 20% recall@10。
  3. Agentic 在 6 个"复杂跨域"查询上 precision@5 ≥ 单 Hybrid 的 1.1×。
  4. 派生记忆 `ParentTurnID` 全部有效。
  5. 过期 foresight 在 Consolidate 后状态变为 archived。

### 13.3 基准

| 指标 | 目标 |
|---|---|
| Hybrid+Rerank 单轮延迟 | < 250ms（含 1 次小 LLM rerank） |
| Agentic 多轮延迟 P95 | < 6s |
| 单 turn 额外 token（开启 Agentic） | < 1500 tokens |
| Recall@10（合成集） | Hybrid ≥ FTS-only + 25% / Vector-only + 15% |
| Profile diff 准确率（人工评） | ≥ 0.85 |

---

## 14. 风险与缓解

| 风险 | 缓解 |
|---|---|
| Reranker 提高了单轮延迟 | shortcut_threshold 命中时跳过；reranker 失败自动降级 |
| Profile 增量编辑出错改坏画像 | 每次 Apply 写新 version，保留旧版；提供 `/profile rollback` |
| Foresight 误判过期时间 | 默认置为"7 天后"如果 LLM 没给；UI 显示来源句子 |
| Boundary 切得过密（每 turn 都切） | HardTurnLimit 下限 + 软阈值 1500 tokens 才允许漂移检测 |
| Hybrid 对外部 provider 不友好 | 自动降级为 base.Recall + Reranker |
| 多轮 token 失控 | per-turn 与 per-session 双上限；触达后只走单轮 |
| Profile 注入泄露敏感信息到外部 LLM provider | 提供 `profile.redact_sections` 配置（v2.1） |

---

## 15. 里程碑（13–17 天）

| 阶段 | 内容 | 工时 |
|---|---|---|
| **P1.1** | Hybrid Recaller（RRF + 信号加权）、storage 字段扩展、迁移脚本 | 3 天 |
| **P1.2** | LLM Reranker + 降级路径 | 2 天 |
| **P1.3** | Boundary Detector + Taxonomy Extractor（带 Provenance） | 2 天 |
| **P2.1** | Agentic Wrapper（shortcut + 1 轮扩展 + token caps） | 2 天 |
| **P2.2** | Lifecycle（OnSessionStart + OnTurnComplete）、prompt 渲染 | 1.5 天 |
| **P3.1** | Living Profile（schema + ProfileUpdater + 注入） | 2 天 |
| **P3.2** | Foresight 时效（ExpiresAt + Consolidate 扩展） | 1 天 |
| **P3.3** | Skill Candidate Emitter → Evolver 接入 | 0.5 天 |
| **P4（按需）** | Clustering、citation UI、reminder 事件 | 2–3 天 |
| **测试 / 调优** | 集成测试 + 基准 + 文档 | 2 天 |

**合计 P1–P3：13–14 天**；含 P4：16–17 天。

---

## 16. 与 v1 的差异速查

| 维度 | v1 | v2 |
|---|---|---|
| MemType 数量 | 6 | 4（profile/skill 移出） |
| Hybrid Retrieval | ❌ | ✅ BM25+Vector+RRF |
| Reranker | ❌ | ✅ LLM-as-reranker |
| 多轮配置 | rounds=2, expansion=3 | rounds=1, expansion=2 |
| Lifecycle hooks | 4 | 2 |
| Skill 抽取 | 并行管线 | 候选事件 → Evolver |
| Profile | MemType=profile | 独立对象 + 增量编辑 |
| Foresight | 仅打标签 | 时效字段 + 过期归档 + 启动注入 |
| MemCell 边界 | ❌ | ✅（lite 版本） |
| Provenance | ❌ | ✅ ParentTurnID / ParentMemID |
| Reinforcement 反哺检索 | ❌ | ✅ 加权 + 惩罚 |
| 顶层 enabled 开关 | 有 | 删除（子能力可关） |
| 自定义 prompt 路径 | 有 | 删除 |
| `sufficiency_model` | 有 | 删除 |
| 双 eviction policy | 有 | 删除 |
| 工时估计 | 9 天 | 13–17 天 |

---

*Drafted 2026-05-21 as v2. v1 (2026-05-20) preserved in git history.*
