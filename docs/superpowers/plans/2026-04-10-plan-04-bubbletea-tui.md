# Plan 4: Bubbletea TUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Plan 1 `bufio.Scanner` REPL with a proper bubbletea-based TUI featuring a 4-zone layout, streaming rendering, tool call formatting, glamour markdown, and keyboard shortcuts.

**Architecture:** bubbletea Model-View-Update loop running in a goroutine alongside the Engine. The Engine's stream-delta and tool callbacks post `tea.Msg` values to the running program via `tea.Program.Send`. A single `runConversation` goroutine is spawned per user input and sends completion/error messages back. Rendering uses lipgloss for styling and glamour for assistant markdown responses. Input uses the `textarea` bubble component.

**Tech Stack:** `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/bubbles/textarea`, `github.com/charmbracelet/bubbles/viewport`, `github.com/charmbracelet/lipgloss`, `github.com/charmbracelet/glamour`. Extends Plans 1-3: uses existing `agent`, `provider`, `config`, `storage`, `tool` packages.

**Deliverable:**
```
┌─────────────────────────────────────────────────────────────┐
│   HERMES AGENT                                              │
│   claude-opus-4-6 · session abc12345                        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  > What files are in this directory?                        │
│                                                             │
│  ◆ Thinking...                                              │
│                                                             │
│  ⚡ list_directory: {"path":"."}                            │
│  │ {"entries": [...], "path": "."}                          │
│  └                                                          │
│                                                             │
│  The directory contains:                                    │
│  • agent/ — core engine                                     │
│  • cli/ — user interface                                    │
│  ...                                                        │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  > type your message here... ▊                              │
├─────────────────────────────────────────────────────────────┤
│  tokens: 1.2k↑ 340↓  cost: $0.08  ◈ idle                   │
└─────────────────────────────────────────────────────────────┘
```

Keyboard shortcuts:
- `Enter` — submit message
- `Shift+Enter` — newline in input
- `Ctrl+C` — interrupt current operation (cancel streaming/tool)
- `Ctrl+D` — exit with session summary (only when input is empty)
- `Ctrl+L` — clear screen (keep history)

**Non-goals for this plan (deferred):**
- Slash command Tab autocomplete (typed commands still work)
- Up/Down arrow history navigation (textarea native behavior only)
- 3 separate skins with config switching (Default truecolor + Minimal no-color only)
- Responsive width breakpoints (assumes >= 80 cols)
- Blinking cursor during streaming
- Ctrl+A/E/K/U line editing (native textarea only)

These go to Plan 4b or are left for the existing terminal/textarea defaults.

**Plans 1-3 dependencies this plan touches:**
- `cli/repl.go` — REPLACED (the bufio.Scanner loop is gone, replaced with a tea.Program.Run call via `ui.Run`)
- Engine is used unchanged — its callbacks now post tea.Msg values

---

## File Structure

```
hermes-agent-go/cli/
├── app.go                         # (unchanged)
├── root.go                        # (unchanged)
├── run.go                         # (unchanged)
├── repl.go                        # MODIFIED: runREPL delegates to ui.Run
├── repl_test.go                   # (unchanged)
├── repl_tool_test.go              # (unchanged)
└── ui/
    ├── skin.go                    # Skin struct + 2 built-in skins (Default, Minimal)
    ├── skin_test.go
    ├── messages.go                # tea.Msg types: streamDelta, toolStart, toolResult, convDone, convError
    ├── model.go                   # Model struct + Init
    ├── update.go                  # Update: key events + tea.Msg handlers
    ├── view.go                    # View: 4-zone layout rendering
    ├── renderer.go                # Markdown (glamour) + tool call rendering
    ├── banner.go                  # ASCII art banner + context bar
    ├── status_bar.go              # Status bar rendering
    ├── slash.go                   # Slash command dispatch (/exit, /clear, /help)
    ├── run.go                     # Run(): constructs Model, starts tea.Program, wires Engine goroutine
    └── ui_test.go                 # Model update tests
```

---

## Task 1: Add TUI Dependencies

**Files:**
- Modify: `hermes-agent-go/go.mod` (via `go get`)

- [ ] **Step 1: Add all required dependencies**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/charmbracelet/glamour@latest
```

- [ ] **Step 2: Verify module still compiles**

```bash
go build ./...
```

Expected: success (no new code yet, just dependencies).

- [ ] **Step 3: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/go.mod hermes-agent-go/go.sum
git commit -m "chore(cli): add bubbletea, bubbles, lipgloss, glamour dependencies"
```

---

## Task 2: Skin System with Auto-Detection

**Files:**
- Create: `hermes-agent-go/cli/ui/skin.go`
- Create: `hermes-agent-go/cli/ui/skin_test.go`

- [ ] **Step 1: Write failing tests**

```go
// cli/ui/skin_test.go
package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultSkinHasColors(t *testing.T) {
	s := DefaultSkin()
	assert.Equal(t, "default", s.Name)
	assert.NotEmpty(t, s.Accent)
	assert.NotEmpty(t, s.Error)
}

func TestMinimalSkinHasNoColors(t *testing.T) {
	s := MinimalSkin()
	assert.Equal(t, "minimal", s.Name)
}

func TestDetectSkinFromTruecolor(t *testing.T) {
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("TERM", "xterm-256color")
	s := DetectSkin()
	assert.Equal(t, "default", s.Name)
}

func TestDetectSkinFromDumbTerminal(t *testing.T) {
	t.Setenv("COLORTERM", "")
	t.Setenv("TERM", "dumb")
	s := DetectSkin()
	assert.Equal(t, "minimal", s.Name)
}
```

- [ ] **Step 2: Implement the skin system**

```go
// cli/ui/skin.go
package ui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Skin holds the visual tokens for the TUI — colors, spinner, prompt character.
// Two built-in skins: Default (truecolor amber on charcoal) and Minimal (no color).
type Skin struct {
	Name string

	// Lipgloss styles
	Accent      lipgloss.Style // amber/gold for headings, prompt character
	Success     lipgloss.Style // teal for successful tool results
	Warning     lipgloss.Style // yellow for budget warnings
	Error       lipgloss.Style // soft red for errors
	Muted       lipgloss.Style // gray for metadata, tool commands
	Code        lipgloss.Style // green for code blocks, file paths
	Border      lipgloss.Style // zone border style

	// Character tokens
	PromptChar     string // ">"
	ThinkingChar   string // "◆"
	ToolChar       string // "⚡"
	OutputPrefix   string // "│"
	OutputEnd      string // "└"
	StreamingChar  string // "▊"
	WarningChar    string // "⚠"
	ErrorChar      string // "✗"
	ActiveChar     string // "◈"
}

// DefaultSkin returns the truecolor amber/gold skin.
func DefaultSkin() Skin {
	return Skin{
		Name: "default",

		Accent:  lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB800")).Bold(true),
		Success: lipgloss.NewStyle().Foreground(lipgloss.Color("#4EC9B0")),
		Warning: lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B")),
		Error:   lipgloss.NewStyle().Foreground(lipgloss.Color("#E06C75")),
		Muted:   lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7A89")),
		Code:    lipgloss.NewStyle().Foreground(lipgloss.Color("#98C379")),
		Border:  lipgloss.NewStyle().Foreground(lipgloss.Color("#3B4252")),

		PromptChar:    ">",
		ThinkingChar:  "◆",
		ToolChar:      "⚡",
		OutputPrefix:  "│",
		OutputEnd:     "└",
		StreamingChar: "▊",
		WarningChar:   "⚠",
		ErrorChar:     "✗",
		ActiveChar:    "◈",
	}
}

// MinimalSkin returns a no-color skin for dumb terminals and CI/CD output.
// Uses only bold/dim styling and ASCII-safe characters.
func MinimalSkin() Skin {
	plain := lipgloss.NewStyle()
	bold := lipgloss.NewStyle().Bold(true)
	faint := lipgloss.NewStyle().Faint(true)

	return Skin{
		Name: "minimal",

		Accent:  bold,
		Success: plain,
		Warning: bold,
		Error:   bold,
		Muted:   faint,
		Code:    plain,
		Border:  faint,

		PromptChar:    ">",
		ThinkingChar:  "*",
		ToolChar:      ">",
		OutputPrefix:  "|",
		OutputEnd:     "+",
		StreamingChar: "_",
		WarningChar:   "!",
		ErrorChar:     "X",
		ActiveChar:    "o",
	}
}

// DetectSkin picks a skin based on terminal capability env vars.
// Returns Minimal for dumb terminals, Default otherwise.
func DetectSkin() Skin {
	term := os.Getenv("TERM")
	if term == "dumb" || term == "" {
		return MinimalSkin()
	}
	// Also respect NO_COLOR (https://no-color.org/)
	if os.Getenv("NO_COLOR") != "" {
		return MinimalSkin()
	}
	return DefaultSkin()
}
```

- [ ] **Step 3: Run tests**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go test -race ./cli/ui/...
```

Expected: PASS, 4 tests.

- [ ] **Step 4: Commit**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git add hermes-agent-go/cli/ui/skin.go hermes-agent-go/cli/ui/skin_test.go
git commit -m "feat(cli/ui): add Skin with Default/Minimal variants and auto-detection"
```

---

## Task 3: Tea Message Types

**Files:**
- Create: `hermes-agent-go/cli/ui/messages.go`

- [ ] **Step 1: Create the message types file**

```go
// cli/ui/messages.go
package ui

import (
	"github.com/nousresearch/hermes-agent/agent"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
)

// Tea message types posted from the Engine goroutine back to the bubbletea Program.
// The Update handler reads these and mutates the Model accordingly.

// streamDeltaMsg is posted for each streaming content chunk from the LLM.
type streamDeltaMsg struct {
	Delta *provider.StreamDelta
}

// toolStartMsg is posted when a tool begins executing.
type toolStartMsg struct {
	Call message.ContentBlock // type="tool_use"
}

// toolResultMsg is posted when a tool finishes executing.
type toolResultMsg struct {
	Call   message.ContentBlock // the original tool_use block
	Result string               // JSON result (possibly truncated for display)
}

// convDoneMsg is posted when a conversation turn completes.
// Carries the full result so the Model can update totals.
type convDoneMsg struct {
	Result *agent.ConversationResult
}

// convErrorMsg is posted if the Engine returns an error.
type convErrorMsg struct {
	Err error
}

// tickMsg is a periodic tick used for the streaming cursor animation.
// Sent by a tea.Tick command; Update handles it by toggling the cursor state.
type tickMsg struct{}

// quitMsg signals the program should exit cleanly (e.g. from /exit).
type quitMsg struct{}
```

- [ ] **Step 2: Build to verify**

```bash
go build ./cli/ui/...
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add hermes-agent-go/cli/ui/messages.go
git commit -m "feat(cli/ui): add tea.Msg types for async Engine events"
```

---

## Task 4: Model + Init

**Files:**
- Create: `hermes-agent-go/cli/ui/model.go`

- [ ] **Step 1: Create the Model struct**

```go
// cli/ui/model.go
package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nousresearch/hermes-agent/agent"
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/storage"
	"github.com/nousresearch/hermes-agent/tool"
)

// State represents the overall REPL state.
type State int

const (
	StateIdle       State = iota // waiting for user input
	StateStreaming              // Engine is streaming a response
	StateToolRunning            // A tool is currently executing
)

// Model holds all TUI state. Single instance per REPL session.
type Model struct {
	// Resources (injected at construction, never mutated)
	cfg       *config.Config
	storage   storage.Storage
	provider  provider.Provider
	toolReg   *tool.Registry
	agentCfg  config.AgentConfig
	skin      Skin
	sessionID string
	model     string

	// bubbletea components
	viewport viewport.Model
	input    textarea.Model

	// REPL state
	state    State
	history  []message.Message
	rendered []string // pre-rendered conversation lines (for viewport)

	// Streaming accumulator for the current in-progress assistant message.
	streamingText strings.Builder
	streamingTool *streamingToolState

	// Totals shown in the status bar
	totalUsage   message.Usage
	turnCount    int
	toolCalls    int
	totalCostUSD float64

	// Error to display in the next render (cleared after display)
	err error

	// Terminal size
	width  int
	height int

	// Quit flag — set when the user invokes /exit
	quitting bool
}

// streamingToolState tracks the currently-running tool call for the UI.
type streamingToolState struct {
	Name  string
	Input string
}

// ModelConfig holds the dependencies needed to construct a Model.
type ModelConfig struct {
	Config    *config.Config
	Storage   storage.Storage
	Provider  provider.Provider
	ToolReg   *tool.Registry
	AgentCfg  config.AgentConfig
	Skin      Skin
	SessionID string
	Model     string
}

// NewModel constructs a fresh Model.
func NewModel(mc ModelConfig) Model {
	ta := textarea.New()
	ta.Placeholder = "type your message, Enter to send, Shift+Enter for newline"
	ta.SetHeight(3)
	ta.Focus()
	ta.ShowLineNumbers = false
	ta.CharLimit = 0 // unlimited
	ta.KeyMap.InsertNewline.SetKeys("shift+enter")

	vp := viewport.New(80, 20)

	return Model{
		cfg:       mc.Config,
		storage:   mc.Storage,
		provider:  mc.Provider,
		toolReg:   mc.ToolReg,
		agentCfg:  mc.AgentCfg,
		skin:      mc.Skin,
		sessionID: mc.SessionID,
		model:     mc.Model,
		viewport:  vp,
		input:     ta,
		state:     StateIdle,
		width:     80,
		height:    24,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

// conversationResult is a helper used by Run() to wrap an Engine call as a tea.Cmd.
// This is not used directly by the Model — it lives here so update.go can dispatch
// to a goroutine via the command pattern.
func (m Model) makeEngineCmd(userMessage string) tea.Cmd {
	return func() tea.Msg {
		// This cmd is replaced with a streaming-aware goroutine in Run().
		// Left as a placeholder so Model + Update can compile independently.
		return convErrorMsg{Err: nil}
	}
}

// appendRenderedLine adds a pre-rendered line to the conversation log.
// Caller is responsible for applying skin styling before passing the string.
func (m *Model) appendRenderedLine(line string) {
	m.rendered = append(m.rendered, line)
	m.viewport.SetContent(strings.Join(m.rendered, "\n"))
	m.viewport.GotoBottom()
}

// formatBytesInt formats large token counts as e.g. "1.2k".
func formatBytesInt(n int) string {
	if n < 1000 {
		return itoa(n)
	}
	if n < 1_000_000 {
		return itoa(n/1000) + "." + itoa((n%1000)/100) + "k"
	}
	return itoa(n/1_000_000) + "." + itoa((n%1_000_000)/100_000) + "M"
}

// itoa is a zero-alloc int → string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
```

- [ ] **Step 2: Build to verify**

```bash
go build ./cli/ui/...
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add hermes-agent-go/cli/ui/model.go
git commit -m "feat(cli/ui): add Model struct with textarea and viewport"
```

---

## Task 5: View Rendering (4-Zone Layout)

**Files:**
- Create: `hermes-agent-go/cli/ui/view.go`
- Create: `hermes-agent-go/cli/ui/banner.go`
- Create: `hermes-agent-go/cli/ui/status_bar.go`

- [ ] **Step 1: Create `cli/ui/banner.go`**

```go
// cli/ui/banner.go
package ui

import "strings"

// bannerASCII is the stylized HERMES logo shown at the top of the REPL.
// Kept short (3 lines) so it doesn't dominate the viewport on small terminals.
const bannerASCII = `╭─────────────────────────╮
│    HERMES AGENT         │
╰─────────────────────────╯`

// renderBanner returns the banner styled by the current skin.
func (m Model) renderBanner() string {
	return m.skin.Accent.Render(bannerASCII)
}

// renderContextBar returns the "claude-opus-4-6 · session abc12345" line.
func (m Model) renderContextBar() string {
	return m.skin.Muted.Render("  " + m.model + "  ·  session " + shortSessionID(m.sessionID))
}

// shortSessionID returns the first 8 characters of a session UUID.
func shortSessionID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// Silence unused import
var _ = strings.Builder{}
```

- [ ] **Step 2: Create `cli/ui/status_bar.go`**

```go
// cli/ui/status_bar.go
package ui

import "strings"

// renderStatusBar builds the bottom status line: "tokens: 1.2k↑ 340↓  cost: $0.08  ◈ state".
func (m Model) renderStatusBar() string {
	var state string
	switch m.state {
	case StateIdle:
		state = m.skin.Muted.Render(m.skin.ActiveChar + " idle")
	case StateStreaming:
		state = m.skin.Accent.Render(m.skin.ActiveChar + " streaming")
	case StateToolRunning:
		state = m.skin.Success.Render(m.skin.ActiveChar + " tool running")
	}

	tokens := "tokens: " + formatBytesInt(m.totalUsage.InputTokens) + "↑ " + formatBytesInt(m.totalUsage.OutputTokens) + "↓"
	cost := "cost: $" + formatCost(m.totalCostUSD)

	parts := []string{
		m.skin.Muted.Render(tokens),
		m.skin.Muted.Render(cost),
		state,
	}
	return strings.Join(parts, "  ")
}

// formatCost returns a dollar amount with 3 decimal places.
func formatCost(c float64) string {
	if c == 0 {
		return "0.000"
	}
	// Cheap formatter: round to milli-dollars.
	n := int(c * 1000)
	dollars := n / 1000
	cents := n % 1000
	// Pad cents to 3 digits
	centsStr := itoa(cents)
	for len(centsStr) < 3 {
		centsStr = "0" + centsStr
	}
	return itoa(dollars) + "." + centsStr
}
```

- [ ] **Step 3: Create `cli/ui/view.go`**

```go
// cli/ui/view.go
package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View implements tea.Model. Renders the 4-zone layout:
//   Zone 1: Banner (3 lines)
//   Zone 2: Context bar (1 line)
//   Zone 3: Conversation viewport (variable)
//   Zone 4: Input textarea (variable)
//   Zone 5: Status bar (1 line)
func (m Model) View() string {
	if m.quitting {
		return m.renderExitSummary()
	}

	// Fixed-size zones
	banner := m.renderBanner()
	contextBar := m.renderContextBar()
	statusBar := m.renderStatusBar()
	input := m.renderInput()

	// Calculate viewport height: total - banner - context - input - status - separators
	bannerLines := strings.Count(banner, "\n") + 1
	inputLines := strings.Count(input, "\n") + 1
	reserved := bannerLines + 1 /*context*/ + inputLines + 1 /*status*/ + 3 /*separators*/
	vpHeight := m.height - reserved
	if vpHeight < 3 {
		vpHeight = 3
	}
	m.viewport.Width = m.width
	m.viewport.Height = vpHeight

	sep := m.skin.Border.Render(strings.Repeat("─", max(1, m.width)))

	// Assemble
	return strings.Join([]string{
		banner,
		contextBar,
		sep,
		m.viewport.View(),
		sep,
		input,
		sep,
		statusBar,
	}, "\n")
}

// renderInput returns the styled input component.
func (m Model) renderInput() string {
	return m.input.View()
}

// renderExitSummary is shown after /exit before the program terminates.
func (m Model) renderExitSummary() string {
	summary := strings.Builder{}
	summary.WriteString("\n")
	summary.WriteString(m.skin.Accent.Render("Session complete."))
	summary.WriteString("\n")
	summary.WriteString(m.skin.Muted.Render(
		"  " + shortSessionID(m.sessionID) +
			"  ·  " + itoa(m.turnCount*2) + " messages" +
			"  ·  " + itoa(m.toolCalls) + " tool calls" +
			"  ·  " + formatBytesInt(m.totalUsage.InputTokens) + "↑ " + formatBytesInt(m.totalUsage.OutputTokens) + "↓" +
			"  ·  $" + formatCost(m.totalCostUSD),
	))
	summary.WriteString("\n")
	return summary.String()
}

// max returns the larger of two ints.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Silence unused import
var _ = lipgloss.NewStyle
```

- [ ] **Step 4: Build**

```bash
go build ./cli/ui/...
```

Expected: success.

- [ ] **Step 5: Commit**

```bash
git add hermes-agent-go/cli/ui/banner.go hermes-agent-go/cli/ui/status_bar.go hermes-agent-go/cli/ui/view.go
git commit -m "feat(cli/ui): add View with 4-zone layout (banner, context, viewport, input, status)"
```

---

## Task 6: Update Handler (Keys + Tea Messages)

**Files:**
- Create: `hermes-agent-go/cli/ui/update.go`

- [ ] **Step 1: Create the Update function**

```go
// cli/ui/update.go
package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nousresearch/hermes-agent/message"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.input.SetWidth(msg.Width - 2)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			// Ctrl+C: interrupt current op or exit if idle
			if m.state == StateIdle {
				return m.quit()
			}
			// Cancel the in-flight conversation (the goroutine watches the context via CancelFunc held by run.go).
			// For Plan 4, we simply transition back to idle and drop the partial output.
			// Plan 6 wires proper context cancellation through a channel.
			m.state = StateIdle
			m.appendRenderedLine(m.skin.Muted.Render("[interrupted]"))
			return m, nil

		case "ctrl+d":
			// Exit only when the input is empty (so it doesn't eat user text).
			if m.input.Value() == "" && m.state == StateIdle {
				return m.quit()
			}

		case "ctrl+l":
			// Clear the viewport (keep history in memory)
			m.rendered = nil
			m.viewport.SetContent("")
			return m, nil

		case "enter":
			// Submit only if not already streaming
			if m.state != StateIdle {
				return m, nil
			}
			text := m.input.Value()
			if text == "" {
				return m, nil
			}
			m.input.Reset()

			// Handle slash commands inline
			if handled, cmd := m.handleSlashCommand(text); handled {
				return m, cmd
			}

			// Render the user message into the conversation
			m.appendRenderedLine(m.skin.Accent.Render(m.skin.PromptChar+" ") + text)
			m.state = StateStreaming
			m.appendRenderedLine(m.skin.Muted.Render(m.skin.ThinkingChar + " Thinking..."))

			// Start the Engine goroutine — run.go provides the actual implementation
			// via a cmd function stored on the Model. For Plan 4 we return a direct cmd.
			return m, m.startConversation(text)
		}

	case streamDeltaMsg:
		if msg.Delta != nil && msg.Delta.Content != "" {
			m.streamingText.WriteString(msg.Delta.Content)
			// Re-render the last assistant line in-place
			m.refreshStreamingLine()
		}

	case toolStartMsg:
		m.state = StateToolRunning
		m.toolCalls++
		m.streamingTool = &streamingToolState{
			Name:  msg.Call.ToolUseName,
			Input: string(msg.Call.ToolUseInput),
		}
		m.appendRenderedLine(
			m.skin.Muted.Render(m.skin.ToolChar+" "+msg.Call.ToolUseName+": ") +
				m.skin.Code.Render(string(msg.Call.ToolUseInput)),
		)

	case toolResultMsg:
		m.state = StateStreaming
		m.streamingTool = nil
		// Render tool result snippet
		lines := renderToolResult(msg.Result, m.skin)
		for _, l := range lines {
			m.appendRenderedLine(l)
		}

	case convDoneMsg:
		if msg.Result != nil {
			m.history = msg.Result.Messages
			m.totalUsage.InputTokens += msg.Result.Usage.InputTokens
			m.totalUsage.OutputTokens += msg.Result.Usage.OutputTokens
			m.turnCount++
		}
		// Finalize the streaming text as a rendered assistant message
		if m.streamingText.Len() > 0 {
			rendered := renderAssistantText(m.streamingText.String(), m.skin)
			m.replaceLastStreamingLine(rendered)
			m.streamingText.Reset()
		}
		m.state = StateIdle

	case convErrorMsg:
		m.state = StateIdle
		m.streamingText.Reset()
		if msg.Err != nil {
			m.appendRenderedLine(m.skin.Error.Render(m.skin.ErrorChar + " error: " + msg.Err.Error()))
		}
	}

	// Delegate input + viewport updates
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// quit transitions the model to exit state.
func (m Model) quit() (tea.Model, tea.Cmd) {
	m.quitting = true
	return m, tea.Quit
}

// startConversation is a placeholder that run.go replaces with an Engine-backed goroutine.
// Since tea.Cmd closures can't reference a Model method that spawns a goroutine with
// Program.Send, the real implementation lives in run.go where the *tea.Program is available.
func (m Model) startConversation(text string) tea.Cmd {
	// This is intentionally a no-op placeholder; run.go uses a separate path
	// (a dispatcher closure stored on the Model via ModelConfig.StartConversationFn)
	// to actually run the Engine. Tasks 9-10 wire this.
	return nil
}

// refreshStreamingLine replaces the last "thinking" line with the current streaming text.
func (m *Model) refreshStreamingLine() {
	if len(m.rendered) == 0 {
		return
	}
	m.rendered[len(m.rendered)-1] = m.streamingText.String() + m.skin.Muted.Render(m.skin.StreamingChar)
	m.viewport.SetContent(joinRendered(m.rendered))
	m.viewport.GotoBottom()
}

// replaceLastStreamingLine replaces the last line with a final rendered text.
func (m *Model) replaceLastStreamingLine(text string) {
	if len(m.rendered) == 0 {
		m.appendRenderedLine(text)
		return
	}
	m.rendered[len(m.rendered)-1] = text
	m.viewport.SetContent(joinRendered(m.rendered))
	m.viewport.GotoBottom()
}

// joinRendered joins rendered lines with newlines.
func joinRendered(lines []string) string {
	total := 0
	for _, l := range lines {
		total += len(l) + 1
	}
	out := make([]byte, 0, total)
	for i, l := range lines {
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, l...)
	}
	return string(out)
}

// Silence unused imports
var (
	_ = context.Background
	_ = message.RoleUser
)
```

- [ ] **Step 2: Build**

```bash
go build ./cli/ui/...
```

Expected: success (even though `startConversation` is a placeholder — Task 9 wires it).

- [ ] **Step 3: Commit**

```bash
git add hermes-agent-go/cli/ui/update.go
git commit -m "feat(cli/ui): add Update handler for keys and tea messages"
```

---

## Task 7: Slash Commands

**Files:**
- Create: `hermes-agent-go/cli/ui/slash.go`

- [ ] **Step 1: Create the slash command dispatcher**

```go
// cli/ui/slash.go
package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// handleSlashCommand intercepts lines starting with "/" and dispatches
// to the matching command handler. Returns (true, cmd) if the input was
// a recognized slash command.
func (m *Model) handleSlashCommand(input string) (bool, tea.Cmd) {
	if !strings.HasPrefix(input, "/") {
		return false, nil
	}
	parts := strings.Fields(strings.TrimPrefix(input, "/"))
	if len(parts) == 0 {
		return true, nil
	}

	switch parts[0] {
	case "exit", "quit":
		m.quitting = true
		return true, tea.Quit

	case "clear":
		m.rendered = nil
		m.history = nil
		m.viewport.SetContent("")
		m.appendRenderedLine(m.skin.Muted.Render("conversation cleared"))
		return true, nil

	case "help":
		m.appendRenderedLine(m.skin.Accent.Render("Commands:"))
		m.appendRenderedLine(m.skin.Muted.Render("  /help     show this help"))
		m.appendRenderedLine(m.skin.Muted.Render("  /exit     save session and exit"))
		m.appendRenderedLine(m.skin.Muted.Render("  /clear    clear conversation"))
		m.appendRenderedLine(m.skin.Muted.Render("  /model    show active model"))
		m.appendRenderedLine(m.skin.Muted.Render("  /cost     show session cost"))
		return true, nil

	case "model":
		m.appendRenderedLine(m.skin.Muted.Render("model: ") + m.skin.Accent.Render(m.model))
		return true, nil

	case "cost":
		line := "tokens: " + formatBytesInt(m.totalUsage.InputTokens) + "↑ " +
			formatBytesInt(m.totalUsage.OutputTokens) + "↓  cost: $" + formatCost(m.totalCostUSD)
		m.appendRenderedLine(m.skin.Muted.Render(line))
		return true, nil

	default:
		m.appendRenderedLine(m.skin.Error.Render("unknown command: /" + parts[0]))
		return true, nil
	}
}
```

- [ ] **Step 2: Build**

```bash
go build ./cli/ui/...
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add hermes-agent-go/cli/ui/slash.go
git commit -m "feat(cli/ui): add slash command dispatcher (exit/clear/help/model/cost)"
```

---

## Task 8: Renderer (Markdown + Tool Output)

**Files:**
- Create: `hermes-agent-go/cli/ui/renderer.go`

- [ ] **Step 1: Create the renderer helpers**

```go
// cli/ui/renderer.go
package ui

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// renderAssistantText runs the assistant's final markdown text through
// glamour for pretty terminal rendering. If glamour fails for any reason,
// falls back to the raw text.
func renderAssistantText(text string, skin Skin) string {
	if text == "" {
		return ""
	}
	// Use glamour's auto style (picks dark/light based on terminal).
	// For the Minimal skin, use the ascii style.
	style := "auto"
	if skin.Name == "minimal" {
		style = "ascii"
	}

	out, err := glamour.Render(text, style)
	if err != nil {
		return text
	}
	return strings.TrimRight(out, "\n")
}

// renderToolResult formats a tool result as a list of pre-rendered lines.
// Wraps the snippet in `│` prefixes and ends with `└`.
// Truncates to ~12 lines to keep the viewport from overflowing.
func renderToolResult(result string, skin Skin) []string {
	const maxLines = 12
	const maxChars = 600

	snippet := result
	if len(snippet) > maxChars {
		snippet = snippet[:maxChars] + "\n... [truncated]"
	}
	lines := strings.Split(snippet, "\n")
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], "... [+"+itoa(len(strings.Split(snippet, "\n"))-maxLines)+" lines]")
	}

	out := make([]string, 0, len(lines)+1)
	for _, l := range lines {
		out = append(out, skin.Muted.Render(skin.OutputPrefix+" ")+l)
	}
	out = append(out, skin.Muted.Render(skin.OutputEnd))
	return out
}
```

- [ ] **Step 2: Build**

```bash
go build ./cli/ui/...
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add hermes-agent-go/cli/ui/renderer.go
git commit -m "feat(cli/ui): add glamour markdown renderer and tool output formatter"
```

---

## Task 9: Run() Entry Point with Engine Goroutine

**Files:**
- Create: `hermes-agent-go/cli/ui/run.go`

This task wires the bubbletea program to the Engine. The Engine runs in a goroutine; its callbacks post `tea.Msg` values to the running program via `tea.Program.Send`.

- [ ] **Step 1: Create `cli/ui/run.go`**

```go
// cli/ui/run.go
package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nousresearch/hermes-agent/agent"
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/storage"
	"github.com/nousresearch/hermes-agent/tool"
)

// RunOptions holds the dependencies required to start a TUI REPL.
type RunOptions struct {
	Config    *config.Config
	Storage   storage.Storage
	Provider  provider.Provider
	ToolReg   *tool.Registry
	AgentCfg  config.AgentConfig
	SessionID string
	Model     string
}

// Run starts the bubbletea TUI. Blocks until the user exits.
// The Engine is driven by a dispatcher closure stored on the Model that
// sends tea.Msg values back to the running Program via program.Send.
func Run(ctx context.Context, opts RunOptions) error {
	skin := DetectSkin()

	m := NewModel(ModelConfig{
		Config:    opts.Config,
		Storage:   opts.Storage,
		Provider:  opts.Provider,
		ToolReg:   opts.ToolReg,
		AgentCfg:  opts.AgentCfg,
		Skin:      skin,
		SessionID: opts.SessionID,
		Model:     opts.Model,
	})

	// Create the bubbletea program using a pointer so we can pass it to the
	// dispatcher closure (which needs program.Send to post messages from the
	// Engine goroutine).
	var program *tea.Program

	// The dispatcher: called from Update when the user submits a message.
	// It spawns a goroutine that runs the Engine and posts tea.Msg values
	// via program.Send.
	dispatcher := func(userInput string, history []message.Message) {
		go func() {
			engine := agent.NewEngineWithTools(
				opts.Provider, opts.Storage, opts.ToolReg,
				opts.AgentCfg, "cli",
			)
			engine.SetStreamDeltaCallback(func(d *provider.StreamDelta) {
				program.Send(streamDeltaMsg{Delta: d})
			})
			engine.SetToolStartCallback(func(call message.ContentBlock) {
				program.Send(toolStartMsg{Call: call})
			})
			engine.SetToolResultCallback(func(call message.ContentBlock, result string) {
				program.Send(toolResultMsg{Call: call, Result: result})
			})

			result, err := engine.RunConversation(ctx, &agent.RunOptions{
				UserMessage: userInput,
				History:     history,
				SessionID:   opts.SessionID,
				Model:       opts.Model,
			})
			if err != nil {
				program.Send(convErrorMsg{Err: err})
				return
			}
			program.Send(convDoneMsg{Result: result})
		}()
	}

	// Install the dispatcher on the model.
	m.dispatch = dispatcher

	program = tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))

	if _, err := program.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Add the `dispatch` field to the Model**

Modify `cli/ui/model.go`: add the `dispatch` field to the `Model` struct and update `NewModel` to leave it nil (it's installed by `Run()` after construction).

Find the `Model` struct and add this field inside it:

```go
	// dispatch is the function that launches an Engine goroutine for a
	// given user message. Installed by Run() after construction so that
	// the goroutine can reach the *tea.Program instance.
	dispatch func(userInput string, history []message.Message)
```

- [ ] **Step 3: Update the `startConversation` placeholder in `update.go`**

Replace the existing `startConversation` placeholder method with:

```go
// startConversation kicks off an Engine goroutine via the dispatcher
// installed by Run(). The goroutine posts tea.Msg values back to the
// running Program.
func (m Model) startConversation(text string) tea.Cmd {
	if m.dispatch == nil {
		return func() tea.Msg {
			return convErrorMsg{Err: fmt.Errorf("ui: no dispatcher installed")}
		}
	}
	// Fire and forget — the dispatcher spawns its own goroutine.
	m.dispatch(text, m.history)
	return nil
}
```

Add `"fmt"` to the imports of `update.go` if not already there.

- [ ] **Step 4: Build**

```bash
go build ./cli/ui/...
```

Expected: success.

- [ ] **Step 5: Commit**

```bash
git add hermes-agent-go/cli/ui/run.go hermes-agent-go/cli/ui/model.go hermes-agent-go/cli/ui/update.go
git commit -m "feat(cli/ui): add Run() entry point with Engine goroutine dispatcher"
```

---

## Task 10: Wire TUI into runREPL

**Files:**
- Modify: `hermes-agent-go/cli/repl.go`

- [ ] **Step 1: Replace the body of `runREPL`**

The current `runREPL` function contains a `bufio.Scanner` loop that prints to stdout. Replace that loop with a call to `ui.Run`. The provider construction, tool registration, storage setup, and session ID generation all stay the same.

Find the section of `cli/repl.go` that starts the REPL loop (the `for { fmt.Print("> ")` block) and replace everything from that point until the session summary print with:

```go
	// Hand off to the bubbletea TUI.
	err = ui.Run(ctx, ui.RunOptions{
		Config:    app.Config,
		Storage:   app.Storage,
		Provider:  p,
		ToolReg:   toolRegistry,
		AgentCfg:  app.Config.Agent,
		SessionID: sessionID,
		Model:     displayModel,
	})
	if err != nil {
		return fmt.Errorf("hermes: tui: %w", err)
	}
	return nil
}
```

Add the `ui` import at the top of `cli/repl.go`:

```go
	"github.com/nousresearch/hermes-agent/cli/ui"
```

Remove imports no longer used by repl.go:
- `bufio` (no longer reading from stdin here)
- `io` (no longer checking for io.EOF from scanner)
- `strings` (keep if other helpers use it; check imports before removing)

The banner/context print, scanner-based loop, and session summary print all become the TUI's responsibility now. Delete those from `repl.go`. The `ensureStorage`, `buildPrimaryProvider`, `defaultModelFromString` helpers and the `banner` constant stay if they're referenced by other files; otherwise delete the `banner` constant. Check references before deleting.

- [ ] **Step 2: Update the `repl.go` streaming/tool callback blocks**

The existing repl.go has callbacks that print `fmt.Print(d.Content)`, `fmt.Printf("\n⚡ ...")`, and `fmt.Printf("│ ...")`. These are moved into the TUI. Delete them from repl.go.

- [ ] **Step 3: Build**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go build ./...
```

Expected: success.

- [ ] **Step 4: Run existing tests**

```bash
go test -race ./...
```

Expected: PASS. All existing tests still work (`TestEndToEndSingleTurn`, `TestEndToEndToolLoop`, etc.) because they use the Engine directly, not runREPL.

- [ ] **Step 5: Commit**

```bash
git add hermes-agent-go/cli/repl.go
git commit -m "feat(cli): delegate REPL loop to bubbletea TUI via ui.Run"
```

---

## Task 11: Basic Model Update Tests

**Files:**
- Create: `hermes-agent-go/cli/ui/ui_test.go`

- [ ] **Step 1: Write update handler tests**

```go
// cli/ui/ui_test.go
package ui

import (
	"encoding/json"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nousresearch/hermes-agent/agent"
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestModel() Model {
	return NewModel(ModelConfig{
		Config:    &config.Config{},
		AgentCfg:  config.AgentConfig{MaxTurns: 10},
		Skin:      MinimalSkin(),
		SessionID: "test-session",
		Model:     "test-model",
	})
}

func TestSlashExitQuits(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("/exit")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)
	assert.True(t, m2.quitting)
	assert.NotNil(t, cmd)
}

func TestSlashClearResetsHistory(t *testing.T) {
	m := newTestModel()
	m.history = []message.Message{{Role: message.RoleUser, Content: message.TextContent("old")}}
	m.rendered = []string{"line1", "line2"}
	m.input.SetValue("/clear")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)
	assert.Empty(t, m2.history)
	// After clearing, we append a "conversation cleared" line.
	require.NotEmpty(t, m2.rendered)
	assert.Contains(t, m2.rendered[len(m2.rendered)-1], "cleared")
}

func TestSlashHelpShowsCommands(t *testing.T) {
	m := newTestModel()
	m.input.SetValue("/help")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)
	joined := ""
	for _, l := range m2.rendered {
		joined += l + "\n"
	}
	assert.Contains(t, joined, "/exit")
	assert.Contains(t, joined, "/clear")
	assert.Contains(t, joined, "/help")
}

func TestStreamDeltaAppendsToStreaming(t *testing.T) {
	m := newTestModel()
	m.state = StateStreaming
	m.appendRenderedLine("thinking")

	updated, _ := m.Update(streamDeltaMsg{Delta: &provider.StreamDelta{Content: "Hello "}})
	m = updated.(Model)
	updated, _ = m.Update(streamDeltaMsg{Delta: &provider.StreamDelta{Content: "world"}})
	m = updated.(Model)

	assert.Equal(t, "Hello world", m.streamingText.String())
}

func TestConvDoneResetsState(t *testing.T) {
	m := newTestModel()
	m.state = StateStreaming
	m.streamingText.WriteString("some partial")
	m.appendRenderedLine("streaming line")

	result := &agent.ConversationResult{
		Response: message.Message{Role: message.RoleAssistant, Content: message.TextContent("done")},
		Messages: []message.Message{
			{Role: message.RoleUser, Content: message.TextContent("hi")},
			{Role: message.RoleAssistant, Content: message.TextContent("done")},
		},
		Usage: message.Usage{InputTokens: 10, OutputTokens: 5},
	}
	updated, _ := m.Update(convDoneMsg{Result: result})
	m = updated.(Model)

	assert.Equal(t, StateIdle, m.state)
	assert.Equal(t, 10, m.totalUsage.InputTokens)
	assert.Equal(t, 5, m.totalUsage.OutputTokens)
	assert.Equal(t, 1, m.turnCount)
	assert.Empty(t, m.streamingText.String())
	assert.Len(t, m.history, 2)
}

func TestConvErrorShowsError(t *testing.T) {
	m := newTestModel()
	m.state = StateStreaming
	updated, _ := m.Update(convErrorMsg{Err: assertError{msg: "boom"}})
	m = updated.(Model)
	assert.Equal(t, StateIdle, m.state)
	joined := ""
	for _, l := range m.rendered {
		joined += l
	}
	assert.Contains(t, joined, "boom")
}

func TestToolStartIncrementsCounter(t *testing.T) {
	m := newTestModel()
	m.state = StateStreaming
	call := message.ContentBlock{
		Type:         "tool_use",
		ToolUseID:    "t1",
		ToolUseName:  "read_file",
		ToolUseInput: json.RawMessage(`{"path":"x"}`),
	}
	updated, _ := m.Update(toolStartMsg{Call: call})
	m = updated.(Model)
	assert.Equal(t, StateToolRunning, m.state)
	assert.Equal(t, 1, m.toolCalls)
}

func TestWindowSizeUpdatesDimensions(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	assert.Equal(t, 120, m.width)
	assert.Equal(t, 40, m.height)
}

// assertError is a minimal error type for tests.
type assertError struct{ msg string }

func (e assertError) Error() string { return e.msg }
```

- [ ] **Step 2: Run tests**

```bash
go test -race ./cli/ui/...
```

Expected: PASS. 12 tests total (4 skin + 8 update).

- [ ] **Step 3: Commit**

```bash
git add hermes-agent-go/cli/ui/ui_test.go
git commit -m "test(cli/ui): add Model update handler tests"
```

---

## Task 12: Final Verification

Run these and report results (no commit):

- [ ] **Step 1: Full test suite with race detector**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermes-agent-go
go test -race -cover ./...
```

Expected: ALL packages pass. `cli/ui` shows non-zero coverage.

- [ ] **Step 2: go vet**

```bash
go vet ./...
```

Expected: clean.

- [ ] **Step 3: golangci-lint (if available)**

```bash
golangci-lint run ./cli/ui/... 2>&1 || true
```

Expected: clean OR not installed. Both are acceptable.

- [ ] **Step 4: Build the binary**

```bash
make build
./bin/hermes version
```

Expected: binary builds and prints version.

- [ ] **Step 5: Manual smoke test (OPTIONAL, requires real ANTHROPIC_API_KEY)**

```bash
export ANTHROPIC_API_KEY=sk-ant-...
./bin/hermes
```

Expected: TUI launches. You can see the 4-zone layout, type a message, see streaming response, watch tool calls render. Ctrl+C cancels, Ctrl+D exits.

- [ ] **Step 6: Verify git log**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite
git log --oneline hermes-agent-go/ | head -15
```

Expected: ~11 new commits from Plan 4.

- [ ] **Step 7: Plan 4 done.**

---

## Plan 4 Self-Review Notes

**Spec coverage:**
- Bubbletea Model-View-Update foundation — Tasks 4-6
- 4-zone layout (banner, context, viewport, input, status) — Tasks 5, 10
- Skin system (Default + Minimal + auto-detect) — Task 2
- Tea message types for async Engine events — Task 3
- Slash commands (/exit, /clear, /help, /model, /cost) — Task 7
- Markdown rendering via glamour — Task 8
- Tool call formatting with `│`/`└` — Task 8
- Engine goroutine dispatcher posting tea.Msg — Task 9
- Integration with runREPL — Task 10
- Model update tests — Task 11

**Explicitly out of scope for Plan 4:**
- Slash command Tab autocomplete — deferred (typing the full command works)
- Up/Down history navigation — deferred (native textarea behavior only)
- 3+ skins with config-based switching — deferred (only Default + Minimal)
- Responsive width breakpoints — deferred (assumes >= 80 cols)
- Blinking cursor animation — deferred (static cursor character)
- Ctrl+A/E/K/U line editing — deferred (native textarea behavior)
- Real context cancellation through Ctrl+C — deferred to Plan 6 (current behavior: drop partial output, transition to idle, but the goroutine keeps running until the Engine finishes)
- SIGWINCH handling beyond tea's native behavior — deferred

**Placeholder check:** None. All code blocks are complete and executable.

**Type consistency:**
- `Model`, `ModelConfig`, `State`, `streamingToolState` — defined in Task 4, used in Tasks 5-11
- `Skin` — defined in Task 2, used throughout
- `streamDeltaMsg`, `toolStartMsg`, `toolResultMsg`, `convDoneMsg`, `convErrorMsg`, `tickMsg`, `quitMsg` — defined in Task 3, used in Tasks 6, 9, 11
- `RunOptions`, `Run` — defined in Task 9, used in Task 10
- `renderAssistantText`, `renderToolResult`, `formatBytesInt`, `formatCost`, `itoa`, `shortSessionID` — defined in Tasks 4, 5, 8

No naming drift.

**Known simplification:** The `dispatch` closure on Model is a pragmatic workaround for bubbletea's functional update model. The alternative (threading `*tea.Program` through `tea.Cmd` values) requires circular knowledge that Go's type system doesn't cleanly support. The dispatch field is nil in tests (so `startConversation` returns a convErrorMsg), which is exactly what we want — unit tests shouldn't spawn real Engine goroutines.
