package workflows

import (
	"fmt"
	"strings"
)

// ToPromptXML generates the workflow context for system prompt injection.
// It includes the workflow structure and current progress.
func ToPromptXML(tracker *Tracker) string {
	if tracker == nil || !tracker.IsActive() {
		return ""
	}

	wf := tracker.Active()
	if wf == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<active_workflow>\n")
	fmt.Fprintf(&sb, "  <name>%s</name>\n", escapeXML(wf.Name))
	if wf.Description != "" {
		fmt.Fprintf(&sb, "  <description>%s</description>\n", escapeXML(wf.Description))
	}

	sb.WriteString("  <steps>\n")
	for i, step := range wf.Steps {
		status := "pending"
		if tracker.completed[step.ID] {
			status = "completed"
		} else if i == tracker.currentStepIndex {
			status = "active"
		}
		sb.WriteString("    <step>\n")
		fmt.Fprintf(&sb, "      <id>%s</id>\n", escapeXML(step.ID))
		fmt.Fprintf(&sb, "      <title>%s</title>\n", escapeXML(step.Title))
		fmt.Fprintf(&sb, "      <status>%s</status>\n", status)
		if step.Description != "" {
			fmt.Fprintf(&sb, "      <description>%s</description>\n", escapeXML(step.Description))
		}
		sb.WriteString("    </step>\n")
	}
	sb.WriteString("  </steps>\n")
	sb.WriteString("</active_workflow>")

	return sb.String()
}

// ToPromptMarkdown generates a human-readable markdown summary.
func ToPromptMarkdown(tracker *Tracker) string {
	if tracker == nil || !tracker.IsActive() {
		return ""
	}

	progress := tracker.Progress()
	if progress == "" {
		return ""
	}
	return fmt.Sprintf("<workflow_context>\n%s\n</workflow_context>", progress)
}

func escapeXML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}
