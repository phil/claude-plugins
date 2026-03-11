#!/usr/bin/env bash
# check-tmux.sh — SessionStart hook
# Resolves tmux coordinates and writes the initial SessionState JSON to
# ~/.claude/sessions/${CLAUDE_SESSION_ID}.json

set -eo pipefail

# Read the Claude-provided JSON payload from stdin first.
# Claude Code passes session_id and cwd in the JSON payload, not as env vars.
PAYLOAD="$(cat)"

SESSION_ID="${CLAUDE_SESSION_ID:-$(printf '%s' "$PAYLOAD" | jq -r '.session_id // ""')}"
WORK_DIR="${CLAUDE_WORKSPACE_PATH:-$(printf '%s' "$PAYLOAD" | jq -r '.cwd // ""')}"
WORK_DIR="${WORK_DIR:-$PWD}"

if [ -z "$SESSION_ID" ]; then
    exit 0  # no session ID, nothing to write
fi

SESSION_FILE="$HOME/.claude/sessions/${SESSION_ID}.json"

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
PROJECT_NAME="$(basename "$WORK_DIR")"

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
    --arg session_id "$SESSION_ID" \
    --arg work_dir "$WORK_DIR" \
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
