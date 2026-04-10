package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type MultiReplaceFileContentTool struct{}

type replacementChunk struct {
	StartLine          int
	EndLine            int
	TargetContent      string
	ReplacementContent string
}

func NewMultiReplaceFileContentTool() *MultiReplaceFileContentTool {
	return &MultiReplaceFileContentTool{}
}

func (t *MultiReplaceFileContentTool) Name() string {
	return "multi_replace_file_content"
}

func (t *MultiReplaceFileContentTool) Description() string {
	return "Apply multiple validated non-contiguous replacements to a single file in one write."
}

func (t *MultiReplaceFileContentTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"TargetFile": map[string]any{
				"type":        "string",
				"description": "The absolute path to the file to modify.",
			},
			"ReplacementChunks": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"StartLine":          map[string]any{"type": "integer", "minimum": 1},
						"EndLine":            map[string]any{"type": "integer", "minimum": 1},
						"TargetContent":      map[string]any{"type": "string"},
						"ReplacementContent": map[string]any{"type": "string"},
					},
					"required": []string{"StartLine", "EndLine", "TargetContent", "ReplacementContent"},
				},
			},
		},
		"required": []string{"TargetFile", "ReplacementChunks"},
	}
}

func (t *MultiReplaceFileContentTool) Permission() PermissionLevel {
	return PermissionWrite
}

func (t *MultiReplaceFileContentTool) IsConcurrencySafe(input ToolInput) bool {
	return false
}

func (t *MultiReplaceFileContentTool) Execute(ctx context.Context, input ToolInput) (ToolOutput, error) {
	select {
	case <-ctx.Done():
		return ToolOutput{}, ctx.Err()
	default:
	}

	targetFile, ok := firstStringParam(input.Params, "TargetFile", "target_file")
	if !ok || strings.TrimSpace(targetFile) == "" {
		return ToolOutput{}, fmt.Errorf("multi_replace_file_content requires TargetFile")
	}
	if !filepath.IsAbs(targetFile) {
		cwd, _ := os.Getwd()
		targetFile = filepath.Join(cwd, targetFile)
	}

	chunks, err := parseReplacementChunks(input.Params)
	if err != nil {
		return ToolOutput{}, err
	}
	if len(chunks) == 0 {
		return ToolOutput{}, fmt.Errorf("multi_replace_file_content requires at least one replacement chunk")
	}

	originalBytes, err := os.ReadFile(targetFile)
	if err != nil {
		return ToolOutput{}, fmt.Errorf("read target file %q: %w", targetFile, err)
	}

	originalContent := string(originalBytes)
	normalizedOriginal, originalLineEnding, hadTrailingNewline := normalizeFileForLineEditing(originalContent)
	lines := splitIntoLogicalLines(normalizedOriginal)

	sortedChunks := append([]replacementChunk(nil), chunks...)
	sort.Slice(sortedChunks, func(i, j int) bool {
		if sortedChunks[i].StartLine == sortedChunks[j].StartLine {
			return sortedChunks[i].EndLine > sortedChunks[j].EndLine
		}
		return sortedChunks[i].StartLine > sortedChunks[j].StartLine
	})

	if err := validateReplacementChunks(lines, sortedChunks); err != nil {
		return ToolOutput{}, err
	}

	updatedLines := append([]string(nil), lines...)
	for _, chunk := range sortedChunks {
		select {
		case <-ctx.Done():
			return ToolOutput{}, ctx.Err()
		default:
		}

		startIdx := chunk.StartLine - 1
		endIdx := chunk.EndLine
		replacementLines := strings.Split(chunk.ReplacementContent, "\n")
		prefix := append([]string(nil), updatedLines[:startIdx]...)
		suffix := append([]string(nil), updatedLines[endIdx:]...)

		combined := make([]string, 0, len(prefix)+len(replacementLines)+len(suffix))
		combined = append(combined, prefix...)
		combined = append(combined, replacementLines...)
		combined = append(combined, suffix...)
		updatedLines = combined
	}

	updatedContent := strings.Join(updatedLines, "\n")
	if hadTrailingNewline {
		updatedContent += "\n"
	}
	if originalLineEnding == "\r\n" {
		updatedContent = strings.ReplaceAll(updatedContent, "\n", "\r\n")
	}

	trackFileBeforeWrite(targetFile)
	if err := os.WriteFile(targetFile, []byte(updatedContent), 0o644); err != nil {
		return ToolOutput{}, fmt.Errorf("write file %q: %w", targetFile, err)
	}

	preview, insertions, deletions := buildFileDiffPreview(normalizedOriginal, strings.ReplaceAll(updatedContent, "\r\n", "\n"))
	return ToolOutput{
		Output:     fmt.Sprintf("Edited file successfully: %s (%d replacement chunk%s)", targetFile, len(sortedChunks), pluralSuffix(len(sortedChunks))),
		FilePath:   targetFile,
		Preview:    preview,
		Insertions: insertions,
		Deletions:  deletions,
	}, nil
}

func parseReplacementChunks(params map[string]any) ([]replacementChunk, error) {
	rawChunks, ok := firstParam(params, "ReplacementChunks", "replacement_chunks")
	if !ok {
		return nil, fmt.Errorf("multi_replace_file_content requires ReplacementChunks")
	}

	rawSlice, ok := rawChunks.([]any)
	if !ok {
		return nil, fmt.Errorf("ReplacementChunks must be an array")
	}

	chunks := make([]replacementChunk, 0, len(rawSlice))
	for index, raw := range rawSlice {
		chunkMap, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("ReplacementChunks[%d] must be an object", index)
		}

		startLine, ok := firstIntParam(chunkMap, "StartLine", "start_line")
		if !ok {
			return nil, fmt.Errorf("ReplacementChunks[%d] requires StartLine", index)
		}
		endLine, ok := firstIntParam(chunkMap, "EndLine", "end_line")
		if !ok {
			return nil, fmt.Errorf("ReplacementChunks[%d] requires EndLine", index)
		}
		targetContent, ok := firstStringParam(chunkMap, "TargetContent", "target_content")
		if !ok {
			return nil, fmt.Errorf("ReplacementChunks[%d] requires TargetContent", index)
		}
		replacementContent, ok := firstStringParam(chunkMap, "ReplacementContent", "replacement_content")
		if !ok {
			return nil, fmt.Errorf("ReplacementChunks[%d] requires ReplacementContent", index)
		}

		chunks = append(chunks, replacementChunk{
			StartLine:          startLine,
			EndLine:            endLine,
			TargetContent:      strings.ReplaceAll(targetContent, "\r\n", "\n"),
			ReplacementContent: strings.ReplaceAll(replacementContent, "\r\n", "\n"),
		})
	}

	return chunks, nil
}

func validateReplacementChunks(lines []string, chunks []replacementChunk) error {
	previousStart := len(lines) + 1
	for _, chunk := range chunks {
		if chunk.StartLine < 1 || chunk.EndLine < chunk.StartLine {
			return fmt.Errorf("invalid line range %d-%d", chunk.StartLine, chunk.EndLine)
		}
		if chunk.EndLine > len(lines) {
			return fmt.Errorf("replacement chunk line range %d-%d exceeds file length %d", chunk.StartLine, chunk.EndLine, len(lines))
		}
		if chunk.EndLine >= previousStart {
			return fmt.Errorf("replacement chunks overlap at lines %d-%d", chunk.StartLine, chunk.EndLine)
		}

		currentSnippet := strings.Join(lines[chunk.StartLine-1:chunk.EndLine], "\n")
		if currentSnippet != chunk.TargetContent {
			return fmt.Errorf("target content mismatch for lines %d-%d", chunk.StartLine, chunk.EndLine)
		}
		previousStart = chunk.StartLine
	}
	return nil
}

func normalizeFileForLineEditing(content string) (string, string, bool) {
	lineEnding := "\n"
	if strings.Contains(content, "\r\n") {
		lineEnding = "\r\n"
	}
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	hadTrailingNewline := strings.HasSuffix(normalized, "\n")
	normalized = strings.TrimSuffix(normalized, "\n")
	return normalized, lineEnding, hadTrailingNewline
}

func splitIntoLogicalLines(content string) []string {
	if content == "" {
		return []string{}
	}
	return strings.Split(content, "\n")
}
