package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAgentWatcher_NewAgentWatcher(t *testing.T) {
	dir := t.TempDir()
	store := &ConfigStore{
		config: &Config{
			Options: &Options{
				AgentsDir: dir,
			},
			Agents: make(map[string]Agent),
		},
	}

	watcher, err := NewAgentWatcher(store)
	if err != nil {
		t.Fatalf("Failed to create agent watcher: %v", err)
	}
	defer watcher.Close()

	if watcher.AgentsDir() != dir {
		t.Errorf("Expected agents dir %s, got %s", dir, watcher.AgentsDir())
	}
}

func TestAgentWatcher_WatchAndReload(t *testing.T) {
	dir := t.TempDir()
	store := &ConfigStore{
		config: &Config{
			Options: &Options{
				AgentsDir: dir,
			},
			Agents: make(map[string]Agent),
		},
	}

	watcher, err := NewAgentWatcher(store)
	if err != nil {
		t.Fatalf("Failed to create agent watcher: %v", err)
	}
	defer watcher.Close()

	// Track reload events
	reloaded := make(chan string, 10)
	watcher.OnReload(func(id string) {
		reloaded <- id
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := watcher.Watch(ctx); err != nil {
		t.Fatalf("Failed to start watching: %v", err)
	}

	// Wait a bit for watcher to start
	time.Sleep(100 * time.Millisecond)

	// Create a new agent file
	agentContent := `---
id: "test-agent"
name: "Test Agent"
description: "A test agent"
tools:
  - "bash"
  - "view"
---

# Test Agent
This is a test agent.
`
	agentPath := filepath.Join(dir, "test-agent.md")
	if err := os.WriteFile(agentPath, []byte(agentContent), 0o644); err != nil {
		t.Fatalf("Failed to write agent file: %v", err)
	}

	// Wait for reload event
	select {
	case id := <-reloaded:
		if id != "test-agent" {
			t.Errorf("Expected reload for test-agent, got %s", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for reload event")
	}

	// Verify agent was loaded
	if _, ok := store.config.Agents["test-agent"]; !ok {
		t.Error("Expected test-agent to be loaded")
	}
}

func TestAgentWatcher_WatchDelete(t *testing.T) {
	dir := t.TempDir()

	// Pre-populate an agent
	store := &ConfigStore{
		config: &Config{
			Options: &Options{
				AgentsDir: dir,
			},
			Agents: map[string]Agent{
				"delete-me": {
					ID:   "delete-me",
					Name: "Delete Me",
				},
			},
		},
	}

	watcher, err := NewAgentWatcher(store)
	if err != nil {
		t.Fatalf("Failed to create agent watcher: %v", err)
	}
	defer watcher.Close()

	// Track reload events
	reloaded := make(chan string, 10)
	watcher.OnReload(func(id string) {
		reloaded <- id
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := watcher.Watch(ctx); err != nil {
		t.Fatalf("Failed to start watching: %v", err)
	}

	// Wait a bit for watcher to start
	time.Sleep(100 * time.Millisecond)

	// Create and then delete an agent file
	agentPath := filepath.Join(dir, "delete-me.md")
	if err := os.WriteFile(agentPath, []byte("# Delete Me"), 0o644); err != nil {
		t.Fatalf("Failed to write agent file: %v", err)
	}

	// Wait for create event
	select {
	case <-reloaded:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for create event")
	}

	// Delete the file
	if err := os.Remove(agentPath); err != nil {
		t.Fatalf("Failed to delete agent file: %v", err)
	}

	// Wait for delete event
	select {
	case id := <-reloaded:
		if id != "delete-me" {
			t.Errorf("Expected reload for delete-me, got %s", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for delete event")
	}

	// Verify agent was removed
	if _, ok := store.config.Agents["delete-me"]; ok {
		t.Error("Expected delete-me to be removed")
	}
}

func TestAgentWatcher_Debounce(t *testing.T) {
	dir := t.TempDir()
	store := &ConfigStore{
		config: &Config{
			Options: &Options{
				AgentsDir: dir,
			},
			Agents: make(map[string]Agent),
		},
	}

	watcher, err := NewAgentWatcher(store)
	if err != nil {
		t.Fatalf("Failed to create agent watcher: %v", err)
	}
	defer watcher.Close()

	// Track reload events
	reloadCount := make(chan int, 10)
	count := 0
	watcher.OnReload(func(id string) {
		count++
		reloadCount <- count
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := watcher.Watch(ctx); err != nil {
		t.Fatalf("Failed to start watching: %v", err)
	}

	// Wait a bit for watcher to start
	time.Sleep(100 * time.Millisecond)

	// Write multiple times quickly
	agentPath := filepath.Join(dir, "debounce-test.md")
	for i := 0; i < 5; i++ {
		content := `---
id: "debounce-test"
name: "Debounce Test"
description: "Test debounce"
---
# Version ` + string(rune('0'+i))
		if err := os.WriteFile(agentPath, []byte(content), 0o644); err != nil {
			t.Fatalf("Failed to write agent file: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for debounce to settle
	select {
	case c := <-reloadCount:
		// Should only get 1 reload due to debounce
		if c > 2 {
			t.Errorf("Expected at most 2 reloads due to debounce, got %d", c)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for reload event")
	}
}

func TestAgentWatcher_ProtectedAgents(t *testing.T) {
	dir := t.TempDir()
	store := &ConfigStore{
		config: &Config{
			Options: &Options{
				AgentsDir: dir,
			},
			Agents: map[string]Agent{
				AgentCoder: {
					ID:   AgentCoder,
					Name: "Coder",
				},
				AgentTask: {
					ID:   AgentTask,
					Name: "Task",
				},
			},
		},
	}

	watcher, err := NewAgentWatcher(store)
	if err != nil {
		t.Fatalf("Failed to create agent watcher: %v", err)
	}
	defer watcher.Close()

	// Track reload events
	reloaded := make(chan string, 10)
	watcher.OnReload(func(id string) {
		reloaded <- id
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := watcher.Watch(ctx); err != nil {
		t.Fatalf("Failed to start watching: %v", err)
	}

	// Wait a bit for watcher to start
	time.Sleep(100 * time.Millisecond)

	// Try to delete coder agent file (should not remove from config)
	coderPath := filepath.Join(dir, AgentCoder+".md")
	if err := os.WriteFile(coderPath, []byte("# Coder"), 0o644); err != nil {
		t.Fatalf("Failed to write coder file: %v", err)
	}

	// Wait for event
	select {
	case <-reloaded:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for event")
	}

	// Delete the file
	if err := os.Remove(coderPath); err != nil {
		t.Fatalf("Failed to delete coder file: %v", err)
	}

	// Wait for delete event
	select {
	case <-reloaded:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for delete event")
	}

	// Verify protected agents still exist
	if _, ok := store.config.Agents[AgentCoder]; !ok {
		t.Error("Expected coder agent to still exist (protected)")
	}
	if _, ok := store.config.Agents[AgentTask]; !ok {
		t.Error("Expected task agent to still exist (protected)")
	}
}
