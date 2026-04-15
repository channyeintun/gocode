package engine

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/channyeintun/chan/internal/api"
	commandspkg "github.com/channyeintun/chan/internal/commands"
	"github.com/channyeintun/chan/internal/config"
)

type providerBehavior interface {
	NewClient(provider, model string, cfg config.Config) (api.LLMClient, error)
	ResolveSelection(input, fallbackProvider string) (string, string)
	DefaultSubagentModel(cfg config.Config, activeModelID string) string
	PolicyModels(cfg config.Config) []string
	RetainsSelectionProvider() bool
}

type standardProviderBehavior struct{}

type gitHubCopilotProviderBehavior struct{}

func providerBehaviorFor(provider string) providerBehavior {
	if normalizeProvider(provider) == "github-copilot" {
		return gitHubCopilotProviderBehavior{}
	}
	return standardProviderBehavior{}
}

func (standardProviderBehavior) NewClient(provider, model string, cfg config.Config) (api.LLMClient, error) {
	return api.NewClientForProvider(provider, model, cfg.APIKey, cfg.BaseURL)
}

func (standardProviderBehavior) ResolveSelection(input, fallbackProvider string) (string, string) {
	provider, model := config.ParseModel(strings.TrimSpace(input))
	if model == "" && provider != "" {
		model = provider
		provider = ""
	}
	if provider != "" {
		return normalizeProvider(provider), model
	}
	return inferProviderFromModel(model, fallbackProvider), model
}

func (standardProviderBehavior) DefaultSubagentModel(cfg config.Config, activeModelID string) string {
	selection := strings.TrimSpace(cfg.SubagentModel)
	if selection != "" {
		provider, model := config.ParseModel(selection)
		if strings.TrimSpace(model) == "" && strings.TrimSpace(provider) != "" {
			model = provider
			provider = ""
		}
		if strings.TrimSpace(model) == "" {
			return defaultSubagentFallback(cfg, activeModelID)
		}
		if strings.TrimSpace(provider) != "" {
			return modelRef(provider, model)
		}
		return model
	}
	return defaultSubagentFallback(cfg, activeModelID)
}

func (standardProviderBehavior) PolicyModels(cfg config.Config) []string {
	return nil
}

func (standardProviderBehavior) RetainsSelectionProvider() bool {
	return false
}

func (gitHubCopilotProviderBehavior) NewClient(provider, model string, cfg config.Config) (api.LLMClient, error) {
	resolved, err := resolveGitHubCopilotConfig(cfg)
	if err != nil {
		return nil, err
	}

	client, err := newGitHubCopilotClient(provider, model, resolved)
	if err != nil {
		return nil, err
	}
	if capabilities, ok := resolveGitHubCopilotCapabilities(resolved, model); ok {
		client = api.WithCapabilities(client, capabilities)
	}
	api.SetAPIKeyFunc(client, newCopilotTokenRefresher(resolved.GitHubCopilot).resolve)
	return client, nil
}

func (gitHubCopilotProviderBehavior) ResolveSelection(input, fallbackProvider string) (string, string) {
	provider, model := config.ParseModel(strings.TrimSpace(input))
	if model == "" && provider != "" {
		model = provider
		provider = ""
	}
	if provider != "" {
		return normalizeProvider(provider), model
	}
	if normalizeProvider(fallbackProvider) == "github-copilot" {
		return "github-copilot", model
	}
	return standardProviderBehavior{}.ResolveSelection(input, fallbackProvider)
}

func (gitHubCopilotProviderBehavior) DefaultSubagentModel(cfg config.Config, activeModelID string) string {
	selection := strings.TrimSpace(cfg.SubagentModel)
	if selection != "" {
		provider, model := config.ParseModel(selection)
		if strings.TrimSpace(model) == "" && strings.TrimSpace(provider) != "" {
			model = provider
			provider = ""
		}
		if strings.TrimSpace(model) == "" {
			return api.GitHubCopilotDefaultSubagentModel
		}
		if strings.TrimSpace(provider) == "" {
			return modelRef("github-copilot", model)
		}
		return modelRef(provider, model)
	}

	activeProvider, _ := config.ParseModel(strings.TrimSpace(activeModelID))
	if normalizeProvider(activeProvider) == "github-copilot" {
		return modelRef("github-copilot", api.GitHubCopilotDefaultSubagentModel)
	}
	if strings.TrimSpace(activeModelID) != "" {
		return strings.TrimSpace(activeModelID)
	}
	return api.GitHubCopilotDefaultSubagentModel
}

func (gitHubCopilotProviderBehavior) PolicyModels(cfg config.Config) []string {
	models := []string{
		api.GitHubCopilotDefaultMainModel,
		api.GitHubCopilotDefaultSubagentModel,
	}
	if provider, model := config.ParseModel(strings.TrimSpace(cfg.Model)); normalizeProvider(provider) == "github-copilot" && strings.TrimSpace(model) != "" {
		models = append(models, model)
	}
	if provider, model := config.ParseModel(strings.TrimSpace(cfg.SubagentModel)); normalizeProvider(provider) == "github-copilot" && strings.TrimSpace(model) != "" {
		models = append(models, model)
	}
	return commandspkg.MergeGitHubCopilotModelIDs(nil, models)
}

func (gitHubCopilotProviderBehavior) RetainsSelectionProvider() bool {
	return true
}

func defaultSubagentFallback(cfg config.Config, activeModelID string) string {
	if strings.TrimSpace(activeModelID) != "" {
		return strings.TrimSpace(activeModelID)
	}

	provider, model := config.ParseModel(strings.TrimSpace(cfg.Model))
	if strings.TrimSpace(model) != "" {
		if strings.TrimSpace(provider) != "" {
			return modelRef(provider, model)
		}
		return model
	}

	provider = normalizeProvider(provider)
	if preset, ok := api.Presets[provider]; ok {
		return modelRef(provider, preset.DefaultModel)
	}
	return api.GitHubCopilotDefaultSubagentModel
}

func inferProviderFromModel(model, fallbackProvider string) string {
	lower := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(lower, "gemini"):
		return "gemini"
	case strings.Contains(lower, "gpt"), strings.HasPrefix(lower, "o1"), strings.HasPrefix(lower, "o3"), strings.HasPrefix(lower, "o4"):
		return "openai"
	case strings.Contains(lower, "deepseek"):
		return "deepseek"
	case strings.Contains(lower, "qwen"):
		return "qwen"
	case strings.Contains(lower, "glm"):
		return "glm"
	case strings.Contains(lower, "mistral"):
		return "mistral"
	case strings.Contains(lower, "llama"), strings.Contains(lower, "maverick"):
		return "groq"
	case strings.Contains(lower, "gemma"), strings.Contains(lower, "ollama"):
		return "ollama"
	case strings.Contains(lower, "claude"), strings.Contains(lower, "sonnet"), strings.Contains(lower, "opus"), strings.Contains(lower, "haiku"):
		return "anthropic"
	default:
		return normalizeProvider(fallbackProvider)
	}
}

func retainSelectionProvider(provider string) bool {
	return providerBehaviorFor(provider).RetainsSelectionProvider()
}

func newGitHubCopilotClient(provider, model string, cfg config.Config) (api.LLMClient, error) {
	baseURL := cfg.BaseURL
	apiKey := cfg.APIKey

	switch {
	case api.GitHubCopilotUsesAnthropicMessages(model):
		return api.NewAnthropicClientForProvider(provider, model, apiKey, baseURL)
	case api.GitHubCopilotUsesOpenAIResponses(model):
		return api.NewOpenAIResponsesClient(provider, model, apiKey, baseURL)
	default:
		return api.NewClientForProvider(provider, model, apiKey, baseURL)
	}
}

func resolveGitHubCopilotConfig(cfg config.Config) (config.Config, error) {
	loaded := config.Load()
	if strings.TrimSpace(loaded.GitHubCopilot.GitHubToken) != "" {
		cfg.GitHubCopilot = loaded.GitHubCopilot
	}

	if strings.TrimSpace(cfg.APIKey) == "" {
		creds := cfg.GitHubCopilot
		if strings.TrimSpace(creds.GitHubToken) == "" {
			return cfg, &api.APIError{Type: api.ErrAuth, Message: "GitHub Copilot is not connected. Run /connect first."}
		}

		expiresAt := time.UnixMilli(creds.ExpiresAtUnixMS)
		if strings.TrimSpace(creds.AccessToken) == "" || time.Now().After(expiresAt) {
			refreshCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			refreshed, err := api.RefreshGitHubCopilotToken(refreshCtx, creds.GitHubToken, creds.EnterpriseDomain)
			if err != nil {
				return cfg, err
			}

			creds.AccessToken = refreshed.AccessToken
			creds.ExpiresAtUnixMS = refreshed.ExpiresAt.UnixMilli()
			cfg.GitHubCopilot = creds

			loaded.GitHubCopilot = creds
			if err := config.Save(loaded); err != nil {
				return cfg, fmt.Errorf("save refreshed GitHub Copilot credentials: %w", err)
			}
		}

		cfg.APIKey = creds.AccessToken
	}

	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = api.GetGitHubCopilotBaseURL(cfg.GitHubCopilot.AccessToken, cfg.GitHubCopilot.EnterpriseDomain)
	}

	return cfg, nil
}

func resolveGitHubCopilotCapabilities(cfg config.Config, model string) (api.ModelCapabilities, bool) {
	accessToken := strings.TrimSpace(cfg.GitHubCopilot.AccessToken)
	if accessToken == "" {
		return api.ModelCapabilities{}, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	capabilities, ok, err := api.ResolveGitHubCopilotModelCapabilities(ctx, accessToken, cfg.GitHubCopilot.EnterpriseDomain, model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to fetch GitHub Copilot model metadata for %q: %v\n", model, err)
		return api.ModelCapabilities{}, false
	}
	return capabilities, ok
}

type copilotTokenRefresher struct {
	mu               sync.Mutex
	githubToken      string
	enterpriseDomain string
	accessToken      string
	expiresAt        time.Time
}

func newCopilotTokenRefresher(creds config.GitHubCopilotAuth) *copilotTokenRefresher {
	return &copilotTokenRefresher{
		githubToken:      creds.GitHubToken,
		enterpriseDomain: creds.EnterpriseDomain,
		accessToken:      creds.AccessToken,
		expiresAt:        time.UnixMilli(creds.ExpiresAtUnixMS),
	}
}

func (r *copilotTokenRefresher) resolve() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.accessToken != "" && time.Now().Before(r.expiresAt) {
		return r.accessToken, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	refreshed, err := api.RefreshGitHubCopilotToken(ctx, r.githubToken, r.enterpriseDomain)
	if err != nil {
		return "", fmt.Errorf("refresh GitHub Copilot token: %w", err)
	}

	r.accessToken = refreshed.AccessToken
	r.expiresAt = refreshed.ExpiresAt

	loaded := config.Load()
	loaded.GitHubCopilot.AccessToken = refreshed.AccessToken
	loaded.GitHubCopilot.ExpiresAtUnixMS = refreshed.ExpiresAt.UnixMilli()
	_ = config.Save(loaded)

	return r.accessToken, nil
}
