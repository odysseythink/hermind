# 工具开关集成到技能设置页设计

## 目标

在现有技能设置页面中，新增一个"工具"面板，列出系统中所有已注册的工具（如 file、web、terminal、browser、memory、chart、vision、MCP 等），并为每个工具提供独立的启用/禁用开关。禁用后的工具不再暴露给 LLM（不进入 function definitions）。

## 约束

- 最小侵入：复用现有 `SkillsSection` UI 结构和交互模式
- 持久化：工具开关状态保存在 `config.yaml` 中，重启后保持
- 即时生效：Save 后新对话立即生效（利用刚完成的 hot-reload 机制）
- 向后兼容：未配置时不禁用任何工具（全部默认启用）
- 工具注册表保持完整：`GET /api/tools` 仍需看到所有工具（包括禁用的），以便用户重新启用

## 当前状态

- 工具在 `BuildEngineDeps` 中注册到 `tool.Registry`，启动后不可变
- `GET /api/tools` 返回空列表（stub）
- 配置中无 `tools` 字段，工具没有禁用机制
- `SkillsSection` 只管理 `skills` 配置段，通过 `props.onField` 回调修改
- `agent/conversation.go` 中 `e.tools.Definitions(nil)` 暴露所有工具给 LLM
- `toolselector.Select`（pantheon 包）内部使用 `reg.Entries(nil)` 和 `reg.Definitions(filter)`

## 方案：动态过滤 + SkillsSection 扩展

### 核心思路

1. **配置层**：新增 `tools.disabled` 列表，保存禁用的工具名
2. **API 层**：`GET /api/tools` 返回所有注册工具（含禁用状态）；`PUT /api/config` 保存后通过 hot-reload 立即生效
3. **Engine 层**：创建 Engine 时，从完整的 `ToolReg` 复制出一个**过滤后的 Registry**（只包含启用的工具），传给 `agent.NewEngineWithToolsAndAux`
4. **前端层**：`SkillsSection` 扩展接口，新增工具面板，调用 `GET /api/tools`

**为什么创建过滤后的 Registry 而不是修改 Registry 本身：**
- `tool.Registry`（pantheon）没有 `Unregister` 方法
- `toolselector.Select`（pantheon）内部调用 `reg.Entries(nil)`，无法从外部注入禁用列表
- 保持 `ToolReg` 完整，API 可以列出所有工具；Engine 使用过滤后的副本

### 数据流

```
用户开关工具 → SkillsSection 调用 onConfigField('tools', 'disabled', [...])
  │
  ▼
前端 PUT /api/config { tools: { disabled: [...] }, ... }
  │
  ▼
handleConfigPut:
  1. 解析配置
  2. rebuildDeps(ctx, &updated) → 更新 Provider
  3. *s.opts.Config = updated → 内存配置已更新（含 tools.disabled）
  4. SaveToPath → 持久化到磁盘
  │
  ▼
新对话请求:
  handleConversationPost / RunTurn:
    s.activeToolReg() → 从 s.opts.Config.Tools.Disabled 动态读取
                        复制 ToolReg，过滤掉禁用工具
                        传给 agent.NewEngineWithToolsAndAux
  │
  ▼
agent/conversation.go:
    e.tools.Definitions(nil) → 只包含启用的工具（因为 Registry 已经是过滤后的）
    toolselector.Select → 同样只处理启用的工具
```

### 后端改动

#### 1. config/config.go — 新增 ToolsConfig

```go
// 在 Config struct 中新增：
Tools ToolsConfig `yaml:"tools,omitempty"`

type ToolsConfig struct {
	Disabled []string `yaml:"disabled,omitempty"`
}
```

#### 2. config/descriptor/tools.go — 新增描述符

```go
package descriptor

func init() {
	Register(Section{
		Key:     "tools",
		Label:   "Tools",
		Summary: "Enable or disable system tools. Disabled tools are hidden from the LLM.",
		GroupID: "skills",
		Shape:   ShapeMap,
		Fields: []FieldSpec{
			{
				Name:  "disabled",
				Label: "Disabled tools",
				Help:  "Tools listed here are not exposed to the LLM.",
				Kind:  FieldMultiSelect,
			},
		},
	})
}
```

#### 3. api/dto.go — 扩展 ToolDTO

```go
type ToolDTO struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Toolset     string `json:"toolset,omitempty"`
	Enabled     bool   `json:"enabled"`
}
```

#### 4. api/server.go — 新增辅助方法

在 `Server` 上新增两个私有方法：

```go
// disabledTools 从当前 Config 构建禁用工具集合。
// 由于 Config 在 handleConfigPut 中被原子更新，此方法始终返回最新状态。
func (s *Server) disabledTools() map[string]bool {
	m := make(map[string]bool)
	for _, name := range s.opts.Config.Tools.Disabled {
		m[name] = true
	}
	return m
}

// activeToolReg 返回一个只包含启用工具的 Registry 副本。
// 每次调用都创建新副本（Registry 很小，开销可忽略）。
func (s *Server) activeToolReg() *tool.Registry {
	deps := s.currentDeps()
	if deps.ToolReg == nil {
		return nil
	}
	disabled := s.disabledTools()
	active := tool.NewRegistry()
	for _, e := range deps.ToolReg.Entries(nil) {
		if !disabled[e.Name] {
			active.Register(e)
		}
	}
	return active
}
```

#### 5. api/handlers_tools.go — 返回实际工具列表

```go
func (s *Server) handleToolsList(w http.ResponseWriter, _ *http.Request) {
	deps := s.currentDeps()
	if deps.ToolReg == nil {
		writeJSON(w, ToolsResponse{Tools: []ToolDTO{}})
		return
	}

	disabled := s.disabledTools()
	entries := deps.ToolReg.Entries(nil)
	out := make([]ToolDTO, 0, len(entries))
	for _, e := range entries {
		out = append(out, ToolDTO{
			Name:        e.Name,
			Description: e.Description,
			Toolset:     e.Toolset,
			Enabled:     !disabled[e.Name],
		})
	}
	// 按名称排序，保证列表稳定
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, ToolsResponse{Tools: out})
}
```

#### 6. api/handlers_conversation.go — 使用过滤后的 Registry

```go
	eng := agent.NewEngineWithToolsAndAux(
		s.currentDeps().Provider, s.currentDeps().AuxProvider, s.currentDeps().Storage,
		s.activeToolReg(), s.currentDeps().AgentCfg, s.currentDeps().Platform,
	)
```

#### 7. api/server.go RunTurn — 使用过滤后的 Registry

```go
	eng := agent.NewEngineWithToolsAndAux(
		deps.Provider, deps.AuxProvider, deps.Storage,
		s.activeToolReg(), deps.AgentCfg, deps.Platform,
	)
```

### 前端改动

#### 8. web/src/components/groups/skills/SkillsSection.tsx — 扩展接口和新增面板

**Props 扩展：**

```tsx
export interface SkillsSectionProps {
  section: ConfigSectionT;
  value: Record<string, unknown>;
  originalValue: Record<string, unknown>;
  onField: (field: string, value: unknown) => void;
  onSectionField?: (sectionKey: string, field: string, value: unknown) => void; // 新增
  config?: Record<string, unknown>;
}
```

**工具面板逻辑：**

```tsx
// 在组件内部新增工具状态管理
type ToolFetchState =
  | { status: 'loading' }
  | { status: 'ok'; tools: { name: string; description: string; toolset: string; enabled: boolean }[] }
  | { status: 'error'; message: string };

// 工具开关回调
function toggleTool(name: string, nextEnabled: boolean) {
  if (!props.onSectionField) return;
  const toolsCfg = (props.config?.tools as Record<string, unknown> | undefined) ?? {};
  const cur = (toolsCfg.disabled as string[] | undefined) ?? [];
  const next = nextEnabled
    ? cur.filter(n => n !== name)
    : [...cur.filter(n => n !== name), name].sort();
  props.onSectionField('tools', 'disabled', next);
}
```

**UI 结构：** 在现有技能列表面板下方新增一个"工具"面板，样式与技能列表一致。

#### 9. web/src/components/shell/SettingsPanel.tsx — 传递 onConfigField

```tsx
<SkillsSection
  section={section}
  value={value ?? {}}
  originalValue={original ?? {}}
  onField={(field, v) => props.onConfigField(section.key, field, v)}
  onSectionField={props.onConfigField}  // 新增
  config={props.config as unknown as Record<string, unknown>}
/>
```

### 边界情况

1. **全部工具禁用**：`activeToolReg()` 返回空 Registry，`agent.NewEngineWithToolsAndAux` 接收 nil 或空 Registry，Engine 以无工具模式运行（与现有 `NewEngine` 行为一致）
2. **禁用列表包含不存在的工具名**： harmless，`Entries` 遍历时不匹配，不会 panic
3. **热重载时工具配置变化**：`handleConfigPut` 更新 `s.opts.Config` 后，`activeToolReg()` 立即反映新配置，无需重建 `EngineDeps`
4. **工具名冲突**：技能和工具的名称空间独立，不会冲突

## 测试策略

1. **api 单元测试**：`handleToolsList` 返回正确列表和 enabled 状态
2. **api 单元测试**：`activeToolReg()` 正确过滤禁用工具
3. **api 集成测试**：`handleConfigPut` 修改 `tools.disabled` 后，`handleToolsList` 立即反映新状态
4. **前端单元测试**：`SkillsSection` 正确渲染工具列表，开关回调传递正确参数

## 文件改动清单

| 文件 | 改动 |
|------|------|
| `config/config.go` | 新增 `ToolsConfig` 和 `Config.Tools` 字段 |
| `config/descriptor/tools.go` | **新建** tools 描述符 |
| `api/dto.go` | 扩展 `ToolDTO`（+Toolset, +Enabled） |
| `api/server.go` | 新增 `disabledTools()` 和 `activeToolReg()`；修改 `RunTurn` 使用 `activeToolReg()` |
| `api/handlers_tools.go` | 实现 `handleToolsList`，返回实际工具列表 |
| `api/handlers_conversation.go` | 修改 `handleConversationPost` 使用 `activeToolReg()` |
| `web/src/components/groups/skills/SkillsSection.tsx` | 扩展 Props，新增工具面板状态和 UI |
| `web/src/components/shell/SettingsPanel.tsx` | 传递 `onSectionField={props.onConfigField}` |

## 后续迭代

- 在 ToolDTO 中增加 `Emoji` 字段，让工具列表更直观
- 按 `Toolset` 分组显示工具（如"文件"、"网络"、"终端"等）
- 增加工具使用统计（调用次数、成功率），帮助用户决定禁用哪些工具
