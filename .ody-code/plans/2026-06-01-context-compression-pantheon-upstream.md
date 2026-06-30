# Context Compression — Pantheon Upstream Gap-Fill

> **Goal:** Apply 8 backward-compatible changes to Pantheon's `agent/compression` package so Hermind can use the engine with accessors, cooldown tiers, fallback retry, injectable redaction, per-tool templates, input truncation, summary markers, and usage calibration.
>
> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Depends on file:** `2026-06-01-context-compression-index.md`

**Repository:** `/Users/ranwei/workspace/go_work/pantheon`
**Target package:** `github.com/odysseythink/pantheon/agent/compression`
**Go version:** 1.26 (same as Hermind)

---

## File Structure

All paths are relative to the Pantheon repo root (`/Users/ranwei/workspace/go_work/pantheon`).

| Path | Action | Responsibility |
|---|---|---|
| `agent/compression/state.go` | Modify | Add accessors (`PreviousSummary`, `SetPreviousSummary`, `LastFallbackUsed`); 600s cooldown tier; fallback-model retry |
| `agent/compression/config.go` | Modify | Add `RedactPatterns []*regexp.Regexp` |
| `agent/compression/summary.go` | Modify | Per-message 6000-char truncation in `renderTranscript`; injectable redact patterns |
| `agent/compression/prune.go` | Modify | Per-tool summary templates; injectable redact patterns |
| `agent/compression/assemble.go` | Modify | Prefix summary with `summaryPrefix`; append end marker |
| `agent/compression/compressor.go` | Modify | Extend `UpdateFromResponse` to record real usage for calibration; add `SetFallbackModel` |
| `agent/compression/helpers.go` | Modify | Add `applyRedaction` helper using `cfg.RedactPatterns` |
| `agent/compression/state_test.go` | Modify | Tests for accessors, cooldown tiers, fallback retry |
| `agent/compression/summary_test.go` | Modify | Tests for truncation and injectable redaction |
| `agent/compression/prune_test.go` | Modify | Tests for per-tool templates |
| `agent/compression/assemble_test.go` | Modify | Tests for prefix + end marker |
| `agent/compression/compressor_test.go` | Modify | Tests for `UpdateFromResponse` usage recording |
| `agent/agent.go` | Modify | Call `contextEngine.UpdateFromResponse(resp.Usage)` after each step |
| `agent/stream.go` | Modify | Call `contextEngine.UpdateFromResponse(usage)` after each step |

---

## Dependency Overview

```
P1 (state.go accessors + cooldown + fallback)
  │
  ├─> P2 (config.go RedactPatterns) ──┐
  │                                    │
  ├─> P3 (summary.go truncation) <─────┤ (both use RedactPatterns)
  │                                    │
  ├─> P4 (prune.go templates) <────────┘
  │
  ├─> P5 (assemble.go markers)
  │
  ├─> P6 (compressor.go UpdateFromResponse)
  │
  └─> P7 (agent.go + stream.go UpdateFromResponse calls)
       │
       ▼
  P8 (whole-tree typecheck + test)
```

P1 must land before P2–P7 because `SetFallbackModel` and accessors are used by later Hermind code.
P2 must land before P3 and P4 because both reference `cfg.RedactPatterns`.
P6 must land before P7 because P7 calls the extended `UpdateFromResponse`.
P8 is verification-only and depends on all prior tasks.

---

## Risks & Open Questions

| # | Risk | Assumption | Impact if wrong |
|---|---|---|---|
| 1 | `SetFallbackModel` changes `DefaultCompressor` API surface | The method is additive only; no existing callers are broken | None — additive method |
| 2 | 600s cooldown may be too aggressive | Design doc specifies 600s for `ineffectiveCount >= 5` | Users experience long waits; mitigation: workspace-level override in Hermind |
| 3 | Per-tool templates need tool-name info in `summarizeToolResult` | `ToolResultPart.Name` is reliably set by callers | Template falls back to generic format; no data loss |
| 4 | `UpdateFromResponse` call in `RunStream` needs usage from stream parts | `core.StreamPartTypeUsage` delivers a `*core.Usage` before `Finish` | Usage calibration misses streaming responses; mitigation: also aggregate from final response |

---

## Phase 0: State & Config

### Task P1: state.go — accessors, 600s cooldown tier, fallback-model retry

**Depends on:** none

**Files:**
- Modify: `agent/compression/state.go`
- Modify: `agent/compression/compressor.go` (add `SetFallbackModel` field + method)
- Test: `agent/compression/state_test.go`

**Context:** `compressionState` is currently unexported. Hermind needs to seed `previousSummary` from a persisted database row before each session, and read it back after compression to persist again. The engine also needs a third cooldown tier (600 s) and a real fallback-model retry instead of the current no-op.

- [ ] **Step 1: Write the failing test**

Add to `agent/compression/state_test.go`:

```go
func TestAccessors(t *testing.T) {
	c := NewDefaultCompressor(DefaultCompressionConfig(), nil)
	if c.PreviousSummary() != "" {
		t.Fatalf("expected empty previousSummary, got %q", c.PreviousSummary())
	}
	c.SetPreviousSummary("hello world")
	if c.PreviousSummary() != "hello world" {
		t.Fatalf("expected previousSummary=hello world, got %q", c.PreviousSummary())
	}
	if c.LastFallbackUsed() {
		t.Fatal("expected LastFallbackUsed=false")
	}
}

func TestCooldownThirdTier(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cfg.CooldownEnabled = true
	cfg.CooldownBase = 30 * time.Second
	cfg.CooldownMax = 60 * time.Second
	c := NewDefaultCompressor(cfg, nil)
	c.state.ineffectiveCount = 5
	c.enterCooldown(nil)
	if time.Now().After(c.state.summaryCooldownUntil) {
		t.Fatal("expected cooldown to be active")
	}
	remaining := time.Until(c.state.summaryCooldownUntil)
	if remaining < 590*time.Second || remaining > 610*time.Second {
		t.Fatalf("expected ~600s cooldown, got %v", remaining)
	}
}

func TestFallbackModelRetry(t *testing.T) {
	// This test uses a mock LanguageModel that fails on first call and succeeds on second.
	primary := &mockFailingLM{failCount: 1}
	fallback := &mockFailingLM{failCount: 0}
	cfg := DefaultCompressionConfig()
	cfg.FallbackModel = "fallback-model"
	c := NewDefaultCompressor(cfg, primary)
	c.SetFallbackModel(fallback)

	msgs := []core.Message{core.NewTextMessage(core.MESSAGE_ROLE_USER, "hello")}
	summary, err := c.generateSummaryWithFallback(context.Background(), msgs, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}
	if !c.LastFallbackUsed() {
		t.Fatal("expected LastFallbackUsed=true")
	}
}
```

You will also need a small mock in the test file (or a `_test.go` helper):

```go
type mockFailingLM struct {
	failCount int
	calls     int
}

func (m *mockFailingLM) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	m.calls++
	if m.failCount > 0 {
		m.failCount--
		return nil, errors.New("primary failure")
	}
	return &core.Response{Message: core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT, "fallback summary")}, nil
}

func (m *mockFailingLM) Stream(ctx context.Context, req *core.Request) (iter.Seq2[*core.StreamPart, error], error) {
	return nil, errors.New("not implemented")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/compression/ -run TestAccessors -v`
Expected: FAIL — `PreviousSummary` / `SetPreviousSummary` / `LastFallbackUsed` / `SetFallbackModel` not defined.

- [ ] **Step 3: Implement accessors and cooldown tier**

In `agent/compression/compressor.go`, add to `DefaultCompressor`:

```go
	// fallback model for retry
	fallbackAux core.LanguageModel
```

Add methods (anywhere in `compressor.go` near existing exported methods):

```go
// PreviousSummary returns the last generated summary text.
func (c *DefaultCompressor) PreviousSummary() string {
	return c.state.previousSummary
}

// SetPreviousSummary seeds the compressor with a persisted summary.
func (c *DefaultCompressor) SetPreviousSummary(s string) {
	c.state.previousSummary = s
}

// LastFallbackUsed reports whether the most recent summary generation fell
// back to the fallback model (or static fallback).
func (c *DefaultCompressor) LastFallbackUsed() bool {
	return c.state.lastFallbackUsed
}

// SetFallbackModel registers an auxiliary model used when the primary
// summarization call fails.
func (c *DefaultCompressor) SetFallbackModel(aux core.LanguageModel) {
	c.fallbackAux = aux
}
```

In `agent/compression/state.go`, modify `enterCooldown`:

```go
func (c *DefaultCompressor) enterCooldown(err error) {
	if !c.cfg.CooldownEnabled {
		return
	}
	c.state.lastSummaryError = err

	var cooldown time.Duration
	if c.state.ineffectiveCount >= 5 {
		cooldown = 600 * time.Second
	} else {
		multiplier := time.Duration(min(c.state.ineffectiveCount+1, 3))
		cooldown = c.cfg.CooldownBase + (c.cfg.CooldownBase/2)*multiplier
		if cooldown > c.cfg.CooldownMax {
			cooldown = c.cfg.CooldownMax
		}
	}
	c.state.summaryCooldownUntil = time.Now().Add(cooldown)
}
```

Add `lastFallbackUsed` to `compressionState` in `compressor.go`:

```go
type compressionState struct {
	previousSummary           string
	lastCompressionSavingsPct float64
	ineffectiveCount          int
	summaryCooldownUntil      time.Time
	lastSummaryError          error
	lastFallbackUsed          bool
}
```

- [ ] **Step 4: Implement fallback-model retry**

In `agent/compression/state.go`, replace the stub in `generateSummaryWithFallback`:

```go
func (c *DefaultCompressor) generateSummaryWithFallback(ctx context.Context, middle []core.Message, focusTopic string) (string, error) {
	c.state.lastFallbackUsed = false

	summary, err := c.generateSummary(ctx, middle, focusTopic)
	if err == nil && summary != "" {
		return summary, nil
	}

	// Level 1: try fallback model
	if c.fallbackAux != nil {
		fallbackSummary, fallbackErr := c.generateSummaryWithAux(ctx, c.fallbackAux, middle, focusTopic)
		if fallbackErr == nil && fallbackSummary != "" {
			c.state.lastFallbackUsed = true
			return fallbackSummary, nil
		}
	}

	// Level 2: static fallback summary
	c.enterCooldown(err)
	c.state.lastFallbackUsed = true
	return c.buildStaticFallbackSummary(middle), nil
}
```

Extract `generateSummaryWithAux` from `generateSummary` so it can accept any `core.LanguageModel`:

In `agent/compression/summary.go`, modify `generateSummary`:

```go
func (c *DefaultCompressor) generateSummary(ctx context.Context, middle []core.Message, focusTopic string) (string, error) {
	return c.generateSummaryWithAux(ctx, c.aux, middle, focusTopic)
}

func (c *DefaultCompressor) generateSummaryWithAux(ctx context.Context, aux core.LanguageModel, middle []core.Message, focusTopic string) (string, error) {
	transcript := renderTranscript(middle)
	if c.cfg.RedactionEnabled {
		transcript = redact.String(transcript)
	}

	var systemPrompt string
	if c.cfg.IterativeUpdateEnabled && c.state.previousSummary != "" &&
		float64(len(c.state.previousSummary))/float64(c.maxSummaryTokens) < c.cfg.IterativeUpdateMaxLength {
		systemPrompt = fmt.Sprintf(
			"You previously summarized this conversation as follows. UPDATE that summary "+
				"with the new turns below, preserving what is still relevant and adding new information. "+
				"Remove information that is no longer relevant.\n\n"+
				"PREVIOUS SUMMARY:\n%s\n\n"+
				"NEW TURNS TO INCORPORATE:\n%s",
			c.state.previousSummary, transcript,
		)
	} else {
		systemPrompt = "Produce a structured summary using exactly these sections: " +
			"Active Task, Goal, Constraints & Preferences, Completed Actions, " +
			"Active State, In Progress, Blocked, Key Decisions, Resolved Questions, " +
			"Pending User Asks, Relevant Files, Remaining Work, Critical Context. " +
			"Be concise. Prioritize facts, decisions, and state over narration."
		if focusTopic != "" {
			systemPrompt += fmt.Sprintf(" Prioritize information related to: %q", focusTopic)
		}
	}

	req := &core.Request{
		SystemPrompt: systemPrompt,
		Messages: []core.Message{{
			Role:    core.MESSAGE_ROLE_USER,
			Content: core.NewTextContent(transcript),
		}},
		MaxTokens: ptrInt(c.maxSummaryTokens),
	}

	resp, err := aux.Generate(ctx, req)
	if err != nil {
		return "", err
	}

	var text string
	for _, part := range resp.Message.Content {
		if p, ok := part.(core.TextPart); ok {
			text += p.Text
		}
	}
	if c.cfg.RedactionEnabled {
		text = redact.String(text)
	}
	return strings.TrimSpace(text), nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/compression/ -run "TestAccessors|TestCooldownThirdTier|TestFallbackModelRetry" -v`
Expected: PASS

- [ ] **Step 6: Whole-tree typecheck (incl. tests)**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go vet ./agent/...`
Expected: no errors.

- [ ] **Step 7: Commit**

```bash
cd /Users/ranwei/workspace/go_work/pantheon
git add agent/compression/state.go agent/compression/compressor.go agent/compression/summary.go agent/compression/state_test.go
git commit -m "feat(compression): accessors, 600s cooldown, fallback model retry

- Add PreviousSummary/SetPreviousSummary/LastFallbackUsed accessors
- Add third cooldown tier (600s) for ineffectiveCount >= 5
- Implement fallback-model retry in generateSummaryWithFallback
- Extract generateSummaryWithAux for reuse with any LanguageModel
- Add SetFallbackModel setter"
```

---

### Task P2: config.go — injectable redact patterns

**Depends on:** none (independent of P1)

**Files:**
- Modify: `agent/compression/config.go`
- Test: `agent/compression/redact_test.go`

**Context:** Hermind wants to inject its own redaction rules (e.g., keep `key`/`token` patterns, drop bare-hex and email). The current code hard-codes `redact.String` from Pantheon utils. Adding `RedactPatterns` lets callers override.

- [ ] **Step 1: Write the failing test**

Add to `agent/compression/redact_test.go` (create if it does not exist, but it already does):

```go
func TestRedactPatternsOverride(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cfg.RedactionEnabled = true
	cfg.RedactPatterns = []*regexp.Regexp{
		regexp.MustCompile(`SECRET_\d+`),
	}
	c := NewDefaultCompressor(cfg, nil)

	input := "token SECRET_123 and SECRET_456"
	got := c.applyRedaction(input)
	want := "token [REDACTED] and [REDACTED]"
	if got != want {
		t.Fatalf("applyRedaction(%q) = %q, want %q", input, got, want)
	}
}

func TestRedactPatternsNilUsesDefault(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cfg.RedactionEnabled = true
	cfg.RedactPatterns = nil
	c := NewDefaultCompressor(cfg, nil)

	input := "contact alice@example.com"
	got := c.applyRedaction(input)
	// default redact.String should scrub emails
	if strings.Contains(got, "alice@example.com") {
		t.Fatalf("expected email redacted, got %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/compression/ -run "TestRedactPatterns" -v`
Expected: FAIL — `RedactPatterns` field not defined; `applyRedaction` method not defined.

- [ ] **Step 3: Add RedactPatterns field**

In `agent/compression/config.go`, add the import and field:

```go
import "time"

// Change to:
import (
	"regexp"
	"time"
)
```

Add to `CompressionConfig` struct:

```go
	RedactPatterns []*regexp.Regexp `yaml:"redact_patterns,omitempty"`
```

- [ ] **Step 4: Implement applyRedaction helper**

In `agent/compression/helpers.go`, add:

```go
func (c *DefaultCompressor) applyRedaction(text string) string {
	if !c.cfg.RedactionEnabled {
		return text
	}
	if len(c.cfg.RedactPatterns) > 0 {
		for _, re := range c.cfg.RedactPatterns {
			text = re.ReplaceAllString(text, "[REDACTED]")
		}
		return text
	}
	return redact.String(text)
}
```

Add the import for `redact` if not already present (it is already imported in `summary.go` and `prune.go`; `helpers.go` currently does not import it):

```go
import (
	"fmt"
	"strings"

	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/utils/redact"
)
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/compression/ -run "TestRedactPatterns" -v`
Expected: PASS

- [ ] **Step 6: Whole-tree typecheck**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go vet ./agent/...`
Expected: no errors.

- [ ] **Step 7: Commit**

```bash
cd /Users/ranwei/workspace/go_work/pantheon
git add agent/compression/config.go agent/compression/helpers.go agent/compression/redact_test.go
git commit -m "feat(compression): injectable redact patterns

- Add RedactPatterns field to CompressionConfig
- Add applyRedaction helper that uses caller patterns when set,
  otherwise falls back to redact.String"
```

---

## Phase 1: Summary, Prune, Assemble

### Task P3: summary.go — per-message input truncation + injectable redaction

**Depends on:** Task P2

**Files:**
- Modify: `agent/compression/summary.go`
- Modify: `agent/compression/helpers.go` (uses `applyRedaction`)
- Test: `agent/compression/summary_test.go`

**Context:** Long tool-result messages (e.g., 200KB terminal output) can blow the summarization prompt. We cap each message at 6000 characters in the transcript sent to the aux model. We also wire `applyRedaction` so Hermind's patterns are used instead of the global `redact.String`.

- [ ] **Step 1: Write the failing test**

Add to `agent/compression/summary_test.go`:

```go
func TestRenderTranscriptTruncatesPerMessage(t *testing.T) {
	longText := strings.Repeat("a", 10000)
	msgs := []core.Message{
		core.NewTextMessage(core.MESSAGE_ROLE_USER, longText),
	}
	transcript := renderTranscript(msgs)
	// Each message should be truncated to 6000 chars of content
	if strings.Contains(transcript, strings.Repeat("a", 7000)) {
		t.Fatal("expected transcript to truncate per-message text")
	}
	if !strings.Contains(transcript, "(truncated") {
		t.Fatal("expected truncation marker in transcript")
	}
}

func TestGenerateSummaryUsesRedactPatterns(t *testing.T) {
	mockAux := &mockSummaryLM{}
	cfg := DefaultCompressionConfig()
	cfg.RedactionEnabled = true
	cfg.RedactPatterns = []*regexp.Regexp{regexp.MustCompile(`SECRET`)}
	c := NewDefaultCompressor(cfg, mockAux)

	msgs := []core.Message{core.NewTextMessage(core.MESSAGE_ROLE_USER, "SECRET data here")}
	_, _ = c.generateSummary(context.Background(), msgs, "")

	// mockSummaryLM should record the redacted prompt
	if strings.Contains(mockAux.lastSystemPrompt, "SECRET") {
		t.Fatal("expected SECRET to be redacted in prompt sent to aux model")
	}
}
```

You need a small mock LM for this test:

```go
type mockSummaryLM struct {
	lastSystemPrompt string
}

func (m *mockSummaryLM) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	m.lastSystemPrompt = req.SystemPrompt
	return &core.Response{Message: core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT, "summary")}, nil
}

func (m *mockSummaryLM) Stream(ctx context.Context, req *core.Request) (iter.Seq2[*core.StreamPart, error], error) {
	return nil, errors.New("not implemented")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/compression/ -run "TestRenderTranscriptTruncatesPerMessage|TestGenerateSummaryUsesRedactPatterns" -v`
Expected: FAIL — `renderTranscript` doesn't truncate; `generateSummary` still calls `redact.String` directly.

- [ ] **Step 3: Implement per-message truncation in renderTranscript**

In `agent/compression/helpers.go`, modify `renderTranscript`:

```go
const perMessageCharLimit = 6000

func renderTranscript(msgs []core.Message) string {
	var out string
	for i, m := range msgs {
		out += fmt.Sprintf("%d. %s: ", i+1, m.Role)
		for _, p := range m.Content {
			switch part := p.(type) {
			case core.TextPart:
				text := part.Text
				if len(text) > perMessageCharLimit {
					text = text[:perMessageCharLimit] + " (truncated)"
				}
				out += text
			case core.ToolCallPart:
				out += "[tool_call: " + part.Name + "]"
			case core.ToolResultPart:
				out += "[tool_result]"
			case core.ToolResultErrorPart:
				out += "[tool_result_error: " + part.Error + "]"
			}
		}
		out += "\n"
	}
	return out
}
```

- [ ] **Step 4: Wire applyRedaction into generateSummaryWithAux**

In `agent/compression/summary.go`, replace the two `redact.String` calls in `generateSummaryWithAux` with `c.applyRedaction`:

```go
	transcript := renderTranscript(middle)
	transcript = c.applyRedaction(transcript)
```

And at the end:

```go	
	text = c.applyRedaction(text)
```

Remove the direct `redact` import from `summary.go` if it is no longer used (it won't be). The file should now only import `context`, `fmt`, `strings`, and `core`.

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/compression/ -run "TestRenderTranscriptTruncatesPerMessage|TestGenerateSummaryUsesRedactPatterns" -v`
Expected: PASS

- [ ] **Step 6: Whole-tree typecheck**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go vet ./agent/...`
Expected: no errors.

- [ ] **Step 7: Commit**

```bash
cd /Users/ranwei/workspace/go_work/pantheon
git add agent/compression/helpers.go agent/compression/summary.go agent/compression/summary_test.go
git commit -m "feat(compression): per-message truncation + injectable redaction in summary

- Truncate each message to 6000 chars in renderTranscript
- Replace hard-coded redact.String with applyRedaction in summary.go"
```

---

### Task P4: prune.go — per-tool summary templates + injectable redaction

**Depends on:** Task P2

**Files:**
- Modify: `agent/compression/prune.go`
- Modify: `agent/compression/helpers.go` (uses `applyRedaction` in prune)
- Test: `agent/compression/prune_test.go`

**Context:** Generic `[tool_result %s: %d chars, %d lines]` is too opaque for tools like `terminal`, `browser_navigate`, `create_files`. We add tool-name-aware templates and wire injectable redaction.

- [ ] **Step 1: Write the failing test**

Add to `agent/compression/prune_test.go`:

```go
func TestSummarizeToolResultPerToolTemplates(t *testing.T) {
	cfg := DefaultCompressionConfig()
	c := NewDefaultCompressor(cfg, nil)

	tests := []struct {
		name string
		tr   core.ToolResultPart
		want string
	}{
		{
			name: "terminal",
			tr:   core.ToolResultPart{Name: "terminal", ToolCallID: "tc1", Content: []core.ContentParter{core.TextPart{Text: "line1\nline2\nline3"}}},
			want: "[terminal_output: 3 lines, 17 chars]",
		},
		{
			name: "browser_navigate",
			tr:   core.ToolResultPart{Name: "browser_navigate", ToolCallID: "tc2", Content: []core.ContentParter{core.TextPart{Text: `{"url":"https://example.com","title":"Example"}`}}},
			want: "[browser_navigate: https://example.com]",
		},
		{
			name: "create_files",
			tr:   core.ToolResultPart{Name: "create_files", ToolCallID: "tc3", Content: []core.ContentParter{core.TextPart{Text: `{"created":["a.go","b.go"]}`}}},
			want: "[create_files: 2 files created]",
		},
		{
			name: "web_scraping",
			tr:   core.ToolResultPart{Name: "web_scraping", ToolCallID: "tc4", Content: []core.ContentParter{core.TextPart{Text: strings.Repeat("x", 500)}}},
			want: "[web_scraping: 500 chars extracted]",
		},
		{
			name: "session_search",
			tr:   core.ToolResultPart{Name: "session_search", ToolCallID: "tc5", Content: []core.ContentParter{core.TextPart{Text: `{"results":[{"id":1},{"id":2}]}`}}},
			want: "[session_search: 2 results]",
		},
		{
			name: "unknown_tool",
			tr:   core.ToolResultPart{Name: "unknown_tool", ToolCallID: "tc6", Content: []core.ContentParter{core.TextPart{Text: "some output"}}},
			want: "[tool_result tc6: 11 chars, 1 lines]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.summarizeToolResult(tt.tr)
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPruneToolResultsUsesRedactPatterns(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cfg.RedactionEnabled = true
	cfg.RedactPatterns = []*regexp.Regexp{regexp.MustCompile(`SECRET_\d+`)}
	c := NewDefaultCompressor(cfg, nil)

	msgs := []core.Message{{
		Role: core.MESSAGE_ROLE_TOOL,
		Content: []core.ContentParter{core.ToolResultPart{
			Name:       "some_tool",
			ToolCallID: "tc1",
			Content:    []core.ContentParter{core.TextPart{Text: "SECRET_123 data"}},
		}},
	}}
	out := c.pruneToolResults(msgs)
	tr := out[0].Content[0].(core.ToolResultPart)
	if strings.Contains(toolResultText(tr), "SECRET_123") {
		t.Fatal("expected SECRET_123 to be redacted")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/compression/ -run "TestSummarizeToolResultPerToolTemplates|TestPruneToolResultsUsesRedactPatterns" -v`
Expected: FAIL — `summarizeToolResult` is not dispatching by tool name; `pruneToolResults` uses `redact.String` not `applyRedaction`.

- [ ] **Step 3: Implement per-tool dispatch**

In `agent/compression/prune.go`, replace `summarizeToolResult`:

```go
func summarizeToolResult(tr core.ToolResultPart) string {
	text := toolResultText(tr)
	lines := strings.Count(text, "\n")

	switch tr.Name {
	case "terminal":
		return fmt.Sprintf("[terminal_output: %d lines, %d chars]", lines, len(text))
	case "browser_navigate":
		url := extractJSONField(text, "url")
		if url == "" {
			url = extractJSONField(text, "title")
		}
		return fmt.Sprintf("[browser_navigate: %s]", url)
	case "create_files":
		count := countJSONArrayItems(text, "created")
		return fmt.Sprintf("[create_files: %d files created]", count)
	case "web_scraping":
		return fmt.Sprintf("[web_scraping: %d chars extracted]", len(text))
	case "session_search":
		count := countJSONArrayItems(text, "results")
		return fmt.Sprintf("[session_search: %d results]", count)
	default:
		return fmt.Sprintf("[tool_result %s: %d chars, %d lines]", tr.ToolCallID, len(text), lines)
	}
}
```

Add helpers to `prune.go` (or `helpers.go` if reusable):

```go
func extractJSONField(jsonText, field string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(jsonText), &m); err != nil {
		return ""
	}
	v, ok := m[field].(string)
	if ok {
		return v
	}
	// also try if it's nested under a "result" or similar
	if vm, ok := m[field].(map[string]any); ok {
		if s, ok := vm["url"].(string); ok {
			return s
		}
	}
	return ""
}

func countJSONArrayItems(jsonText, field string) int {
	var m map[string]any
	if err := json.Unmarshal([]byte(jsonText), &m); err != nil {
		return 0
	}
	arr, ok := m[field].([]any)
	if ok {
		return len(arr)
	}
	return 0
}
```

- [ ] **Step 4: Wire applyRedaction into pruneToolResults**

In `agent/compression/prune.go`, replace the `redact.String` call block:

```go
	// Redact secrets in tool results before dedup
	if c.cfg.RedactionEnabled {
		for i := range messages {
			for j := range messages[i].Content {
				if tr, ok := messages[i].Content[j].(core.ToolResultPart); ok {
					messages[i].Content[j] = redactToolResultWithPatterns(c, tr)
				}
			}
		}
	}
```

Replace `redactToolResult` with `redactToolResultWithPatterns`:

```go
func redactToolResultWithPatterns(c *DefaultCompressor, tr core.ToolResultPart) core.ToolResultPart {
	redacted := make([]core.ContentParter, len(tr.Content))
	for i, p := range tr.Content {
		if tp, ok := p.(core.TextPart); ok {
			redacted[i] = core.TextPart{Text: c.applyRedaction(tp.Text)}
		} else {
			redacted[i] = p
		}
	}
	tr.Content = redacted
	return tr
}
```

Remove the old `redactToolResult` function if no longer referenced. Update imports: `redact` package import can be removed from `prune.go` if it was only used by the old `redactToolResult`.

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/compression/ -run "TestSummarizeToolResultPerToolTemplates|TestPruneToolResultsUsesRedactPatterns" -v`
Expected: PASS

- [ ] **Step 6: Whole-tree typecheck**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go vet ./agent/...`
Expected: no errors.

- [ ] **Step 7: Commit**

```bash
cd /Users/ranwei/workspace/go_work/pantheon
git add agent/compression/prune.go agent/compression/prune_test.go
git commit -m "feat(compression): per-tool summary templates + injectable redaction in prune

- Add tool-name-aware templates for terminal, browser_navigate,
  create_files, web_scraping, session_search
- Fallback to generic format for unknown tools
- Wire applyRedaction into pruneToolResults via redactToolResultWithPatterns"
```

---

### Task P5: assemble.go — summary prefix + end marker

**Depends on:** none (independent)

**Files:**
- Modify: `agent/compression/assemble.go`
- Modify: `agent/compression/summary.go` (uses `summaryPrefix` constant)
- Test: `agent/compression/assemble_test.go`

**Context:** When a summary is injected into the message list, it should be clearly demarcated so the LLM knows it is background reference, not active instructions.

- [ ] **Step 1: Write the failing test**

Add to `agent/compression/assemble_test.go`:

```go
func TestAssemblePrefixesSummary(t *testing.T) {
	cfg := DefaultCompressionConfig()
	c := NewDefaultCompressor(cfg, nil)

	head := []core.Message{core.NewTextMessage(core.MESSAGE_ROLE_SYSTEM, "sys")}
	tail := []core.Message{core.NewTextMessage(core.MESSAGE_ROLE_USER, "user")}
	summary := "task: do X"

	result := c.assemble(head, tail, summary)

	// Find the assistant message that carries the summary
	var found bool
	for _, m := range result {
		if m.Role == core.MESSAGE_ROLE_ASSISTANT {
			text := m.Text()
			if !strings.HasPrefix(text, "=== CONTEXT SUMMARY") {
				t.Fatalf("expected summary prefix, got: %q", text)
			}
			if !strings.Contains(text, "=== END CONTEXT SUMMARY ===") {
				t.Fatalf("expected end marker, got: %q", text)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("expected an assistant summary message")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/compression/ -run TestAssemblePrefixesSummary -v`
Expected: FAIL — `assemble` does not prefix with `summaryPrefix` or append end marker.

- [ ] **Step 3: Implement prefix and end marker**

In `agent/compression/assemble.go`, replace the `assemble` method:

```go
const endMarker = "=== END CONTEXT SUMMARY ==="

func (c *DefaultCompressor) assemble(head, tail []core.Message, summary string) []core.Message {
	// Add compaction note to system prompt in head
	for i := range head {
		if head[i].Role == core.MESSAGE_ROLE_SYSTEM {
			head[i].Content = append(head[i].Content, core.TextPart{
				Text: "[Context has been compressed. A summary of earlier turns follows.]",
			})
		}
	}

	summaryText := summaryPrefix + summary + "\n" + endMarker
	summaryMsg := core.Message{
		Role:    core.MESSAGE_ROLE_ASSISTANT,
		Content: core.NewTextContent(summaryText),
	}

	// Avoid consecutive same-role messages
	if len(tail) > 0 && tail[0].Role == core.MESSAGE_ROLE_ASSISTANT {
		tail[0].Content = append(summaryMsg.Content, tail[0].Content...)
		result := make([]core.Message, 0, len(head)+len(tail))
		result = append(result, head...)
		result = append(result, tail...)
		return result
	}

	result := make([]core.Message, 0, len(head)+1+len(tail))
	result = append(result, head...)
	result = append(result, summaryMsg)
	result = append(result, tail...)
	return result
}
```

Note: `summaryPrefix` is already defined in `summary.go` as `const summaryPrefix = "=== CONTEXT SUMMARY (background reference, NOT active instructions) ===\n"`. The `endMarker` constant is new and can live in `assemble.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/compression/ -run TestAssemblePrefixesSummary -v`
Expected: PASS

- [ ] **Step 5: Whole-tree typecheck**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go vet ./agent/...`
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
cd /Users/ranwei/workspace/go_work/pantheon
git add agent/compression/assemble.go agent/compression/assemble_test.go
git commit -m "feat(compression): prefix summary with marker and append end marker

- assemble now wraps summary with summaryPrefix and endMarker
- Makes it explicit to the LLM that the summary is background reference"
```

---

## Phase 2: Usage Calibration & Agent Wiring

### Task P6: compressor.go — extend UpdateFromResponse for real usage recording

**Depends on:** none (independent)

**Files:**
- Modify: `agent/compression/compressor.go`
- Test: `agent/compression/compressor_test.go`

**Context:** `UpdateFromResponse` currently only initializes token budgets on first call. Hermind's calibration feature (Task E3) needs the compressor to record real prompt/completion usage so it can adjust thresholds. We add usage accumulation without breaking the existing "init once" behavior.

- [ ] **Step 1: Write the failing test**

Add to `agent/compression/compressor_test.go`:

```go
func TestUpdateFromResponseRecordsUsage(t *testing.T) {
	c := NewDefaultCompressor(DefaultCompressionConfig(), nil)
	c.UpdateModel("gpt-4", 8192)

	usage1 := core.Usage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150}
	err := c.UpdateFromResponse(usage1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After first call, thresholdTokens should be initialized
	if c.thresholdTokens == 0 {
		t.Fatal("expected thresholdTokens to be initialized after first UpdateFromResponse")
	}

	usage2 := core.Usage{PromptTokens: 200, CompletionTokens: 100, TotalTokens: 300}
	err = c.UpdateFromResponse(usage2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Total recorded usage should be accumulated
	if c.state.totalPromptTokens != 300 {
		t.Fatalf("expected totalPromptTokens=300, got %d", c.state.totalPromptTokens)
	}
	if c.state.totalCompletionTokens != 150 {
		t.Fatalf("expected totalCompletionTokens=150, got %d", c.state.totalCompletionTokens)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/compression/ -run TestUpdateFromResponseRecordsUsage -v`
Expected: FAIL — `compressionState` lacks `totalPromptTokens` / `totalCompletionTokens`.

- [ ] **Step 3: Add usage tracking fields**

In `agent/compression/compressor.go`, add to `compressionState`:

```go
type compressionState struct {
	previousSummary           string
	lastCompressionSavingsPct float64
	ineffectiveCount          int
	summaryCooldownUntil      time.Time
	lastSummaryError          error
	lastFallbackUsed          bool
	totalPromptTokens         int
	totalCompletionTokens     int
}
```

Modify `UpdateFromResponse`:

```go
// UpdateFromResponse initializes token budgets from the first usage response
// and accumulates real usage for calibration.
func (c *DefaultCompressor) UpdateFromResponse(usage core.Usage) error {
	if c.thresholdTokens == 0 && c.contextLength > 0 {
		c.thresholdTokens = int(float64(c.contextLength) * c.cfg.Threshold)
		c.tailTokenBudget = int(float64(c.thresholdTokens) * c.cfg.SummaryTargetRatio)
		c.maxSummaryTokens = min(int(float64(c.contextLength)*0.05), 12000)
	}
	c.state.totalPromptTokens += usage.PromptTokens
	c.state.totalCompletionTokens += usage.CompletionTokens
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/compression/ -run TestUpdateFromResponseRecordsUsage -v`
Expected: PASS

- [ ] **Step 5: Whole-tree typecheck**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go vet ./agent/...`
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
cd /Users/ranwei/workspace/go_work/pantheon
git add agent/compression/compressor.go agent/compression/compressor_test.go
git commit -m "feat(compression): accumulate real usage in UpdateFromResponse

- Add totalPromptTokens and totalCompletionTokens to compressionState
- UpdateFromResponse now accumulates usage for calibration"
```

---

### Task P7: agent.go + stream.go — call UpdateFromResponse after each step

**Depends on:** Task P6

**Files:**
- Modify: `agent/agent.go`
- Modify: `agent/stream.go`
- Test: `agent/compression/compressor_test.go` (integration test verifying calls)

**Context:** The agent loop compresses before each step, but never tells the compressor how many tokens the step actually consumed. Calling `UpdateFromResponse` after each LLM response lets the compressor calibrate its threshold from real data.

- [ ] **Step 1: Write the failing test**

Add an integration-style test in `agent/compression/compressor_test.go`:

```go
func TestUpdateFromResponseCalledByAgent(t *testing.T) {
	// This is a lightweight check that the agent package compiles with the
	// new call sites. Full behavioral test lives in agent_test.go.
	// We verify the method exists on the interface.
	var eng compression.ContextEngine = NewDefaultCompressor(DefaultCompressionConfig(), nil)
	_ = eng.UpdateFromResponse(core.Usage{PromptTokens: 10})
}
```

The real verification is a compile-time one: if the agent code calls `a.contextEngine.UpdateFromResponse(...)` and the interface does not have it, it won't compile. Since `UpdateFromResponse` is already on `ContextEngine`, the compile check is sufficient.

- [ ] **Step 2: Add call in agent.go Run**

In `agent/agent.go`, after the `stepModel.Generate` call (around line 268), add:

```go
	resp, err := stepModel.Generate(ctx, &core.Request{...})
	if err != nil {
		return nil, err
	}

	if a.contextEngine != nil {
		_ = a.contextEngine.UpdateFromResponse(resp.Usage)
	}
```

This goes immediately after the `resp, err := stepModel.Generate(...)` block and before the usage accumulation lines (`totalUsage.PromptTokens += resp.Usage.PromptTokens`).

- [ ] **Step 3: Add call in stream.go RunStream**

In `agent/stream.go`, after the stream loop finishes (around line 315, after the `core.StreamPartTypeFinish` case), the `usage` variable holds the accumulated usage. Add the call right after the stream loop ends, before the defensive reasoning-end block:

```go
			// After stream consumed, update compressor with real usage.
			if a.contextEngine != nil {
				_ = a.contextEngine.UpdateFromResponse(usage)
			}
```

Specifically, insert this block after the `for part, err := range stream { ... }` loop closes, around line 316 (before the `// Defensive: if provider emitted reasoning_start...` comment).

- [ ] **Step 4: Verify compilation**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go build ./agent/...`
Expected: success.

- [ ] **Step 5: Run existing agent tests**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/... -v`
Expected: PASS (existing tests should not break — `UpdateFromResponse` is a no-op when `contextEngine` is nil, which is the default).

- [ ] **Step 6: Commit**

```bash
cd /Users/ranwei/workspace/go_work/pantheon
git add agent/agent.go agent/stream.go agent/compression/compressor_test.go
git commit -m "feat(agent): call UpdateFromResponse after each step

- Wire contextEngine.UpdateFromResponse(resp.Usage) in Run (agent.go)
- Wire contextEngine.UpdateFromResponse(usage) in RunStream (stream.go)
- Enables real-token calibration for compression thresholds"
```

---

## Phase 3: Verification

### Task P8: Whole-tree typecheck + full test run

**Depends on:** Tasks P1–P7

**Files:** all modified in P1–P7

- [ ] **Step 1: Full typecheck including tests**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go vet ./agent/...`
Expected: no errors.

- [ ] **Step 2: Run all agent/compression tests**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/compression/... -v`
Expected: PASS

- [ ] **Step 3: Run all agent tests**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
cd /Users/ranwei/workspace/go_work/pantheon
git commit --allow-empty -m "chore(compression): verify all upstream gap-fill changes

- go vet ./agent/... clean
- go test ./agent/compression/... pass
- go test ./agent/... pass"
```

---

## Self-Review

Reproduce all seven as `- [ ]` checkboxes — do not shrink to five:

- [ ] **1. Spec coverage (build the table).**

| Spec section | Task(s) | Status |
|---|---|---|
| §10.1 accessors (PreviousSummary/SetPreviousSummary/LastFallbackUsed) | P1 | covered |
| §10.1 600s cooldown tier | P1 | covered |
| §10.4 fallback model retry | P1 | covered |
| §10.6 injectable redact patterns | P2, P3, P4 | covered |
| §10.2 per-message 6000-char truncation | P3 | covered |
| §10.3 per-tool summary templates | P4 | covered |
| §10.5 summary prefix + end marker | P5 | covered |
| §10.7 UpdateFromResponse usage accumulation | P6 | covered |
| §10.7 UpdateFromResponse agent loop calls | P7 | covered |

- [ ] **2. Placeholder scan:** No TODO/TBD/deferred-by-dependency excuses found.
- [ ] **3. No phantom tasks:** Every task produces a verifiable change (code + test + commit).
- [ ] **4. Dependency soundness:** P1 has no deps. P2 has no deps. P3 depends on P2 (RedactPatterns). P4 depends on P2 (RedactPatterns). P5 has no deps. P6 has no deps. P7 depends on P6. P8 depends on P1–P7. All satisfied.
- [ ] **5. Caller & build soundness:**
  - P1 adds methods to `DefaultCompressor` (additive only, no stale callers).
  - P2 adds a struct field (additive only).
  - P3–P7 modify internal methods; no exported signatures changed except additive methods.
  - Every task ends with `go vet ./agent/...` which typechecks `_test.go` files too.
- [ ] **6. Test-the-risk:**
  - P1 tests cooldown tier (state mutation) and fallback retry (behavioral).
  - P3 tests truncation (boundary) and redact pattern override.
  - P4 tests per-tool dispatch and redaction in prune.
  - P5 tests prefix/end marker assembly.
  - P6 tests usage accumulation.
  - P7 is compile-time verification.
- [ ] **7. Type consistency:** `applyRedaction` used consistently in P3 and P4. `summaryPrefix` used in P5. `UpdateFromResponse` signature unchanged, only body extended.
