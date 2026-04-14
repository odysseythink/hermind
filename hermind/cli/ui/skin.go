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
	Accent  lipgloss.Style // amber/gold for headings, prompt character
	Success lipgloss.Style // teal for successful tool results
	Warning lipgloss.Style // yellow for budget warnings
	Error   lipgloss.Style // soft red for errors
	Muted   lipgloss.Style // gray for metadata, tool commands
	Code    lipgloss.Style // green for code blocks, file paths
	Border  lipgloss.Style // zone border style

	// Character tokens
	PromptChar    string // ">"
	ThinkingChar  string // "◆"
	ToolChar      string // "⚡"
	OutputPrefix  string // "│"
	OutputEnd     string // "└"
	StreamingChar string // "▊"
	WarningChar   string // "⚠"
	ErrorChar     string // "✗"
	ActiveChar    string // "◈"
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
