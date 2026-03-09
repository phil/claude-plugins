package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/philbalchin/agent-overview-tui/internal/schema"
)

// RenderSessionRow renders a single session as a formatted string row.
func RenderSessionRow(sess *schema.SessionState, selected bool, width int) string {
	isDead := sess.Status == schema.StatusDead

	// Selection marker
	marker := "  "
	if selected {
		marker = "▶ "
	}

	// Window.pane identifier
	paneID := fmt.Sprintf("window %d.%d", sess.TmuxWindow, sess.TmuxPane)

	// Status icon and label
	icon := StatusIcon(sess.Status)
	statusStr := string(sess.Status)
	statusStyle := StatusStyle(sess.Status)

	// Extra info (tool/message/time)
	extra := ""
	if isDead {
		// Show how long ago the session died
		since := time.Since(sess.UpdatedAt)
		extra = formatDuration(since) + " ago"
	} else if sess.CurrentTool != "" {
		extra = sess.CurrentTool
	} else if sess.Message != "" {
		extra = sess.Message
	}

	// Model / worktree info (right side)
	rightInfo := ""
	if sess.Model != "" {
		rightInfo = sess.Model
	}

	// Build the row
	indent := "  "
	left := fmt.Sprintf("%s%s%-14s %s  %-14s  %-20s",
		indent, marker, paneID,
		statusStyle.Render(icon),
		statusStyle.Render(statusStr),
		extra,
	)

	// Truncate left portion if needed
	availRight := width - lipgloss.Width(left) - 2
	if availRight > 0 && rightInfo != "" {
		right := StyleDim.Render(rightInfo)
		padding := strings.Repeat(" ", max(0, availRight-lipgloss.Width(rightInfo)))
		left = left + padding + right
	}

	// Trim to width
	if lipgloss.Width(left) > width {
		left = left[:width]
	}

	if isDead {
		left = StyleDim.Render(left)
	}

	if selected {
		left = StyleSelected.Width(width).Render(left)
	}

	return left
}

// formatDuration formats a duration into a short human-readable string.
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("00:%02d", int(d.Seconds()))
	}
	if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%02d:%02d", m, s)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%02dm", h, m)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
