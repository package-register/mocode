package model

import "strings"

func deleteTrailingMention(value string) (string, bool) {
	trimmed := strings.TrimRight(value, " \t\r\n")
	if trimmed == "" {
		return value, false
	}

	lastSpace := strings.LastIndexAny(trimmed, " \t\r\n")
	start := lastSpace + 1
	mention := trimmed[start:]
	if !strings.HasPrefix(mention, "@") {
		return value, false
	}
	if !isMentionToken(mention) {
		return value, false
	}

	return strings.TrimRight(trimmed[:start], " \t\r\n") + value[len(trimmed):], true
}

func isMentionToken(token string) bool {
	for _, prefix := range []string{"@file:", "@dir:", "@skill:", "@workflow:", "@mcp:"} {
		if strings.HasPrefix(token, prefix) && len(token) > len(prefix) {
			return true
		}
	}
	return false
}
