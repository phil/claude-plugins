package store

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/philbalchin/agent-overview-tui/internal/schema"
)

// Watcher watches a directory for session JSON files and updates the store.
type Watcher struct {
	dir     string
	store   *Store
	watcher *fsnotify.Watcher
}

// NewWatcher creates a new Watcher for the given directory.
func NewWatcher(dir string, store *Store) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		fw.Close()
		return nil, err
	}

	if err := fw.Add(dir); err != nil {
		fw.Close()
		return nil, err
	}

	return &Watcher{
		dir:     dir,
		store:   store,
		watcher: fw,
	}, nil
}

// Start begins watching the directory for changes in a background goroutine.
// It first scans existing files, then watches for new changes.
func (w *Watcher) Start(ctx context.Context) {
	// Initial scan of existing files
	w.scanExisting()

	go func() {
		defer w.watcher.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-w.watcher.Events:
				if !ok {
					return
				}
				if !strings.HasSuffix(event.Name, ".json") {
					continue
				}
				switch {
				case event.Has(fsnotify.Create) || event.Has(fsnotify.Write):
					w.loadFile(event.Name)
				case event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename):
					sessionID := sessionIDFromPath(event.Name)
					if sessionID != "" {
						w.store.MarkDead(sessionID)
					}
				}
			case err, ok := <-w.watcher.Errors:
				if !ok {
					return
				}
				log.Printf("watcher error: %v", err)
			}
		}
	}()
}

// scanExisting reads all existing *.json files in the directory and upserts them.
func (w *Watcher) scanExisting() {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		log.Printf("watcher: scan existing: %v", err)
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		w.loadFile(filepath.Join(w.dir, entry.Name()))
	}
}

// loadFile reads and parses a JSON session file and upserts it into the store.
func (w *Watcher) loadFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("watcher: read %s: %v", path, err)
		return
	}
	var state schema.SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		log.Printf("watcher: parse %s: %v", path, err)
		return
	}
	w.store.Upsert(&state)
}

// sessionIDFromPath extracts the session ID from a file path by stripping the .json extension.
func sessionIDFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, ".json")
}
