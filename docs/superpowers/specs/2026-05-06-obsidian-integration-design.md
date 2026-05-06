# hermind × Obsidian 集成设计文档

## 背景与目标

hermind 是一个本地 LLM Agent 框架，目前支持 Web UI、CLI、IM 网关（Telegram/飞书）和 MCP Server 等多种交互方式，但缺少与笔记工具/编辑器的原生集成。

本设计目标是为 hermind 提供双向 Obsidian 集成：

1. **Obsidian 作为客户端** — 在 Obsidian 内部直接与 hermind 对话，获得流式回复
2. **hermind 能操作 Obsidian Vault** — Agent 可读取、搜索、编辑笔记，理解 front-matter、wikilink、tags 等 Obsidian 语义
3. **对话即笔记** — 聊天记录可保存为 Obsidian 笔记，支持 front-matter 元数据

## 非目标

- Obsidian 插件不管理 hermind 生命周期（不启动/停止 hermind 进程）
- 不实现远程 Vault 访问（仅操作本地文件系统）
- 第一期不实现 Obsidian 双向链接图谱的可视化操作

## 架构概述

```
┌─────────────────────────────────────────────────────────────┐
│                        Obsidian (Electron)                   │
│  ┌──────────────┐  ┌─────────────┐  ┌──────────────────┐   │
│  │ 侧边栏聊天面板 │  │ 命令面板入口 │  │ "保存对话为笔记"  │   │
│  │  (React/TS)   │  │             │  │   (Obsidian API) │   │
│  └──────┬───────┘  └─────────────┘  └──────────────────┘   │
│         │                                                    │
│         │ POST /api/conversation/messages                   │
│         │ GET  /api/sse (SSE 流式响应)                       │
│         └──────────────────────────────────────┐            │
└────────────────────────────────────────────────┼────────────┘
                                                 │
                                                 ▼
┌─────────────────────────────────────────────────────────────┐
│                      hermind (Go, 本地运行)                   │
│  ┌─────────────────┐      ┌──────────────────────────┐     │
│  │   api/server.go │◄────►│   tool/obsidian/ (新增)   │     │
│  │  (现有 REST API)│      │  - read_note             │     │
│  │                 │      │  - write_note            │     │
│  │                 │      │  - search_vault          │     │
│  │                 │      │  - list_links            │     │
│  │                 │      │  - update_frontmatter    │     │
│  │                 │      │  - append_to_note        │     │
│  └─────────────────┘      └──────────────────────────┘     │
│         │                                                    │
│         ▼                                                    │
│  ┌─────────────────┐      ┌──────────────────────────┐     │
│  │   Agent Engine  │◄────►│   Vault (本地文件系统)    │     │
│  │  (LLM + 工具调用) │      │  Markdown + front-matter │     │
│  └─────────────────┘      └──────────────────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

### 核心数据流

1. **用户提问**：在 Obsidian 侧边栏输入问题 → 插件提取当前笔记路径/选中文本 → 通过 `POST /api/conversation/messages` 发送给 hermind
2. **上下文注入**：插件在请求中附加 `context` 字段（vault 路径、当前笔记、选中文本），hermind API 层将其注入对话系统提示
3. **Agent 思考**：LLM 判断是否需要调用 Obsidian 工具（如搜索相关笔记、读取 front-matter）→ 工具操作本地 Vault 文件 → 结果返回给 LLM
4. **流式回复**：hermind 通过 SSE 向插件推送 `message_chunk` → 插件实时渲染
5. **保存对话**：插件前端直接通过 Obsidian API 将聊天记录写入 `.md` 文件（不经过 hermind，避免循环）

### 关键边界

- Obsidian 插件只负责：UI 渲染、网络请求、文件保存
- hermind 只负责：LLM 推理、工具执行、对话管理
- Vault 文件操作：hermind 工具直接读写文件系统，不通过 Obsidian 插件中转

## Obsidian 插件设计

插件代码放在项目 `integrations/obsidian/` 目录下。

### 目录结构

```
integrations/obsidian/
├── manifest.json          # 插件元数据（id, name, version）
├── package.json
├── esbuild.config.mjs
└── src/
    ├── main.ts            # 插件入口：注册视图、命令、设置页
    ├── settings.ts        # 设置页 UI
    ├── api.ts             # hermind REST API 客户端
    ├── sse.ts             # SSE EventSource 封装
    ├── context.ts         # Obsidian 上下文提取（当前笔记、选中文本）
    └── chat/
        ├── ChatView.ts    # 侧边栏视图（继承 ItemView）
        ├── ChatUI.ts      # DOM 渲染（消息列表、输入框、工具调用展示）
        └── types.ts
```

### 核心模块

#### ChatView（侧边栏面板）

- 继承 Obsidian `ItemView`，挂在左侧边栏（和文件浏览器同栏）
- UI 用原生 TypeScript + DOM（不引入 React/Vue，减少打包体积和复杂度）
- 界面元素：
  - 消息列表区（用户气泡右对齐、AI 气泡左对齐、工具调用可折叠）
  - 输入框（底部固定，`Shift+Enter` 换行，`Enter` 发送）
  - 上下文指示器（显示"已附加：当前笔记 xxx.md"）
  - "保存对话"按钮（生成带 front-matter 的 Markdown 笔记）

#### 上下文提取（context.ts）

```typescript
interface ObsidianContext {
  vault_path: string;      // vault 绝对路径
  current_note?: string;   // 当前活动笔记相对路径
  selected_text?: string;  // 编辑器选中文本
  cursor_line?: number;    // 光标所在行号
}
```

- 每次发送消息前自动收集
- 用户可在设置中关闭"自动注入当前笔记"

#### 命令（Commands）

注册三条命令到 Obsidian 命令面板：

- `Open Hermind Chat` — 打开/聚焦侧边栏聊天面板
- `Send Selection to Hermind` — 将当前选中文本作为新问题发送
- `Save Conversation to Note` — 将当前会话保存为 `.md`

#### API 与 SSE（api.ts / sse.ts）

- `POST /api/conversation/messages` 发送消息 + `context` payload
- `GET /api/sse` 建立 EventSource，监听 `message_chunk` / `tool_call` / `tool_result` / `done` / `error`
- 端口发现：设置项填 hermind 地址（默认 `http://127.0.0.1:30000`），支持自动扫描 30000-40000 端口

#### 设置页

- Hermind 服务地址
- 是否自动附加当前笔记上下文
- 对话保存的默认文件夹路径
- 是否在消息中显示工具调用详情（默认折叠）

## hermind Obsidian 工具集设计

新增 `tool/obsidian/` 包，工具集名称为 `obsidian`，在 `cli/engine_deps.go` 中注册。

### 上下文传递机制

Obsidian 插件发送的请求体包含 `context` 字段。API 层需要做两件事：

1. **注入系统提示**：在调用 `eng.RunConversation()` 前，自动生成一条系统消息，告知 LLM 当前 Obsidian 上下文：
   ```
   [Obsidian Context]
   Vault: /Users/xxx/Documents/MyVault
   Active Note: Projects/Idea.md
   Selected Text: 这里是选中的段落...
   Cursor Line: 42
   ```

2. **传递 vault 路径给工具**：通过 `context.WithValue` 将 `vault_path` 注入 Go 上下文，工具 handler 从中读取。

### 工具列表

| 工具名 | 用途 | 关键参数 |
|--------|------|---------|
| `obsidian_read_note` | 读取笔记内容，解析 front-matter | `path`: 笔记相对路径 |
| `obsidian_write_note` | 写入/覆盖笔记 | `path`, `content`, `frontmatter` (可选) |
| `obsidian_search_vault` | 全文搜索 vault | `query`: 搜索关键词 |
| `obsidian_list_links` | 列出笔记的出链和反链 | `path`, `direction`: outgoing/incoming/both |
| `obsidian_update_frontmatter` | 更新 front-matter（标签、属性） | `path`, `updates`: key-value 映射 |
| `obsidian_append_to_note` | 在笔记末尾追加内容 | `path`, `content` |

### 安全边界

- 所有工具操作的路径必须是 **vault 目录的子路径**，禁止 `../` 越界
- 使用 `filepath.Join(vaultPath, path)` + `strings.HasPrefix(resolvedPath, vaultPath)` 做校验
- 工具失败时返回清晰错误，LLM 可重试或换方案
- 写操作前自动备份原文件到 `.hermind/obsidian-backups/`

### 技术实现

- **front-matter 解析**：`yaml.v3`，识别 `---` 分隔的 YAML 块
- **wikilink 解析**：正则 `\[\[([^\]|]+)(?:\|[^\]]+)?\]\]`，提取链接目标
- **tag 解析**：正则 `#([a-zA-Z0-9_\-/]+)`
- **全文搜索**：遍历 vault 下所有 `.md` 文件，基于内容匹配（未来可接入 ripgrep 或 Obsidian 的 cache）

### 工具 Schema 示例（obsidian_read_note）

```json
{
  "name": "obsidian_read_note",
  "description": "Read an Obsidian note from the vault, parsing its front-matter and content.",
  "parameters": {
    "type": "object",
    "properties": {
      "path": { "type": "string", "description": "Relative path to the note within the vault" }
    },
    "required": ["path"]
  }
}
```

## 对话保存机制

完全由 Obsidian 插件前端实现，不经过 hermind API，避免循环依赖。

### 保存触发方式

- 手动：点击聊天面板"保存对话"按钮
- 自动：设置中开启"自动保存对话"，每次 SSE `done` 事件后自动保存

### 保存格式

```markdown
---
title: "对话：关于 Idea.md 的总结"
date: 2026-05-06T10:30:00
tags: [hermind, conversation]
model: anthropic/claude-opus-4-6
message_count: 4
obsidian_context:
  vault_path: /Users/xxx/Documents/MyVault
  current_note: Projects/Idea.md
---

## User
帮我总结一下这篇笔记的核心观点

## Assistant
这篇笔记的核心观点包括...

## User
再详细展开第一点

## Assistant
...
```

### 保存路径

`{vault}/Hermind Conversations/YYYY-MM-DD-HH-MM-SS-{topic}.md`

文件夹和模板可在设置中配置。

## 错误处理

### Obsidian 插件侧

| 场景 | 处理方式 |
|------|---------|
| hermind 未运行 / 端口不通 | 输入框上方显示红色提示条："无法连接到 hermind，请确认 `hermind web` 已运行" |
| SSE 连接中断 | 自动重试 3 次，间隔 2s；仍失败则提示用户手动重连 |
| 发送消息时无响应 | 超时 30s 后提示"hermind 响应超时" |
| 保存对话失败 | Toast 提示具体错误（文件夹不存在等），提供"重试"按钮 |

### hermind 工具侧

| 场景 | 处理方式 |
|------|---------|
| 路径越界 | 返回明确错误：`"path must be within the vault directory"`，LLM 可修正 |
| 笔记不存在 | 返回 `"note not found: Projects/Idea.md"`，LLM 可调用 `search_vault` 查找 |
| front-matter 解析失败 | 返回原始内容 + 警告，不阻断读取 |
| vault 路径未传入 | `CheckFn` 返回不可用，LLM 不会调用该工具 |

## 测试策略

### hermind 工具集（Go 单元测试）

- 每个工具独立测试，使用 `os.MkdirTemp` 创建临时 vault
- 覆盖：正常读写、front-matter 解析、wikilink 提取、路径越界防护、搜索命中/未命中
- 放在 `tool/obsidian/*_test.go`

### Obsidian 插件

- 插件逻辑（API 客户端、SSE 解析、上下文提取）用 Jest 单元测试
- UI 组件测试以 e2e 手动验证为主（Obsidian 插件生态测试基础设施较薄弱）

### 集成测试

- 启动 hermind 实例 → 启动 Obsidian（或模拟插件环境）→ 发送消息 → 验证 SSE 流和工具调用
- 可放在项目 `integration/` 目录下，复用现有的 replay/smoke 测试框架

## 风险与缓解

| 风险 | 缓解措施 |
|------|---------|
| Agent 误操作覆盖重要笔记 | 工具实现中写操作前先备份原文件到 `.hermind/obsidian-backups/` |
| front-matter 格式被写坏 | `write_note` 和 `update_frontmatter` 使用 `yaml.v3` 标准库序列化，保留原有字段 |
| 插件与 hermind 版本不兼容 | manifest.json 中声明 `minAppVersion`，API 层增加版本协商（`/api/status`） |
| 大 vault 搜索性能差 | `search_vault` 初期遍历文件实现，后续可接入 ripgrep 或 SQLite FTS 索引 |
