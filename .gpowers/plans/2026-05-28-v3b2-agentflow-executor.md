# v3-B2 — AgentFlow Executor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 PR-AR-3 投影出来的 stub flow tool 真正可执行 —— 实现 3 种 block(`apiCall` / `llmInstruction` / `webScraping`),用 `{{var}}` 简单字符串插值让 step 间传递数据,SSRF 防护开箱。**估算 9h**。

**先决条件**: PR-AR-3 已合(`internal/agent/tools/flow.go` stub Handler 是替换目标)。

**Source spec:** `.gpowers/designs/2026-05-27-v3b-user-facing-design.md` §4。

**Reference Node implementation:**
- `server/utils/agentFlows/executors/{api-call,llm-instruction,web-scraping}.js`
- `server/utils/agentFlows/executor.js` — context + introspect

---

## Pre-task

### 现状

- `internal/services/agent_flow_service.go` — `AgentFlowService` 有 `ListFlows/LoadFlow/SaveFlow/DeleteFlow`,**无执行器**
- `internal/agent/tools/flow.go` (PR-AR-3) — `flowToEntry` 的 Handler 返回 `{status:"deferred", note:"flow execution is not yet implemented"}`
- 现有 `tools.NewWebScrapingSkill` Handler 内有可复用的 HTML 抽取逻辑(`extractMainText`),executor 的 webScraping block 可以借用

### 新增 surface

```
backend/internal/agent/flow/
├── doc.go
├── executor.go              # type Executor + Run(ctx, flow, vars, emit)
├── executor_test.go
├── step.go                  # parseStep + Step 类型
├── interpolate.go           # {{var}} 替换
├── interpolate_test.go
├── api_call.go              # apiCall block
├── api_call_test.go
├── llm_instruction.go       # llmInstruction block
├── llm_instruction_test.go
├── web_scraping.go          # webScraping block (复用 tools/web_scraping 的抽取)
├── web_scraping_test.go
├── ssrf_guard.go            # 私有 IP 黑名单
└── ssrf_guard_test.go

backend/internal/agent/tools/
├── flow.go                  # MODIFY — Handler 重写,调用 flow.Executor
└── builder.go               # MODIFY — BuilderDeps 加 FlowExecutor

backend/internal/agent/
├── runtime.go               # MODIFY — Deps 加 FlowExecutor
└── handler.go               # MODIFY — buildSessionRegistry 透传

backend/internal/config/config.go  # MODIFY — 加 AgentFlowAllowPrivateIPs bool
backend/cmd/server/main.go         # MODIFY — 构造 + wire
```

### Step JSON 协议(Node parity)

每个 step 都形如:

```json
{
  "type": "apiCall" | "llmInstruction" | "webScraping",
  "config": { /* per-type fields */ }
}
```

具体 config 字段参考 design §4.1。

### `Context` 类型

```go
type Context struct {
    Variables  map[string]string  // var name → value;step output 自动写入 var.ResultVariable
    Emit       func(message string)
    LM         core.LanguageModel
    HTTPClient *http.Client
    AllowPrivateIPs bool
}
```

### TDD discipline

每 task 一 commit。

---

## Task 1: Step 类型 + 变量插值

**Files:**
- `backend/internal/agent/flow/doc.go` (NEW)
- `backend/internal/agent/flow/step.go` (NEW)
- `backend/internal/agent/flow/interpolate.go` (NEW)
- `backend/internal/agent/flow/interpolate_test.go` (NEW)

**Tests:**
- `TestInterpolate_SingleVar_Replaced`
- `TestInterpolate_MissingVar_LeftAsIs`
- `TestInterpolate_MultipleVars_AllReplaced`
- `TestInterpolate_LastStepDotOutput_Reads__last_output`
- `TestInterpolate_NestedBraces_OnlyOuterMatched`
- `TestParseStep_ValidApiCall_ReturnsStruct`
- `TestParseStep_UnknownType_ReturnsError`

### Steps

- [ ] 写 doc.go:
  ```go
  // Package flow executes agent flows composed of api-call, llm-instruction,
  // and web-scraping blocks. See server/utils/agentFlows/executor.js for the
  // Node reference. Variables flow between steps via {{varname}} interpolation.
  package flow
  ```

- [ ] 写 step.go:
  ```go
  type Step struct {
      Type      string                 `json:"type"`
      Config    map[string]any         `json:"config"`
      ResultVar string                 // 抽取自 config.resultVariable / config.responseVariable
  }

  func ParseStep(raw any) (*Step, error) {
      blob, _ := json.Marshal(raw)
      var s Step
      if err := json.Unmarshal(blob, &s); err != nil { return nil, err }
      if s.Type == "" { return nil, fmt.Errorf("step missing type") }
      if cfg := s.Config; cfg != nil {
          if v, _ := cfg["resultVariable"].(string); v != "" { s.ResultVar = v }
          if v, _ := cfg["responseVariable"].(string); v != "" { s.ResultVar = v }
      }
      return &s, nil
  }
  ```

- [ ] 写 interpolate.go(用正则 `{{var}}` 替换;支持 `lastStep.output` 简单点号):
  ```go
  var interpolateRE = regexp.MustCompile(`\{\{(\w+(?:\.\w+)?)\}\}`)

  func Interpolate(s string, vars map[string]string) string {
      return interpolateRE.ReplaceAllStringFunc(s, func(m string) string {
          name := strings.TrimSpace(m[2 : len(m)-2])
          if name == "lastStep.output" {
              return vars["__last_output"]
          }
          if v, ok := vars[name]; ok { return v }
          return m  // 未匹配保持原样
      })
  }
  ```

- [ ] 7 个测试。

### Acceptance

- 7 个测试通过
- `lastStep.output` 与 `__last_output` 互通
- 未匹配变量保留 `{{...}}` 原文(不返回空字符串)

### Commit

`feat(agent/flow): step parsing + variable interpolation`

---

## Task 2: SSRF guard

**Files:**
- `backend/internal/agent/flow/ssrf_guard.go` (NEW)
- `backend/internal/agent/flow/ssrf_guard_test.go` (NEW)
- `backend/internal/config/config.go` (MODIFY — add `AgentFlowAllowPrivateIPs`)

**Tests:**
- `TestSSRFGuard_PublicIP_Allowed`
- `TestSSRFGuard_Localhost_Blocked`
- `TestSSRFGuard_127001_Blocked`
- `TestSSRFGuard_169254_AWSMetadata_Blocked`
- `TestSSRFGuard_10dot_PrivateA_Blocked`
- `TestSSRFGuard_172_16to31_PrivateB_Blocked`
- `TestSSRFGuard_192168_PrivateC_Blocked`
- `TestSSRFGuard_IPv6Loopback_Blocked`
- `TestSSRFGuard_AllowOverride_PermitsPrivate`
- `TestSSRFGuard_InvalidURL_Blocked`
- `TestSSRFGuard_NonHTTPScheme_Blocked` (`ftp://`/`file://`)

### Steps

- [ ] 加 config:
  ```go
  AgentFlowAllowPrivateIPs bool `env:"AGENT_FLOW_ALLOW_PRIVATE_IPS" envDefault:"false"`
  ```

- [ ] 实装 ssrf_guard.go:
  ```go
  package flow

  import (
      "fmt"
      "net"
      "net/url"
      "strings"
  )

  var (
      privateBlocks []*net.IPNet
  )

  func init() {
      for _, cidr := range []string{
          "127.0.0.0/8",      // loopback
          "::1/128",          // IPv6 loopback
          "10.0.0.0/8",       // RFC1918 A
          "172.16.0.0/12",    // RFC1918 B
          "192.168.0.0/16",   // RFC1918 C
          "169.254.0.0/16",   // link-local (AWS metadata)
          "fc00::/7",         // IPv6 ULA
          "fe80::/10",        // IPv6 link-local
      } {
          _, block, _ := net.ParseCIDR(cidr)
          privateBlocks = append(privateBlocks, block)
      }
  }

  // CheckURL returns nil if the URL is safe to fetch given allowPrivate.
  func CheckURL(rawURL string, allowPrivate bool) error {
      u, err := url.Parse(rawURL)
      if err != nil { return fmt.Errorf("invalid URL: %w", err) }
      if u.Scheme != "http" && u.Scheme != "https" {
          return fmt.Errorf("only http/https schemes allowed (got %q)", u.Scheme)
      }
      host := u.Hostname()
      if allowPrivate { return nil }
      // Reject literal localhost
      if strings.EqualFold(host, "localhost") {
          return fmt.Errorf("private host blocked: localhost")
      }
      // Resolve and check each IP
      ips, err := net.LookupIP(host)
      if err != nil {
          // Can't resolve → treat hostname as suspicious; but in tests we may use ".invalid" — give a clear message
          return fmt.Errorf("DNS lookup failed for %s: %w", host, err)
      }
      for _, ip := range ips {
          for _, block := range privateBlocks {
              if block.Contains(ip) {
                  return fmt.Errorf("private IP blocked: %s (%s)", host, ip)
              }
          }
      }
      return nil
  }
  ```

- [ ] 11 个测试 — 用 hosts 文件不能修改,所以测试用 IP literals 直接构造 URL:
  ```go
  func TestSSRFGuard_127001_Blocked(t *testing.T) {
      err := flow.CheckURL("http://127.0.0.1/admin", false)
      require.Error(t, err)
      require.Contains(t, err.Error(), "private IP")
  }
  ```

### Acceptance

- 11 个测试通过
- IPv4 + IPv6 私有/loopback 都被拒
- `AllowPrivateIPs=true` 时全部放行
- 非 http(s) scheme 被拒

### Commit

`feat(agent/flow): SSRF guard against private IPs + non-http schemes`

---

## Task 3: apiCall block

**Files:**
- `backend/internal/agent/flow/api_call.go` (NEW)
- `backend/internal/agent/flow/api_call_test.go` (NEW)

**Tests:**
- `TestAPICall_GET_HappyPath`
- `TestAPICall_POST_JSONBody`
- `TestAPICall_POST_FormBody`
- `TestAPICall_POST_TextBody`
- `TestAPICall_HeadersForwarded`
- `TestAPICall_VarInterpolation_URL`
- `TestAPICall_VarInterpolation_Body`
- `TestAPICall_HTTPError_ReturnsError`
- `TestAPICall_SSRFGuardBlocks`
- `TestAPICall_ResponseTextReturned`

### Steps

- [ ] 实装 api_call.go:
  ```go
  package flow

  import (
      "bytes"
      "context"
      "encoding/json"
      "fmt"
      "io"
      "net/http"
      "net/url"
      "strings"
  )

  func ExecuteAPICall(ctx context.Context, fc *Context, config map[string]any) (string, error) {
      rawURL, _ := config["url"].(string)
      method, _ := config["method"].(string)
      if method == "" { method = "GET" }
      bodyType, _ := config["bodyType"].(string)
      bodyStr, _ := config["body"].(string)
      hdrsRaw, _ := config["headers"].([]any)
      formDataRaw, _ := config["formData"].([]any)

      // Variable interpolation on URL + body
      rawURL = Interpolate(rawURL, fc.Variables)
      bodyStr = Interpolate(bodyStr, fc.Variables)

      // SSRF guard
      if err := CheckURL(rawURL, fc.AllowPrivateIPs); err != nil { return "", err }

      fc.Emit(fmt.Sprintf("API call: %s %s", method, rawURL))

      // Build body based on bodyType
      var body io.Reader
      var contentType string
      switch bodyType {
      case "json":
          // parse + re-marshal for sanity
          var v any
          if err := json.Unmarshal([]byte(bodyStr), &v); err == nil {
              data, _ := json.Marshal(v)
              body = bytes.NewReader(data)
          } else {
              body = strings.NewReader(bodyStr)  // raw fall-back
          }
          contentType = "application/json"
      case "form":
          form := url.Values{}
          for _, item := range formDataRaw {
              m, _ := item.(map[string]any)
              k, _ := m["key"].(string)
              v, _ := m["value"].(string)
              form.Add(k, Interpolate(v, fc.Variables))
          }
          body = strings.NewReader(form.Encode())
          contentType = "application/x-www-form-urlencoded"
      case "text":
          body = strings.NewReader(bodyStr)
          contentType = "text/plain"
      default:
          if bodyStr != "" { body = strings.NewReader(bodyStr) }
      }

      req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
      if err != nil { return "", err }
      if contentType != "" { req.Header.Set("Content-Type", contentType) }

      // Forward headers
      for _, item := range hdrsRaw {
          m, _ := item.(map[string]any)
          k, _ := m["key"].(string)
          v, _ := m["value"].(string)
          req.Header.Set(k, Interpolate(v, fc.Variables))
      }

      resp, err := fc.HTTPClient.Do(req)
      if err != nil { return "", fmt.Errorf("api call: %w", err) }
      defer resp.Body.Close()

      respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))  // 4 MiB cap

      if resp.StatusCode >= 400 {
          fc.Emit(fmt.Sprintf("API call failed: %d", resp.StatusCode))
          return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 200))
      }

      return string(respBody), nil
  }

  func truncate(s string, max int) string {
      if len(s) <= max { return s }
      return s[:max] + "..."
  }
  ```

- [ ] 10 个测试,用 httptest 桩。

### Acceptance

- 10 个测试通过
- GET / POST {json,form,text} 4 种 body 模式都对
- 变量在 URL / body / header / formData value 都能替换
- SSRF guard 拦截私有 IP
- 4 MiB response cap

### Commit

`feat(agent/flow): apiCall block (GET/POST + json/form/text body)`

---

## Task 4: llmInstruction block

**Files:**
- `backend/internal/agent/flow/llm_instruction.go` (NEW)
- `backend/internal/agent/flow/llm_instruction_test.go` (NEW)

**Tests:**
- `TestLLMInstruction_HappyPath_CallsLM`
- `TestLLMInstruction_InterpolatesInstruction`
- `TestLLMInstruction_LMError_Returns`
- `TestLLMInstruction_EmptyInstruction_ReturnsError`
- `TestLLMInstruction_NilLM_ReturnsError`

### Steps

- [ ] 实装 llm_instruction.go:
  ```go
  package flow

  import (
      "context"
      "fmt"

      "github.com/odysseythink/pantheon/core"
  )

  func ExecuteLLMInstruction(ctx context.Context, fc *Context, config map[string]any) (string, error) {
      instr, _ := config["instruction"].(string)
      if instr == "" { return "", fmt.Errorf("instruction is required") }
      if fc.LM == nil { return "", fmt.Errorf("LLM not available") }

      instr = Interpolate(instr, fc.Variables)
      fc.Emit("LLM instruction: " + truncate(instr, 60))

      resp, err := fc.LM.Generate(ctx, &core.Request{
          Messages: []core.Message{core.NewTextMessage(core.MESSAGE_ROLE_USER, instr)},
      })
      if err != nil { return "", fmt.Errorf("llm: %w", err) }
      return resp.Message.Text(), nil
  }
  ```

- [ ] 5 个测试,用 mock LanguageModel(PR-AR-2 已有 `mockLanguageModel`)。

### Acceptance

- 5 个测试通过
- 变量替换在 instruction 上工作
- nil LM 报清晰 error

### Commit

`feat(agent/flow): llmInstruction block`

---

## Task 5: webScraping block

**Files:**
- `backend/internal/agent/flow/web_scraping.go` (NEW)
- `backend/internal/agent/flow/web_scraping_test.go` (NEW)

**Tests:**
- `TestWebScraping_HappyPath_ExtractsArticle`
- `TestWebScraping_InterpolatesURL`
- `TestWebScraping_SSRFGuard_Blocks`
- `TestWebScraping_HTTPError_Returns`
- `TestWebScraping_NonHTML_ReturnsRawText`

### Steps

- [ ] 实装 web_scraping.go(复用 PR-AR-3 `tools.extractMainText` 如果导出;否则在 flow 子包内 inline 一份):
  ```go
  package flow

  import (
      "context"
      "fmt"
      "io"
      "net/http"
      "strings"

      "golang.org/x/net/html"
  )

  func ExecuteWebScraping(ctx context.Context, fc *Context, config map[string]any) (string, error) {
      rawURL, _ := config["url"].(string)
      if rawURL == "" { return "", fmt.Errorf("url is required") }
      rawURL = Interpolate(rawURL, fc.Variables)
      if err := CheckURL(rawURL, fc.AllowPrivateIPs); err != nil { return "", err }
      fc.Emit("Scraping " + rawURL)

      req, _ := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
      req.Header.Set("User-Agent", "AnythingLLM-AgentFlow/1.0")
      resp, err := fc.HTTPClient.Do(req)
      if err != nil { return "", fmt.Errorf("scrape: %w", err) }
      defer resp.Body.Close()
      if resp.StatusCode >= 400 {
          return "", fmt.Errorf("HTTP %d", resp.StatusCode)
      }
      body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))  // 1 MiB cap

      if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
          return string(body), nil  // 非 HTML 直接返回
      }
      text, _ := extractMainText(body)
      return text, nil
  }

  // extractMainText 与 tools/web_scraping.go 相同;若不便复用,inline 一份(20 行)
  func extractMainText(body []byte) (string, string) { /* 同 PR-AR-3 实装 */ }
  ```

- [ ] 5 个测试,httptest 桩 + sample HTML(article / body / 纯文本)。

### Acceptance

- 5 个测试通过
- article 优先 → main → body fallback
- 非 HTML 内容直接返回 raw

### Commit

`feat(agent/flow): webScraping block`

---

## Task 6: Executor 主循环

**Files:**
- `backend/internal/agent/flow/executor.go` (NEW)
- `backend/internal/agent/flow/executor_test.go` (NEW)

**Tests:**
- `TestExecutor_SingleStep_ApiCall_ReturnsOutput`
- `TestExecutor_ChainedSteps_VarFlow` (step1 → step2 引用 step1 输出)
- `TestExecutor_StepError_StopsAndReturns`
- `TestExecutor_NoSteps_ReturnsEmpty`
- `TestExecutor_LastOutputAutoPopulated`
- `TestExecutor_UnknownBlockType_ReturnsError`

### Steps

- [ ] 实装 executor.go:
  ```go
  package flow

  import (
      "context"
      "fmt"
      "net/http"
      "time"

      "github.com/odysseythink/hermind/backend/internal/services"
      "github.com/odysseythink/pantheon/core"
  )

  type Executor struct {
      lm         core.LanguageModel
      httpClient *http.Client
      allowPrivate bool
  }

  func New(lm core.LanguageModel, allowPrivateIPs bool) *Executor {
      return &Executor{
          lm: lm,
          httpClient: &http.Client{Timeout: 30 * time.Second},
          allowPrivate: allowPrivateIPs,
      }
  }

  type Context struct {
      Variables       map[string]string
      Emit            func(string)
      LM              core.LanguageModel
      HTTPClient      *http.Client
      AllowPrivateIPs bool
  }

  func (e *Executor) Run(ctx context.Context, flow *services.LoadedFlow, initialVars map[string]string, emit func(string)) (string, error) {
      if emit == nil { emit = func(string) {} }
      if initialVars == nil { initialVars = map[string]string{} }
      fc := &Context{
          Variables: initialVars, Emit: emit,
          LM: e.lm, HTTPClient: e.httpClient,
          AllowPrivateIPs: e.allowPrivate,
      }
      for i, raw := range flow.Config.Steps {
          step, err := ParseStep(raw)
          if err != nil { return "", fmt.Errorf("step %d parse: %w", i, err) }
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
      case "apiCall":         return ExecuteAPICall(ctx, fc, step.Config)
      case "llmInstruction":  return ExecuteLLMInstruction(ctx, fc, step.Config)
      case "webScraping":     return ExecuteWebScraping(ctx, fc, step.Config)
      default:                return "", fmt.Errorf("unknown block type: %s", step.Type)
      }
  }
  ```

- [ ] 6 个测试,串接 mock httptest + mock LM。

### Acceptance

- 6 个测试通过
- 变量在 step 间正确流转
- 任一 step 错误停止链路并返回 error

### Commit

`feat(agent/flow): Executor — sequential step runner`

---

## Task 7: 集成进 tools/flow.go

**Files:**
- `backend/internal/agent/tools/flow.go` (MODIFY — Handler 重写)
- `backend/internal/agent/tools/builder.go` (MODIFY — BuilderDeps 加 FlowExecutor)
- `backend/internal/agent/tools/flow_test.go` (MODIFY — 替换 deferred-stub 测试)
- `backend/internal/agent/runtime.go` (MODIFY — Deps 加 FlowExecutor)
- `backend/internal/agent/handler.go` (MODIFY — buildSessionRegistry 透传)
- `backend/cmd/server/main.go` (MODIFY — 构造 FlowExecutor + 注入)

**Tests:**
- `TestFlowTool_HappyPath_ExecutesFlow_ReturnsOutput`
- `TestFlowTool_ExecutorError_ReturnsToolError`
- `TestFlowTool_ApprovalRequiredOnDestructive` (PR-AR-5 hook;实际上 PR-AR-3 把 flow 标 approval-required,沿用)

### Steps

- [ ] 加 `BuilderDeps.FlowExecutor *flow.Executor`(nilable;nil 时回到 deferred 行为)。

- [ ] 重写 `flowToEntry` Handler:
  ```go
  Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
      emit("Invoking flow: " + f.Name)
      loaded, err := flowSvc.LoadFlow(f.UUID)
      if err != nil { return tool.Error("flow load: " + err.Error()), nil }

      if executor == nil {
          return tool.Error("flow execution requires AgentFlowExecutor (not configured)"), nil
      }

      var args map[string]string
      _ = json.Unmarshal(raw, &args)
      if args == nil { args = map[string]string{} }
      args["__flow_invoked_by"] = "agent"

      output, err := executor.Run(ctx, loaded, args, emit)
      if err != nil { return tool.Error(err.Error()), nil }
      return tool.Result(map[string]any{
          "flow":   f.Name,
          "output": output,
      }), nil
  },
  ```

- [ ] `flowToEntry` 签名加参数 `executor *flow.Executor`;在 Builder.Build 内传入。

- [ ] `agent.Deps` 加 `FlowExecutor *flow.Executor`。

- [ ] main.go:
  ```go
  // 在 LLM provider 构造后
  flowExec := flow.New(llmProv.LanguageModel(), cfg.AgentFlowAllowPrivateIPs)
  agentRuntime := agent.NewRuntime(agent.Deps{
      /* ... existing ... */
      FlowExecutor: flowExec,
  })
  ```

- [ ] 改 3 个 test,删 "deferred" 字串断言,改为断言真 output。

### Acceptance

- 3 个测试通过
- 之前 PR-AR-3 写的 deferred-status 测试要么删除要么调整(可能需要单独保留一个 "executor nil → deferred error" 测试)
- 集成 e2e:LLM 决定调用 flow → flow.Run → 返回真实 output → agent 继续 reasoning

### Commit

`feat(agent/tools): wire flow.Executor into flow tool — real execution`

---

## Post-PR checklist

- [ ] `go build ./...` 干净
- [ ] `go vet ./...` 干净
- [ ] `go test ./... -race` 100% 绿
- [ ] `AGENT_FLOW_ALLOW_PRIVATE_IPS` 文档化 in `.env.example`
- [ ] PR-AR-3 的 "deferred" decision artefact 标记 superseded by v3-B2
- [ ] Manual smoke:在 `.gpowers/plans/` 旁建个 sample flow.json(api-call → llm-instruction),@agent 触发,看 step-by-step status 帧
- [ ] No new TODOs without follow-up reference

## Risk notes

| Risk | Mitigation |
|---|---|
| 变量插值在 URL 上没 escape → 用户输入含 `&` 会破坏 query string | 文档说明:变量应在 step config 设计时考虑 escape;不在 server 自动 escape(否则 JSON body 等场景反而坏) |
| SSRF DNS lookup 增加 ~10ms latency / step | acceptable;比误连内网值得 |
| 4 MiB API response cap 挡住合理大 JSON | 写 documented limit;用户拆 step 分页 |
| Flow 用户写循环 → 死循环 | 当前 executor 顺序执行,无循环原语;Node 也无 |
| LLM 在 llmInstruction 卡 30s+ | http.Client.Timeout = 30s;LM.Generate 内部 ctx 同步 |
| 私有 IP 白名单不完整(如 metadata endpoint 变更) | 当前 list 涵盖 RFC1918 + 169.254 + IPv6 ULA + link-local;社区文档建议的足够 |

## Estimate

| Task | Hours |
|---|---|
| 1. Step + Interpolate | 1.5 |
| 2. SSRF guard | 1.0 |
| 3. apiCall block | 2.5 |
| 4. llmInstruction block | 0.5 |
| 5. webScraping block | 1.0 |
| 6. Executor 主循环 | 1.0 |
| 7. 集成进 tools/flow.go + main.go | 1.5 |
| **Total** | **9.0** (design 估 9h ✓) |

—— end of plan
