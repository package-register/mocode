package records

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/package-register/mocode/internal/config"
	"github.com/stretchr/testify/require"
)

func TestNewFileRecorder(t *testing.T) {
	// Test with nil options (should return noop)
	r := NewFileRecorder(nil)
	require.IsType(t, &noopRecorder{}, r)

	// Test with disabled options (should return noop)
	opts := &config.RecordsOptions{Enabled: false}
	r = NewFileRecorder(opts)
	require.IsType(t, &noopRecorder{}, r)

	// Test with enabled but empty path
	opts = &config.RecordsOptions{Enabled: true, Path: ""}
	r = NewFileRecorder(opts)
	require.IsType(t, &fileRecorder{}, r)
	fr := r.(*fileRecorder)
	require.Equal(t, ".records", fr.basePath)
}

func TestFileRecorder_ToolCalls(t *testing.T) {
	tmpDir := t.TempDir()

	opts := &config.RecordsOptions{
		Enabled:     true,
		Path:        tmpDir,
		RecordTypes: []config.RecordType{config.RecordTypeToolCalls},
	}
	r := NewFileRecorder(opts)
	require.IsType(t, &fileRecorder{}, r)

	// Record a tool call
	record := ToolCallRecord{
		Timestamp:  time.Now(),
		SessionID:  "test-session-123",
		ToolName:   "mcp_notes_query",
		Input:      map[string]any{"keyword": "Taskfile"},
		RawInput:   `{"keyword": "Taskfile"}`,
		Output:     "Found 3 notes",
		DurationMs: 150,
		MCPName:    "notes",
	}
	err := r.RecordToolCall(context.Background(), record)
	require.NoError(t, err)

	// Verify file was created
	entries, err := os.ReadDir(filepath.Join(tmpDir, "tool_calls"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Contains(t, entries[0].Name(), ".jsonl")

	// Record another call - should append to same file
	record2 := ToolCallRecord{
		Timestamp:  time.Now(),
		SessionID:  "test-session-456",
		ToolName:   "mcp_notes_search",
		Input:      map[string]any{"tags": []string{"go", "test"}},
		DurationMs: 100,
	}
	err = r.RecordToolCall(context.Background(), record2)
	require.NoError(t, err)

	entries, err = os.ReadDir(filepath.Join(tmpDir, "tool_calls"))
	require.NoError(t, err)
	require.Len(t, entries, 1) // Still 1 file
}

func TestFileRecorder_Errors(t *testing.T) {
	tmpDir := t.TempDir()

	opts := &config.RecordsOptions{
		Enabled:     true,
		Path:        tmpDir,
		RecordTypes: []config.RecordType{config.RecordTypeErrors},
	}
	r := NewFileRecorder(opts)

	// Record an error
	record := ErrorRecord{
		Timestamp: time.Now(),
		SessionID: "test-session-789",
		ErrorType: "mcp_tool_error",
		Message:   "至少提供 tags, exact_name 或 keyword 中的一个参数",
		Context: map[string]any{
			"tool_name": "query",
			"input":     map[string]any{"keyword": nil},
		},
	}
	err := r.RecordError(context.Background(), record)
	require.NoError(t, err)

	// Verify file was created
	entries, err := os.ReadDir(filepath.Join(tmpDir, "errors"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestFileRecorder_DisabledType(t *testing.T) {
	tmpDir := t.TempDir()

	// Only enable tool_calls, not errors
	opts := &config.RecordsOptions{
		Enabled:     true,
		Path:        tmpDir,
		RecordTypes: []config.RecordType{config.RecordTypeToolCalls},
	}
	r := NewFileRecorder(opts)

	// This should be a no-op since errors are not enabled
	record := ErrorRecord{
		Timestamp: time.Now(),
		SessionID: "test-session",
		ErrorType: "test_error",
		Message:   "test error",
	}
	err := r.RecordError(context.Background(), record)
	require.NoError(t, err) // No error, but nothing was recorded

	// Verify errors directory was NOT created
	_, err = os.Stat(filepath.Join(tmpDir, "errors"))
	require.True(t, os.IsNotExist(err))
}

func TestNoopRecorder(t *testing.T) {
	r := &noopRecorder{}

	// All methods should be no-ops
	require.NoError(t, r.RecordToolCall(context.Background(), ToolCallRecord{}))
	require.NoError(t, r.RecordError(context.Background(), ErrorRecord{}))
	require.NoError(t, r.RecordMCPRequest(context.Background(), MCPRequestRecord{}))
	require.NoError(t, r.RecordMCPResponse(context.Background(), MCPResponseRecord{}))
	require.NoError(t, r.RecordLoopDetection(context.Background(), LoopDetectionRecord{}))
}

func TestSetService(t *testing.T) {
	// Save original
	original := defaultRecorder
	defer SetService(original)

	// Set a new recorder
	newRecorder := &noopRecorder{}
	SetService(newRecorder)
	require.Equal(t, newRecorder, Service())

	// Service should return the new recorder
	require.Equal(t, newRecorder, Service())
}

func TestCreateToolCallRecord(t *testing.T) {
	sessionID := "sess-123"
	mcpName := "notes"
	toolName := "query"
	rawInput := `{"keyword": "test"}`
	args := map[string]any{"keyword": "test"}
	output := "found"
	duration := 100 * time.Millisecond

	rec := CreateToolCallRecord(sessionID, mcpName, toolName, rawInput, args, output, nil, duration)

	require.Equal(t, sessionID, rec.SessionID)
	require.Equal(t, mcpName, rec.MCPName)
	require.Equal(t, toolName, rec.ToolName)
	require.Equal(t, rawInput, rec.RawInput)
	require.Equal(t, args, rec.Input)
	require.Equal(t, output, rec.Output)
	require.Equal(t, int64(100), rec.DurationMs)
	require.Empty(t, rec.Error)
}

func TestCreateToolCallRecord_WithError(t *testing.T) {
	rec := CreateToolCallRecord(
		"sess-123",
		"notes",
		"query",
		`{"keyword": null}`,
		map[string]any{"keyword": nil},
		"",
		&testError{msg: "test error"},
		50*time.Millisecond,
	)

	require.Equal(t, "test error", rec.Error)
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
