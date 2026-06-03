package records

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/package-register/mocode/internal/config"
)

var (
	defaultRecorder Recorder = &noopRecorder{}
	mu              sync.RWMutex
)

// Recorder is the interface for recording debug/execution data.
type Recorder interface {
	RecordToolCall(ctx context.Context, record ToolCallRecord) error
	RecordError(ctx context.Context, record ErrorRecord) error
	RecordMCPRequest(ctx context.Context, record MCPRequestRecord) error
	RecordMCPResponse(ctx context.Context, record MCPResponseRecord) error
	RecordLoopDetection(ctx context.Context, record LoopDetectionRecord) error
}

// Service returns the global recorder instance.
func Service() Recorder {
	mu.RLock()
	defer mu.RUnlock()
	return defaultRecorder
}

// SetService sets the global recorder instance.
func SetService(r Recorder) {
	mu.Lock()
	defer mu.Unlock()
	defaultRecorder = r
}

// ToolCallRecord represents a recorded tool call.
type ToolCallRecord struct {
	Timestamp  time.Time      `json:"timestamp"`
	SessionID  string         `json:"session_id"`
	ToolName   string         `json:"tool_name"`
	Input      map[string]any `json:"input"`
	RawInput   string         `json:"raw_input"`
	Output     string         `json:"output,omitempty"`
	Error      string         `json:"error,omitempty"`
	DurationMs int64          `json:"duration_ms"`
	MCPName    string         `json:"mcp_name,omitempty"`
}

// ErrorRecord represents a recorded error.
type ErrorRecord struct {
	Timestamp time.Time      `json:"timestamp"`
	SessionID string         `json:"session_id"`
	ErrorType string         `json:"error_type"`
	Message   string         `json:"message"`
	Context   map[string]any `json:"context,omitempty"`
	Stack     string         `json:"stack,omitempty"`
	ToolName  string         `json:"tool_name,omitempty"`
	MCPName   string         `json:"mcp_name,omitempty"`
}

// MCPRequestRecord represents a recorded MCP request.
type MCPRequestRecord struct {
	Timestamp time.Time      `json:"timestamp"`
	SessionID string         `json:"session_id"`
	MCPName   string         `json:"mcp_name"`
	Method    string         `json:"method"`
	Params    map[string]any `json:"params"`
	RawParams string         `json:"raw_params"`
}

// MCPResponseRecord represents a recorded MCP response.
type MCPResponseRecord struct {
	Timestamp  time.Time `json:"timestamp"`
	SessionID  string    `json:"session_id"`
	MCPName    string    `json:"mcp_name"`
	Method     string    `json:"method"`
	Response   any       `json:"response,omitempty"`
	Error      string    `json:"error,omitempty"`
	DurationMs int64     `json:"duration_ms"`
}

// LoopDetectionRecord represents a loop detection event.
type LoopDetectionRecord struct {
	Timestamp   time.Time `json:"timestamp"`
	SessionID   string    `json:"session_id"`
	ToolName    string    `json:"tool_name"`
	RepeatCount int       `json:"repeat_count"`
	WindowSize  int       `json:"window_size"`
	MaxRepeats  int       `json:"max_repeats"`
	Signature   string    `json:"signature"`
}

// fileRecorder implements Recorder by writing to local files.
type fileRecorder struct {
	basePath    string
	recordTypes map[config.RecordType]bool
	maxFileSize int64
	mu          sync.Mutex
}

// NewFileRecorder creates a new file-based recorder.
func NewFileRecorder(opts *config.RecordsOptions) Recorder {
	if opts == nil || !opts.Enabled {
		return &noopRecorder{}
	}

	recordTypes := make(map[config.RecordType]bool)
	if len(opts.RecordTypes) == 0 {
		recordTypes[config.RecordTypeToolCalls] = true
		recordTypes[config.RecordTypeErrors] = true
	} else {
		for _, rt := range opts.RecordTypes {
			recordTypes[rt] = true
		}
	}

	path := opts.Path
	if path == "" {
		path = ".records"
	}

	maxSize := int64(opts.MaxFileSizeMB) * 1024 * 1024
	if maxSize == 0 {
		maxSize = 10 * 1024 * 1024
	}

	return &fileRecorder{
		basePath:    path,
		recordTypes: recordTypes,
		maxFileSize: maxSize,
	}
}

// RecordToolCall records a tool call if enabled.
func (r *fileRecorder) RecordToolCall(ctx context.Context, record ToolCallRecord) error {
	if !r.recordTypes[config.RecordTypeToolCalls] {
		return nil
	}
	return r.writeRecord(ctx, "tool_calls", record)
}

// RecordError records an error if enabled.
func (r *fileRecorder) RecordError(ctx context.Context, record ErrorRecord) error {
	if !r.recordTypes[config.RecordTypeErrors] {
		return nil
	}
	return r.writeRecord(ctx, "errors", record)
}

// RecordMCPRequest records an MCP request if enabled.
func (r *fileRecorder) RecordMCPRequest(ctx context.Context, record MCPRequestRecord) error {
	if !r.recordTypes[config.RecordTypeMCPRequests] {
		return nil
	}
	return r.writeRecord(ctx, "mcp_requests", record)
}

// RecordMCPResponse records an MCP response if enabled.
func (r *fileRecorder) RecordMCPResponse(ctx context.Context, record MCPResponseRecord) error {
	if !r.recordTypes[config.RecordTypeMCPResponses] {
		return nil
	}
	return r.writeRecord(ctx, "mcp_responses", record)
}

// RecordLoopDetection records a loop detection event if enabled.
func (r *fileRecorder) RecordLoopDetection(ctx context.Context, record LoopDetectionRecord) error {
	if !r.recordTypes[config.RecordTypeLoopDetections] {
		return nil
	}
	return r.writeRecord(ctx, "loop_detections", record)
}

func (r *fileRecorder) writeRecord(ctx context.Context, category string, record any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Ensure directory exists
	dir := filepath.Join(r.basePath, category)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create records directory: %w", err)
	}

	// Determine filename
	date := time.Now().Format("2006-01-02")
	baseFile := filepath.Join(dir, date+".jsonl")

	// Check file size and rotate if necessary
	if info, err := os.Stat(baseFile); err == nil && info.Size() >= r.maxFileSize {
		// Find next available number
		num := 1
		for {
			newFile := filepath.Join(dir, fmt.Sprintf("%s_%03d.jsonl", date, num))
			if _, err := os.Stat(newFile); os.IsNotExist(err) {
				baseFile = newFile
				break
			}
			num++
			if num > 999 {
				baseFile = filepath.Join(dir, fmt.Sprintf("%s_%d.jsonl", date, time.Now().Unix()))
				break
			}
		}
	}

	// Marshal record
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	// Write as JSONL
	file, err := os.OpenFile(baseFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open record file: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write record: %w", err)
	}

	return nil
}

// noopRecorder is a recorder that does nothing.
type noopRecorder struct{}

func (n *noopRecorder) RecordToolCall(ctx context.Context, record ToolCallRecord) error {
	return nil
}

func (n *noopRecorder) RecordError(ctx context.Context, record ErrorRecord) error {
	return nil
}

func (n *noopRecorder) RecordMCPRequest(ctx context.Context, record MCPRequestRecord) error {
	return nil
}

func (n *noopRecorder) RecordMCPResponse(ctx context.Context, record MCPResponseRecord) error {
	return nil
}

func (n *noopRecorder) RecordLoopDetection(ctx context.Context, record LoopDetectionRecord) error {
	return nil
}

// InitFromConfig initializes the global recorder from config.
func InitFromConfig(cfg *config.Config) {
	if cfg == nil || cfg.Options == nil || cfg.Options.Records == nil {
		SetService(&noopRecorder{})
		return
	}
	SetService(NewFileRecorder(cfg.Options.Records))
}

// Helper functions for creating records from tool calls

// CreateToolCallRecord creates a ToolCallRecord from MCP tool execution data.
func CreateToolCallRecord(sessionID, mcpName, toolName, rawInput string, args map[string]any, output string, err error, duration time.Duration) ToolCallRecord {
	record := ToolCallRecord{
		Timestamp:  time.Now(),
		SessionID:  sessionID,
		ToolName:   toolName,
		Input:      args,
		RawInput:   rawInput,
		Output:     output,
		DurationMs: duration.Milliseconds(),
		MCPName:    mcpName,
	}
	if err != nil {
		record.Error = err.Error()
	}
	return record
}

// CreateErrorRecord creates an ErrorRecord from an error.
func CreateErrorRecord(sessionID, errorType string, err error, ctx map[string]any) ErrorRecord {
	record := ErrorRecord{
		Timestamp: time.Now(),
		SessionID: sessionID,
		ErrorType: errorType,
		Message:   err.Error(),
		Context:   ctx,
	}
	if err != nil {
		record.Stack = fmt.Sprintf("%+v", err)
	}
	return record
}

// ReadRecords reads and returns records of a specific type from a directory.
func ReadRecords(recordType string, dir string) ([]map[string]any, error) {
	pattern := filepath.Join(dir, recordType, "*.jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for _, file := range matches {
		records, err := readJSONLFile(file)
		if err != nil {
			slog.Warn("Failed to read records file", "file", file, "error", err)
			continue
		}
		results = append(results, records...)
	}

	slices.SortFunc(results, func(a, b map[string]any) int {
		ta, _ := a["timestamp"].(string)
		tb, _ := b["timestamp"].(string)
		return strings.Compare(ta, tb)
	})

	return results, nil
}

func readJSONLFile(path string) ([]map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		results = append(results, record)
	}
	return results, nil
}
