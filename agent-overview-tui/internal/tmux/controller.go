package tmux

import (
	"fmt"
	"os"
	"os/exec"
)

// PaneMeta holds metadata about a tmux pane.
type PaneMeta struct {
	Session string
	Window  int
	Pane    int
	PID     string
	Title   string
}

// SwitchToPane switches tmux to the target pane.
// Detects if running inside tmux ($TMUX env var) and uses switch-client vs attach-session.
func SwitchToPane(socket, session string, window, pane int) error {
	target := fmt.Sprintf("%s:%d.%d", session, window, pane)
	var cmd *exec.Cmd
	if os.Getenv("TMUX") != "" {
		cmd = exec.Command("tmux", "-L", socket, "switch-client", "-t", target)
	} else {
		cmd = exec.Command("tmux", "-L", socket, "attach-session", "-t", target)
	}
	return cmd.Run()
}

// ListPanes returns metadata for all panes on the given socket.
func ListPanes(socket string) ([]PaneMeta, error) {
	// tmux list-panes -a -F "#{session_name}:#{window_index}:#{pane_index}:#{pane_pid}:#{pane_title}"
	args := []string{"-L", socket, "list-panes", "-a", "-F",
		"#{session_name}:#{window_index}:#{pane_index}:#{pane_pid}:#{pane_title}"}
	cmd := exec.Command("tmux", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("tmux list-panes: %w", err)
	}
	return parsePanes(string(out))
}
