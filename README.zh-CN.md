# hermind

hermes AI Agent 框架的 Go 实现。单一二进制、目录绑定、每实例一个持久会话。内置 Web UI、REST API、SSE 流式输出、多供应商 LLM 路由、技能系统、记忆回放缓冲区，以及兼容 Anthropic 的 `/v1/messages` 代理端点。

> **状态**：1.0 之前（`v0.3.x`）。schema 与 CLI 接口仍在演进。详见 [CHANGELOG.md](CHANGELOG.md)。

[English](README.md) · [设计说明](DESIGN.md) · [更新日志](CHANGELOG.md)

---

## 它做什么

hermind 在本地运行一个 LLM Agent。在任意工作目录启动它，该目录就成为一个 hermind 实例 —— 配置、对话历史、技能、轨迹日志全部存放在 `./.hermind/` 下。一个目录、一段对话、一份持久状态。

Agent 主循环与具体供应商解耦。把支持的某个供应商（Anthropic、OpenAI、Bedrock、OpenRouter、Qwen、Kimi、DeepSeek、Minimax、Wenxin、Copilot）配为主模型，再可选地配置一个辅助模型，hermind 自动处理工具调用、多轮推理、上下文压缩、回退链路与结构化轨迹日志。

可以这样跑：

- **浏览器聊天 Web 应用**：`hermind web`
- **CLI 单次问答**：`hermind run`
- **后台守护**：定时跑 prompt（`hermind cron`）或对接 IM 平台（Telegram / 飞书 等）（`hermind listen`）
- **Anthropic API 代理**：把 `proxy.enabled` 打开，将 `ANTHROPIC_BASE_URL` 指向 hermind，Claude Code、Cursor、Anthropic 官方 SDK 即可透明地通过 hermind 调用任何已配置的供应商

## 快速开始

```bash
# 编译
go build -o bin/hermind ./cmd/hermind

# 配置一个供应商（写入 ./.hermind/config.yaml）
./bin/hermind auth set anthropic sk-ant-...

# 启动 Web UI（绑定到 127.0.0.1，端口在 [30000,40000) 区间随机选取）
./bin/hermind web
```

在浏览器中打开输出的 URL。首次运行会用合理的默认值生成 `./.hermind/config.yaml`，之后所有设置都可在 Settings 面板中修改。

CLI 单次执行：

```bash
./bin/hermind run "总结最近 5 次提交的变更"
```

## 能力一览

**多供应商 LLM 路由。** 一等公民支持 Anthropic、OpenAI、AWS Bedrock、GitHub Copilot、OpenRouter，以及多家国内供应商（通义千问、Kimi、DeepSeek、MiniMax、文心、智谱）。当某个供应商返回限速/超额错误时，可按配置回退到下一个。

**技能（Skills）。** 从 `<instance>/skills/` 热加载技能包。每轮会话自动注入最多 N 个检索到的相关技能（数量可配）。可选地从每段对话中自动提取新技能片段（`auto_extract`）。记忆强化信号会随着技能库代际演进而衰减（`generation_half_life`）。

**记忆回放缓冲区。** `hermind bench replay {generate,run,judge,report}` 把 `state.db` 中真实的历史用户轮次对当前配置重新跑一遍，过程中使用一个隔离的临时 sqlite，不会污染线上 `state.db`。三种评分模式（`none` / `pairwise` / `rubric+pairwise`）；两种抽取模式（`cold` 不带历史 / `contextual` 带完整历史上下文）。

**Anthropic `/v1/messages` 代理。** 可选启用的传输层代理。hermind 把 Anthropic Messages 请求翻译为内部 provider 抽象、分发到任意已配置的 LLM、再把响应翻译回 Anthropic 格式（包括 SSE 流式与 tool-use 数据块）。当你想让 Claude Code 这种客户端驱动非 Anthropic 模型时非常方便。

**用户在线状态框架。** 后台工作进程的三态（Unknown / Absent / Present）门控。已实现两个信号源：HTTP 空闲（连续 N 秒无入站请求则投 Absent）、睡眠时段（在配置的本地时段内投 Absent）。预留键盘空闲、日历忙碌等扩展点。

**MCP 服务器。** 在 Settings → MCP 中可配置任意数量的 MCP 服务器。其工具调用与原生工具同等地通过对话引擎分发。

**浏览器自动化。** 通过 Browserbase 或 Camofox 提供真实浏览器工具。

**Cron。** 用 YAML 定义定时 prompt（`every 5m`、`every 1h` 等）。每次 cron 运行都是临时性的 —— 写入独立的轨迹文件，绝不污染主对话。

**多平台 IM 网关。** Telegram、飞书适配器（更多在路上）。在 Settings → IM Channels 中配置；hermind 通过长轮询或 webhook 接入各平台，把入站消息路由到对话引擎，并在原话题中回复。

**Bench 工具链。** `hermind bench` 在多组配置预设上做合成数据 A/B 评测，数据生成可复现。回放（见上）通过可插拔的 `Item` 接口共用同一份 runner。

**Web UI。** React + Vite 单页应用，编译进二进制。Settings 面板由 descriptor 驱动 —— `config.yaml` 中的每个字段都可以从浏览器编辑，支持实时 `visible_when` 门控、点号路径嵌套表单，以及国际化（英文 + 简体中文）。

## 项目结构

| 路径 | 内容 |
|------|------|
| `cmd/hermind` | 单一二进制入口 |
| `cli/` | 全部 cobra 子命令（`web`、`run`、`bench`、`replay`、`skills`、`cron`、`listen`…） |
| `agent/` | 对话引擎、批处理 runner、空闲整合器、在线状态框架 |
| `provider/` | LLM 供应商实现（anthropic / openai / bedrock / qwen / …） |
| `tool/` | 内置工具实现（file、web、browser、mcp、memory、terminal、…） |
| `skills/` | 技能加载器与注册表 |
| `replay/` | 记忆回放缓冲区（数据集生成、runner、judge、report） |
| `benchmark/` | 合成数据 A/B 评测工具 |
| `gateway/` | 多平台 IM 适配器 |
| `mcp/` | MCP 服务器/客户端集成 |
| `storage/` | `state.db` 的 SQLite 与 Postgres 后端 |
| `config/` | 配置加载器与驱动 Web UI schema 的 descriptor 注册表 |
| `api/` | HTTP 服务器、REST 端点、SSE 流式、内嵌 Web 资源 |
| `web/` | React + Vite 前端（发版时编译到 `api/webroot/`） |

## 配置

配置文件位于 `./.hermind/config.yaml`。可用 `HERMIND_HOME=/some/dir` 覆盖位置。每个字段在 Web UI 的 Settings 面板都有对应入口 —— 日常使用无需直接编辑 YAML。

敏感值（API key、token）可以写在 `config.yaml` 内，也可通过环境变量提供（`ANTHROPIC_API_KEY`、`OPENAI_API_KEY`、`BROWSERBASE_API_KEY` 等）。加载时环境变量优先于 YAML。

## 开发

```bash
# 后端
go build ./...
go test ./...

# 前端
cd web && npm install && npm run dev   # vite 开发服务器，/api 反代到 127.0.0.1:9119
cd web && npm test -- --run            # vitest 单次运行

# 编译内嵌 SPA
cd web && npm run build                # 输出到 api/webroot/

# 重新生成 config-schema fixture（新增或修改 descriptor 之后）
go test -tags fixture ./api -run TestDumpSchemaFixture
```

`Makefile` 汇总了常用工作流；`flake.nix` 提供基于 Nix 的开发 shell。

## 许可证

详见仓库根目录的 LICENSE。
