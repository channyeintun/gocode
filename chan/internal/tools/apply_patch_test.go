package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyPatchExecuteUpdateWithContextLines(t *testing.T) {
	workspace := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir workspace: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWD); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	}()

	filePath := filepath.Join(workspace, "sample.txt")
	if err := os.WriteFile(filePath, []byte("alpha\nbeta\ngamma\n"), 0o644); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	tool := NewApplyPatchTool()
	patch := strings.Join([]string{
		"*** Begin Patch",
		"*** Update File: sample.txt",
		"@@",
		" alpha",
		"-beta",
		"+beta updated",
		" gamma",
		"*** End Patch",
	}, "\n")

	output, err := tool.Execute(context.Background(), ToolInput{Params: map[string]any{"input": patch}})
	if err != nil {
		t.Fatalf("execute apply_patch: %v", err)
	}
	if output.IsError {
		t.Fatalf("expected success, got error output: %s", output.Output)
	}

	updated, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if got, want := string(updated), "alpha\nbeta updated\ngamma\n"; got != want {
		t.Fatalf("updated file mismatch\nwant: %q\n got: %q", want, got)
	}
}
