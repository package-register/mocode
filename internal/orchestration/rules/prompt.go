package rules

import (
	"fmt"
	"strings"
)

// ToPromptXML generates XML for injection into the system prompt.
func ToPromptXML(rules []*Rule) string {
	if len(rules) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<available_rules>\n")
	for _, r := range rules {
		sb.WriteString("  <rule>\n")
		fmt.Fprintf(&sb, "    <name>%s</name>\n", escapeXML(r.Name))
		fmt.Fprintf(&sb, "    <description>%s</description>\n", escapeXML(r.Description))
		if r.FilePattern != "" {
			fmt.Fprintf(&sb, "    <file_pattern>%s</file_pattern>\n", escapeXML(r.FilePattern))
		}
		if r.Global {
			sb.WriteString("    <global>true</global>\n")
		}
		fmt.Fprintf(&sb, "    <location>%s</location>\n", escapeXML(r.FilePath))
		sb.WriteString("  </rule>\n")
	}
	sb.WriteString("</available_rules>")
	return sb.String()
}

// ToPromptContent returns the content of all matching rules concatenated
// for injection into the system prompt.
func ToPromptContent(rules []*Rule) string {
	if len(rules) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Active Rules\n\n")
	for _, r := range rules {
		if r.Content == "" {
			continue
		}
		fmt.Fprintf(&sb, "### %s\n", r.Name)
		if r.Description != "" {
			fmt.Fprintf(&sb, "%s\n\n", r.Description)
		}
		sb.WriteString(r.Content)
		sb.WriteString("\n\n")
	}
	return sb.String()
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
