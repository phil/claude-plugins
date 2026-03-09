#!/usr/bin/env bash
# check-tmux.sh — SessionStart hook
# Resolves tmux coordinates and writes the initial SessionState JSON to
# ~/.claude/sessions/${CLAUDE_SESSION_ID}.json

set -euo pipefail

SESSION_FILE="$HOME/.claude/sessions/${CLAUDE_SESSION_ID}.json"

# Read the Claude-provided JSON payload from stdin (not used for initial write,
# but consumed to avoid broken-pipe warnings from Claude Code).
PAYLOAD="$(cat)"

# ---------------------------------------------------------------------------
# Resolve tmux coordinates
# $TMUX is set by tmux as "<socket-path>,<pid>,<session-id>" when running
# inside a tmux pane.  When not inside tmux the variable is unset.
# ---------------------------------------------------------------------------
if [ -n "${TMUX:-}" ]; then
    TMUX_SOCKET_PATH="${TMUX%%,*}"           # e.g. /private/tmp/tmux-501/default
    TMUX_SOCKET_NAME="${TMUX_SOCKET_PATH##*/}"  # e.g. "default"

    TMUX_WINDOW_INDEX="$(tmux display-message -p '#{window_index}')"
    TMUX_PANE_INDEX="$(tmux display-message -p '#{pane_index}')"
    TMUX_SESSION_NAME="$(tmux display-message -p '#{session_name}')"
else
    TMUX_SOCKET_NAME=""
    TMUX_WINDOW_INDEX="0"
    TMUX_PANE_INDEX="0"
    TMUX_SESSION_NAME=""
fi

# Derive project name — basename of working directory.
PROJECT_NAME="$(basename "${CLAUDE_WORKSPACE_PATH:-$PWD}")"

# ---------------------------------------------------------------------------
# Ensure sessions directory exists
# ---------------------------------------------------------------------------
mkdir -p "$HOME/.claude/sessions"

# ---------------------------------------------------------------------------
# Read existing state (if any) — preserves started_at and version on restart
# ---------------------------------------------------------------------------
if [ -f "$SESSION_FILE" ]; then
    CURRENT="$(cat "$SESSION_FILE")"
else
    CURRENT="{}"
fi

NOW="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# ---------------------------------------------------------------------------
# Atomic write: build JSON via jq, write to .tmp, then mv into place
# ---------------------------------------------------------------------------
TMP="$(mktemp)"
jq -n \
    --argjson current "$CURRENT" \
    --arg session_id "$CLAUDE_SESSION_ID" \
    --arg work_dir "${CLAUDE_WORKSPACE_PATH:-$PWD}" \
    --arg project_name "$PROJECT_NAME" \
    --arg tmux_socket "$TMUX_SOCKET_NAME" \
    --arg tmux_session "$TMUX_SESSION_NAME" \
    --argjson tmux_window "$TMUX_WINDOW_INDEX" \
    --argjson tmux_pane "$TMUX_PANE_INDEX" \
    --arg now "$NOW" \
    '($current) * {
        session_id: $session_id,
        work_dir: $work_dir,
        project_name: $project_name,
        tmux_socket: $tmux_socket,
        tmux_session: $tmux_session,
        tmux_window: $tmux_window,
        tmux_pane: $tmux_pane,
        status: "unknown",
        current_tool: "",
        message: "",
        started_at: ($current.started_at // $now),
        updated_at: $now,
        ended_at: null,
        version: (if ($current.version == null) then 1 else ($current.version + 1) end)
    }' > "$TMP"
mv "$TMP" "$SESSION_FILE"
