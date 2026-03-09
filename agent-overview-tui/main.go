package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/philbalchin/agent-overview-tui/internal/schema"
	"github.com/philbalchin/agent-overview-tui/internal/store"
	"github.com/philbalchin/agent-overview-tui/internal/tmux"
	"github.com/philbalchin/agent-overview-tui/internal/tui"
)

var sessionsDir string

var rootCmd = &cobra.Command{
	Use:   "ccsdash",
	Short: "Claude Code Session Dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := store.New()

		w, err := store.NewWatcher(sessionsDir, s)
		if err != nil {
			return fmt.Errorf("watcher: %w", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		w.Start(ctx)

		// Start stale-session reaper goroutine (30s ticker)
		go runReaper(ctx, s)

		m := tui.New(s)
		p := tea.NewProgram(m, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

// runReaper periodically marks dead any session that is no longer alive in tmux
// or has not been updated in more than 5 minutes.
func runReaper(ctx context.Context, s *store.Store) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reap(s)
		}
	}
}

// reap checks all sessions and marks dead any that are no longer alive.
func reap(s *store.Store) {
	sessions := s.All()
	if len(sessions) == 0 {
		return
	}

	// Collect unique sockets
	sockets := make(map[string]struct{})
	for _, sess := range sessions {
		if sess.TmuxSocket != "" {
			sockets[sess.TmuxSocket] = struct{}{}
		}
	}

	// Build a set of live panes per socket: "socket:session:window:pane"
	livePanes := make(map[string]struct{})
	for socket := range sockets {
		panes, err := tmux.ListPanes(socket)
		if err != nil {
			log.Printf("reaper: list panes for socket %q: %v", socket, err)
			continue
		}
		for _, p := range panes {
			key := fmt.Sprintf("%s:%s:%d:%d", socket, p.Session, p.Window, p.Pane)
			livePanes[key] = struct{}{}
		}
	}

	staleThreshold := 5 * time.Minute
	now := time.Now()

	for _, sess := range sessions {
		if sess.Status == schema.StatusDead {
			continue
		}

		// Check if pane is still alive
		if sess.TmuxSocket != "" {
			key := fmt.Sprintf("%s:%s:%d:%d", sess.TmuxSocket, sess.TmuxSession, sess.TmuxWindow, sess.TmuxPane)
			if _, alive := livePanes[key]; !alive {
				log.Printf("reaper: marking dead (pane gone): %s", sess.SessionID)
				s.MarkDead(sess.SessionID)
				continue
			}
		}

		// Check for stale (no update > 5 minutes)
		if now.Sub(sess.UpdatedAt) > staleThreshold {
			log.Printf("reaper: marking dead (stale): %s", sess.SessionID)
			s.MarkDead(sess.SessionID)
		}
	}
}

func main() {
	home, _ := os.UserHomeDir()
	defaultDir := filepath.Join(home, ".claude", "sessions")
	rootCmd.Flags().StringVar(&sessionsDir, "sessions-dir", defaultDir, "Path to sessions directory")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
