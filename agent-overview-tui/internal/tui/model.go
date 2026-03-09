package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/philbalchin/agent-overview-tui/internal/store"
	"github.com/philbalchin/agent-overview-tui/internal/tmux"
)

// SessionUpdateMsg is sent when the store has new data.
type SessionUpdateMsg struct{}

// ErrorMsg is sent when an error occurs.
type ErrorMsg struct{ Err string }

// Model is the main Bubble Tea model.
type Model struct {
	store       *store.Store
	keys        KeyMap
	help        help.Model
	showHelp    bool
	selectedIdx int   // index into flattened navigable session IDs
	navigable   []string // flattened list of session IDs in display order
	width       int
	height      int
	err         string
}

// New creates a new Model.
func New(s *store.Store) *Model {
	h := help.New()
	h.ShowAll = false
	return &Model{
		store: s,
		keys:  DefaultKeyMap,
		help:  h,
	}
}

// Init initialises the model.
func (m *Model) Init() tea.Cmd {
	return waitForUpdate(m.store.Updates())
}

// waitForUpdate returns a Cmd that blocks until the store signals an update.
func waitForUpdate(updates <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		<-updates
		return SessionUpdateMsg{}
	}
}

// Update handles incoming messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width

	case SessionUpdateMsg:
		m.rebuildNavigable()
		return m, waitForUpdate(m.store.Updates())

	case ErrorMsg:
		m.err = msg.Err

	case tea.KeyMsg:
		// Clear error on any key
		m.err = ""

		switch {
		case msg.String() == "ctrl+c":
			return m, tea.Quit

		case msg.Type == tea.KeyRunes && msg.String() == "q":
			return m, tea.Quit

		case msg.String() == "up" || (msg.Type == tea.KeyRunes && msg.String() == "k"):
			if m.selectedIdx > 0 {
				m.selectedIdx--
			}

		case msg.String() == "down" || (msg.Type == tea.KeyRunes && msg.String() == "j"):
			if m.selectedIdx < len(m.navigable)-1 {
				m.selectedIdx++
			}

		case msg.String() == "enter":
			return m, m.switchToSelected()

		case msg.Type == tea.KeyRunes && msg.String() == "d":
			m.store.DeleteDead()
			m.rebuildNavigable()

		case msg.Type == tea.KeyRunes && msg.String() == "r":
			m.store.DeleteDead()
			m.rebuildNavigable()

		case msg.Type == tea.KeyRunes && msg.String() == "?":
			m.showHelp = !m.showHelp
			m.help.ShowAll = m.showHelp
		}
	}

	return m, nil
}

// switchToSelected returns a Cmd that switches tmux to the selected pane.
func (m *Model) switchToSelected() tea.Cmd {
	if len(m.navigable) == 0 {
		return nil
	}
	if m.selectedIdx < 0 || m.selectedIdx >= len(m.navigable) {
		return nil
	}
	selectedID := m.navigable[m.selectedIdx]

	// Find the session
	var sess *store.ProjectGroup
	_ = sess
	groups := m.store.GroupByProject()
	for _, g := range groups {
		for _, s := range g.Sessions {
			if s.SessionID == selectedID {
				err := tmux.SwitchToPane(s.TmuxSocket, s.TmuxSession, s.TmuxWindow, s.TmuxPane)
				if err != nil {
					m.store.MarkDead(selectedID)
					return func() tea.Msg {
						return ErrorMsg{Err: fmt.Sprintf("failed to switch to pane: %v", err)}
					}
				}
				return nil
			}
		}
	}
	return nil
}

// rebuildNavigable rebuilds the flattened list of navigable session IDs.
func (m *Model) rebuildNavigable() {
	groups := m.store.GroupByProject()
	prev := ""
	if m.selectedIdx >= 0 && m.selectedIdx < len(m.navigable) {
		prev = m.navigable[m.selectedIdx]
	}

	m.navigable = nil
	for _, g := range groups {
		for _, s := range g.Sessions {
			m.navigable = append(m.navigable, s.SessionID)
		}
	}

	// Try to preserve selection
	if prev != "" {
		for i, id := range m.navigable {
			if id == prev {
				m.selectedIdx = i
				return
			}
		}
	}
	// Clamp index
	if m.selectedIdx >= len(m.navigable) {
		m.selectedIdx = len(m.navigable) - 1
	}
	if m.selectedIdx < 0 {
		m.selectedIdx = 0
	}
}

// View renders the full TUI.
func (m *Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Rebuild navigable in case it hasn't been built yet
	if len(m.navigable) == 0 {
		m.rebuildNavigable()
	}

	var sb strings.Builder

	// Header
	sb.WriteString(m.renderHeader())
	sb.WriteString("\n")

	// Separator
	sb.WriteString(StyleSeparator.Render(strings.Repeat("─", m.width)))
	sb.WriteString("\n")

	// Count available height for body
	footerHeight := 1
	if m.showHelp {
		footerHeight = 3
	}
	if m.err != "" {
		footerHeight++
	}
	bodyHeight := m.height - 3 - footerHeight // 3 = header + separator + bottom separator

	// Body: render groups
	groups := m.store.GroupByProject()
	var bodyLines []string
	selectedID := ""
	if m.selectedIdx >= 0 && m.selectedIdx < len(m.navigable) {
		selectedID = m.navigable[m.selectedIdx]
	}

	for i, g := range groups {
		rendered := RenderGroup(g, selectedID, m.width)
		groupLines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
		bodyLines = append(bodyLines, groupLines...)
		// Add separator between groups (not after the last one)
		if i < len(groups)-1 {
			bodyLines = append(bodyLines, StyleSeparator.Render(strings.Repeat("─", m.width)))
		}
	}

	if len(bodyLines) == 0 {
		bodyLines = append(bodyLines, StyleDim.Render("  No sessions found. Waiting for updates..."))
	}

	// Scroll to keep selected visible
	visibleLines := m.visibleBodyLines(bodyLines, bodyHeight, selectedID)
	for _, line := range visibleLines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Pad remaining body space
	rendered := strings.Count(sb.String(), "\n")
	targetLines := m.height - footerHeight - 1
	for rendered < targetLines {
		sb.WriteString("\n")
		rendered++
	}

	// Separator before footer
	sb.WriteString(StyleSeparator.Render(strings.Repeat("─", m.width)))
	sb.WriteString("\n")

	// Error message
	if m.err != "" {
		sb.WriteString(StyleError.Render("  Error: "+m.err))
		sb.WriteString("\n")
	}

	// Footer help
	sb.WriteString(m.renderFooter())

	return sb.String()
}

// visibleBodyLines returns a window of lines that keeps the selected session visible.
func (m *Model) visibleBodyLines(lines []string, maxLines int, selectedID string) []string {
	if maxLines <= 0 || len(lines) == 0 {
		return lines
	}
	if len(lines) <= maxLines {
		return lines
	}

	// Find which line contains the selected session
	selectedLine := 0
	for i, line := range lines {
		if selectedID != "" && strings.Contains(line, "▶") {
			selectedLine = i
			break
		}
	}

	// Calculate window start
	start := selectedLine - maxLines/2
	if start < 0 {
		start = 0
	}
	end := start + maxLines
	if end > len(lines) {
		end = len(lines)
		start = end - maxLines
		if start < 0 {
			start = 0
		}
	}

	return lines[start:end]
}

// renderHeader renders the title bar.
func (m *Model) renderHeader() string {
	all := m.store.All()
	active := 0
	dead := 0
	for _, s := range all {
		if s.Status == "dead" {
			dead++
		} else {
			active++
		}
	}

	now := time.Now().Format("01-02 15:04:05")
	title := fmt.Sprintf("  Claude Code Sessions — %d active / %d dead", active, dead)
	timeStr := now + "  "

	// Right-align the time
	titleWidth := lipgloss.Width(title)
	timeWidth := lipgloss.Width(timeStr)
	padding := m.width - titleWidth - timeWidth
	if padding < 1 {
		padding = 1
	}

	header := title + strings.Repeat(" ", padding) + timeStr
	return StyleHeader.Width(m.width).Render(header)
}

// renderFooter renders the help footer.
func (m *Model) renderFooter() string {
	return StyleFooter.Render(m.help.View(m.keys))
}
