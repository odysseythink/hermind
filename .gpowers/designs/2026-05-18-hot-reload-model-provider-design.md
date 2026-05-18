# 模型/Provider 配置热重载设计

## 目标

用户在 Settings 面板修改模型或 provider 配置（如切换模型、修改 API key、更换 provider 类型）并点击 Save 后，新配置**立即生效**，后续新对话无需重启 hermind 进程即可使用新的模型/provider。

## 约束

- **单对话模式**：当前 hermind 只有一个对话，没有多对话切换
- **设置与对话互斥**：用户无法在对话进行过程中打开 Settings 面板，因此配置修改只影响**后续**新发起的对话
- **保持现有 Save 流程**：前端保留 dirty-tracking + Save 按钮的交互，不改为实时自动保存
- **最小副作用**：不重创建 terminal backend、browser、MCP manager、presence 等生命周期重的组件

## 当前问题分析

1. `cli.BuildEngineDeps()` 在启动时构建 `api.EngineDeps`（包含 Provider、AuxProvider、ToolReg 等），之后永不更新
2. `api.ServerOpts.Deps` 是**值类型** `EngineDeps`，直接嵌入在 `ServerOpts` 中
3. `api/handlers_config.go` 的 `handleConfigPut` 已能原子更新内存中的 `*config.Config`，但 `s.opts.Deps.Provider` 仍是启动时的旧实例
4. 所有对话 handler 直接读取 `s.opts.Deps.Provider`，没有并发保护

## 方案：DepsBuilder 回调 + atomic 替换

### 核心思路

1. `ServerOpts.Deps` 改为 `*EngineDeps` 指针
2. `Server` 内部用 `atomic.Pointer[EngineDeps]` 保护当前 deps，无锁读取
3. `ServerOpts` 新增 `DepsBuilder func(ctx, cfg, current) (newDeps, error)` 回调，由 `cli` 包注入
4. `handleConfigPut` 在保存配置到磁盘**之前**，先调用 builder 验证/重建；成功后再保存并原子替换
5. 所有 `s.opts.Deps.Xxx` 改为 `s.deps.Load().Xxx`

### 为什么用回调而非直接在 api 包内重建

Provider 的解析逻辑（从 `cfg.Model` 提取 provider name、fallback 链组装、auxiliary 解析、默认模型推断、环境变量回退等）目前全部集中在 `cli/engine_deps.go` 中。如果直接在 `api` 包内重建，需要复制一份相同的逻辑，导致**维护两份代码**。

通过回调，重建逻辑保留在 `cli` 包，可以复用现有的 `pantheonadapter.BuildPrimaryModel()` / `BuildFallbackModel()` / `BuildModel()` 等函数，以及 `embedding` 包的工具。`api` 包保持干净，只负责触发和原子替换。

### 重建范围

Builder 基于 `current *EngineDeps` 做**浅拷贝**，只替换以下字段：

| 字段 | 是否重建 | 原因 |
|------|---------|------|
| `Provider` | ✅ 是 | 主对话模型，直接受 model/provider 配置影响 |
| `AuxProvider` | ✅ 是 | auxiliary 配置（如 vision/judge 模型）可能独立变化 |
| `Storage` | ❌ 否 | 与模型配置无关 |
| `ToolReg` | ❌ 否 | 与模型配置无关（工具注册表） |
| `SkillsReg` | ❌ 否 | 与模型配置无关 |
| `AgentCfg` | ❌ 否 | 与模型配置无关（max_turns 等） |
| `Platform` | ❌ 否 | 常量 "web" |
| `SkillsEvolver` | ❌ 否（第一版） | 依赖 Provider，但重建有副作用（skills 目录扫描），后续可迭代 |
| `SkillsRetriever` | ❌ 否（第一版） | 依赖 embedder，但重建成本可控；与 SkillsEvolver 保持一致暂不重建 |
| `MemProvider` | ❌ 否（第一版） | 重建需要重新 `Initialize()`，可能触发 memory 系统重同步，副作用大 |
| `SkillsTracker` | ❌ 否 | 与模型配置无关 |
| `HTTPIdle` | ❌ 否 | 与模型配置无关 |
| `Presence` | ❌ 否 | 与模型配置无关 |

**第一版聚焦核心诉求**：切换模型后，新对话的主生成模型立即生效。辅助功能（memory、skills）在后续迭代中可扩展为同步重建。

## 数据流

```
用户修改 model/provider → 点击 Save
  │
  ▼
前端 PUT /api/config {config: {...}}
  │
  ▼
handleConfigPut:
  1. 解析 JSON → config.Config
  2. preserveSecrets（保留空白 secret）
  3. 【新增】调用 s.rebuildDeps(ctx, &updated):
       a. 调用 opts.DepsBuilder(ctx, &updated, currentDeps)
       b. Builder: 浅拷贝 currentDeps → 重建 Provider/AuxProvider → 返回 newDeps
       c. 如果 builder 返回 error，中止，返回 400/500，配置**不保存**
  4. config.SaveToPath() 保存到磁盘
  5. *s.opts.Config = updated
  6. s.deps.Store(&newDeps) 原子替换
  7. 返回 200 OK
  │
  ▼
用户回到对话界面 → 发送新消息
  │
  ▼
RunTurn / handleConversationPost:
  s.deps.Load().Provider → 使用新模型
```

**关键设计**：先重建、后保存。如果新配置无法构建出有效的 Provider（如 API key 错误、provider 名称拼写错误），builder 会返回 error，`handleConfigPut` 会中止并返回错误，配置**不会**写入磁盘。这保证了磁盘上的配置始终和运行时的 Provider 一致。

## 文件改动清单

### api/server.go

1. `ServerOpts.Deps`：`EngineDeps` → `*EngineDeps`
2. `ServerOpts` 新增：`DepsBuilder func(ctx context.Context, cfg *config.Config, current *EngineDeps) (*EngineDeps, error)`
3. `Server` 新增：`deps atomic.Pointer[EngineDeps]`
4. `NewServer`：初始化 `s.deps.Store(opts.Deps)`
5. 新增辅助方法：`func (s *Server) currentDeps() *EngineDeps { return s.deps.Load() }`
6. `buildRouter` 中间件：`s.opts.Deps.HTTPIdle` → `s.currentDeps().HTTPIdle`
7. `RunTurn`：所有 `s.opts.Deps.Xxx` → `s.currentDeps().Xxx`

### api/handlers_conversation.go

- `handleConversationPost`：`s.opts.Deps.Provider` → `s.currentDeps().Provider`

### api/handlers_v1_messages.go

- `handleV1Messages`：`s.opts.Deps.Provider` → `s.currentDeps().Provider`

### api/memory_health.go

- `handleMemoryHealth`：`s.opts.Deps.Presence` → `s.currentDeps().Presence`

### api/handlers_config.go

- `handleConfigPut`：调整顺序 — 先 `rebuildDeps`，成功后再 `SaveToPath` 和 `*s.opts.Config = updated`
- 新增 `rebuildDeps` 私有方法

### cli/web.go

- `ServerOpts` 创建：`Deps: &deps`（指针）
- `ServerOpts` 新增 `DepsBuilder` 字段，注入 builder 函数

### cli/engine_deps.go

- 提取 provider 解析 + 重建逻辑为 `BuildProviderDeps(ctx, cfg, current) (*api.EngineDeps, error)` 函数
- `BuildEngineDeps` 复用该函数，保持现有行为不变

## 边界情况

### 1. 对话进行中时修改配置

`runMu` 已经序列化对话 turn。`atomic` 替换发生在 turn 之间，不会中断正在进行的对话。正在进行的对话继续使用旧 Provider 完成，这是预期行为。

### 2. Provider 配置无效

Builder 在重建时会尝试调用 `pantheonadapter.BuildPrimaryModel()`，如果配置无效（如 API key 为空、provider 名称未知），会返回 error。

- `handleConfigPut` 先调用 builder，失败则返回 400/500，配置**不会**保存到磁盘
- 前端 Save 按钮显示错误，用户修正后再 Save
- 这保证了磁盘配置和运行时 Provider 始终一致

### 3. 只有 model 变、provider 没变

Builder 会完整重建 Provider，这是正确的。即使 provider 配置相同但 model 变了（如从 `claude-opus-4` 切换到 `claude-sonnet-4`），`BuildModel` 会创建新的 `LanguageModel` 实例指向新模型。

### 4. Fallback provider 变化

Builder 复用 `pantheonadapter.BuildFallbackModel()`，fallback 链的增删改都会被正确反映。

### 5. Auxiliary provider 变化

Builder 同样重建 `AuxProvider`，vision/judge 等辅助功能使用新模型。

## 测试策略

1. **api 层单元测试**：扩展 `handlers_config_test.go`，验证 `handleConfigPut` 成功后 `s.deps.Load().Provider` 被更新（可用 mock builder）
2. **api 层失败测试**：验证 builder 返回 error 时，配置不保存、deps 不变
3. **cli 层单元测试**：新增 `engine_deps_rebuild_test.go`，验证 `BuildProviderDeps` 能从新配置正确重建 Provider/AuxProvider
4. **集成测试**：修改 config 后发送对话请求，验证响应来自新模型（可用 mock provider 或 test double）

## 前端改动

**无前端改动。** 现有 Save 流程和 `PUT /api/config` API 完全不变。`OKResponse` 格式不变（`{ok: true}`）。如果 builder 失败，`handleConfigPut` 返回非 200，前端现有的错误处理会显示错误消息。

## 后续迭代

- **Phase 2**：扩展 builder 重建 `SkillsEvolver` + `SkillsRetriever`，让 skills 功能也使用新模型
- **Phase 3**：评估 `MemProvider` 的重建成本和副作用，决定是否纳入热重载范围
- **Phase 4**：前端增加 "Test Provider" 按钮，在 Save 前验证配置有效性
