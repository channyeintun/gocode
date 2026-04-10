package tools

import "strings"

const maxDiffPreviewLines = 12

func buildFileDiffPreview(oldContent, newContent string) (string, int, int) {
	oldLines := splitDiffLines(oldContent)
	newLines := splitDiffLines(newContent)

	prefix := 0
	for prefix < len(oldLines) && prefix < len(newLines) && oldLines[prefix] == newLines[prefix] {
		prefix++
	}

	suffix := 0
	for suffix < len(oldLines)-prefix && suffix < len(newLines)-prefix {
		oldIndex := len(oldLines) - 1 - suffix
		newIndex := len(newLines) - 1 - suffix
		if oldLines[oldIndex] != newLines[newIndex] {
			break
		}
		suffix++
	}

	changedOld := oldLines[prefix : len(oldLines)-suffix]
	changedNew := newLines[prefix : len(newLines)-suffix]
	insertions := len(changedNew)
	deletions := len(changedOld)

	previewLines := make([]string, 0, minInt(maxDiffPreviewLines, insertions+deletions+1))
	if prefix > 0 {
		previewLines = append(previewLines, "@@")
	}
	for _, line := range changedOld {
		if len(previewLines) >= maxDiffPreviewLines {
			break
		}
		previewLines = append(previewLines, "-"+line)
	}
	for _, line := range changedNew {
		if len(previewLines) >= maxDiffPreviewLines {
			break
		}
		previewLines = append(previewLines, "+"+line)
	}
	if insertions+deletions > len(previewLines) {
		previewLines = append(previewLines[:maxDiffPreviewLines-1], "...")
	}

	return strings.Join(previewLines, "\n"), insertions, deletions
}

func splitDiffLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
