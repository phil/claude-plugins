package store

import (
	"sort"
	"sync"

	"github.com/philbalchin/agent-overview-tui/internal/schema"
)

// ProjectGroup holds sessions grouped by project name.
type ProjectGroup struct {
	ProjectName string
	Sessions    []*schema.SessionState
}

// Store is a thread-safe in-memory store of session states.
type Store struct {
	mu      sync.RWMutex
	data    map[string]*schema.SessionState
	updates chan struct{}
}

// New creates a new Store.
func New() *Store {
	return &Store{
		data:    make(map[string]*schema.SessionState),
		updates: make(chan struct{}),
	}
}

// Upsert inserts or updates a session by SessionID and signals the update channel.
func (s *Store) Upsert(state *schema.SessionState) {
	s.mu.Lock()
	s.data[state.SessionID] = state
	s.mu.Unlock()
	s.notify()
}

// MarkDead sets a session's status to dead and signals the update channel.
func (s *Store) MarkDead(sessionID string) {
	s.mu.Lock()
	if sess, ok := s.data[sessionID]; ok {
		copy := *sess
		copy.Status = schema.StatusDead
		s.data[sessionID] = &copy
	}
	s.mu.Unlock()
	s.notify()
}

// DeleteDead removes all dead sessions from the store and signals the update channel.
func (s *Store) DeleteDead() {
	s.mu.Lock()
	for id, sess := range s.data {
		if sess.Status == schema.StatusDead {
			delete(s.data, id)
		}
	}
	s.mu.Unlock()
	s.notify()
}

// All returns a sorted snapshot of all sessions (by ProjectName then StartedAt).
func (s *Store) All() []*schema.SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*schema.SessionState, 0, len(s.data))
	for _, v := range s.data {
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ProjectName != result[j].ProjectName {
			return result[i].ProjectName < result[j].ProjectName
		}
		return result[i].StartedAt.Before(result[j].StartedAt)
	})
	return result
}

// GroupByProject returns sessions grouped by project, sorted alphabetically.
// Sessions within each group are sorted by StartedAt.
func (s *Store) GroupByProject() []ProjectGroup {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Build map of project -> sessions
	groupMap := make(map[string][]*schema.SessionState)
	for _, sess := range s.data {
		groupMap[sess.ProjectName] = append(groupMap[sess.ProjectName], sess)
	}

	// Sort project names
	projectNames := make([]string, 0, len(groupMap))
	for name := range groupMap {
		projectNames = append(projectNames, name)
	}
	sort.Strings(projectNames)

	// Build sorted groups
	groups := make([]ProjectGroup, 0, len(projectNames))
	for _, name := range projectNames {
		sessions := groupMap[name]
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].StartedAt.Before(sessions[j].StartedAt)
		})
		groups = append(groups, ProjectGroup{
			ProjectName: name,
			Sessions:    sessions,
		})
	}
	return groups
}

// Updates returns the channel that receives a signal when the store changes.
func (s *Store) Updates() <-chan struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.updates
}

// notify closes the current updates channel and creates a new one,
// waking up any goroutine blocked on the old channel.
func (s *Store) notify() {
	s.mu.Lock()
	old := s.updates
	s.updates = make(chan struct{})
	s.mu.Unlock()
	close(old)
}
