package workflows

import (
	"fmt"
	"strings"
	"sync"
)

// Tracker manages the progress of active workflows during a session.
type Tracker struct {
	mu sync.RWMutex
	// active holds the currently active workflow, if any.
	active *Workflow
	// completed tracks which step IDs have been completed.
	completed map[string]bool
	// currentStepIndex tracks the step we're currently on.
	currentStepIndex int
}

// NewTracker creates a new workflow tracker.
func NewTracker() *Tracker {
	return &Tracker{
		completed: make(map[string]bool),
	}
}

// StartSession begins tracking the given workflow.
func (t *Tracker) StartSession(wf *Workflow) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.active = wf
	t.completed = make(map[string]bool)
	t.currentStepIndex = 0
}

// Active returns the currently active workflow.
func (t *Tracker) Active() *Workflow {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.active
}

// IsActive returns true if a workflow is active.
func (t *Tracker) IsActive() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.active != nil
}

// CompleteStep marks a step as completed and advances to the next step.
func (t *Tracker) CompleteStep(stepID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.active == nil {
		return fmt.Errorf("no active workflow")
	}

	// Find the step by ID to validate it exists
	found := false
	for i, s := range t.active.Steps {
		if s.ID == stepID {
			found = true
			t.currentStepIndex = i + 1
			break
		}
	}
	if !found {
		return fmt.Errorf("step %q not found in workflow %q", stepID, t.active.Name)
	}

	t.completed[stepID] = true
	return nil
}

// CompleteCurrentStep marks the current step as completed.
func (t *Tracker) CompleteCurrentStep() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.active == nil {
		return fmt.Errorf("no active workflow")
	}
	if t.currentStepIndex >= len(t.active.Steps) {
		return fmt.Errorf("all steps already completed")
	}

	step := t.active.Steps[t.currentStepIndex]
	t.completed[step.ID] = true
	t.currentStepIndex++
	return nil
}

// CurrentStep returns the current step, or nil if all steps are done.
func (t *Tracker) CurrentStep() *Step {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.active == nil || t.currentStepIndex >= len(t.active.Steps) {
		return nil
	}
	return &t.active.Steps[t.currentStepIndex]
}

// Progress returns a human-readable progress summary.
func (t *Tracker) Progress() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.active == nil {
		return ""
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## Workflow: %s\n", t.active.Name)
	if t.active.Description != "" {
		fmt.Fprintf(&sb, "%s\n\n", t.active.Description)
	}

	for i, step := range t.active.Steps {
		status := "[ ]"
		if t.completed[step.ID] {
			status = "[x]"
		} else if i == t.currentStepIndex {
			status = "[→]"
		}
		fmt.Fprintf(&sb, "- %s **%s**: %s\n", status, step.Title, step.Description)
	}

	completedCount := len(t.completed)
	totalCount := len(t.active.Steps)
	fmt.Fprintf(&sb, "\n> Progress: %d/%d steps completed\n", completedCount, totalCount)

	return sb.String()
}

// IsComplete returns true when all steps are completed.
func (t *Tracker) IsComplete() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.active != nil && len(t.completed) == len(t.active.Steps)
}

// NextStep returns the next uncompleted step for forced progression.
func (t *Tracker) NextStep() *Step {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.active == nil {
		return nil
	}
	for _, s := range t.active.Steps {
		if !t.completed[s.ID] {
			return &s
		}
	}
	return nil
}

// EndSession stops tracking the current workflow.
func (t *Tracker) EndSession() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.active = nil
	t.completed = make(map[string]bool)
	t.currentStepIndex = 0
}
