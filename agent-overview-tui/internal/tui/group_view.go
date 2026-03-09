package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/philbalchin/agent-overview-tui/internal/store"
)

// RenderGroup renders a project group as a multi-line string.
func RenderGroup(group store.ProjectGroup, selectedID string, width int) string {
	var sb strings.Builder

	// Determine the tmux session label from the first session
	tmuxLabel := ""
	if len(group.Sessions) > 0 {
		tmuxLabel = fmt.Sprintf("[tmux: %s]", group.Sessions[0].TmuxSession)
	}

	// Header: bold project name on the left, tmux label on the right
	headerLeft := StyleGroupHeader.Render(group.ProjectName)
	headerRight := StyleTmuxLabel.Render(tmuxLabel)

	leftWidth := lipgloss.Width(headerLeft)
	rightWidth := lipgloss.Width(headerRight)
	padding := width - leftWidth - rightWidth - 2 // 2 for left indent
	if padding < 1 {
		padding = 1
	}
	header := "  " + headerLeft + strings.Repeat(" ", padding) + headerRight
	sb.WriteString(header)
	sb.WriteString("\n")

	// Session rows
	for _, sess := range group.Sessions {
		selected := sess.SessionID == selectedID
		row := RenderSessionRow(sess, selected, width)
		sb.WriteString(row)
		sb.WriteString("\n")
	}

	return sb.String()
}
