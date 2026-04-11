package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const subagentTypeExplore = "explore"
const subagentTypeGeneralPurpose = "general-purpose"

type AgentRunRequest struct {
	Description  string
	Prompt       string
	SubagentType string
}

type AgentRunResult struct {
	Status         string   `json:"status"`
	SubagentType   string   `json:"subagent_type"`
	SessionID      string   `json:"session_id"`
	TranscriptPath string   `json:"transcript_path"`
	Summary        string   `json:"summary"`
	Tools          []string `json:"tools,omitempty"`
}

type AgentRunner func(context.Context, AgentRunRequest) (AgentRunResult, error)

type AgentTool struct {
	runner AgentRunner
}

func NewAgentTool(runner AgentRunner) *AgentTool {
	return &AgentTool{runner: runner}
}

func (t *AgentTool) Name() string {
	return "agent"
}

func (t *AgentTool) Description() string {
	return "Spawn a bounded child agent in a fresh context and return its final report."
}

func (t *AgentTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"description": map[string]any{
				"type":        "string",
				"description": "Short 3-5 word summary of the delegated task.",
			},
			"prompt": map[string]any{
				"type":        "string",
				"description": "The full task description for the child agent.",
			},
			"subagent_type": map[string]any{
				"type":        "string",
				"description": "The child agent type. Supported values are explore and general-purpose.",
				"enum":        []string{subagentTypeExplore, subagentTypeGeneralPurpose},
			},
		},
		"required": []string{"description", "prompt"},
	}
}

func (t *AgentTool) Permission() PermissionLevel {
	return PermissionExecute
}

func (t *AgentTool) Concurrency(input ToolInput) ConcurrencyDecision {
	return ConcurrencySerial
}

func (t *AgentTool) Validate(input ToolInput) error {
	description, ok := stringParam(input.Params, "description")
	if !ok || strings.TrimSpace(description) == "" {
		return fmt.Errorf("agent requires description")
	}
	prompt, ok := stringParam(input.Params, "prompt")
	if !ok || strings.TrimSpace(prompt) == "" {
		return fmt.Errorf("agent requires prompt")
	}
	if subagentType, ok := stringParam(input.Params, "subagent_type"); ok && strings.TrimSpace(subagentType) != "" && strings.TrimSpace(subagentType) != subagentTypeExplore && strings.TrimSpace(subagentType) != subagentTypeGeneralPurpose {
		return fmt.Errorf("agent subagent_type %q is not supported", subagentType)
	}
	return nil
}

func (t *AgentTool) Execute(ctx context.Context, input ToolInput) (ToolOutput, error) {
	if t == nil || t.runner == nil {
		return ToolOutput{}, fmt.Errorf("agent runner is not configured")
	}

	description, _ := stringParam(input.Params, "description")
	prompt, _ := stringParam(input.Params, "prompt")
	subagentType, _ := stringParam(input.Params, "subagent_type")
	if strings.TrimSpace(subagentType) == "" {
		subagentType = subagentTypeExplore
	}

	result, err := t.runner(ctx, AgentRunRequest{
		Description:  strings.TrimSpace(description),
		Prompt:       strings.TrimSpace(prompt),
		SubagentType: strings.TrimSpace(subagentType),
	})
	if err != nil {
		return ToolOutput{}, err
	}

	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return ToolOutput{}, fmt.Errorf("marshal agent result: %w", err)
	}
	return ToolOutput{Output: string(encoded)}, nil
}
