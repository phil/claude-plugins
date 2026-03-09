# Claude Code Session Dashboard — Architecture Plan (Agent 3)

## 1. Approach

### Design Philosophy

The dashboard is built on a clean separation between three concerns: **event ingestion** (hooks writing structured state), **state management** (a Go process aggregating that state), and **presentation** (a Bubble Tea TUI rendering and enabling navigation). The design avoids any in-process communication with Claude Code itself — Claude remains a black box. All telemetry flows through files on disk written by hook scripts, which means the dashboard can be started and stopped independently without affecting running Claude sessions.

This mirrors how the existing `agent-overview` plugin already works: hooks write state into `~/.claude/`, and other processes can read it. The dashboard is the consumer of that data contract.

### Repository Layout

The system spans two directories within this repository (the TUI will eventually be extracted into its own repository):

```
claude-plugins/
├── plugins/
│   └── agent-overview/          # Claude Code plugin — hook scripts and config
│       ├── hooks/
│       │   ├── claude-hook.sh   # universal hook dispatcher (writes session JSON)
│       │   └── check-tmux.sh    # SessionStart: resolves tmux coords
│       └── hooks.json           # Claude Code hook registrations
└── agent-overview-tui/          # Go TUI dashboard (future: own repository)
    ├── go.mod
    ├── go.sum
    ├── main.go
    └── internal/
        ├── schema/
        ├── store/
        ├── tmux/
        └── tui/
```

The hook scripts in `plugins/agent-overview/` write state files to `~/.claude/sessions/`. The `agent-overview-tui` binary reads that same directory — the shared filesystem path is the only coupling between the two components.

### High-Level Component Overview

```
┌───────────────────────────────────────────────────────────────────┐
│  Claude Code Process (per pane)                                   │
│  ┌──────────────────────────────────────────────────────────┐     │
│  │  hooks: SessionStart / PreToolUse / Stop / etc.          │     │
│  │          └─► hook script ─► writes JSON ─► ~/.claude/    │     │
│  └──────────────────────────────────────────────────────────┘     │
└───────────────────────────────────────────────────────────────────┘
           │ files on disk (~/.claude/sessions/<session_id>.json)
           ▼
┌─────────────────────────────┐
│  ccsdash (Go binary)        │
│  ┌─────────────────────┐    │
│  │  fsnotify Watcher   │    │  watches ~/.claude/sessions/
│  └────────┬────────────┘    │
│           │ events          │
│  ┌────────▼────────────┐    │
│  │  State Store        │    │  in-memory map[sessionID]SessionState
│  └────────┬────────────┘    │
│           │ model updates   │
│  ┌────────▼────────────┐    │
│  │  Bubble Tea Model   │    │  renders dashboard, handles keyboard
│  └────────┬────────────┘    │
│           │ tmux commands   │
│  ┌────────▼────────────┐    │
│  │  tmux Controller    │    │  exec: tmux switch-client / select-pane
│  └─────────────────────┘    │
└─────────────────────────────┘
```

### Session Discovery

Discovery is entirely file-system-based. Every Claude Code session, on start, writes a JSON file to `~/.claude/sessions/<session_id>.json`. That file records the tmux session, window, and pane coordinates alongside the Claude session ID and working directory. The dashboard watches that directory with `fsnotify`. No polling, no hardcoded session lists.

The hook script resolves tmux coordinates from environment variables Claude Code inherits from its pane (`$TMUX`, `$TMUX_PANE`, `$TMUX_SESSION_NAME`, `$TMUX_WINDOW_INDEX`, `$TMUX_PANE_INDEX`) at `SessionStart` time.

### Multiple Instances Per tmux Session

A tmux **session** maps to a **project** (or a git worktree branch of a project). Multiple Claude Code instances can run within the same tmux session — each in its own pane or window. The `tmux_session` name alone is therefore not a unique identifier for a Claude Code instance; the full `(tmux_session, tmux_window, tmux_pane)` triplet is.

The dashboard uses `basename(work_dir)` as the **project name**, which is displayed as a group header. This works naturally for both standalone projects and git worktrees, where each worktree has a distinct directory name (typically the branch name).

Example — tmux session `"work"` with three Claude instances:

```
tmux session: work
  window 1, pane 0  →  ~/projects/my-app         project: my-app      (main branch)
  window 1, pane 1  →  ~/projects/my-app-feat-x   project: my-app-feat-x (worktree)
  window 2, pane 0  →  ~/projects/api-service     project: api-service
```

The dashboard groups and renders these under their project names, with the tmux session name shown as a secondary label.

---

## 2. Implementation

### File Structure

**Claude Code plugin** (hook scripts — live in `plugins/agent-overview/`):

```
plugins/agent-overview/
├── hooks/
│   ├── claude-hook.sh         # universal hook dispatcher; writes ~/.claude/sessions/*.json
│   └── check-tmux.sh          # SessionStart: resolves tmux coords, writes initial JSON
└── hooks.json                 # Claude Code hook registrations
```

**Go TUI dashboard** (lives in `agent-overview-tui/`; will become its own repository):

```
agent-overview-tui/
├── go.mod
├── go.sum
├── main.go                    # entry point, flag parsing, starts everything
├── cmd/
│   └── root.go                # cobra root command (--sessions-dir, --socket flags)
└── internal/
    ├── tmux/
    │   ├── discovery.go       # list panes, parse tmux output
    │   └── controller.go      # switch-client, select-pane wrappers
    ├── store/
    │   ├── store.go           # thread-safe SessionState map + GroupByProject()
    │   └── watcher.go         # fsnotify loop, parses JSON, publishes updates
    ├── tui/
    │   ├── model.go           # Bubble Tea Model, Update, View
    │   ├── styles.go          # Lip Gloss style definitions
    │   ├── group_view.go      # renders a project group header + its session rows
    │   ├── session_row.go     # renders one session as a table row
    │   └── keybindings.go     # key map definitions
    └── schema/
        └── session.go         # SessionState struct, JSON tags, Status enum
```

### Go Packages / Libraries

| Purpose | Library |
|---|---|
| TUI framework | `github.com/charmbracelet/bubbletea` |
| Styling | `github.com/charmbracelet/lipgloss` |
| Table widget | `github.com/charmbracelet/bubbles/table` |
| Spinner widget | `github.com/charmbracelet/bubbles/spinner` |
| Key bindings | `github.com/charmbracelet/bubbles/key` |
| Help overlay | `github.com/charmbracelet/bubbles/help` |
| File watching | `github.com/fsnotify/fsnotify` |
| JSON parsing | stdlib `encoding/json` |
| CLI flags | `github.com/spf13/cobra` |
| Process exec | stdlib `os/exec` |

### Key Go Struct Definitions

```go
// internal/schema/session.go

package schema

import "time"

type Status string

const (
    StatusUnknown   Status = "unknown"
    StatusIdle      Status = "idle"
    StatusThinking  Status = "thinking"
    StatusToolUse   Status = "tool_use"
    StatusWaiting   Status = "waiting_input"
    StatusDead      Status = "dead"
)

// SessionState is the canonical in-memory and on-disk representation.
// Every hook write is a full replacement of this struct.
type SessionState struct {
    // Identity
    SessionID   string    `json:"session_id"`
    WorkDir     string    `json:"work_dir"`
    // ProjectName is basename(WorkDir) — e.g. "my-app" or "my-app-feat-x" for a worktree.
    // Set by the hook script; used by the TUI to group sessions visually.
    ProjectName string    `json:"project_name"`
    Model       string    `json:"model,omitempty"`

    // tmux coordinates (set once at SessionStart, never mutated).
    // The full (TmuxSession, TmuxWindow, TmuxPane) triplet uniquely identifies
    // one Claude Code instance even when multiple instances share the same TmuxSession.
    // TmuxSocket is the socket basename (e.g. "default"), extracted from $TMUX.
    // TmuxWindow and TmuxPane are numeric indices from `tmux display-message -p`.
    // The switch-client target is built as: "<TmuxSession>:<TmuxWindow>.<TmuxPane>"
    TmuxSocket  string    `json:"tmux_socket"`   // e.g. "default"
    TmuxSession string    `json:"tmux_session"`  // e.g. "work"
    TmuxWindow  int       `json:"tmux_window"`   // e.g. 2  (window index, 0-based)
    TmuxPane    int       `json:"tmux_pane"`     // e.g. 1  (pane index within window, 0-based)

    // Dynamic state, updated by each hook invocation
    Status      Status    `json:"status"`
    CurrentTool string    `json:"current_tool,omitempty"`  // set during PreToolUse
    Message     string    `json:"message,omitempty"`        // last notification text

    // Lifecycle timestamps
    StartedAt   time.Time  `json:"started_at"`
    UpdatedAt   time.Time  `json:"updated_at"`
    EndedAt     *time.Time `json:"ended_at,omitempty"`

    // Version counter — monotonically increasing, used to detect staleness
    Version     int64     `json:"version"`
}
```

```go
// internal/store/store.go

package store

import (
    "sync"
    "github.com/user/ccsdash/internal/schema"
)

type Store struct {
    mu       sync.RWMutex
    sessions map[string]*schema.SessionState
    updates  chan struct{}  // closed and recreated on each change to signal TUI
}

func (s *Store) Upsert(state *schema.SessionState) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.sessions[state.SessionID] = state
    s.notify()
}

func (s *Store) MarkDead(sessionID string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if sess, ok := s.sessions[sessionID]; ok {
        sess.Status = schema.StatusDead
    }
    s.notify()
}

func (s *Store) All() []*schema.SessionState {
    s.mu.RLock()
    defer s.mu.RUnlock()
    // returns sorted snapshot (by ProjectName then StartedAt)
    return nil // implement with sorted copy
}

// ProjectGroup is a named set of sessions sharing the same project name.
type ProjectGroup struct {
    ProjectName string
    Sessions    []*schema.SessionState
}

// GroupByProject returns sessions grouped by ProjectName, sorted alphabetically
// by project name. Within each group, sessions are sorted by StartedAt ascending.
// A single tmux session may contain multiple groups if multiple projects are open.
func (s *Store) GroupByProject() []ProjectGroup {
    s.mu.RLock()
    defer s.mu.RUnlock()

    groups := make(map[string]*ProjectGroup)
    for _, sess := range s.sessions {
        g, ok := groups[sess.ProjectName]
        if !ok {
            g = &ProjectGroup{ProjectName: sess.ProjectName}
            groups[sess.ProjectName] = g
        }
        g.Sessions = append(g.Sessions, sess)
    }
    // sort groups alphabetically, sessions within by StartedAt
    // ... return sorted []ProjectGroup
    return nil
}

// Updates returns a channel that receives a signal whenever state changes.
// The TUI reads this channel and sends a tea.Cmd to re-render.
func (s *Store) Updates() <-chan struct{} { return s.updates }
```

```go
// internal/tmux/controller.go

package tmux

import (
    "fmt"
    "os/exec"
)

// SwitchToPane attaches/switches the calling terminal to the target pane.
// Uses switch-client to move to the pane's tmux session, then select-pane.
func SwitchToPane(socket, session string, window, pane int) error {
    target := fmt.Sprintf("%s:%d.%d", session, window, pane)
    // If the dashboard is running inside the same tmux server:
    cmd := exec.Command("tmux", "-L", socket, "switch-client", "-t", target)
    return cmd.Run()
}

// ListPanes runs `tmux list-panes -a` and returns raw pane metadata.
// Used for cross-validation: check that recorded pane IDs still exist.
func ListPanes(socket string) ([]PaneMeta, error) {
    out, err := exec.Command(
        "tmux", "-L", socket,
        "list-panes", "-a",
        "-F", "#{session_name}:#{window_index}:#{pane_index}:#{pane_pid}:#{pane_title}",
    ).Output()
    if err != nil {
        return nil, err
    }
    _ = out // parse and return
    return nil, nil
}
```

### Concurrency Model

There are three concurrent actors communicating via channels:

1. **fsnotify goroutine** (`store/watcher.go`) — blocks on `fsnotify.Events`. On each CREATE or WRITE event under `~/.claude/sessions/`, it reads and parses the JSON file, calls `store.Upsert()`. On REMOVE, calls `store.MarkDead()`. Runs in one goroutine.

2. **Stale-session reaper goroutine** — ticks every 30 seconds. Calls `tmux.ListPanes()` to get live pane PIDs. Compares against `store.All()`. Any session whose pane no longer exists gets `store.MarkDead()`. Handles the case where Claude crashes without firing `SessionEnd`.

3. **Bubble Tea program** — single goroutine owned by `bubbletea.Program`. Receives `tea.Msg` values. The `store.Updates()` channel is bridged into Bubble Tea messages via a `tea.Cmd` that returns a `SessionUpdateMsg` whenever the store signals a change.

```go
// tui/model.go — bridging the store channel into Bubble Tea

type SessionUpdateMsg struct{}

func waitForUpdate(updates <-chan struct{}) tea.Cmd {
    return func() tea.Msg {
        <-updates
        return SessionUpdateMsg{}
    }
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg.(type) {
    case SessionUpdateMsg:
        m.rows = buildRows(m.store.All())
        return m, waitForUpdate(m.store.Updates())
    // ... keyboard handling
    }
    return m, nil
}
```

---

## 3. Technical Specification

### 3.1 Data Collection: Hook Scripts

The existing `agent-overview` plugin hooks fire on every lifecycle event. The dashboard extends this pattern. Each hook invocation runs `claude-hook.sh <EventName>` which:

1. Reads the Claude-provided JSON payload from stdin (Claude Code passes event context as JSON).
2. Resolves tmux coordinates from environment variables.
3. Merges with any existing session file.
4. Atomically writes the updated `SessionState` JSON to `~/.claude/sessions/<session_id>.json`.

The write is atomic: the script writes to a `.tmp` file, then `mv`s it into place. This prevents the dashboard from reading a partial file.

Example hook script excerpt:

```bash
#!/usr/bin/env bash
# claude-hook.sh — called as: claude-hook.sh <EventName>

EVENT="$1"
SESSION_FILE="$HOME/.claude/sessions/${CLAUDE_SESSION_ID}.json"
PAYLOAD="$(cat)"  # JSON from Claude on stdin

# Resolve tmux coordinates.
# $TMUX is set by tmux as "<socket-path>,<pid>,<session-id>".
TMUX_SOCKET="${TMUX%%,*}"                    # full socket path, e.g. /private/tmp/tmux-501/default
TMUX_SOCKET_NAME="${TMUX_SOCKET##*/}"        # basename, e.g. "default"

# $TMUX_PANE is set by tmux to the unique pane ID, e.g. "%3".
# We also need numeric window and pane indices for display and for constructing
# switch-client targets like "work:2.1". Use tmux display-message to get them.
TMUX_WINDOW_INDEX="$(tmux display-message -p '#{window_index}')"
TMUX_PANE_INDEX="$(tmux display-message -p '#{pane_index}')"
TMUX_SESSION_NAME="$(tmux display-message -p '#{session_name}')"

# Derive project name — basename of working directory.
# Works for both regular projects and git worktrees (worktree dir name = branch name).
PROJECT_NAME="$(basename "${CLAUDE_WORKSPACE_PATH:-$PWD}")"

# Read current state or create skeleton
if [ -f "$SESSION_FILE" ]; then
    CURRENT="$(cat "$SESSION_FILE")"
else
    CURRENT="{}"
fi

# Determine status transition
case "$EVENT" in
    PreToolUse)
        TOOL_NAME="$(echo "$PAYLOAD" | jq -r '.tool_name // ""')"
        STATUS="tool_use"
        ;;
    UserPromptSubmit)
        STATUS="thinking"
        TOOL_NAME=""
        ;;
    Notification)
        STATUS="waiting_input"
        MESSAGE="$(echo "$PAYLOAD" | jq -r '.message // ""')"
        ;;
    Stop)
        STATUS="idle"
        TOOL_NAME=""
        ;;
    SessionEnd)
        STATUS="dead"
        ;;
    *)
        STATUS="unknown"
        ;;
esac

# Merge and write atomically
TMP="$(mktemp)"
jq -n \
    --argjson current "$CURRENT" \
    --arg session_id "$CLAUDE_SESSION_ID" \
    --arg work_dir "$CLAUDE_WORKSPACE_PATH" \
    --arg project_name "$PROJECT_NAME" \
    --arg tmux_socket "$TMUX_SOCKET_NAME" \
    --arg tmux_session "$TMUX_SESSION_NAME" \
    --argjson tmux_window "$TMUX_WINDOW_INDEX" \
    --argjson tmux_pane "$TMUX_PANE_INDEX" \
    --arg status "$STATUS" \
    --arg tool "$TOOL_NAME" \
    --arg message "${MESSAGE:-}" \
    --arg updated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
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
        version: (($current.version // 0) + 1)
    }' > "$TMP"
mv "$TMP" "$SESSION_FILE"
```

### 3.2 Exact Data Format / Schema

On-disk file: `~/.claude/sessions/<session_id>.json`

```json
{
  "session_id": "clde-01abc123def456",
  "work_dir": "/Users/phil/projects/my-app-feat-x",
  "project_name": "my-app-feat-x",
  "model": "claude-opus-4-5",
  "tmux_socket": "default",
  "tmux_session": "work",
  "tmux_window": 2,
  "tmux_pane": 1,
  "status": "tool_use",
  "current_tool": "Bash",
  "message": "",
  "started_at": "2026-03-08T09:00:00Z",
  "updated_at": "2026-03-08T09:15:34Z",
  "ended_at": null,
  "version": 47
}
```

`project_name` is always `basename(work_dir)`. For a standard project at `~/projects/my-app` it is `"my-app"`. For a git worktree at `~/projects/my-app-feat-x` it is `"my-app-feat-x"` — which naturally surfaces the branch name when worktrees follow the common `<repo>-<branch>` naming convention.
```

Status values (the `status` field):

| Value | Meaning |
|---|---|
| `idle` | Claude is waiting, nothing running |
| `thinking` | User prompt submitted, Claude processing |
| `tool_use` | PreToolUse fired, `current_tool` is set |
| `waiting_input` | Notification fired (needs user input) |
| `dead` | SessionEnd fired or pane no longer exists |
| `unknown` | No update received yet |

### 3.3 Real-Time Update Mechanism

The dashboard uses `fsnotify` to watch `~/.claude/sessions/` with zero polling delay. `fsnotify` delivers `CREATE`, `WRITE`, and `REMOVE` events as they occur. On macOS this uses FSEvents (kqueue); on Linux it uses inotify. Both provide sub-100ms latency for local file writes.

The flow from hook script write to screen update:

```
hook script mv .tmp → file        ~1ms
fsnotify delivers WRITE event     ~5-20ms
watcher reads + parses JSON       ~1ms
store.Upsert() + channel signal   <1ms
Bubble Tea Update() renders       ~1ms
────────────────────────────────────────
Total end-to-end latency:         ~10-25ms
```

This is faster than polling and requires no persistent socket or daemon.

### 3.4 tmux Pane Switching

From within the dashboard, pressing `Enter` on a selected session calls:

```go
// Uses the tmux_socket from the SessionState
err := tmux.SwitchToPane(
    state.TmuxSocket,    // e.g. "default"
    state.TmuxSession,   // e.g. "work"
    state.TmuxWindow,    // e.g. 2
    state.TmuxPane,      // e.g. 1
)
```

Which executes:

```
tmux -L default switch-client -t work:2.1
```

This works whether the dashboard is running inside the same tmux server (most common) or from an external terminal connected to the server socket. The `-L <socket>` flag explicitly targets the correct tmux server, avoiding confusion when multiple tmux servers are running.

If the dashboard is running outside tmux entirely (pure external terminal), it falls back to:

```
tmux -L default attach-session -t work:2.1
```

The controller detects whether it is inside tmux by checking `$TMUX` environment variable and chooses the appropriate subcommand.

### 3.5 Dashboard Layout (TUI)

Sessions are grouped by `project_name`. Each group has a bold header line. Within a group, multiple Claude Code instances (e.g. main branch + worktrees, or multiple windows in the same tmux session) are listed as individual rows. The tmux session name is shown as a secondary label so the user knows where to navigate.

```
╔══════════════════════════════════════════════════════════════════════╗
║  Claude Code Sessions — 5 active / 1 dead         03-08 09:15:34   ║
╠══════════════════════════════════════════════════════════════════════╣
║  my-app                                           [tmux: work]      ║
║  ▶  window 1.0   ⚙  tool_use   Bash              main              ║
║     window 1.1   ● thinking                       feat-x worktree   ║
╠══════════════════════════════════════════════════════════════════════╣
║  api-service                                      [tmux: work]      ║
║     window 2.0   ◌  idle                                            ║
╠══════════════════════════════════════════════════════════════════════╣
║  frontend                                         [tmux: infra]     ║
║     window 1.0   ⚠  waiting    Need input                           ║
╠══════════════════════════════════════════════════════════════════════╣
║  old-project                                      [tmux: work]      ║
║     window 3.0   ✗  dead       00:03 ago                            ║
╚══════════════════════════════════════════════════════════════════════╝
  [Enter] Switch to pane   [d] Dismiss dead   [q] Quit   [?] Help
```

Notes on grouping behaviour:
- Sessions are grouped by `project_name` (basename of `work_dir`) regardless of which tmux session they live in. This means two worktrees of the same repo that happen to be in different tmux sessions still appear as separate project groups (their basenames differ).
- Multiple instances within the same project group (e.g. two worktrees both named consistently) are sorted by `TmuxWindow.TmuxPane` so the layout is stable.
- Dead sessions are dimmed and pushed to the bottom of their project group. Pressing `d` removes all dead entries.

The `group_view.go` component renders each `ProjectGroup` as a Lip Gloss-styled block:
- Group header: bold project name + right-aligned tmux session label
- Session rows: indented, with status icon, status label, info column, and pane target

Key bindings:
- `j/k` or arrows — navigate rows (skips group headers automatically)
- `Enter` — switch to selected pane (runs `tmux switch-client`)
- `d` — remove dead sessions from display
- `r` — force re-scan of sessions directory
- `q` or `Ctrl+C` — quit dashboard
- `?` — toggle help overlay

### 3.6 Failure Mode Handling

**Claude Code session crashes without SessionEnd:**
The stale-session reaper goroutine runs every 30 seconds. It calls `tmux list-panes -a` and computes the set of live pane IDs. Any `SessionState` in the store whose `(TmuxSession, TmuxWindow, TmuxPane)` triplet no longer appears in the live pane list is marked `StatusDead`. The reaper also checks `UpdatedAt`: if a session has had no update for more than 5 minutes and is not `StatusDead`, it is marked dead.

**tmux server restarted:**
When tmux restarts, all pane IDs become invalid. The reaper's next tick will find no matching panes and mark all sessions dead. Meanwhile, the `check-tmux.sh` hook at `SessionStart` will detect a fresh tmux session and write new state files with new coordinates. Old dead entries are cleared when the user presses `d` or after a configurable auto-purge TTL (default: 10 minutes after `ended_at`).

**Session file partially written / corrupted:**
The watcher's JSON parser returns an error on invalid JSON. It logs the error and retries on the next fsnotify event for that file. The previous valid state remains in the store. Since writes use atomic `mv`, partial reads are only possible if the system crashes during the `mv` itself, which the OS makes extremely rare.

**Dashboard crash / restart:**
On startup, the watcher does an initial directory scan — reads all existing `*.json` files in `~/.claude/sessions/` and populates the store. The dashboard is fully stateless between runs; all state lives in the files written by hooks. Restarting the dashboard is safe at any time.

**No tmux pane found for switch:**
If `tmux switch-client` fails (pane was closed between the last reaper tick and the keypress), the dashboard catches the error and displays an inline error message: "Pane no longer exists — session marked dead." It then calls `store.MarkDead()`.

**Multiple tmux servers:**
The `TmuxSocket` field in `SessionState` carries the socket name. The `check-tmux.sh` script extracts this from `$TMUX` at session start. All `tmux -L <socket>` calls use this field. The dashboard can therefore manage sessions across multiple tmux servers simultaneously.

---

## 4. Startup Sequence

```
1. ccsdash starts
2. Parse flags: --sessions-dir (~/.claude/sessions/), --socket (auto-detect)
3. Initialize Store (empty)
4. Scan sessions-dir: read all *.json files → store.Upsert() each
5. Start fsnotify watcher goroutine on sessions-dir
6. Start stale-session reaper goroutine (30s ticker)
7. Start Bubble Tea program
8. Bubble Tea immediately renders current store state
9. waitForUpdate() cmd queued — dashboard is live
```

---

## 5. Hooks Configuration

The hooks live in `plugins/agent-overview/hooks.json`. The hook scripts are in `plugins/agent-overview/hooks/`. `${CLAUDE_PLUGIN_ROOT}` resolves to the `agent-overview` plugin directory at runtime.

```json
{
  "description": "Claude Code Session Dashboard hooks",
  "hooks": {
    "SessionStart":     [{"hooks": [{"type": "command", "command": "${CLAUDE_PLUGIN_ROOT}/hooks/check-tmux.sh",                              "timeout": 5}]}],
    "UserPromptSubmit": [{"hooks": [{"type": "command", "command": "${CLAUDE_PLUGIN_ROOT}/hooks/claude-hook.sh UserPromptSubmit", "timeout": 5}]}],
    "PreToolUse":       [{"hooks": [{"type": "command", "command": "${CLAUDE_PLUGIN_ROOT}/hooks/claude-hook.sh PreToolUse",       "timeout": 5}]}],
    "Stop":             [{"hooks": [{"type": "command", "command": "${CLAUDE_PLUGIN_ROOT}/hooks/claude-hook.sh Stop",             "timeout": 5}]}],
    "Notification":     [{"hooks": [{"type": "command", "command": "${CLAUDE_PLUGIN_ROOT}/hooks/claude-hook.sh Notification",     "timeout": 5}]}],
    "SessionEnd":       [{"hooks": [{"type": "command", "command": "${CLAUDE_PLUGIN_ROOT}/hooks/claude-hook.sh SessionEnd",       "timeout": 5}]}]
  }
}
```

The `agent-overview-tui` binary is independent — it is started manually or via a dedicated tmux pane and has no entry in `hooks.json`. It only reads the state directory that the hooks populate.
```

---

## Summary of Design Decisions

1. **File-based IPC over sockets**: Files are observable, debuggable with standard tools, and survive dashboard crashes. A Unix socket would require a running daemon; files do not.

2. **fsnotify over polling**: Zero-latency updates, no CPU waste between events.

3. **Atomic file writes from hooks**: Prevents partial-read races. Simple `mv` is sufficient on a local filesystem.

4. **Full SessionState replacement per write**: Hooks write the complete struct rather than patches. Simpler to reason about, no merge conflicts, and Go reads the whole file once per event anyway.

5. **Stale-session reaper as safety net**: Hooks can be missed (process kill -9, OOM). The reaper ensures the dashboard eventually converges to truth.

6. **tmux `-L socket` for multi-server support**: Works correctly whether one or many tmux servers are running. Extracted from `$TMUX` at session creation time, never inferred at switch time.
