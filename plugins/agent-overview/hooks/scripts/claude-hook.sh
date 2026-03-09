#!/usr/bin/env bash
# claude-hook.sh — universal hook dispatcher
# Called as: claude-hook.sh <EventName>
#
# Reads the Claude-provided JSON payload from stdin, merges with existing
# session state, and atomically writes the updated SessionState JSON to
# ~/.claude/sessions/${CLAUDE_SESSION_ID}.json

set -euo pipefail

EVENT="${1:-}"
SESSION_FILE="$HOME/.claude/sessions/${CLAUDE_SESSION_ID}.json"

# Read the Claude-provided JSON payload from stdin.
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
# Read current session state, or use an empty object as the base
# ---------------------------------------------------------------------------
if [ -f "$SESSION_FILE" ]; then
    CURRENT="$(cat "$SESSION_FILE")"
else
    CURRENT="{}"
fi

# ---------------------------------------------------------------------------
# Determine status and extract any event-specific fields
# ---------------------------------------------------------------------------
TOOL_NAME=""
MESSAGE=""
ENDED_AT_EXPR="null"

case "$EVENT" in
    PreToolUse)
        STATUS="tool_use"
        TOOL_NAME="$(printf '%s' "$PAYLOAD" | jq -r '.tool_name // ""')"
        ;;
    UserPromptSubmit)
        STATUS="thinking"
        TOOL_NAME=""
        ;;
    Notification)
        STATUS="waiting_input"
        MESSAGE="$(printf '%s' "$PAYLOAD" | jq -r '.message // ""')"
        ;;
    Stop)
        STATUS="idle"
        TOOL_NAME=""
        ;;
    SessionEnd)
        STATUS="dead"
        ENDED_AT_EXPR="\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\""
        ;;
    *)
        STATUS="unknown"
        ;;
esac

# ---------------------------------------------------------------------------
# Ensure sessions directory exists (check-tmux.sh normally creates it, but
# guard here in case this hook fires first)
# ---------------------------------------------------------------------------
mkdir -p "$HOME/.claude/sessions"

# ---------------------------------------------------------------------------
# Atomic write: build JSON via jq merge, write to .tmp, then mv into place
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
    --arg status "$STATUS" \
    --arg tool "${TOOL_NAME:-}" \
    --arg message "${MESSAGE:-}" \
    --arg updated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --argjson ended_at "$ENDED_AT_EXPR" \
    '($current) * {
        session_id: $session_id,
        work_dir: $work_dir,
        project_name: $project_name,
        tmux_socket: $tmux_socket,
        tmux_session: $tmux_session,
        tmux_window: $tmux_window,
        tmux_pane: $tmux_pane,
        status: $status,
        current_tool: $tool,
        message: $message,
        updated_at: $updated_at,
        ended_at: $ended_at,
        version: (($current.version // 0) + 1)
    }' > "$TMP"
mv "$TMP" "$SESSION_FILE"
