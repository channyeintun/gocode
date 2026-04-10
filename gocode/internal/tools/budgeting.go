package tools

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	defaultBudgetScaleTokens = 8192
	minBudgetChars           = 25_000
	maxBudgetChars           = 250_000
	minPreviewBudgetChars    = 1000
	maxPreviewBudgetChars    = 8000
)

// ResultBudget defines limits for tool output size.
type ResultBudget struct {
	MaxChars   int
	PreviewLen int
	SpillDir   string
}

// DefaultResultBudget returns the standard budget.
func DefaultResultBudget(sessionDir string) ResultBudget {
	return DefaultResultBudgetForModel(sessionDir, defaultBudgetScaleTokens)
}

// DefaultResultBudgetForModel scales tool output budgets to the active model's
// output capacity so smaller models keep tighter inline results.
func DefaultResultBudgetForModel(sessionDir string, maxOutputTokens int) ResultBudget {
	return ResultBudget{
		MaxChars:   scaleBudget(MaxResultSizeChars, maxOutputTokens, minBudgetChars, maxBudgetChars),
		PreviewLen: scaleBudget(PreviewChars, maxOutputTokens, minPreviewBudgetChars, maxPreviewBudgetChars),
		SpillDir:   filepath.Join(sessionDir, "artifacts", "tool-log"),
	}
}

func scaleBudget(base, maxOutputTokens, minValue, maxValue int) int {
	if maxOutputTokens <= 0 {
		maxOutputTokens = defaultBudgetScaleTokens
	}
	scaled := base * maxOutputTokens / defaultBudgetScaleTokens
	if scaled < minValue {
		return minValue
	}
	if scaled > maxValue {
		return maxValue
	}
	return scaled
}

// ApplyBudget truncates output if it exceeds the budget, spilling to disk.
// Returns the (possibly truncated) output and the spill path if any.
func ApplyBudget(budget ResultBudget, toolID string, output string) (string, string, error) {
	if len(output) <= budget.MaxChars {
		return output, "", nil
	}

	if err := os.MkdirAll(budget.SpillDir, 0o755); err != nil {
		return output[:budget.MaxChars], "", fmt.Errorf("create spill dir: %w", err)
	}

	spillPath := filepath.Join(budget.SpillDir, toolID+".log")
	if err := os.WriteFile(spillPath, []byte(output), 0o644); err != nil {
		return output[:budget.MaxChars], "", fmt.Errorf("write spill file: %w", err)
	}

	preview := output[:budget.PreviewLen]
	truncated := fmt.Sprintf(
		"%s\n\n[Output truncated. Full result saved to %s (%d chars)]",
		preview, spillPath, len(output),
	)
	return truncated, spillPath, nil
}
