package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"charm.land/fantasy"

	"github.com/package-register/mocode/internal/agent/prompt"
	"github.com/package-register/mocode/internal/agent/tools"
	"github.com/package-register/mocode/internal/config"
	"github.com/package-register/mocode/internal/knowledge/memory"
	"golang.org/x/sync/errgroup"
)

//go:embed templates/agent_tool.md
var agentToolDescription []byte

type AgentParams struct {
	Prompt string            `json:"prompt" description:"The task for the agent to perform"`
	Tasks  []AgentTaskParams `json:"tasks,omitempty" description:"Optional batch of tasks to run in parallel"`
}

type AgentTaskParams struct {
	// ID is an optional identifier for the task, used for dependency tracking
	// and structured result attribution.
	ID string `json:"id,omitempty" description:"Optional unique ID for DAG ordering and result attribution"`
	// Prompt is the task description for the sub-agent.
	Prompt string `json:"prompt" description:"The task for the agent to perform"`
	// AgentName is the target agent/mode ID. Defaults to "task".
	AgentName string `json:"agent_name,omitempty" description:"Optional target agent/mode ID. Defaults to task"`
	// AllowEdit enables write tools for this sub-agent.
	AllowEdit bool `json:"allow_edit,omitempty" description:"Allow this sub-agent to use edit/write tools when the selected agent supports them"`
	// DependsOn lists IDs of tasks that must complete before this one starts.
	DependsOn []string `json:"depends_on,omitempty" description:"Optional list of task IDs this task depends on"`
}

// TaskResult is the structured output envelope returned by sub-agent execution.
// It provides machine-parseable status/summary/artifacts alongside the raw text.
type TaskResult struct {
	Status      string   `json:"status"`                 // "success" or "error"
	Summary     string   `json:"summary,omitempty"`      // one-line summary of the sub-agent's work
	Artifacts   []string `json:"artifacts,omitempty"`    // files created or modified
	Risks       []string `json:"risks,omitempty"`        // risks or concerns found
	Evidence    []string `json:"evidence,omitempty"`     // evidence refs (e.g. "file:line")
	NextActions []string `json:"next_actions,omitempty"` // suggested follow-up actions
	Error       string   `json:"error,omitempty"`        // error message if status == "error"
}

const (
	AgentToolName = "agent"
)

func (c *coordinator) agentTool(ctx context.Context) (fantasy.AgentTool, error) {
	defaultAgent, err := c.buildSubAgent(ctx, config.AgentTask, false)
	if err != nil {
		return nil, err
	}
	return fantasy.NewParallelAgentTool(
		AgentToolName,
		tools.FirstLineDescription(agentToolDescription),
		func(ctx context.Context, params AgentParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Prompt == "" && len(params.Tasks) == 0 {
				return fantasy.NewTextErrorResponse("prompt or tasks is required"), nil
			}

			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, errors.New("session id missing from context")
			}

			agentMessageID := tools.GetMessageFromContext(ctx)
			if agentMessageID == "" {
				return fantasy.ToolResponse{}, errors.New("agent message id missing from context")
			}

			if len(params.Tasks) > 0 {
				return c.runSubAgentBatch(ctx, subAgentBatchParams{
					SessionID:      sessionID,
					AgentMessageID: agentMessageID,
					ToolCallID:     call.ID,
					Tasks:          params.Tasks,
				})
			}

			return c.runSubAgent(ctx, subAgentParams{
				Agent:          defaultAgent,
				SessionID:      sessionID,
				AgentMessageID: agentMessageID,
				ToolCallID:     call.ID,
				Prompt:         params.Prompt,
				SessionTitle:   "New Agent Session",
			})
		}), nil
}

func (c *coordinator) buildSubAgent(ctx context.Context, agentID string, allowEdit bool) (SessionAgent, error) {
	if agentID == "" {
		agentID = config.AgentTask
	}
	agentCfg, ok := c.cfg.Config().Agents[agentID]
	if !ok {
		return nil, fmt.Errorf("agent %q not configured", agentID)
	}
	if !allowEdit {
		agentCfg.AllowedTools = readOnlyAgentTools(agentCfg.AllowedTools)
	} else {
		agentCfg.AllowedTools = withEditAgentTools(agentCfg.AllowedTools)
	}
	agentPrompt, err := promptForAgent(agentCfg, prompt.WithWorkingDir(c.cfg.WorkingDir()))
	if err != nil {
		return nil, err
	}
	return c.buildAgent(ctx, agentPrompt, agentCfg, true)
}

type subAgentBatchParams struct {
	SessionID      string
	AgentMessageID string
	ToolCallID     string
	Tasks          []AgentTaskParams
}

func (c *coordinator) runSubAgentBatch(ctx context.Context, params subAgentBatchParams) (fantasy.ToolResponse, error) {
	// If any task has dependencies, use DAG scheduler.
	hasDeps := false
	for _, t := range params.Tasks {
		if len(t.DependsOn) > 0 {
			hasDeps = true
			break
		}
	}
	if hasDeps {
		return c.runSubAgentDAG(ctx, params)
	}
	return c.runSubAgentParallel(ctx, params)
}

// runSubAgentParallel runs all tasks concurrently (no dependencies).
func (c *coordinator) runSubAgentParallel(ctx context.Context, params subAgentBatchParams) (fantasy.ToolResponse, error) {
	type batchEntry struct {
		idx    int
		task   AgentTaskParams
		result string
		err    string
	}
	entries := make([]batchEntry, len(params.Tasks))

	group, ctx := errgroup.WithContext(ctx)
	for i, task := range params.Tasks {
		entries[i] = batchEntry{idx: i, task: task}
		e := &entries[i]
		group.Go(func() error {
			if strings.TrimSpace(e.task.Prompt) == "" {
				e.err = "prompt is required"
				return nil
			}
			agentID := strings.TrimSpace(e.task.AgentName)
			if agentID == "" {
				agentID = config.AgentTask
			}
			agent, err := c.buildSubAgent(ctx, agentID, e.task.AllowEdit)
			if err != nil {
				e.err = err.Error()
				return nil
			}
			resp, err := c.runSubAgent(ctx, subAgentParams{
				Agent:          agent,
				SessionID:      params.SessionID,
				AgentMessageID: params.AgentMessageID,
				ToolCallID:     fmt.Sprintf("%s-%d", params.ToolCallID, e.idx+1),
				Prompt:         e.task.Prompt,
				SessionTitle:   fmt.Sprintf("Agent %s: %s", agentID, e.task.Prompt),
			})
			if err != nil {
				e.err = err.Error()
				return nil
			}
			e.result = strings.TrimSpace(resp.Content)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return fantasy.ToolResponse{}, err
	}

	// Build structured results array.
	taskResults := make([]TaskResult, len(entries))
	for _, e := range entries {
		tr := TaskResult{}
		if e.err != "" {
			tr.Status = "error"
			tr.Error = e.err
		} else if e.result == "" {
			tr.Status = "error"
			tr.Error = "empty result"
		} else {
			tr.Status = "success"
			tr.Summary = firstLine(e.result)
		}
		taskResults[e.idx] = tr
	}

	// Text output: legacy + structured JSON appendix.
	var out strings.Builder
	out.WriteString("Batch agent results:\n")
	for _, e := range entries {
		label := e.task.ID
		if label == "" {
			label = fmt.Sprintf("Agent %d", e.idx+1)
		}
		agentName := e.task.AgentName
		if agentName == "" {
			agentName = config.AgentTask
		}
		out.WriteString(fmt.Sprintf("\n## %s [%s]\n\n", label, agentName))
		if e.err != "" {
			out.WriteString(fmt.Sprintf("**error**: %s\n", e.err))
		} else if e.result != "" {
			out.WriteString(e.result)
			out.WriteString("\n")
		} else {
			out.WriteString("(empty result)\n")
		}
	}

	// Append structured envelope.
	envJSON, err := json.Marshal(taskResults)
	if err == nil {
		out.WriteString("\n\n<task_results>\n")
		out.WriteString(string(envJSON))
		out.WriteString("\n</task_results>")
	}

	return fantasy.NewTextResponse(out.String()), nil
}

// firstLine returns the first non-empty line of s, truncated to 200 chars.
func firstLine(s string) string {
	lines := strings.SplitN(strings.TrimSpace(s), "\n", 2)
	l := strings.TrimSpace(lines[0])
	if len(l) > 200 {
		l = l[:200] + "..."
	}
	return l
}

// runSubAgentDAG executes tasks respecting the DependsOn DAG.
// It runs tasks in phases: each phase executes all ready tasks in parallel,
// then advances to the next phase. Failed/errored tasks block their dependents.
func (c *coordinator) runSubAgentDAG(ctx context.Context, params subAgentBatchParams) (fantasy.ToolResponse, error) {
	n := len(params.Tasks)
	if n == 0 {
		return fantasy.NewTextResponse("No tasks to run."), nil
	}

	// Assign default IDs if not provided.
	for i := range params.Tasks {
		if params.Tasks[i].ID == "" {
			params.Tasks[i].ID = fmt.Sprintf("task-%d", i+1)
		}
	}

	// Build ID index.
	byID := make(map[string]int, n)
	for i, t := range params.Tasks {
		if _, dup := byID[t.ID]; dup {
			return fantasy.NewTextErrorResponse(fmt.Sprintf("Duplicate task ID: %q", t.ID)), nil
		}
		byID[t.ID] = i
	}

	// Validate all DependsOn references exist.
	for _, t := range params.Tasks {
		for _, dep := range t.DependsOn {
			if _, ok := byID[dep]; !ok {
				return fantasy.NewTextErrorResponse(
					fmt.Sprintf("Task %q depends on unknown task %q", t.ID, dep),
				), nil
			}
		}
	}

	// Cycle detection via Kahn's algorithm on a copy.
	inDegree := make([]int, n)
	for i, t := range params.Tasks {
		inDegree[i] = len(t.DependsOn)
	}
	{
		queue := make([]int, 0, n)
		for i, d := range inDegree {
			if d == 0 {
				queue = append(queue, i)
			}
		}
		visited := 0
		for len(queue) > 0 {
			idx := queue[0]
			queue = queue[1:]
			visited++
			for j, t := range params.Tasks {
				for _, dep := range t.DependsOn {
					if dep == params.Tasks[idx].ID {
						inDegree[j]--
						if inDegree[j] == 0 {
							queue = append(queue, j)
						}
						break
					}
				}
			}
		}
		if visited != n {
			return fantasy.NewTextErrorResponse("Cycle detected in task dependencies"), nil
		}
	}

	// Run in phases.
	done := make(map[string]bool, n)
	failed := make(map[string]bool, n)
	results := make([]string, n)

	for len(done)+len(failed) < n {
		// Find ready tasks: all deps done, not yet processed, and no dep failed.
		var ready []int
		for i, t := range params.Tasks {
			if done[t.ID] || failed[t.ID] {
				continue
			}
			blocked := false
			for _, dep := range t.DependsOn {
				if failed[dep] || !done[dep] {
					blocked = true
					break
				}
			}
			if !blocked {
				ready = append(ready, i)
			}
		}

		if len(ready) == 0 {
			// Remaining tasks are all blocked.
			for _, t := range params.Tasks {
				if !done[t.ID] && !failed[t.ID] {
					results[byID[t.ID]] = "blocked: upstream dependency failed"
					failed[t.ID] = true
				}
			}
			break
		}

		// Run ready tasks in parallel.
		group, gctx := errgroup.WithContext(ctx)
		for _, idx := range ready {
			i := idx
			group.Go(func() error {
				task := params.Tasks[i]
				if strings.TrimSpace(task.Prompt) == "" {
					results[i] = "error: prompt is required"
					failed[task.ID] = true
					return nil
				}
				agentID := strings.TrimSpace(task.AgentName)
				if agentID == "" {
					agentID = config.AgentTask
				}
				agent, err := c.buildSubAgent(gctx, agentID, task.AllowEdit)
				if err != nil {
					results[i] = "error: " + err.Error()
					failed[task.ID] = true
					return nil
				}
				resp, err := c.runSubAgent(gctx, subAgentParams{
					Agent:          agent,
					SessionID:      params.SessionID,
					AgentMessageID: params.AgentMessageID,
					ToolCallID:     fmt.Sprintf("%s-%s", params.ToolCallID, task.ID),
					Prompt:         task.Prompt,
					SessionTitle:   fmt.Sprintf("Agent %s: %s", agentID, task.Prompt),
				})
				if err != nil {
					results[i] = "error: " + err.Error()
					failed[task.ID] = true
					return nil
				}
				results[i] = strings.TrimSpace(resp.Content)
				done[task.ID] = true
				return nil
			})
		}
		if err := group.Wait(); err != nil {
			return fantasy.ToolResponse{}, err
		}
	}

	// Build output.
	var out strings.Builder
	out.WriteString("DAG batch results:\n")
	for i, t := range params.Tasks {
		agentName := t.AgentName
		if agentName == "" {
			agentName = config.AgentTask
		}
		status := "✅"
		if failed[t.ID] {
			status = "❌"
		}
		out.WriteString(fmt.Sprintf("\n## %s %s [%s]\n\n", status, t.ID, agentName))
		if results[i] != "" {
			out.WriteString(results[i])
			out.WriteString("\n")
		} else {
			out.WriteString("(empty result)\n")
		}
	}

	// Structured envelope.
	taskResults := make([]TaskResult, n)
	for i := range params.Tasks {
		tr := TaskResult{}
		if results[i] == "" || strings.HasPrefix(results[i], "error:") || strings.HasPrefix(results[i], "blocked:") {
			tr.Status = "error"
			tr.Error = strings.TrimPrefix(results[i], "error: ")
		} else {
			tr.Status = "success"
			tr.Summary = firstLine(results[i])
		}
		taskResults[i] = tr
	}
	envJSON, err := json.Marshal(taskResults)
	if err == nil {
		out.WriteString("\n\n<task_results>\n")
		out.WriteString(string(envJSON))
		out.WriteString("\n</task_results>")
	}

	return fantasy.NewTextResponse(out.String()), nil
}

func readOnlyAgentTools(allowed []string) []string {
	readOnly := []string{"glob", "grep", "ls", "sourcegraph", "view", "crawl", "download_docs", tools.SessionSummaryToolName, tools.SessionExportToolName, memory.SearchToolName, memory.LoadToolName}
	if allowed == nil {
		return readOnly
	}
	filtered := make([]string, 0, len(allowed))
	for _, name := range allowed {
		if slices.Contains(readOnly, name) {
			filtered = append(filtered, name)
		}
	}
	return filtered
}

func withEditAgentTools(allowed []string) []string {
	next := append([]string{}, readOnlyAgentTools(allowed)...)
	for _, name := range []string{"edit", "write", "multiedit"} {
		if !slices.Contains(next, name) {
			next = append(next, name)
		}
	}
	return next
}
