package tmux

import (
	"fmt"
	"strconv"
	"strings"
)

// parsePanes parses the output of `tmux list-panes -a -F "..."` into []PaneMeta.
// Expected format per line: session_name:window_index:pane_index:pane_pid:pane_title
func parsePanes(output string) ([]PaneMeta, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	panes := make([]PaneMeta, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Split on ":" but only on the first 4 colons (title may contain colons)
		parts := strings.SplitN(line, ":", 5)
		if len(parts) < 4 {
			continue
		}

		window, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("parse window index %q: %w", parts[1], err)
		}

		pane, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("parse pane index %q: %w", parts[2], err)
		}

		title := ""
		if len(parts) == 5 {
			title = parts[4]
		}

		panes = append(panes, PaneMeta{
			Session: parts[0],
			Window:  window,
			Pane:    pane,
			PID:     parts[3],
			Title:   title,
		})
	}

	return panes, nil
}
