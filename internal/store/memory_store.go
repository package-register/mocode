package store

import (
	"context"
	"strings"
	"time"

	"github.com/package-register/mocode/internal/knowledge/memory"
)

// MemoryStore provides file-based memory persistence.
// It loads entries into memory at startup for fast keyword search.
// Future: persist to memory/entries.jsonl.
type MemoryStore struct {
	store   *Store
	entries []*memory.Entry
}

func newMemoryStore(s *Store) *MemoryStore {
	ms := &MemoryStore{store: s}
	ms.loadEntries()
	return ms
}

func (ms *MemoryStore) loadEntries() {
	// Future: load from memory/entries.jsonl
	ms.entries = nil
}

func (ms *MemoryStore) AddMemory(ctx context.Context, appName, userID, mem string,
	topics []string, kind memory.Kind, eventTime *time.Time,
	participants []string, location string,
) error {
	now := time.Now()
	entry := &memory.Entry{
		AppName: appName,
		UserID:  userID,
		Memory: &memory.Memory{
			Memory:       mem,
			Topics:       topics,
			Kind:         kind,
			EventTime:    eventTime,
			Participants: participants,
			Location:     location,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	ms.entries = append(ms.entries, entry)
	return nil
}

func (ms *MemoryStore) UpdateMemory(ctx context.Context, appName, userID, memoryID, mem string,
	topics []string, kind memory.Kind, eventTime *time.Time,
	participants []string, location string,
) error {
	for _, e := range ms.entries {
		if e.ID == memoryID {
			e.Memory.Memory = mem
			e.Memory.Topics = topics
			e.Memory.Kind = kind
			e.Memory.EventTime = eventTime
			e.Memory.Participants = participants
			e.Memory.Location = location
			e.UpdatedAt = time.Now()
			return nil
		}
	}
	return nil
}

func (ms *MemoryStore) DeleteMemory(ctx context.Context, appName, userID, memoryID string) error {
	for i, e := range ms.entries {
		if e.ID == memoryID && e.AppName == appName && e.UserID == userID {
			ms.entries = append(ms.entries[:i], ms.entries[i+1:]...)
			return nil
		}
	}
	return nil
}

func (ms *MemoryStore) ClearMemories(ctx context.Context, appName, userID string) error {
	var kept []*memory.Entry
	for _, e := range ms.entries {
		if e.AppName != appName || e.UserID != userID {
			kept = append(kept, e)
		}
	}
	ms.entries = kept
	return nil
}

func (ms *MemoryStore) ReadMemories(ctx context.Context, appName, userID string, limit int) ([]*memory.Entry, error) {
	var result []*memory.Entry
	for _, e := range ms.entries {
		if e.AppName == appName && e.UserID == userID {
			result = append(result, e)
		}
	}
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	return result, nil
}

func (ms *MemoryStore) SearchMemories(ctx context.Context, appName, userID, query string, limit int) ([]*memory.Entry, error) {
	// Filter by app/user first, then do simple keyword search
	var filtered []*memory.Entry
	for _, e := range ms.entries {
		if e.AppName == appName && e.UserID == userID {
			filtered = append(filtered, e)
		}
	}
	// Simple keyword match search
	var results []*memory.Entry
	ql := strings.ToLower(query)
	for _, e := range filtered {
		if e.Memory == nil {
			continue
		}
		if strings.Contains(strings.ToLower(e.Memory.Memory), ql) {
			results = append(results, e)
		}
	}
	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}
	return results, nil
}

func (ms *MemoryStore) Close() error {
	return nil
}
