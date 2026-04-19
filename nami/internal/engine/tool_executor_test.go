package engine

import (
	"testing"

	toolpkg "github.com/channyeintun/nami/internal/tools"
)

func TestApplyPatchPermissionTargetsUsesInputAlias(t *testing.T) {
	patch := "*** Begin Patch\n*** Update File: sample.txt\n@@\n-old\n+new\n*** End Patch"
	call := toolpkg.PendingCall{
		Tool: toolpkg.NewApplyPatchTool(),
		Input: toolpkg.ToolInput{
			Params: map[string]any{"input": patch},
		},
	}

	targets, summary := applyPatchPermissionTargets(call)
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0] != "sample.txt" {
		t.Fatalf("unexpected target: %q", targets[0])
	}
	if summary != "sample.txt" {
		t.Fatalf("unexpected summary: %q", summary)
	}
}
