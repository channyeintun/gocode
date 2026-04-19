package tools

import "strings"

func normalizeFileForLineEditing(content string) (string, string, bool) {
	originalLineEnding := "\n"
	if strings.Contains(content, "\r\n") {
		originalLineEnding = "\r\n"
	}
	hadTrailingNewline := strings.HasSuffix(content, "\n") || strings.HasSuffix(content, "\r")
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return normalized, originalLineEnding, hadTrailingNewline
}
