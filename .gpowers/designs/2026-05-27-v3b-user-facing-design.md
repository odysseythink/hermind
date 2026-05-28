# v3-B — User-Facing Completeness Design (TTS + AgentFlow Executor + Binary File Generation)

**Date**: 2026-05-27
**Status**: Draft
**Author**: brainstorming session
**Scope**: 三块独立但同主题("用户立刻能感知")的功能交付:
1. **TTS** 三个 provider(elevenLabs / openai / openai-generic)+ workspace TTS endpoint
2. **AgentFlow executor** 让 PR-AR-3 投影的 stub flow 真正可执行(api-call / llm-instruction / web-scraping 三种 block)
3. **create-files-agent 二进制格式**(docx / pdf / pptx / xlsx)补全 PR-AR-6 punt 的部分

**Total estimate**: 15-20h(分三块独立子-PR 或一个大 PR,优先级 1 → 2 → 3)。

**先决条件**: v1/v2 主链路就绪(尤其 PR-AR-3 的 agent runtime,因为 AgentFlow executor 跑在 agent tool registry 里)。可以与 v3-A 并行。

---

## 1. 现状盘点

### 1.1 TTS

| 端 | 现状 |
|---|---|
| Node `server/utils/TextToSpeech/` | 3 provider(elevenLabs / openAi / openAiGeneric)+ `native`(浏览器端 Web Speech API,纯客户端) |
| Node endpoint | `GET /workspace/:slug/tts/:chatId` → `TTSProvider.ttsBuffer(text)` 返回 `audio/mpeg` |
| Go `cfg.TTSProvider` | env 已声明 `TTS_PROVIDER`,**default `native`** |
| Go 实装 | **零** — 没有 `internal/tts/` 包,没有 handler,没有 endpoint |

### 1.2 AgentFlow

| 端 | 现状 |
|---|---|
| Node executor | 3 个 block:`api-call.js` / `llm-instruction.js` / `web-scraping.js`;`executor.js` 串接执行 |
| Node 集成 | 通过 `AgentFlows.activeFlowPlugins()` 暴露给 aibitat |
| Go `AgentFlowService` | CRUD only(`ListFlows` / `LoadFlow` / `SaveFlow` / `DeleteFlow`);**无 executor** |
| Go `tools/flow.go` | PR-AR-3 实装了"投影为 tool.Entry",但 Handler 直接返回 `{"status":"deferred","note":"flow execution is not yet implemented"}` |

### 1.3 create-files binary 格式

| 格式 | PR-AR-6 现状 | Go 库候选 |
|---|---|---|
| txt / md | ✅ 已实装(stdlib `os.WriteFile`) | — |
| docx | ❌ 返回 deferred 错误 | `github.com/fumiama/go-docx` (MIT) / `github.com/lukasjarosch/go-docx` (MIT) / unidoc/unioffice (AGPL/Commercial) |
| pdf | ❌ 返回 deferred 错误 | `github.com/jung-kurt/gofpdf` (MIT, archived) / `github.com/go-pdf/fpdf`(MIT, 活跃 fork) |
| pptx | ❌ 返回 deferred 错误 | unioffice (AGPL) — **唯一选择** |
| xlsx | ❌ 返回 deferred 错误 | `github.com/xuri/excelize/v2` (BSD-2) / unioffice |

---

## 2. 目标与边界

### 2.1 目标

- **TTS**:Go 提供 3 个 hosted provider 的 buffer-style 接口(`Synthesize(ctx, text) ([]byte, contentType string, err error)`)+ `GET /api/workspace/:slug/tts/:chatId` endpoint,Node parity
- **AgentFlow executor**:把 stub Handler 换成真执行器,支持 Node 三种 block,共享 `flow.context` 概念(下个 step 能引用上个 step 输出)
- **Binary file generation**:txt/md 之外补 docx / pdf / xlsx 三种(**pptx 显式 punt**,因为 unioffice 是 AGPL,license 顾虑大)

### 2.2 非目标(本 PR 集合)

- **TTS native(浏览器 Web Speech API)**:纯前端;Go 不参与
- **TTS streaming**:elevenLabs 支持流式,本 PR 先做 buffer 模式
- **TTS 缓存层**:相同文本重复请求不缓存,后续可加
- **AgentFlow 可视化编辑器**:前端已经存在,Go 只需 executor + CRUD
- **AgentFlow 嵌套调用**(flow 调 flow):YAGNI
- **AgentFlow 并行 step**:顺序执行就够
- **pptx 格式**:license 风险,等用户明确请求再做
- **PDF 模板/分页/中文字体**:gofpdf 默认只支持西文字体,中文需要 TTF 注册;本 PR 先做西文,中文标注为限制

---

## 3. 子设计 A:TTS

### 3.1 包布局

```
backend/internal/tts/
├── doc.go
├── provider.go          # Provider interface + Synthesis 结果类型
├── factory.go           # NewProvider(cfg, settings) Provider
├── elevenlabs.go        # ElevenLabs API client
├── elevenlabs_test.go
├── openai.go            # OpenAI TTS API client
├── openai_test.go
├── openai_generic.go    # 任意 openai-compat /v1/audio/speech endpoint
├── openai_generic_test.go
├── native.go            # NoopProvider — 占位,前端走 Web Speech API
└── factory_test.go

backend/internal/handlers/
├── tts.go               # GET /api/workspace/:slug/tts/:chatId
└── tts_test.go
```

### 3.2 接口签名

```go
// internal/tts/provider.go

type Synthesis struct {
    Audio       []byte
    ContentType string  // "audio/mpeg" | "audio/wav" | "audio/ogg"
}

type Provider interface {
    Synthesize(ctx context.Context, text string) (*Synthesis, error)
    Available() bool         // 配置完整且 enabled
    Name() string            // "elevenlabs" / "openai" / "openai-generic" / "native"
}

// internal/tts/factory.go
func NewProvider(cfg *config.Config, settings map[string]string) Provider {
    name := pick("TTSProvider", settings, cfg.TTSProvider)
    switch strings.ToLower(name) {
    case "elevenlabs":
        return NewElevenLabsProvider(cfg, settings)
    case "openai":
        return NewOpenAIProvider(cfg, settings)
    case "openai-generic":
        return NewOpenAIGenericProvider(cfg, settings)
    default:
        return &nativeProvider{}  // 前端处理
    }
}
```

### 3.3 各 provider 实现要点

**ElevenLabs**:
- Endpoint:`POST https://api.elevenlabs.io/v1/text-to-speech/{voice_id}`
- Header:`xi-api-key: <key>`,`Content-Type: application/json`
- Body:`{"text":"...","model_id":"eleven_monolingual_v1","voice_settings":{"stability":0.5,"similarity_boost":0.5}}`
- Response:`audio/mpeg` 二进制
- 配置:`ELEVENLABS_API_KEY` + `ELEVENLABS_VOICE_ID`(default `21m00Tcm4TlvDq8ikWAM` — Rachel)

**OpenAI**:
- Endpoint:`POST https://api.openai.com/v1/audio/speech`
- Header:`Authorization: Bearer <key>`
- Body:`{"model":"tts-1","input":"...","voice":"alloy"}`
- Response:`audio/mpeg` 二进制
- 配置:`OPEN_AI_KEY` + `OPEN_AI_TTS_MODEL`(default `tts-1`)+ `OPEN_AI_TTS_VOICE`(default `alloy`)

**OpenAI Generic**:
- 同 OpenAI shape,但 BaseURL 可配
- 配置:`TTS_OPEN_AI_COMPATIBLE_KEY` + `TTS_OPEN_AI_COMPATIBLE_ENDPOINT` + voice/model

### 3.4 Handler

```go
// GET /api/workspace/:slug/tts/:chatId
func (h *TTSHandler) Synthesize(c *gin.Context) {
    chatID := c.Param("chatId")
    chat, err := h.chatSvc.GetChatByID(c.Request.Context(), chatID)
    if err != nil { c.JSON(404, gin.H{"error":"chat not found"}); return }

    text := h.extractAssistantText(chat)  // strip markdown / tags
    if text == "" { c.JSON(422, gin.H{"error":"no text to synthesize"}); return }

    out, err := h.ttsProvider.Synthesize(c.Request.Context(), text)
    if err != nil { c.JSON(500, gin.H{"error":err.Error()}); return }

    c.Data(200, out.ContentType, out.Audio)
}
```

### 3.5 配置 envs

```go
// config.go additions
TTSProvider               string `env:"TTS_PROVIDER" envDefault:"native"`  // 已有

// === ElevenLabs ===
ElevenLabsAPIKey          string `env:"ELEVENLABS_API_KEY"`
ElevenLabsVoiceID         string `env:"ELEVENLABS_VOICE_ID" envDefault:"21m00Tcm4TlvDq8ikWAM"`
ElevenLabsModel           string `env:"ELEVENLABS_MODEL" envDefault:"eleven_monolingual_v1"`

// === OpenAI TTS ===
OpenAITTSModel            string `env:"OPEN_AI_TTS_MODEL" envDefault:"tts-1"`
OpenAITTSVoice            string `env:"OPEN_AI_TTS_VOICE" envDefault:"alloy"`

// === OpenAI Generic TTS ===
TTSOpenAICompatKey        string `env:"TTS_OPEN_AI_COMPATIBLE_KEY"`
TTSOpenAICompatEndpoint   string `env:"TTS_OPEN_AI_COMPATIBLE_ENDPOINT"`
TTSOpenAICompatModel      string `env:"TTS_OPEN_AI_COMPATIBLE_MODEL" envDefault:"tts-1"`
TTSOpenAICompatVoice      string `env:"TTS_OPEN_AI_COMPATIBLE_VOICE" envDefault:"alloy"`
```

### 3.6 估算

| Task | 工时 |
|---|---|
| `tts` 包骨架 + Provider 接口 + factory | 1h |
| ElevenLabs provider + httptest mock 测试 | 1.5h |
| OpenAI provider + 测试 | 1h |
| OpenAI Generic provider + 测试 | 0.5h |
| Handler + endpoint + workspace 鉴权接入 | 1h |
| main.go wire | 0.5h |
| **TTS 小计** | **5.5h** |

---

## 4. 子设计 B:AgentFlow Executor

### 4.1 三种 Block 协议

**api-call**:
```json
{
  "type": "apiCall",
  "config": {
    "url": "https://api.example.com/data?id={{userId}}",
    "method": "GET",
    "headers": [{"key": "X-Api-Key", "value": "abc"}],
    "bodyType": "json",  // json | form | text
    "body": "{\"query\":\"{{lastStep.output}}\"}",
    "formData": [{"key":"foo","value":"bar"}],
    "responseVariable": "apiResult"
  }
}
```

**llm-instruction**:
```json
{
  "type": "llmInstruction",
  "config": {
    "instruction": "Summarize this in 3 bullet points: {{apiResult}}",
    "resultVariable": "summary"
  }
}
```

**web-scraping**:
```json
{
  "type": "webScraping",
  "config": {
    "url": "https://example.com/article",
    "resultVariable": "scraped"
  }
}
```

### 4.2 Context + 变量替换

```go
// internal/agent/flow/executor.go
type Context struct {
    Variables map[string]string  // step name → output text
    Emit      func(message string)
    LM        core.LanguageModel
    HTTPClient *http.Client
}

func InterpolateString(s string, vars map[string]string) string {
    // {{varname}} → vars[varname]
    re := regexp.MustCompile(`\{\{(\w+(?:\.\w+)?)\}\}`)
    return re.ReplaceAllStringFunc(s, func(m string) string {
        name := strings.Trim(m, "{}")
        // 支持 {{lastStep.output}} 简单点号路径
        if strings.HasPrefix(name, "lastStep.") {
            return vars["__last_output"]
        }
        if v, ok := vars[name]; ok { return v }
        return m  // 未匹配保持原样
    })
}
```

### 4.3 Executor 入口

```go
// internal/agent/flow/executor.go
type Executor struct {
    lm        core.LanguageModel
    httpClient *http.Client
}

func New(lm core.LanguageModel, http *http.Client) *Executor { /* ... */ }

func (e *Executor) Run(ctx context.Context, flow *services.LoadedFlow, initialVars map[string]string, emit func(string)) (string, error) {
    fc := &Context{
        Variables: initialVars,
        Emit: emit,
        LM: e.lm,
        HTTPClient: e.httpClient,
    }
    for i, raw := range flow.Config.Steps {
        step, err := parseStep(raw)
        if err != nil { return "", fmt.Errorf("step %d: %w", i, err) }
        fc.Emit(fmt.Sprintf("Flow step %d/%d: %s", i+1, len(flow.Config.Steps), step.Type))
        output, err := e.runStep(ctx, fc, step)
        if err != nil { return "", fmt.Errorf("step %d (%s): %w", i, step.Type, err) }
        if step.ResultVar != "" { fc.Variables[step.ResultVar] = output }
        fc.Variables["__last_output"] = output
    }
    return fc.Variables["__last_output"], nil
}

func (e *Executor) runStep(ctx context.Context, fc *Context, step *Step) (string, error) {
    switch step.Type {
    case "apiCall":      return e.executeAPICall(ctx, fc, step.Config)
    case "llmInstruction": return e.executeLLMInstruction(ctx, fc, step.Config)
    case "webScraping":  return e.executeWebScraping(ctx, fc, step.Config)
    default:             return "", fmt.Errorf("unknown step type: %s", step.Type)
    }
}
```

### 4.4 集成到 tools/flow.go

替换 PR-AR-3 的 stub Handler:

```go
// internal/agent/tools/flow.go (重写 Handler)
Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
    emit("Invoking flow: " + f.Name)
    loaded, err := flowSvc.LoadFlow(f.UUID)
    if err != nil { return tool.Error("flow load: " + err.Error()), nil }

    // 把 LLM 传给 flow 的参数当作 initialVars
    var initialVars map[string]string
    _ = json.Unmarshal(raw, &initialVars)
    initialVars["__flow_invoked_by"] = "agent"

    output, err := flowExecutor.Run(ctx, loaded, initialVars, emit)
    if err != nil { return tool.Error(err.Error()), nil }
    return tool.Result(map[string]any{"output": output, "flow": f.Name}), nil
},
```

### 4.5 估算

| Task | 工时 |
|---|---|
| `flow` 子包 + Executor 骨架 + variable interpolation | 1.5h |
| `apiCall` block 实装(包括 form/json/text body 三种) | 2h |
| `llmInstruction` block(单 LLM.Generate 调用) | 1h |
| `webScraping` block(复用 PR-AR-3 web_scraping skill 的 html 抽取) | 1.5h |
| `tools/flow.go` Handler 重写 + 集成测试(mock LLM + httptest endpoint) | 2h |
| `BuilderDeps.FlowExecutor` 注入 + main.go wire | 1h |
| **AgentFlow 小计** | **9h** |

---

## 5. 子设计 C:Binary File Generation

### 5.1 库选型决策

| 格式 | 推荐库 | License | 选择理由 |
|---|---|---|---|
| docx | `github.com/lukasjarosch/go-docx` | MIT | 模板替换强,API 简洁 |
| pdf | `github.com/go-pdf/fpdf` | MIT | gofpdf 活跃 fork,API 稳定 |
| xlsx | `github.com/xuri/excelize/v2` | BSD-2 | 行业标准,功能全 |
| pptx | (skip) | — | 仅 unioffice 支持,AGPL,license 风险 |

### 5.2 实装路径

`create_files_agent.go` 的 Handler 分支扩展:

```go
case "txt", "md":
    return writeTextFile(tc, args)
case "docx":
    return writeDocxFile(tc, args)  // NEW
case "pdf":
    return writePDFFile(tc, args)   // NEW
case "xlsx":
    return writeXLSXFile(tc, args)  // NEW
case "pptx":
    return tool.Error("pptx format not supported (license constraints); use docx instead"), nil
```

### 5.3 各 writer 实装要点

**docx**:
- 默认模板:1 个 paragraph per `args.Content` 行
- 标题:`args.Filename`(去后缀)
- 字体:Calibri 11pt
- 支持参数 `args.Headings []string` 标记标题级别

**pdf**:
- 页面:A4,默认西文字体 Helvetica
- 内容:逐行写入,自动分页
- 中文限制:**显式不支持**(需要 TTF 注册,本 PR 不做);如检测到非 ASCII 字符且未配置中文字体,返回 `tool.Error("PDF skill does not yet support non-ASCII text")`

**xlsx**:
- args 形状扩展:
  ```json
  {
    "format": "xlsx",
    "filename": "report",
    "content": {
      "sheets": [
        {"name": "Sheet1", "rows": [["Name","Score"],["Alice","95"],["Bob","87"]]}
      ]
    }
  }
  ```
- 表头自动加粗

### 5.4 估算

| Task | 工时 |
|---|---|
| go.mod 加 3 个新依赖 + decision artefact(pptx skip) | 0.5h |
| `writeDocxFile` + 测试 | 1.5h |
| `writePDFFile` + 测试 + 中文限制提示 | 1.5h |
| `writeXLSXFile` + 测试(content shape 扩展) | 2h |
| Handler 集成 + 测试 | 0.5h |
| **Binary 小计** | **6h** |

---

## 6. 总估算 & 优先级

| 子设计 | 工时 | 用户感知 | 推荐顺序 |
|---|---|---|---|
| A. TTS | 5.5h | **高**(前端 chat 旁的 🔊 按钮直接可用) | 1 |
| B. AgentFlow executor | 9h | **高**(企业用户的工作流真正能跑) | 2 |
| C. Binary file generation | 6h | 中(LLM 输出 docx/pdf 报告) | 3 |
| **Total** | **20.5h** | — | — |

---

## 7. 风险与权衡

| 风险 | 缓解 |
|---|---|
| TTS audio 体积大 → 内存占用高 | 30s 文本约 50KB 音频;不缓存,直出 |
| ElevenLabs voice ID 错误 → 400 | 启动 log 不验证,等首次请求才知;文档说明 |
| AgentFlow `apiCall` 用户可指向内网 URL → SSRF | 默认黑名单 `127.0.0.1`/`localhost`/`169.254.169.254`/`10.*`/`172.16-31.*`/`192.168.*`;可由 `AGENT_FLOW_ALLOW_PRIVATE_IPS=true` 关闭 |
| AgentFlow 变量替换 → 注入风险(SQL/Shell) | 变量是字符串拼接,**不在用户 SQL 上下文**;但 webScraping 的 URL 不做白名单是限制,做 SSRF 防御足够 |
| docx/pdf/xlsx 默认输出可能引入 license-encumbered 字体 | gofpdf 默认 Helvetica 是开源;excelize 默认 Calibri 是 MS 字体但仅引用名,不嵌入文件 |
| 中文 PDF 限制让中文用户失望 | 文档明确;给 unicode 字体注册的 follow-up 留位 |
| AgentFlow llmInstruction 阻塞过久 | 30s timeout,与 pantheon tool timeout 一致 |
| binary file 生成 + 大数据 → OOM | 强制 50KB content cap(无论 txt/md 还是 docx);超过返回 error |

---

## 8. 分期内容

按子设计独立 PR:

- **PR-V3B-1 TTS**(`.gpowers/plans/2026-05-28-v3b1-tts.md`):5.5h
- **PR-V3B-2 AgentFlow Executor**(`.gpowers/plans/2026-05-28-v3b2-agentflow-executor.md`):9h
- **PR-V3B-3 Binary File Generation**(`.gpowers/plans/2026-05-28-v3b3-binary-files.md`):6h

三个独立 PR 可以并行交给不同开发者;Plan 文档分别落地。

---

## 9. 后续(不在 v3-B 范围)

- TTS streaming(elevenLabs 的 SSE 接口)
- TTS 缓存(同文本+voice → 同音频,内存 LRU)
- AgentFlow flow-of-flows(嵌套调用)
- AgentFlow 并行 step
- AgentFlow visual editor 后端(目前前端已经能编辑)
- pptx 格式(用户明确请求后再评估 unioffice license)
- PDF 中文 / unicode 字体(注册 TTF)
- docx 模板(用户上传 .docx 当模板,变量替换)

—— end of design
