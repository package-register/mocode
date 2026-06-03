package workflows

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseWorkflow_WithFrontmatter(t *testing.T) {
	content := []byte(`---
name: code-review
description: Standard code review workflow
steps:
  - id: setup
    title: Setup Environment
    description: Prepare the development environment
  - id: review
    title: Review Changes
    description: Review all changed files
  - id: approve
    title: Approve
    description: Approve or request changes
---
`)
	wf, err := ParseContent(content)
	require.NoError(t, err)
	require.Equal(t, "code-review", wf.Name)
	require.Equal(t, "Standard code review workflow", wf.Description)
	require.Len(t, wf.Steps, 3)
	require.Equal(t, "setup", wf.Steps[0].ID)
	require.Equal(t, "Review Changes", wf.Steps[1].Title)
}

func TestParseWorkflow_NoFrontmatter(t *testing.T) {
	content := []byte(`- [ ] Step One: First step
- [x] Step Two: Already done`)
	_, err := ParseContent(content)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no YAML frontmatter")
}

func TestParseWorkflow_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-workflow.md")
	err := os.WriteFile(path, []byte(`---
name: test-wf
description: A test workflow
steps:
  - id: step1
    title: Step 1
    description: First step
---
`), 0o644)
	require.NoError(t, err)

	wf, err := Parse(path)
	require.NoError(t, err)
	require.Equal(t, "test-wf", wf.Name)
	require.Equal(t, dir, wf.Path)
}

func TestValidateWorkflow_MissingName(t *testing.T) {
	wf := &Workflow{Steps: []Step{{ID: "a", Title: "A"}}}
	err := wf.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "name is required")
}

func TestValidateWorkflow_NoSteps(t *testing.T) {
	wf := &Workflow{Name: "empty"}
	err := wf.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "at least one step")
}

func TestValidateWorkflow_Valid(t *testing.T) {
	wf := &Workflow{
		Name: "valid",
		Steps: []Step{
			{ID: "a", Title: "Step A"},
		},
	}
	err := wf.Validate()
	require.NoError(t, err)
}

func TestDiscover_Workflows(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "wf1.md"), []byte(`---
name: workflow-one
steps:
  - id: s1
    title: Step 1
---
`), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "wf2.md"), []byte(`---
name: workflow-two
steps:
  - id: s1
    title: Step 1
  - id: s2
    title: Step 2
---
`), 0o644)
	require.NoError(t, err)

	workflows := Discover([]string{dir})
	require.Len(t, workflows, 2)
}

func TestTracker_StartAndProgress(t *testing.T) {
	wf := &Workflow{
		Name: "test",
		Steps: []Step{
			{ID: "plan", Title: "Plan", Description: "Make a plan"},
			{ID: "exec", Title: "Execute", Description: "Execute the plan"},
		},
	}

	tracker := NewTracker()
	require.False(t, tracker.IsActive())

	tracker.StartSession(wf)
	require.True(t, tracker.IsActive())
	require.NotNil(t, tracker.CurrentStep())
	require.Equal(t, "plan", tracker.CurrentStep().ID)

	progress := tracker.Progress()
	require.Contains(t, progress, "Workflow: test")
	require.Contains(t, progress, "[→]")
	require.Contains(t, progress, "[ ]")
}

func TestTracker_CompleteSteps(t *testing.T) {
	wf := &Workflow{
		Name: "test",
		Steps: []Step{
			{ID: "a", Title: "A"},
			{ID: "b", Title: "B"},
		},
	}

	tracker := NewTracker()
	tracker.StartSession(wf)

	err := tracker.CompleteStep("a")
	require.NoError(t, err)
	require.True(t, tracker.completed["a"])

	require.Equal(t, "b", tracker.CurrentStep().ID)
	require.False(t, tracker.IsComplete())

	err = tracker.CompleteStep("b")
	require.NoError(t, err)
	require.True(t, tracker.IsComplete())
}

func TestTracker_CompleteStep_NotFound(t *testing.T) {
	wf := &Workflow{
		Name: "test",
		Steps: []Step{
			{ID: "a", Title: "A"},
		},
	}

	tracker := NewTracker()
	tracker.StartSession(wf)
	err := tracker.CompleteStep("nonexistent")
	require.Error(t, err)
}

func TestTracker_CompleteCurrentStep(t *testing.T) {
	wf := &Workflow{
		Name: "test",
		Steps: []Step{
			{ID: "a", Title: "A"},
		},
	}

	tracker := NewTracker()
	tracker.StartSession(wf)

	err := tracker.CompleteCurrentStep()
	require.NoError(t, err)
	require.True(t, tracker.IsComplete())
	require.Nil(t, tracker.CurrentStep())
}

func TestTracker_NextStep(t *testing.T) {
	wf := &Workflow{
		Name: "test",
		Steps: []Step{
			{ID: "a", Title: "A"},
			{ID: "b", Title: "B"},
		},
	}

	tracker := NewTracker()
	tracker.StartSession(wf)

	next := tracker.NextStep()
	require.NotNil(t, next)
	require.Equal(t, "a", next.ID)

	tracker.CompleteStep("a")
	next = tracker.NextStep()
	require.Equal(t, "b", next.ID)

	tracker.CompleteStep("b")
	next = tracker.NextStep()
	require.Nil(t, next)
}

func TestTracker_EndSession(t *testing.T) {
	wf := &Workflow{
		Name: "test",
		Steps: []Step{
			{ID: "a", Title: "A"},
		},
	}

	tracker := NewTracker()
	tracker.StartSession(wf)
	require.True(t, tracker.IsActive())

	tracker.EndSession()
	require.False(t, tracker.IsActive())
	require.Nil(t, tracker.Active())
}

func TestTracker_ProgressFormat(t *testing.T) {
	wf := &Workflow{
		Name: "review",
		Steps: []Step{
			{ID: "check", Title: "Check", Description: "Initial check"},
			{ID: "fix", Title: "Fix", Description: "Fix issues"},
			{ID: "verify", Title: "Verify", Description: "Verify fixes"},
		},
	}

	tracker := NewTracker()
	tracker.StartSession(wf)

	// Complete first step
	tracker.CompleteStep("check")

	progress := tracker.Progress()
	require.Contains(t, progress, "review")
	require.Contains(t, progress, "[x] **Check**")
	require.Contains(t, progress, "[→] **Fix**")
	require.Contains(t, progress, "[ ] **Verify**")
	require.Contains(t, progress, "1/3 steps completed")
}

func TestToPromptXML_NoTracker(t *testing.T) {
	require.Empty(t, ToPromptXML(nil))
}

func TestToPromptXML_NoActiveWorkflow(t *testing.T) {
	tracker := NewTracker()
	require.Empty(t, ToPromptXML(tracker))
}

func TestToPromptXML_WithActiveWorkflow(t *testing.T) {
	wf := &Workflow{
		Name: "test-wf",
		Steps: []Step{
			{ID: "step1", Title: "Step 1"},
		},
	}
	tracker := NewTracker()
	tracker.StartSession(wf)

	xml := ToPromptXML(tracker)
	require.Contains(t, xml, "test-wf")
	require.Contains(t, xml, "step1")
	require.Contains(t, xml, "active")
}

func TestToPromptMarkdown(t *testing.T) {
	wf := &Workflow{
		Name: "test",
		Steps: []Step{
			{ID: "a", Title: "A"},
		},
	}
	tracker := NewTracker()
	tracker.StartSession(wf)

	md := ToPromptMarkdown(tracker)
	require.Contains(t, md, "Workflow: test")
	require.Contains(t, md, "<workflow_context>")
	require.Contains(t, md, "</workflow_context>")
}
