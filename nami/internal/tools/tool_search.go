package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/channyeintun/nami/internal/api"
)

type ToolSearchRuntime interface {
	Definitions() []api.ToolDefinition
}

type toolSearchRuntimeState struct {
	mu      sync.RWMutex
	runtime ToolSearchRuntime
}

type ToolSearchTool struct{}

type toolSearchResult struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Score       int    `json:"score"`
}

type toolSearchResponse struct {
	Query       string             `json:"query,omitempty"`
	ResultCount int                `json:"result_count"`
	Results     []toolSearchResult `json:"results"`
}

var globalToolSearchRuntime toolSearchRuntimeState

func SetToolSearchRuntime(runtime ToolSearchRuntime) {
	globalToolSearchRuntime.mu.Lock()
	defer globalToolSearchRuntime.mu.Unlock()
	globalToolSearchRuntime.runtime = runtime
}

func getToolSearchRuntime() (ToolSearchRuntime, error) {
	globalToolSearchRuntime.mu.RLock()
	defer globalToolSearchRuntime.mu.RUnlock()
	if globalToolSearchRuntime.runtime == nil {
		return nil, fmt.Errorf("tool search is unavailable")
	}
	return globalToolSearchRuntime.runtime, nil
}

func NewToolSearchTool() *ToolSearchTool {
	return &ToolSearchTool{}
}

func (t *ToolSearchTool) Name() string {
	return "tool_search"
}

func (t *ToolSearchTool) Description() string {
	return "Search the currently available tool surface by name and description to find the most relevant tool for a task."
}

func (t *ToolSearchTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query":       map[string]any{"type": "string", "description": "Search text to rank matching tools. If omitted, the tool lists available tools alphabetically."},
			"maxResults":  map[string]any{"type": "integer", "minimum": 1, "description": "Maximum results to return. Defaults to 10."},
			"max_results": map[string]any{"type": "integer", "minimum": 1, "description": "Snake_case alias for maxResults."},
		},
	}
}

func (t *ToolSearchTool) Permission() PermissionLevel                     { return PermissionReadOnly }
func (t *ToolSearchTool) Concurrency(input ToolInput) ConcurrencyDecision { return ConcurrencyParallel }

func (t *ToolSearchTool) Validate(input ToolInput) error {
	if value, ok := firstIntParam(input.Params, "maxResults", "max_results"); ok && value < 1 {
		return fmt.Errorf("tool_search maxResults must be >= 1")
	}
	return nil
}

func (t *ToolSearchTool) Execute(ctx context.Context, input ToolInput) (ToolOutput, error) {
	select {
	case <-ctx.Done():
		return ToolOutput{}, ctx.Err()
	default:
	}
	runtime, err := getToolSearchRuntime()
	if err != nil {
		return ToolOutput{}, err
	}
	query := strings.TrimSpace(firstStringOrEmpty(input.Params, "query"))
	maxResults := 10
	if value, ok := firstIntParam(input.Params, "maxResults", "max_results"); ok {
		maxResults = value
	}
	results := rankToolDefinitions(runtime.Definitions(), query, maxResults)
	encoded, err := json.MarshalIndent(toolSearchResponse{Query: query, ResultCount: len(results), Results: results}, "", "  ")
	if err != nil {
		return ToolOutput{}, fmt.Errorf("marshal tool_search response: %w", err)
	}
	return ToolOutput{Output: string(encoded)}, nil
}

func rankToolDefinitions(defs []api.ToolDefinition, query string, maxResults int) []toolSearchResult {
	results := make([]toolSearchResult, 0, len(defs))
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	for _, def := range defs {
		score := scoreToolDefinition(def, lowerQuery)
		if lowerQuery != "" && score == 0 {
			continue
		}
		results = append(results, toolSearchResult{Name: def.Name, Description: strings.TrimSpace(def.Description), Score: score})
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Name < results[j].Name
		}
		return results[i].Score > results[j].Score
	})
	if maxResults > 0 && len(results) > maxResults {
		results = results[:maxResults]
	}
	return results
}

func scoreToolDefinition(def api.ToolDefinition, lowerQuery string) int {
	if lowerQuery == "" {
		return 1
	}
	name := strings.ToLower(strings.TrimSpace(def.Name))
	description := strings.ToLower(strings.TrimSpace(def.Description))
	score := 0
	if name == lowerQuery {
		score += 100
	}
	if strings.HasPrefix(name, lowerQuery) {
		score += 50
	}
	if strings.Contains(name, lowerQuery) {
		score += 20
	}
	if strings.Contains(description, lowerQuery) {
		score += 10
	}
	for _, token := range strings.Fields(lowerQuery) {
		if token == "" {
			continue
		}
		if strings.Contains(name, token) {
			score += 8
		}
		if strings.Contains(description, token) {
			score += 3
		}
	}
	return score
}
