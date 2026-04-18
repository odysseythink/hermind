// cli/ui/model.go
package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
)

// State represents the overall REPL state.
type State int

const (
	StateIdle        State = iota // waiting for user input
	StateStreaming                // Engine is streaming a response
	StateToolRunning              // A tool is currently executing
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

	// Slash-command completion popup. Nil when hidden.
	completion *completionState

	// dispatch is the function that launches an Engine goroutine for a
	// given user message. Installed by Run() after construction so that
	// the goroutine can reach the *tea.Program instance.
	dispatch func(userInput string, history []message.Message)
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

