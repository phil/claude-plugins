package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/philbalchin/agent-overview-tui/internal/schema"
)

var (
	// Border/frame styles
	StyleBorder = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("240"))

	// Header style for the title bar
	StyleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color("57"))

	// Group header (project name)
	StyleGroupHeader = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39"))

	// Tmux session label (right-aligned in group header)
	StyleTmuxLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	// Selected row highlight
	StyleSelected = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("255")).
			Bold(true)

	// Dim style for dead sessions
	StyleDim = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	// Normal row style
	StyleRow = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	// Footer / help bar
	StyleFooter = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	// Error style
	StyleError = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	// Separator line
	StyleSeparator = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

// StatusStyle returns the lipgloss style for a given status.
func StatusStyle(status schema.Status) lipgloss.Style {
	switch status {
	case schema.StatusIdle:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	case schema.StatusThinking:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	case schema.StatusToolUse:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("51"))
	case schema.StatusWaiting:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	case schema.StatusDead:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Faint(true)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	}
}

// StatusIcon returns the icon character for a given status.
func StatusIcon(status schema.Status) string {
	switch status {
	case schema.StatusIdle:
		return "◌"
	case schema.StatusThinking:
		return "●"
	case schema.StatusToolUse:
		return "⚙"
	case schema.StatusWaiting:
		return "⚠"
	case schema.StatusDead:
		return "✗"
	default:
		return "?"
	}
}
