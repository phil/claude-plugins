package schema

import "time"

type Status string

const (
	StatusUnknown  Status = "unknown"
	StatusIdle     Status = "idle"
	StatusThinking Status = "thinking"
	StatusToolUse  Status = "tool_use"
	StatusWaiting  Status = "waiting_input"
	StatusDead     Status = "dead"
)

type SessionState struct {
	SessionID   string     `json:"session_id"`
	WorkDir     string     `json:"work_dir"`
	ProjectName string     `json:"project_name"`
	Model       string     `json:"model,omitempty"`
	TmuxSocket  string     `json:"tmux_socket"`
	TmuxSession string     `json:"tmux_session"`
	TmuxWindow  int        `json:"tmux_window"`
	TmuxPane    int        `json:"tmux_pane"`
	Status      Status     `json:"status"`
	CurrentTool string     `json:"current_tool,omitempty"`
	Message     string     `json:"message,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	EndedAt     *time.Time `json:"ended_at,omitempty"`
	Version     int64      `json:"version"`
}
