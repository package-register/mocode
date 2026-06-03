package config

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// AgentWatcher monitors the agents directory for file changes
// and automatically reloads agent configurations.
type AgentWatcher struct {
	store     *ConfigStore
	watcher   *fsnotify.Watcher
	agentsDir string
	mu        sync.Mutex
	debounce  map[string]*time.Timer
	done      chan struct{}
	onReload  func(id string) // callback when agent is reloaded
}

// NewAgentWatcher creates a new agent file watcher.
func NewAgentWatcher(store *ConfigStore) (*AgentWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	agentsDir := resolveAgentsDir(store.config)

	return &AgentWatcher{
		store:     store,
		watcher:   w,
		agentsDir: agentsDir,
		debounce:  make(map[string]*time.Timer),
		done:      make(chan struct{}),
	}, nil
}

// OnReload sets a callback function that is called when an agent is reloaded.
func (aw *AgentWatcher) OnReload(fn func(id string)) {
	aw.mu.Lock()
	defer aw.mu.Unlock()
	aw.onReload = fn
}

// Watch starts watching the agents directory.
// It performs an initial scan and then processes file system events.
func (aw *AgentWatcher) Watch(ctx context.Context) error {
	// Add agents directory to watcher
	if err := aw.watcher.Add(aw.agentsDir); err != nil {
		if !strings.Contains(err.Error(), "no such file") {
			return err
		}
		// Directory doesn't exist, try to create it
		if err := createAgentsDir(aw.agentsDir); err != nil {
			slog.Warn("Failed to create agents directory", "dir", aw.agentsDir, "error", err)
		} else if err := aw.watcher.Add(aw.agentsDir); err != nil {
			return err
		}
	}

	// Start event loop
	go aw.eventLoop(ctx)

	slog.Info("Agent watcher started", "dir", aw.agentsDir)
	return nil
}

// eventLoop processes file system events.
func (aw *AgentWatcher) eventLoop(ctx context.Context) {
	defer close(aw.done)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-aw.watcher.Events:
			if !ok {
				return
			}
			aw.handleEvent(ctx, event)
		case err, ok := <-aw.watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("AgentWatcher error", "error", err)
		}
	}
}

// handleEvent processes a single file system event with debouncing.
func (aw *AgentWatcher) handleEvent(ctx context.Context, event fsnotify.Event) {
	// Only care about .md files
	if !strings.HasSuffix(strings.ToLower(event.Name), ".md") {
		return
	}

	// Ignore hidden files and version file
	base := filepath.Base(event.Name)
	if strings.HasPrefix(base, ".") || base == agentsVersionFile {
		return
	}

	if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) || event.Has(fsnotify.Remove) {
		slog.Debug("Agent file event detected", "type", event.Op.String(), "path", event.Name)
		aw.debounceEvent(ctx, event.Name)
	}
}

func (aw *AgentWatcher) debounceEvent(ctx context.Context, path string) {
	aw.mu.Lock()
	defer aw.mu.Unlock()

	if timer, ok := aw.debounce[path]; ok {
		timer.Stop()
	}

	aw.debounce[path] = time.AfterFunc(500*time.Millisecond, func() {
		aw.handleChange(ctx, path)

		aw.mu.Lock()
		delete(aw.debounce, path)
		aw.mu.Unlock()
	})
}

func (aw *AgentWatcher) handleChange(_ context.Context, path string) {
	// Extract agent ID from filename
	base := filepath.Base(path)
	id := strings.TrimSuffix(base, filepath.Ext(base))
	if id == "" {
		return
	}

	// Check if file was deleted
	if _, err := os.Stat(path); os.IsNotExist(err) {
		slog.Info("Agent file deleted, removing", "id", id, "path", path)
		aw.store.RemoveAgent(id)

		// Call reload callback
		aw.mu.Lock()
		onReload := aw.onReload
		aw.mu.Unlock()

		if onReload != nil {
			onReload(id)
		}
		return
	}

	slog.Info("Agent file changed, reloading", "id", id, "path", path)

	// Reload the specific agent
	if err := aw.store.ReloadAgent(id, path); err != nil {
		slog.Warn("Failed to reload agent", "id", id, "error", err)
		return
	}

	// Call reload callback
	aw.mu.Lock()
	onReload := aw.onReload
	aw.mu.Unlock()

	if onReload != nil {
		onReload(id)
	}
}

// Close stops the watcher.
func (aw *AgentWatcher) Close() error {
	return aw.watcher.Close()
}

// Done returns a channel that's closed when the event loop exits.
func (aw *AgentWatcher) Done() <-chan struct{} {
	return aw.done
}

// AgentsDir returns the agents directory being watched.
func (aw *AgentWatcher) AgentsDir() string {
	return aw.agentsDir
}

// createAgentsDir creates the agents directory if it doesn't exist.
func createAgentsDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}
