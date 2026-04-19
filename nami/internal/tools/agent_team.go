package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type AgentTeamTask struct {
	Description  string `json:"description"`
	Prompt       string `json:"prompt"`
	SubagentType string `json:"subagent_type,omitempty"`
}

type AgentTeamLaunchRequest struct {
	Description string
	Tasks       []AgentTeamTask
}

type AgentTeamLaunchResult struct {
	Status      string           `json:"status"`
	TeamID      string           `json:"team_id"`
	Description string           `json:"description,omitempty"`
	Agents      []AgentRunResult `json:"agents"`
}

type AgentTeamStatusRequest struct {
	TeamID string
	WaitMs int
}

type AgentTeamStatusResult struct {
	Status      string           `json:"status"`
	TeamID      string           `json:"team_id"`
	Description string           `json:"description,omitempty"`
	Agents      []AgentRunResult `json:"agents"`
}

type AgentTeamLauncher func(context.Context, AgentTeamLaunchRequest) (AgentTeamLaunchResult, error)
type AgentTeamStatusLookup func(context.Context, AgentTeamStatusRequest) (AgentTeamStatusResult, error)

type AgentTeamTool struct {
	launcher AgentTeamLauncher
}

type AgentTeamStatusTool struct {
	lookup AgentTeamStatusLookup
}

func NewAgentTeamTool(launcher AgentTeamLauncher) *AgentTeamTool {
	return &AgentTeamTool{launcher: launcher}
}

func NewAgentTeamStatusTool(lookup AgentTeamStatusLookup) *AgentTeamStatusTool {
	return &AgentTeamStatusTool{lookup: lookup}
}

func (t *AgentTeamTool) Name() string { return "agent_team" }

func (t *AgentTeamTool) Description() string {
	return "Launch a coordinated team of background child agents for parallel delegated tasks and return a shared team identifier."
}

func (t *AgentTeamTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"description": map[string]any{"type": "string", "description": "Short summary for the overall team objective."},
			"tasks": map[string]any{
				"type":     "array",
				"minItems": 1,
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"description":   map[string]any{"type": "string"},
						"prompt":        map[string]any{"type": "string"},
						"subagent_type": map[string]any{"type": "string", "enum": []string{subagentTypeExplore, subagentTypeGeneralPurpose, subagentTypeVerification}},
					},
					"required": []string{"description", "prompt"},
				},
			},
		},
		"required": []string{"description", "tasks"},
	}
}

func (t *AgentTeamTool) Permission() PermissionLevel                     { return PermissionExecute }
func (t *AgentTeamTool) Concurrency(input ToolInput) ConcurrencyDecision { return ConcurrencySerial }

func (t *AgentTeamTool) Validate(input ToolInput) error {
	if t == nil || t.launcher == nil {
		return fmt.Errorf("agent team launcher is not configured")
	}
	description, ok := stringParam(input.Params, "description")
	if !ok || strings.TrimSpace(description) == "" {
		return fmt.Errorf("agent_team requires description")
	}
	rawTasks, ok := input.Params["tasks"].([]any)
	if !ok || len(rawTasks) == 0 {
		return fmt.Errorf("agent_team requires at least one task")
	}
	for index, rawTask := range rawTasks {
		task, ok := rawTask.(map[string]any)
		if !ok {
			return fmt.Errorf("agent_team task %d is invalid", index+1)
		}
		if value, ok := stringParam(task, "description"); !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("agent_team task %d requires description", index+1)
		}
		if value, ok := stringParam(task, "prompt"); !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("agent_team task %d requires prompt", index+1)
		}
		if subagentType, ok := stringParam(task, "subagent_type"); ok && strings.TrimSpace(subagentType) != "" && !IsSupportedSubagentType(subagentType) {
			return fmt.Errorf("agent_team task %d subagent_type %q is not supported", index+1, subagentType)
		}
	}
	return nil
}

func (t *AgentTeamTool) Execute(ctx context.Context, input ToolInput) (ToolOutput, error) {
	description, _ := stringParam(input.Params, "description")
	rawTasks, _ := input.Params["tasks"].([]any)
	tasks := make([]AgentTeamTask, 0, len(rawTasks))
	for _, rawTask := range rawTasks {
		task := rawTask.(map[string]any)
		tasks = append(tasks, AgentTeamTask{
			Description:  strings.TrimSpace(firstStringOrEmpty(task, "description")),
			Prompt:       strings.TrimSpace(firstStringOrEmpty(task, "prompt")),
			SubagentType: NormalizeSubagentType(firstStringOrEmpty(task, "subagent_type")),
		})
	}
	result, err := t.launcher(ctx, AgentTeamLaunchRequest{Description: strings.TrimSpace(description), Tasks: tasks})
	if err != nil {
		return ToolOutput{}, err
	}
	encoded, err := json.MarshalIndent(displaySafeTeamLaunchResult(result), "", "  ")
	if err != nil {
		return ToolOutput{}, fmt.Errorf("marshal agent team result: %w", err)
	}
	return ToolOutput{Output: string(encoded)}, nil
}

func (t *AgentTeamStatusTool) Name() string { return "agent_team_status" }

func (t *AgentTeamStatusTool) Description() string {
	return "Return aggregated status for a launched child-agent team, including the latest result for each member agent."
}

func (t *AgentTeamStatusTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"team_id": map[string]any{"type": "string", "description": "Team identifier returned by agent_team."},
			"wait_ms": map[string]any{"type": "integer", "minimum": 0, "description": "Optional time to wait for member updates before returning."},
		},
		"required": []string{"team_id"},
	}
}

func (t *AgentTeamStatusTool) Permission() PermissionLevel { return PermissionReadOnly }
func (t *AgentTeamStatusTool) Concurrency(input ToolInput) ConcurrencyDecision {
	return ConcurrencySerial
}

func (t *AgentTeamStatusTool) Validate(input ToolInput) error {
	if t == nil || t.lookup == nil {
		return fmt.Errorf("agent team status lookup is not configured")
	}
	teamID, ok := stringParam(input.Params, "team_id")
	if !ok || strings.TrimSpace(teamID) == "" {
		return fmt.Errorf("agent_team_status requires team_id")
	}
	if value, ok := intParam(input.Params, "wait_ms"); ok && value < 0 {
		return fmt.Errorf("wait_ms must be >= 0")
	}
	return nil
}

func (t *AgentTeamStatusTool) Execute(ctx context.Context, input ToolInput) (ToolOutput, error) {
	teamID, _ := stringParam(input.Params, "team_id")
	result, err := t.lookup(ctx, AgentTeamStatusRequest{TeamID: strings.TrimSpace(teamID), WaitMs: intOrDefault(input.Params, "wait_ms", 0)})
	if err != nil {
		return ToolOutput{}, err
	}
	encoded, err := json.MarshalIndent(displaySafeTeamStatusResult(result), "", "  ")
	if err != nil {
		return ToolOutput{}, fmt.Errorf("marshal agent team status: %w", err)
	}
	return ToolOutput{Output: string(encoded)}, nil
}

func displaySafeTeamLaunchResult(result AgentTeamLaunchResult) AgentTeamLaunchResult {
	display := result
	display.Agents = make([]AgentRunResult, 0, len(result.Agents))
	for _, agentResult := range result.Agents {
		display.Agents = append(display.Agents, DisplaySafeAgentResult(agentResult))
	}
	return display
}

func displaySafeTeamStatusResult(result AgentTeamStatusResult) AgentTeamStatusResult {
	display := result
	display.Agents = make([]AgentRunResult, 0, len(result.Agents))
	for _, agentResult := range result.Agents {
		display.Agents = append(display.Agents, DisplaySafeAgentResult(agentResult))
	}
	return display
}
