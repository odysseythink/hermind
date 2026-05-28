# MCP Hypervisor (Go) 设计

**Date**: 2026-05-26
**Status**: Draft
**Author**: brainstorming session
**Scope**: backend 复刻 Node `server/utils/MCP/`，与 Node-parity + 1 个 Go 私有 tool-call REST 代理

---

## 1. 目标与边界

### 1.1 目标

在 backend 中复刻 Node `server/utils/MCP/`，使其作为独立子系统提供：

- 与 Node 共享的 `STORAGE_DIR/plugins/hermind_mcp_servers.json` 配置（双进程部署可热切换）
- stdio / streamable HTTP / SSE 三种 transport，连接超时 30s，PID 跟踪
- 5 个 Node-parity Admin REST + 1 个 Go 私有 tool-call 代理
- 进程级单例 + goroutine 安全
- 可插拔的工具发布接口（为未来 Go agent 框架预留 `ActiveServers()` / `ToolsAsPlugins()`）

### 1.2 非目标

- 不实现 aibitat agent 注入（待 Go agent 框架就绪）
- 不实现 `@@mcp_xxx` 命名约定的 plugin 调度（agent 层职责）
- v1 不把 MCP 工具暴露成 OpenAI function-calling tool 列表（Phase 2）

---

## 2. Node 行为速览

| 维度 | Node 实现 |
|---|---|
| 类层级 | `MCPHypervisor`（底层 process/connection）← `MCPCompatibilityLayer`（业务封装，singleton） |
| 配置文件 | `STORAGE_DIR/plugins/hermind_mcp_servers.json`，结构 `{ mcpServers: { name: { command/args/env / url/headers / type, hermind: { autoStart, suppressedTools[] } } } }` |
| Transport 选择 | `type ∈ {sse, streamable, http}` → HTTP；否则有 `command` → stdio；否则有 `url` → HTTP/SSE 默认 |
| 启动 | 异步并发逐个启动，30s 超时，失败入 `mcpLoadingResults[name]` |
| 列表 | `servers()` → `[{name, config, running, tools, error, process:{pid}}]`，对每个 server `ping` 后 `listTools` |
| Toggle | 在线则 SIGTERM + transport.close，离线则启动 |
| Delete | toggle off + 从配置文件移除 |
| Tool suppression | `config.hermind.suppressedTools[]`，写回 JSON |
| 子进程 env | `patchShellEnvironmentPath()`（执行 `$SHELL -ic env`），Docker 下硬编码 fallback |
| Agent 集成 | `activeMCPServers()` 返回 `@@mcp_{name}[]`；`convertServerToolsToPlugins(name)` 把每个工具包装成 aibitat function |

---

## 3. Go 架构

### 3.1 包布局

```
backend/internal/
├── mcp/                        # 新增包：MCP 子系统
│   ├── hypervisor.go           # Hypervisor 类型 + 生命周期
│   ├── config.go               # JSON config 读写（Node 兼容）
│   ├── transport.go            # Transport 接口 + 工厂
│   ├── transport_stdio.go      # stdio 子进程 transport
│   ├── transport_http.go       # streamable HTTP transport
│   ├── transport_sse.go        # SSE transport
│   ├── shell_env.go            # patchShellEnvironmentPath 等价物
│   ├── singleton.go            # 进程级单例
│   ├── plugins.go              # ToolPlugin 接口（Phase 预留）
│   └── mcp_test.go             # 集成测试（用 echo MCP fixture）
├── services/
│   └── mcp_service.go          # 业务封装层：暴露给 handler 的 thin facade
└── handlers/
    └── mcp.go                  # 替换现有 stub：5 admin + 1 tool-call
```

> 拆 `hypervisor.go` 与 `services/mcp_service.go` 的原因：handler/test 只依赖 service interface，便于未来在 agent 模块中以相同 interface 注入。

### 3.2 核心类型

```go
// mcp/hypervisor.go
type Hypervisor struct {
    mu         sync.RWMutex
    cfgMu      sync.Mutex            // 文件 IO 串行化
    cfg        *config.Config
    configPath string
    mcps       map[string]*Client    // 在跑的 MCP 客户端
    results    map[string]LoadResult // 启动结果
    log        *mlog.Logger
}

type LoadResult struct {
    Status  string // "success" | "failed"
    Message string
}

type Client struct {
    Name      string
    Transport Transport
    Process   *ProcessInfo          // stdio only
    closeOnce sync.Once
}

type ProcessInfo struct {
    PID int    `json:"pid"`
    Cmd string `json:"cmd,omitempty"`
}

type ServerConfig struct {
    Name        string              `json:"-"`
    Command     string              `json:"command,omitempty"`
    Args        []string            `json:"args,omitempty"`
    Env         map[string]string   `json:"env,omitempty"`
    URL         string              `json:"url,omitempty"`
    Type        string              `json:"type,omitempty"` // sse|streamable|http
    Headers     map[string]string   `json:"headers,omitempty"`
    Hermind *HermindOptions `json:"hermind,omitempty"`
}

type HermindOptions struct {
    AutoStart       *bool    `json:"autoStart,omitempty"`
    SuppressedTools []string `json:"suppressedTools,omitempty"`
}

type ServerView struct {                  // 返回给 frontend
    Name    string        `json:"name"`
    Config  *ServerConfig `json:"config"`
    Running bool          `json:"running"`
    Tools   []ToolSchema  `json:"tools"`
    Error   *string       `json:"error"`
    Process *ProcessInfo  `json:"process"`
}

type ToolSchema struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    InputSchema json.RawMessage `json:"inputSchema"` // 透传 JSON Schema
}
```

### 3.3 Transport 抽象

```go
// mcp/transport.go
type Transport interface {
    Connect(ctx context.Context) error
    Close() error
    Ping(ctx context.Context) bool
    ListTools(ctx context.Context) ([]ToolSchema, error)
    CallTool(ctx context.Context, name string, args map[string]any) (any, error)
    ProcessInfo() *ProcessInfo // stdio 才返回非空
}

func newTransport(srv *ServerConfig) (Transport, error) {
    switch parseServerType(srv) {
    case "stdio":
        return newStdioTransport(srv)
    case "http":
        return newHTTPTransport(srv)
    case "sse":
        return newSSETransport(srv)
    }
    return nil, ErrInvalidServerType
}
```

> 不直接复用某个 Go MCP SDK 的 client 类型，而是在 transport 层包一层适配：
> - Node SDK 和 Go SDK 接口形状不同
> - 加 Transport interface 后测试可 mock
> - 但 transport 内部仍调用底层 SDK（候选见 §6）

### 3.4 单例

```go
// mcp/singleton.go
var (
    instance *Hypervisor
    once     sync.Once
)

func Instance(cfg *config.Config) *Hypervisor {
    once.Do(func() { instance = newHypervisor(cfg) })
    return instance
}
```

> 比 Node 单例更安全：`sync.Once` 保证只初始化一次。`Boot()` 不在 `Instance()` 中执行（与 Node 一致），由 caller 在 main.go 启动时显式 fire-and-forget。

### 3.5 Service facade

```go
// services/mcp_service.go
type MCPService struct{ hv *mcp.Hypervisor }

func (s *MCPService) Servers(ctx context.Context) ([]mcp.ServerView, error)
func (s *MCPService) Reload(ctx context.Context) ([]mcp.ServerView, error)
func (s *MCPService) ToggleServer(ctx context.Context, name string) (bool, error)
func (s *MCPService) DeleteServer(ctx context.Context, name string) (bool, error)
func (s *MCPService) ToggleTool(ctx context.Context, serverName, toolName string, enabled bool) ([]string, error)

// Phase 1.5：tool-call 代理
func (s *MCPService) CallTool(ctx context.Context, serverName, toolName string, args map[string]any) (any, error)

// Phase 2 预留（agent 接入）
func (s *MCPService) ActiveServers(ctx context.Context) ([]string, error)        // 返回 @@mcp_{name}
func (s *MCPService) ToolsAsPlugins(ctx context.Context, name string) ([]mcp.ToolPlugin, error)
```

---

## 4. 关键实现细节

### 4.1 子进程 env 移植（stdio 关键路径）

Node `patchShellEnvironmentPath` 执行 `$SHELL -ic env` 拿 PATH/NODE_PATH。Go 等价物：

```go
// mcp/shell_env.go
func patchShellEnv(ctx context.Context) (map[string]string, error) {
    shell := os.Getenv("SHELL")
    if shell == "" {
        return osEnvMap(), nil
    }
    cmd := exec.CommandContext(ctx, shell, "-ic", "env")
    out, err := cmd.Output()
    if err != nil {
        return osEnvMap(), nil // fallback，与 Node 一致
    }
    return parseEnvOutput(out), nil
}

func buildMCPServerEnv(srv *ServerConfig) map[string]string {
    base := patchShellEnv(timeoutCtx(5 * time.Second))
    if runtime.GOOS != "windows" && os.Getenv("HERMIND_RUNTIME") == "docker" {
        // Node 行为：Docker 下硬编码 PATH/NODE_PATH 默认，再让 shellEnv 覆盖
    }
    return mergeEnv(base, srv.Env)
}
```

> Windows 上没有 `-ic`；通过 `runtime.GOOS == "windows"` 跳过 shell sniffing，直接用 `os.Environ()`。

### 4.2 stdio transport

- 用 `os/exec.CommandContext` 启动，`cmd.Env = envSlice`
- `cmd.Stdin = w`, `cmd.Stdout = r` 双向管道
- 把 reader/writer 喂给 MCP SDK 的 stdio transport
- `cmd.Process.Pid` 暴露到 `ProcessInfo`
- `Close()` 先 `cmd.Process.Signal(syscall.SIGTERM)`，3s 内未退出再 `Kill()`
- main.go 退出钩子调 `hv.PruneAll()`

### 4.3 HTTP / SSE transport

- Streamable HTTP：标准 MCP HTTP 协议，POST JSON-RPC 到 `server.url`，请求头携带 `srv.Headers`
- SSE：先 GET event-stream 拿 endpoint URL，再 POST 到该 URL
- 复用 `net/http.Client`，每个 server 独立 client（cookie/header 隔离）
- 关键：URL 解析失败要在 `validateServerDefinition` 阶段拦下，不在 connect 时报错

### 4.4 连接超时

```go
ctx, cancel := context.WithTimeout(parent, 30*time.Second)
defer cancel()
if err := transport.Connect(ctx); err != nil { ... }
```

与 Node 一致，不需要单独的 `Promise.race`。

### 4.5 配置文件并发写

读多写少。`hv.mu` 保护内存状态；JSON 文件 IO 用单独的 `cfgMu` 串行化（toggle-tool / delete / 写 hermind.suppressedTools 等）：

写入用 `os.WriteFile(tmp, ...) → os.Rename(tmp, final)` 原子替换。

### 4.6 工具压制名单

对齐 Node：写在 `mcpServers[name].hermind.suppressedTools[]`，listTools 时过滤。`ToolsAsPlugins()` 过滤后再产出 plugin。

---

## 5. REST API

### 5.1 5 个 Node-parity 路由（Admin 全量）

| Method | Path | Handler | 鉴权 |
|---|---|---|---|
| GET  | `/api/mcp-servers/list` | ListServers | `validatedRequest + admin` |
| GET  | `/api/mcp-servers/force-reload` | ForceReload | `validatedRequest + admin` |
| POST | `/api/mcp-servers/toggle` | ToggleServer (`{name}`) | `validatedRequest + admin` |
| POST | `/api/mcp-servers/delete` | DeleteServer (`{name}`) | `validatedRequest + admin` |
| POST | `/api/mcp-servers/toggle-tool` | ToggleTool (`{serverName, toolName, enabled}`) | `validatedRequest + admin` |

响应体严格对齐 Node JSON 字段（`success / error / servers / suppressedTools`），frontend 不需要改。

### 5.2 新增 1 个 Go 私有路由

| Method | Path | Body | 鉴权 |
|---|---|---|---|
| POST | `/api/mcp/:name/tools/:tool/call` | `{ arguments: {} }` | `validatedRequest + admin`（Phase 2 可改为按 workspace 权限） |

响应 `{ success, result, error }`，`result` 透传 MCP server 原始返回。

> 选择放在 `/api/mcp/...` 而非 `/api/mcp-servers/...`，明确标识"backend 私有扩展，非 Node-compat"。

---

## 6. Go MCP SDK 选型（决策点）

| 候选 | import path | 优势 | 风险 |
|---|---|---|---|
| 官方 Go SDK | `github.com/modelcontextprotocol/go-sdk` | 协议同步最及时；社区背书 | 较新，API 可能未稳定 |
| 社区 mcp-go | `github.com/mark3labs/mcp-go` | 成熟，stdio/HTTP/SSE 全支持 | 第三方维护，跟进协议更新有延迟 |

**推荐**：优先尝试官方 SDK；如其 client 侧 API 不完整或不稳，回退到 mark3labs/mcp-go。决策应在 PR-A 开工前的 1-2 小时 spike（写 `transport_stdio.go` 的 echo 测试）后定下。

> 不论选哪个，都把它包在 `mcp/transport_*.go` 内，业务代码只依赖我们的 `Transport` interface —— 后期换库零成本。

---

## 7. 存储路径

```go
// Node 兼容路径
func (h *Hypervisor) configFilePath() string {
    return filepath.Join(h.cfg.StorageDir, "plugins", "hermind_mcp_servers.json")
}
```

首次启动：

```go
if !fileExists(path) {
    os.MkdirAll(filepath.Dir(path), 0755)
    os.WriteFile(path, []byte(`{"mcpServers":{}}`), 0644)
}
```

**双进程共享**：Node + Go 同时跑时（迁移期）指向同一 STORAGE_DIR，配置文件共享。Toggle 由 Go 写后，Node 端要靠 force-reload 才能感知 —— 与 Node 当前的单进程行为一致。

---

## 8. 分相位交付

| PR | 范围 | 估时 |
|---|---|---|
| **PR-A**：骨架 + REST ✅ **已完成** | `mcp` 包结构、config 读写、`MCPService` interface、5 个 admin route + 1 个 Go-private tool-call proxy（均返回 `error: "MCP transport not implemented"`），handler 完整测试（≥ 12 个），mcp 包单元测试（≥ 30 个） | 1 天 |
| **PR-B**：stdio transport | stdio + shell env patch + 进程生命周期 + connect timeout + PID 跟踪 + 用 echo MCP server 做集成测试 | 3-4 天 |
| **PR-C**：HTTP/SSE transport | streamable HTTP + SSE 两个 transport | 2-3 天 |
| **PR-D**：tool-call REST 代理 | `POST /api/mcp/:name/tools/:tool/call` + 错误码（404/422/502）+ 集成测试 | 1 天 |
| **PR-E**：plugin 接口预留 | `ToolPlugin` interface + `ActiveServers()` + `ToolsAsPlugins()`；为 Go agent 框架预留接入点 | 1 天 |

**合计 8-11 个工作日**。PR-B / PR-C 可并行；其余顺序。

---

## 9. 测试策略

| 层 | 工具 | 用例 |
|---|---|---|
| Unit | testify | config 读写、tool suppression、parseServerType、validateServerDefinition |
| Transport | 起 echo MCP server 子进程 | stdio：listTools/callTool/ping/kill |
| Transport | httptest | HTTP：模拟 streamable JSON-RPC 端点 |
| Transport | httptest + manual SSE | SSE：模拟 endpoint discovery + JSON-RPC roundtrip |
| Service | testify + sqlite | toggleServerStatus 状态机、reload 幂等性 |
| Handler | gin httptest + apiKeyAuth | 5 admin route 完整往返，错误格式严格匹配 Node |

**关键集成测试**：用 npm-installed 的 `@modelcontextprotocol/server-everything` 作 fixture；CI 跑 `make test-mcp`。如担心 npm 依赖，可写一个 100 行的 Go echo MCP 自带 fixture。

---

## 10. 风险与开放问题

### 10.1 风险

| 风险 | 缓解 |
|---|---|
| Go MCP SDK 协议跟进慢于 TS SDK | Transport interface 隔离；需要时自行实现 JSON-RPC 层 |
| Docker 镜像里 `npx`/`uvx` 不可用 → stdio MCP 启动失败 | 文档说明依赖；Dockerfile 预装 Node + Python（与 Node 镜像一致） |
| `$SHELL -ic env` 在 alpine/scratch 镜像里没有交互 shell → PATH 缺失 | Docker 分支硬编码 fallback PATH（同 Node） |
| ~~Go 子进程 SIGTERM 不优雅退出~~ | ~~3s grace + SIGKILL（比 Node 的 SIGTERM-only 更稳）~~ ✅ PR-B 已实现 |
| 双进程共享配置文件读到旧值 | toggle/delete 完后总是重新读盘，不缓存 |
| 工具调用 args/result 含二进制或 BigInt | `any` + `json.RawMessage` 透传；Node 的 `returnMCPResult` 兜底逻辑用 `json.Marshal` 替代，失败时返回 `"[Unserializable]"` |
| 单例 + 测试隔离 | 提供 `mcp.ResetForTesting()`（build tag `testing`），避免污染 |

### 10.2 开放问题

1. **MCP 工具是否要在 chat 流程里直接被调用（不经 agent）**？  
   → Phase 1.5 的 `/api/mcp/:name/tools/:tool/call` 已能支撑这条 path；后续 chat handler 可在 system prompt 里塞工具描述并执行 function call，但是 chat 层职责，不在 MCP 包内。

2. **`hermind.autoStart=false` 的服务什么时候启动**？  
   → Node：永不自动启，必须显式 toggle on。Go 保持一致。

3. **如何让 frontend 看到 MCP 工具的实时 stdout/stderr**？  
   → Node 是 console.log；Go 通过 `mlog` 走统一日志即可。如需 SSE 实时推到前端，是后续增强。

---

## 11. 进度

- ✅ **PR-A**（2026-05-26）：骨架 + REST + handler + service facade
- ✅ **PR-B**（2026-05-26）：stdio transport + shell env + 进程生命周期 + graceful shutdown
- ✅ **PR-C**（2026-05-26）：HTTP/SSE transport（streamable HTTP + SSE，完整 e2e 测试）
- ✅ **PR-D**（2026-05-26）：tool-call REST 代理生产级加固
- ✅ **PR-E**（2026-05-26）：plugin 预留接口（ToolPlugin + ActiveServers + ToolsAsPlugins）

## 12. Delivered PRs

| PR | 日期 | 一句话总结 |
|---|---|---|
| PR-A | 2026-05-26 | 骨架 + REST + handler + service facade |
| PR-B | 2026-05-26 | stdio transport + shell env + 进程生命周期 + graceful shutdown |
| PR-C | 2026-05-26 | HTTP/SSE transport（streamable HTTP + SSE，完整 e2e 测试，Windows 清理修复） |
| PR-D | 2026-05-26 | tool-call REST 代理生产级加固（10 个错误码、schema 校验、并发限制、审计日志） |
| PR-E | 2026-05-26 | agent plugin 预留接口（ToolPlugin 类型 + ActiveServers/ToolsAsPlugins） |

## 13. PR-D delivered: tool-call hardening

PR-D 将 `POST /api/mcp/:name/tools/:tool/call` 从「可用」提升到「可上生产」。新增 6 项能力：

1. **Stable error codes** — 10 个稳定错误码（`INVALID_BODY`、`INVALID_PARAMS`、`SERVER_NOT_FOUND`、`TOOL_NOT_FOUND`、`ARGS_SCHEMA_MISMATCH`、`BODY_TOO_LARGE`、`CONCURRENCY_LIMIT`、`CALL_TIMEOUT`、`TRANSPORT_ERROR`、`INTERNAL_ERROR`），每个映射到明确的 HTTP 状态，客户端可程序化重试。
2. **Tool-schema cache** — Hypervisor 在 `Connect` 成功后立即缓存 `tools/list`，`Servers()` 不再每次 RPC 重新枚举；`GetToolSchema` 支持 O(1) 级本地查找。
3. **Input-schema validation** — 使用 `gojsonschema` 对 `arguments` 进行 JSON Schema 校验，失败时聚合全部不匹配项返回 `422 ARGS_SCHEMA_MISMATCH`。
4. **Per-call timeout** — 默认 30s，支持 `?timeout=30s` 查询参数覆盖，边界 `[1s, 300s]`；超时返回 `504 CALL_TIMEOUT`。
5. **Body-size cap** — 硬限制 10 MiB，超出返回 `413 BODY_TOO_LARGE`。
6. **Per-server concurrency limit** — 默认每服务器 4 个并发调用，支持 `hermind.maxConcurrency` 覆盖；超限返回 `429 CONCURRENCY_LIMIT`，不排队、不阻塞。
7. **Audit logging** — 每次工具调用（成功/失败/被拒）异步写入 `event_logs`，包含 `server`、`tool`、`duration_ms`、`error_code`；写失败不阻塞调用。

### 配置示例

```json
{
  "mcpServers": {
    "heavy-server": {
      "command": "node",
      "args": ["heavy.js"],
      "hermind": {
        "maxConcurrency": 1,
        "autoStart": true
      }
    }
  }
}
```

环境变量：
- `MCP_CALL_TIMEOUT_DEFAULT`（默认 `30s`）
- `MCP_CALL_CONCURRENCY_PER_SERVER`（默认 `4`）

## 14. 下一步

Phase 1 全部完成。后续方向：

1. **Go agent 框架集成** — 消费 PR-E 的 `ToolPlugin` / `ActiveServers` / `ToolsAsPlugins` 接口，实现 aibitat 等价的 Go agent 插件解析器。
2. **MCP 工具直接注入 chat 流程** — 不经 agent，在 chat handler 的 system prompt 中直接塞入 MCP 工具描述并执行 function call。
3. **前端实时 stdout/stderr** — SSE 推到前端，替代 console.log。
4. **OpenAI function-calling 格式转换** — 将 `ToolSchema` 转换为 OpenAI functions 列表（Phase 2）。
