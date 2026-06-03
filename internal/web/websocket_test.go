package web

import (
	"testing"

	"github.com/package-register/mocode/internal/session"
)

func TestStatusTokenUsageSplitsCachedTokens(t *testing.T) {
	got := statusTokenUsage(session.Session{
		PromptTokens:        135,
		CompletionTokens:    50,
		CacheReadTokens:     25,
		CacheCreationTokens: 10,
	})

	if got["input_other"] != int64(100) {
		t.Fatalf("expected regular input tokens 100, got %#v", got["input_other"])
	}
	if got["input_cache_read"] != int64(25) {
		t.Fatalf("expected cache read tokens 25, got %#v", got["input_cache_read"])
	}
	if got["input_cache_creation"] != int64(10) {
		t.Fatalf("expected cache creation tokens 10, got %#v", got["input_cache_creation"])
	}
	if got["output"] != int64(50) {
		t.Fatalf("expected output tokens 50, got %#v", got["output"])
	}
}

func TestStatusTokenUsageClampsNegativeRegularInput(t *testing.T) {
	got := statusTokenUsage(session.Session{
		PromptTokens:        10,
		CacheReadTokens:     25,
		CacheCreationTokens: 10,
	})

	if got["input_other"] != int64(0) {
		t.Fatalf("expected regular input tokens to clamp at 0, got %#v", got["input_other"])
	}
}
