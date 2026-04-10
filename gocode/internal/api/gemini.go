package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

// GeminiClient implements the native Gemini streaming GenerateContent API.
type GeminiClient struct {
	model        string
	baseURL      string
	apiKey       string
	httpClient   *http.Client
	capabilities ModelCapabilities
}

// NewGeminiClient constructs a native Gemini streaming client.
func NewGeminiClient(model, apiKey, baseURL string) (*GeminiClient, error) {
	preset := Presets["gemini"]
	if model == "" {
		model = preset.DefaultModel
	}
	if baseURL == "" {
		baseURL = preset.BaseURL
	}
	if apiKey == "" {
		apiKey = os.Getenv(preset.EnvKeyVar)
	}
	if apiKey == "" {
		return nil, &APIError{Type: ErrAuth, Message: "missing Gemini API key"}
	}

	return &GeminiClient{
		model:        model,
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       apiKey,
		httpClient:   &http.Client{},
		capabilities: preset.Capabilities,
	}, nil
}

// ModelID returns the active model identifier.
func (c *GeminiClient) ModelID() string {
	return c.model
}

// Capabilities reports Gemini model capabilities.
func (c *GeminiClient) Capabilities() ModelCapabilities {
	return c.capabilities
}

// Stream opens a Gemini streamGenerateContent request and yields model events.
func (c *GeminiClient) Stream(ctx context.Context, req ModelRequest) (iter.Seq2[ModelEvent, error], error) {
	payload, err := c.buildRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.openStream(ctx, payload)
	if err != nil {
		return nil, err
	}

	return func(yield func(ModelEvent, error) bool) {
		defer resp.Body.Close()

		state := geminiStreamState{}
		err := readSSE(ctx, resp.Body, func(_ string, data string) error {
			return c.handleEvent(data, &state, yield)
		})
		if err != nil && !errors.Is(err, errStopStream) {
			yield(ModelEvent{}, err)
		}
	}, nil
}

func (c *GeminiClient) openStream(ctx context.Context, payload geminiGenerateContentRequest) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal Gemini request: %w", err)
	}

	var (
		resp *http.Response
		mu   sync.Mutex
	)

	err = RetryWithBackoff(ctx, DefaultRetryPolicy(), func() error {
		endpoint := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse", c.baseURL, url.PathEscape(geminiModelName(c.model)))
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create Gemini request: %w", err)
		}
		req.Header.Set("content-type", "application/json")
		req.Header.Set("accept", "text/event-stream")
		req.Header.Set("x-goog-api-key", c.apiKey)

		currentResp, err := c.httpClient.Do(req)
		if err != nil {
			return &APIError{Type: ErrNetwork, Message: "Gemini request failed", Err: err}
		}
		if currentResp.StatusCode >= http.StatusMultipleChoices {
			defer currentResp.Body.Close()
			bodyBytes, _ := io.ReadAll(io.LimitReader(currentResp.Body, 1<<20))
			return classifyGeminiStatus(currentResp.StatusCode, bodyBytes)
		}

		mu.Lock()
		resp = currentResp
		mu.Unlock()
		return nil
	})
	if err != nil {
		return nil, err
	}

	mu.Lock()
	defer mu.Unlock()
	return resp, nil
}

func (c *GeminiClient) buildRequest(req ModelRequest) (geminiGenerateContentRequest, error) {
	contents, systemInstruction, err := buildGeminiContents(req.SystemPrompt, req.Messages)
	if err != nil {
		return geminiGenerateContentRequest{}, err
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = c.capabilities.MaxOutputTokens
	}

	payload := geminiGenerateContentRequest{
		Contents:          contents,
		SystemInstruction: systemInstruction,
		Tools:             buildGeminiTools(req.Tools),
		GenerationConfig: geminiGenerationConfig{
			MaxOutputTokens: maxTokens,
			Temperature:     req.Temperature,
			StopSequences:   req.StopSequences,
		},
	}

	return payload, nil
}

func (c *GeminiClient) handleEvent(
	data string,
	state *geminiStreamState,
	yield func(ModelEvent, error) bool,
) error {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return nil
	}

	var response geminiGenerateContentResponse
	if err := json.Unmarshal([]byte(trimmed), &response); err != nil {
		return fmt.Errorf("decode Gemini stream chunk: %w", err)
	}
	if response.Error != nil {
		return &APIError{
			Type:    classifyGeminiErrorType(0, response.Error.Status, response.Error.Message),
			Message: response.Error.Message,
		}
	}
	if response.UsageMetadata != nil {
		state.usage.merge(response.UsageMetadata)
		if !yield(ModelEvent{Type: ModelEventUsage, Usage: state.usage.toUsage()}, nil) {
			return errStopStream
		}
	}

	for _, candidate := range response.Candidates {
		for _, part := range candidate.Content.Parts {
			switch {
			case part.FunctionCall != nil:
				input, err := json.Marshal(part.FunctionCall.Args)
				if err != nil {
					return fmt.Errorf("encode Gemini function call args: %w", err)
				}
				if !yield(ModelEvent{
					Type: ModelEventToolCall,
					ToolCall: &ToolCall{
						ID:    firstNonEmpty(part.FunctionCall.ID, part.FunctionCall.Name),
						Name:  part.FunctionCall.Name,
						Input: string(input),
					},
				}, nil) {
					return errStopStream
				}
			case part.Text != "" && part.Thought:
				if !yield(ModelEvent{Type: ModelEventThinking, Text: part.Text}, nil) {
					return errStopStream
				}
			case part.Text != "":
				if !yield(ModelEvent{Type: ModelEventToken, Text: part.Text}, nil) {
					return errStopStream
				}
			}
		}

		if candidate.FinishReason != "" {
			state.stopReason = mapGeminiStopReason(candidate.FinishReason)
		}
	}

	if response.PromptFeedback != nil && response.PromptFeedback.BlockReason != "" {
		state.stopReason = mapGeminiStopReason(response.PromptFeedback.BlockReason)
	}

	if state.stopReason != "" && !state.sentStop {
		state.sentStop = true
		if !yield(ModelEvent{Type: ModelEventStop, StopReason: state.stopReason}, nil) {
			return errStopStream
		}
	}

	return nil
}

func buildGeminiContents(systemPrompt string, messages []Message) ([]geminiContent, *geminiContent, error) {
	systemParts := make([]geminiPart, 0, len(messages)+1)
	if trimmed := strings.TrimSpace(systemPrompt); trimmed != "" {
		systemParts = append(systemParts, geminiPart{Text: trimmed})
	}

	contents := make([]geminiContent, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == RoleSystem {
			if trimmed := strings.TrimSpace(msg.Content); trimmed != "" {
				systemParts = append(systemParts, geminiPart{Text: trimmed})
			}
			continue
		}

		converted, err := convertGeminiMessage(msg)
		if err != nil {
			return nil, nil, err
		}
		contents = append(contents, converted...)
	}

	var instruction *geminiContent
	if len(systemParts) > 0 {
		instruction = &geminiContent{Role: "system", Parts: systemParts}
	}
	return contents, instruction, nil
}

func convertGeminiMessage(msg Message) ([]geminiContent, error) {
	trimmed := strings.TrimSpace(msg.Content)
	parts := make([]geminiPart, 0, 1+len(msg.ToolCalls)+len(msg.Images))

	switch msg.Role {
	case RoleUser:
		if trimmed != "" {
			parts = append(parts, geminiPart{Text: trimmed})
		}
		for _, image := range msg.Images {
			parts = append(parts, geminiPart{
				InlineData: &geminiInlineData{
					MimeType: image.MediaType,
					Data:     image.Data,
				},
			})
		}
		if msg.ToolResult != nil {
			resultPart, err := geminiFunctionResponsePart(*msg.ToolResult)
			if err != nil {
				return nil, err
			}
			parts = append(parts, resultPart)
		}
		if len(parts) == 0 {
			return nil, nil
		}
		return []geminiContent{{Role: "user", Parts: parts}}, nil
	case RoleTool:
		if msg.ToolResult == nil {
			if trimmed == "" {
				return nil, nil
			}
			return []geminiContent{{Role: "user", Parts: []geminiPart{{Text: trimmed}}}}, nil
		}
		resultPart, err := geminiFunctionResponsePart(*msg.ToolResult)
		if err != nil {
			return nil, err
		}
		return []geminiContent{{Role: "user", Parts: []geminiPart{resultPart}}}, nil
	case RoleAssistant:
		if trimmed != "" {
			parts = append(parts, geminiPart{Text: trimmed})
		}
		for _, toolCall := range msg.ToolCalls {
			args, err := decodeToolInput(toolCall.Input)
			if err != nil {
				return nil, err
			}
			parts = append(parts, geminiPart{
				FunctionCall: &geminiFunctionCall{
					ID:   toolCall.ID,
					Name: toolCall.Name,
					Args: args,
				},
			})
		}
		if len(parts) == 0 {
			return nil, nil
		}
		return []geminiContent{{Role: "model", Parts: parts}}, nil
	default:
		if trimmed == "" {
			return nil, nil
		}
		return []geminiContent{{Role: string(msg.Role), Parts: []geminiPart{{Text: trimmed}}}}, nil
	}
}

func geminiFunctionResponsePart(result ToolResult) (geminiPart, error) {
	response := map[string]any{}
	if strings.TrimSpace(result.Output) != "" {
		var decoded any
		if err := json.Unmarshal([]byte(result.Output), &decoded); err == nil {
			response["output"] = decoded
		} else {
			response["output"] = result.Output
		}
	}
	if result.IsError {
		response["is_error"] = true
	}
	return geminiPart{
		FunctionResponse: &geminiFunctionResponse{
			Name:     result.ToolCallID,
			Response: response,
		},
	}, nil
}

func buildGeminiTools(tools []ToolDefinition) []geminiTool {
	if len(tools) == 0 {
		return nil
	}

	decls := make([]geminiFunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		decls = append(decls, geminiFunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.InputSchema,
		})
	}

	return []geminiTool{{FunctionDeclarations: decls}}
}

func geminiModelName(model string) string {
	return strings.TrimPrefix(model, "models/")
}

func classifyGeminiStatus(statusCode int, body []byte) error {
	var envelope geminiErrorEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Error == nil {
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = http.StatusText(statusCode)
		}
		return &APIError{
			Type:       classifyGeminiErrorType(statusCode, "", message),
			StatusCode: statusCode,
			Message:    message,
		}
	}

	message := envelope.Error.Message
	if message == "" {
		message = http.StatusText(statusCode)
	}
	return &APIError{
		Type:       classifyGeminiErrorType(statusCode, envelope.Error.Status, message),
		StatusCode: statusCode,
		Message:    message,
	}
}

func classifyGeminiErrorType(statusCode int, status, message string) APIErrorType {
	lowerStatus := strings.ToLower(status)
	lowerMessage := strings.ToLower(message)

	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden || strings.Contains(lowerStatus, "permission"):
		return ErrAuth
	case statusCode == http.StatusTooManyRequests || strings.Contains(lowerStatus, "resource_exhausted"):
		return ErrRateLimit
	case statusCode >= http.StatusInternalServerError || strings.Contains(lowerStatus, "unavailable"):
		return ErrOverloaded
	case strings.Contains(lowerMessage, "token") && strings.Contains(lowerMessage, "limit"):
		return ErrPromptTooLong
	case strings.Contains(lowerMessage, "max output tokens"):
		return ErrMaxTokens
	default:
		return ErrUnknown
	}
}

func mapGeminiStopReason(reason string) string {
	upper := strings.ToUpper(reason)
	switch upper {
	case "STOP":
		return "end_turn"
	case "MAX_TOKENS":
		return "max_tokens"
	case "MALFORMED_FUNCTION_CALL", "UNEXPECTED_TOOL_CALL", "TOOL_CALL", "FUNCTION_CALL":
		return "tool_use"
	default:
		return strings.ToLower(reason)
	}
}

type geminiGenerateContentRequest struct {
	Contents          []geminiContent        `json:"contents"`
	SystemInstruction *geminiContent         `json:"systemInstruction,omitempty"`
	Tools             []geminiTool           `json:"tools,omitempty"`
	GenerationConfig  geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiGenerationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	Thought          bool                    `json:"thought,omitempty"`
	InlineData       *geminiInlineData       `json:"inlineData,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type geminiFunctionCall struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
	Args any    `json:"args,omitempty"`
}

type geminiFunctionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response,omitempty"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type geminiFunctionDeclaration struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type geminiGenerateContentResponse struct {
	Candidates     []geminiCandidate     `json:"candidates,omitempty"`
	PromptFeedback *geminiPromptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  *geminiUsageMetadata  `json:"usageMetadata,omitempty"`
	Error          *geminiErrorBody      `json:"error,omitempty"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason,omitempty"`
}

type geminiPromptFeedback struct {
	BlockReason string `json:"blockReason,omitempty"`
}

type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount,omitempty"`
	CandidatesTokenCount int `json:"candidatesTokenCount,omitempty"`
	TotalTokenCount      int `json:"totalTokenCount,omitempty"`
}

func (u *geminiUsageMetadata) merge(other *geminiUsageMetadata) {
	if other == nil {
		return
	}
	if other.PromptTokenCount > 0 {
		u.PromptTokenCount = other.PromptTokenCount
	}
	if other.CandidatesTokenCount > 0 {
		u.CandidatesTokenCount = other.CandidatesTokenCount
	}
	if other.TotalTokenCount > 0 {
		u.TotalTokenCount = other.TotalTokenCount
	}
}

func (u geminiUsageMetadata) toUsage() *Usage {
	return &Usage{
		InputTokens:  u.PromptTokenCount,
		OutputTokens: u.CandidatesTokenCount,
	}
}

type geminiErrorEnvelope struct {
	Error *geminiErrorBody `json:"error,omitempty"`
}

type geminiErrorBody struct {
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

type geminiStreamState struct {
	usage      geminiUsageMetadata
	stopReason string
	sentStop   bool
}
