package store

import (
	"fmt"
	"time"
)

// SessionStats holds aggregated usage stats for a single session.
type SessionStats struct {
	SessionID        string
	Title            string
	MessageCount     int64
	PromptTokens     int64
	CompletionTokens int64
	Cost             float64
	CreatedAt        int64
}

// DayUsage holds stats aggregated by day.
type DayUsage struct {
	Date             string  `json:"date"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	Cost             float64 `json:"cost"`
	Sessions         int     `json:"sessions"`
}

// TotalStats holds aggregate stats across all sessions.
type TotalStats struct {
	Sessions         int     `json:"sessions"`
	Messages         int64   `json:"messages"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	Cost             float64 `json:"cost"`
}

// StatsEngine computes usage statistics from the file-based store.
type StatsEngine struct {
	store *Store
}

// UsageByDay returns token/cost usage aggregated by day.
func (se *StatsEngine) UsageByDay() ([]DayUsage, error) {
	byDay := make(map[string]*DayUsage)
	for _, meta := range se.store.sessions.index.Sessions {
		if meta.PromptTokens == 0 && meta.CompletionTokens == 0 {
			continue
		}
		date := formatTime(meta.CreatedAt)
		if date == "" {
			continue
		}
		if _, ok := byDay[date]; !ok {
			byDay[date] = &DayUsage{Date: date}
		}
		du := byDay[date]
		du.PromptTokens += meta.PromptTokens
		du.CompletionTokens += meta.CompletionTokens
		du.Cost += meta.Cost
		du.Sessions++
	}

	result := make([]DayUsage, 0, len(byDay))
	for _, du := range byDay {
		result = append(result, *du)
	}
	return result, nil
}

// TotalStats returns aggregate usage statistics across all sessions.
func (se *StatsEngine) TotalStats() TotalStats {
	var total TotalStats
	for _, meta := range se.store.sessions.index.Sessions {
		total.Sessions++
		total.Messages += meta.MessageCount
		total.PromptTokens += meta.PromptTokens
		total.CompletionTokens += meta.CompletionTokens
		total.Cost += meta.Cost
	}
	return total
}

// SessionStats returns stats for a specific session.
func (se *StatsEngine) SessionStats(sessionID string) (*SessionStats, error) {
	meta, ok := se.store.sessions.index.Sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return &SessionStats{
		SessionID:        meta.ID,
		Title:            meta.Title,
		MessageCount:     meta.MessageCount,
		PromptTokens:     meta.PromptTokens,
		CompletionTokens: meta.CompletionTokens,
		Cost:             meta.Cost,
		CreatedAt:        meta.CreatedAt,
	}, nil
}

// formatTimeUnix is a convenience wrapper around time.Unix.
func _timeUnix(sec int64) time.Time {
	return time.Unix(sec, 0)
}
