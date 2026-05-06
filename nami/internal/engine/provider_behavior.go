package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/channyeintun/nami/internal/api"
	commandspkg "github.com/channyeintun/nami/internal/commands"
	"github.com/channyeintun/nami/internal/config"
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

type codexProviderBehavior struct{}

func providerBehaviorFor(provider string) providerBehavior {
	switch normalizeProvider(provider) {
	case "github-copilot":
		return gitHubCopilotProviderBehavior{}
	case "codex":
		return codexProviderBehavior{}
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
	if selection, ok := configuredSubagentModel(cfg, activeModelID, ""); ok {
		return selection
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
	api.SetGitHubCopilotEnterpriseDomain(client, resolved.GitHubCopilot.EnterpriseDomain)
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
	if selection, ok := configuredSubagentModel(cfg, activeModelID, "github-copilot"); ok {
		return selection
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

func (codexProviderBehavior) NewClient(provider, model string, cfg config.Config) (api.LLMClient, error) {
	resolved, err := resolveCodexConfig(cfg)
	if err != nil {
		return nil, err
	}
	client, err := api.NewClientForProvider(provider, model, resolved.APIKey, resolved.BaseURL)
	if err != nil {
		return nil, err
	}
	api.SetCodexAccountID(client, resolved.Codex.AccountID)
	if strings.TrimSpace(resolved.Codex.RefreshToken) != "" {
		api.SetAPIKeyFunc(client, newCodexTokenRefresher(resolved.Codex).resolve)
	}
	return client, nil
}

func (codexProviderBehavior) ResolveSelection(input, fallbackProvider string) (string, string) {
	return standardProviderBehavior{}.ResolveSelection(input, fallbackProvider)
}

func (codexProviderBehavior) DefaultSubagentModel(cfg config.Config, activeModelID string) string {
	return standardProviderBehavior{}.DefaultSubagentModel(cfg, activeModelID)
}

func (codexProviderBehavior) PolicyModels(cfg config.Config) []string {
	return nil
}

func (codexProviderBehavior) RetainsSelectionProvider() bool {
	return false
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

func configuredSubagentModel(cfg config.Config, activeModelID string, defaultProvider string) (string, bool) {
	return normalizeUsableSubagentSelection(cfg, activeModelID, cfg.SubagentModel, defaultProvider)
}

func coerceSessionSubagentModel(cfg config.Config, activeModelID string, selection string) string {
	activeProvider, _ := config.ParseModel(strings.TrimSpace(activeModelID))
	defaultProvider := ""
	if retainSelectionProvider(activeProvider) {
		defaultProvider = normalizeProvider(activeProvider)
	}
	if normalized, ok := normalizeUsableSubagentSelection(cfg, activeModelID, selection, defaultProvider); ok {
		return normalized
	}
	return defaultSessionSubagentModel(cfg, activeModelID)
}

func normalizeUsableSubagentSelection(cfg config.Config, activeModelID string, selection string, defaultProvider string) (string, bool) {
	selection = strings.TrimSpace(selection)
	if selection == "" {
		return "", false
	}

	provider, model := config.ParseModel(selection)
	provider = normalizeProvider(provider)
	if strings.TrimSpace(model) == "" && provider != "" {
		model = provider
		provider = ""
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return "", false
	}

	activeProvider, _ := config.ParseModel(strings.TrimSpace(activeModelID))
	activeProvider = normalizeProvider(activeProvider)

	resolvedProvider := provider
	resolvedModel := model
	if resolvedProvider == "" {
		if preferredProvider := normalizeProvider(defaultProvider); preferredProvider != "" {
			resolvedProvider = preferredProvider
		} else {
			resolvedProvider, resolvedModel = resolveModelSelection(model, activeProvider)
		}
	}

	if resolvedProvider != "" && !isSubagentProviderUsable(cfg, activeModelID, resolvedProvider) {
		return "", false
	}
	return modelRef(resolvedProvider, resolvedModel), true
}

func isSubagentProviderUsable(cfg config.Config, activeModelID string, providerID string) bool {
	providerID = normalizeProvider(providerID)
	if providerID == "" {
		return true
	}

	statusCfg := cfg
	statusCfg.Model = strings.TrimSpace(activeModelID)
	snapshot := commandspkg.DiscoverProviderSnapshot(statusCfg)
	status, ok := snapshot.LookupProvider(providerID)
	if !ok {
		return false
	}
	return status.Usable
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
		if strings.TrimSpace(creds.GitHubToken) == "" && strings.TrimSpace(creds.AccessToken) == "" {
			return cfg, &api.APIError{Type: api.ErrAuth, Message: "GitHub Copilot is not connected. Run /connect first."}
		}
		cfg.GitHubCopilot = creds
		cfg.APIKey = creds.AccessToken
	}

	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = api.GetGitHubCopilotBaseURL(strings.TrimSpace(cfg.GitHubCopilot.AccessToken), cfg.GitHubCopilot.EnterpriseDomain)
	}

	return cfg, nil
}

func resolveCodexConfig(cfg config.Config) (config.Config, error) {
	loaded := config.Load()
	if strings.TrimSpace(loaded.Codex.AccessToken) != "" || strings.TrimSpace(loaded.Codex.RefreshToken) != "" {
		cfg.Codex = loaded.Codex
	}

	if strings.TrimSpace(cfg.APIKey) == "" {
		creds := cfg.Codex
		if strings.TrimSpace(creds.AccessToken) == "" && strings.TrimSpace(creds.RefreshToken) == "" {
			return cfg, &api.APIError{Type: api.ErrAuth, Message: "Codex is not connected. Run /connect codex or set CODEX_ACCESS_TOKEN."}
		}
		cfg.Codex = creds
		cfg.APIKey = creds.AccessToken
	}

	return cfg, nil
}

func resolveGitHubCopilotCapabilities(cfg config.Config, model string) (api.ModelCapabilities, bool) {
	capabilities, ok := api.ResolveGitHubCopilotModelCapabilitiesCached(
		strings.TrimSpace(cfg.GitHubCopilot.AccessToken),
		cfg.GitHubCopilot.EnterpriseDomain,
		model,
	)
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

type codexTokenRefresher struct {
	mu           sync.Mutex
	refreshToken string
	accessToken  string
	accountID    string
	expiresAt    time.Time
}

func newCodexTokenRefresher(creds config.CodexAuth) *codexTokenRefresher {
	return &codexTokenRefresher{
		refreshToken: creds.RefreshToken,
		accessToken:  creds.AccessToken,
		accountID:    creds.AccountID,
		expiresAt:    time.UnixMilli(creds.ExpiresAtUnixMS),
	}
}

func (r *codexTokenRefresher) resolve() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.accessToken != "" && time.Now().Before(r.expiresAt) {
		return r.accessToken, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tokens, err := api.RefreshCodexAccessToken(ctx, r.refreshToken)
	if err != nil {
		return "", fmt.Errorf("refresh Codex token: %w", err)
	}

	r.accessToken = tokens.AccessToken
	if tokens.RefreshToken != "" {
		r.refreshToken = tokens.RefreshToken
	}
	if accountID := api.ExtractCodexAccountID(tokens); accountID != "" {
		r.accountID = accountID
	}
	expiresIn := tokens.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	r.expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)

	loaded := config.Load()
	loaded.Codex.AccessToken = r.accessToken
	loaded.Codex.RefreshToken = r.refreshToken
	loaded.Codex.ExpiresAtUnixMS = r.expiresAt.UnixMilli()
	loaded.Codex.AccountID = r.accountID
	_ = config.Save(loaded)

	return r.accessToken, nil
}

func (r *codexTokenRefresher) currentAccountID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.accountID
}
