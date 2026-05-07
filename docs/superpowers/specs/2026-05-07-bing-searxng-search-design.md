# Bing & SearXNG 搜索供应商设计文档

## 背景与动机

中国大陆网络环境下，现有搜索供应商存在可访问性问题：
- **Tavily / Brave / Exa**：均为海外 API，中国大陆访问不稳定
- **DuckDuckGo**：虽然免 API key，但在中国大陆可访问性一般，且依赖 HTML 爬虫（实验性）

用户希望引入**必应（Bing）**作为搜索供应商，因为 Bing 在中国大陆可直接访问。同时用户希望提供**免 API key**的方案，降低配置门槛。

## 关键发现：Bing Search API 已退役

Azure Bing Web Search API 已于 **2025 年 8 月 11 日**被微软正式退役（retired）。现在无法在 Azure Portal 创建新的 Bing Search 资源或获取 API key。微软将开发者推向 Azure AI Foundry 的 "Grounding with Bing Search" 服务，但这不是传统搜索 API。

因此，**无法通过官方 Bing API + API Key 的方式接入**。

## 目标

1. 引入 **Bing HTML 爬虫**作为免 key 搜索供应商（类似 DuckDuckGo 模式）
2. 引入 **SearXNG** 作为自托管免 key 搜索供应商（参考 OpenClaw 方案）
3. 两个供应商均遵循现有 `tool/web/SearchProvider` 接口
4. 保持配置简单，UI 描述符自动适配

## 方案对比

| 方案 | 是否需要 API Key | 是否需要部署 | 中国大陆可访问性 | 稳定性 |
|------|-----------------|-------------|-----------------|--------|
| Bing HTML 爬虫 | ❌ 否 | ❌ 否 | ✅ 好 | ⚠️ 实验性（HTML 结构可能变化） |
| SearXNG | ❌ 否 | ✅ 需要 Docker | ✅ 好（自建实例在国内） | ✅ 稳定 |

最终决策：**两个方案同时实现**，由用户根据场景选择。

## 架构设计

### 新增文件

| 文件 | 用途 |
|------|------|
| `tool/web/search_bing.go` | Bing HTML 爬虫供应商 |
| `tool/web/search_searxng.go` | SearXNG JSON API 供应商 |
| `tool/web/search_bing_test.go` | Bing 爬虫单元测试 |
| `tool/web/search_searxng_test.go` | SearXNG 单元测试 |

### 修改文件

| 文件 | 改动 |
|------|------|
| `tool/web/search_dispatcher.go` | 注册 Bing 和 SearXNG，调整 `priorityOrder` |
| `tool/web/register.go` | `Options` 结构增加 `BingMarket` 和 `SearXNGBaseURL` |
| `config/config.go` | `SearchProvidersConfig` 增加 `Bing` 和 `SearXNG` 字段 |
| `config/descriptor/web.go` | 增加 `bing` 和 `searxng` 的 UI 描述符 |
| `cli/engine_deps.go` | 把新配置值传入 `web.Options` |

### 新的 Auto-Priority 顺序

```
tavily → brave → exa → searxng → bing → DuckDuckGo
```

- **付费 API 优先**：Tavily / Brave / Exa 配置了 key 时排最前
- **SearXNG 次之**：用户若配置了自建实例，优先使用（可控、稳定）
- **Bing 爬虫兜底前排**：零配置，中国大陆访问性好，排在 DuckDuckGo 前面
- **DuckDuckGo 最后兜底**：保留现有行为

## Bing HTML 爬虫详细设计

### 爬取策略

- **目标 URL**：`https://www.bing.com/search?q={query}&count={n}`
- **HTTP 方法**：GET
- **请求头**：设置标准桌面浏览器 `User-Agent`，避免被识别为爬虫
- **超时**：复用全局 `httpTimeout = 60s`

### HTML 解析（goquery）

Bing 搜索结果页结构：
- 每个结果在 `.b_algo` 块中
- **标题**：`.b_algo h2 a` 的文本
- **URL**：`.b_algo h2 a` 的 `href`
  - 注意：Bing 有时会将链接包装为 `/search?q=...&u=a1b2...` 的重定向格式，需要解包或直接提取原始目标 URL
- **摘要**：`.b_algo .b_caption p` 或 `.b_algo p`

### 反爬虫与错误处理

| 场景 | 处理方式 |
|------|---------|
| HTTP 非 200 | 返回包装错误，dispatcher 会 fallback 到下一个供应商 |
| CAPTCHA 拦截 | 检测响应内容是否包含 `captcha` 关键文本，或 URL 被重定向到验证页。返回特定错误触发 fallback |
| HTML 结构变化导致解析不到结果 | 返回空切片 `[]SearchResult{}`，不报错（避免阻断 fallback） |
| 网络超时 | 透传 `context.DeadlineExceeded`，dispatcher 处理 |

### 可选配置：`market`

- 支持 `bing.market` 配置项（如 `zh-CN`、`en-US`）
- 传给 Bing 的 `setmkt` 参数：`&setmkt=zh-CN`
- 默认空字符串，不指定市场

### 代码结构

```go
type bingProvider struct {
    client *http.Client
    market string
}

func newBingProvider(market string) *bingProvider
func (p *bingProvider) ID() string      { return "Bing" }
func (p *bingProvider) Configured() bool { return true }
func (p *bingProvider) Search(ctx context.Context, q string, n int) ([]SearchResult, error)
```

## SearXNG 集成详细设计

### API 调用

- **端点**：`{base_url}/search?q={query}&format=json`
- **HTTP 方法**：GET
- **参数**：
  - `q`：搜索词
  - `format=json`：强制返回 JSON（SearXNG 实例需在 `settings.yml` 中开启 `json` 格式支持）

### 响应解析

SearXNG JSON 响应中 `results` 数组每个元素映射：

| SearXNG 字段 | SearchResult 字段 |
|--------------|-------------------|
| `title` | `Title` |
| `url` | `URL` |
| `content` | `Snippet` |
| `publishedDate` | `PublishedDate`（可选） |

### 配置项

```yaml
web:
  search:
    provider: ""           # 留空自动选择
    providers:
      searxng:
        base_url: "http://localhost:8080"  # 必填
```

### `Configured()` 逻辑

```go
func (p *searxngProvider) Configured() bool {
    return p.baseURL != ""
}
```

用户未配置 `base_url` 时，`Configured()` 为 `false`，auto-priority 会跳过它。

### 错误处理

| 场景 | 处理方式 |
|------|---------|
| `base_url` 为空 | `Configured() = false`，不参与调度 |
| 连接失败（SearXNG 没启动） | 返回错误，dispatcher fallback 到 Bing/DDG |
| HTTP 非 200 | 返回错误，触发 fallback |
| JSON 解析失败 | 返回错误，触发 fallback |

### 代码结构

```go
type searxngProvider struct {
    baseURL string
    client  *http.Client
}

func newSearXNGProvider(baseURL string) *searxngProvider
func (p *searxngProvider) ID() string      { return "SearXNG" }
func (p *searxngProvider) Configured() bool { return p.baseURL != "" }
func (p *searxngProvider) Search(ctx context.Context, q string, n int) ([]SearchResult, error)
```

## 配置与 UI 描述符

### `config/config.go`

```go
type SearchProvidersConfig struct {
    Tavily     ProviderKeyConfig `yaml:"tavily,omitempty"`
    Brave      ProviderKeyConfig `yaml:"brave,omitempty"`
    Exa        ProviderKeyConfig `yaml:"exa,omitempty"`
    DuckDuckGo *DDGProxyConfig   `yaml:"duckduckgo,omitempty"`
    Bing       BingConfig        `yaml:"bing,omitempty"`
    SearXNG    SearXNGConfig     `yaml:"searxng,omitempty"`
}

type BingConfig struct {
    Market string `yaml:"market,omitempty"`
}

type SearXNGConfig struct {
    BaseURL string `yaml:"base_url,omitempty"`
}
```

### `tool/web/register.go` / `Options`

```go
type Options struct {
    SearchProvider  string
    TavilyAPIKey    string
    BraveAPIKey     string
    ExaAPIKey       string
    DDGProxyConfig  *config.DDGProxyConfig
    FirecrawlAPIKey string
    BingMarket      string
    SearXNGBaseURL  string
}
```

### `cli/engine_deps.go`

```go
web.RegisterAll(toolRegistry, web.Options{
    // ... existing fields ...
    BingMarket:     app.Config.Web.Search.Providers.Bing.Market,
    SearXNGBaseURL: app.Config.Web.Search.Providers.SearXNG.BaseURL,
})
```

### `config/descriptor/web.go`

- `search.provider` enum 增加 `"bing"`、`"searxng"`
- 新增字段：
  - `search.providers.bing.market`：文本输入，提示如 `"e.g. zh-CN, en-US. Leave blank for default."`
  - `search.providers.searxng.base_url`：文本输入，提示如 `"e.g. http://localhost:8080"`

## 测试策略

### `search_bing_test.go`

- **正常解析**：本地 test server 返回真实 Bing HTML 片段，验证解析出正确的 `SearchResult` 切片
- **空结果**：返回无 `.b_algo` 的 HTML，验证返回空切片
- **CAPTCHA 拦截**：返回含 `captcha` 关键字的页面，验证返回错误（触发 dispatcher fallback）
- **HTTP 500**：验证返回错误

### `search_searxng_test.go`

- **正常解析**：本地 test server 返回 SearXNG JSON，验证字段映射正确
- **空 results**：返回 `{"results": []}`，验证返回空切片
- **连接失败**：test server 关闭，验证返回错误

### `search_dispatcher_test.go`

- 更新 priority order 断言，验证新顺序包含 `searxng` 和 `bing`

## 风险与降级策略

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| Bing 修改 HTML 结构 | Bing 爬虫解析失败 | 返回空结果，dispatcher 自动 fallback 到 DuckDuckGo |
| Bing 对爬虫 IP 封禁/CAPTCHA | Bing 爬虫不可用 | 返回错误，dispatcher 自动 fallback 到 DuckDuckGo |
| SearXNG 实例未启动 | SearXNG 不可用 | `Configured() = false`，dispatcher 自动跳过 |
| SearXNG 响应慢 | 搜索延迟高 | 复用 60s HTTP 超时，超时时 fallback |

## 参考

- [SearXNG 官方文档](https://docs.searxng.org/)
- OpenClaw SearXNG 集成方案（Issue #43822）
- OpenClaw DuckDuckGo 爬虫实现（docs.openclaw.ai/tools/duckduckgo-search）
- 现有代码：`tool/web/search_ddg.go`、`tool/web/search_tavily.go`
