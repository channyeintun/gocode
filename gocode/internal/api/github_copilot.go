package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	githubCopilotDeviceFlowScope         = "read:user"
	githubCopilotDefaultBaseURL          = "https://api.individual.githubcopilot.com"
	githubCopilotDefaultEnterpriseDomain = "github.com"
	githubCopilotRefreshSkew             = 5 * time.Minute
	GitHubCopilotDefaultMainModel        = "gpt-5.4"
	GitHubCopilotDefaultSubagentModel    = "claude-haiku-4.5"
)

var githubCopilotClientID = mustDecodeBase64("SXYxLmI1MDdhMDhjODdlY2ZlOTg=")

type GitHubCopilotCredentials struct {
	GitHubToken      string
	AccessToken      string
	EnterpriseDomain string
	ExpiresAt        time.Time
}

type GitHubCopilotDeviceCode struct {
	DeviceCode      string
	UserCode        string
	VerificationURI string
	IntervalSeconds int
	ExpiresIn       int
}

type gitHubCopilotTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

type gitHubCopilotDeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	Interval        int    `json:"interval"`
	ExpiresIn       int    `json:"expires_in"`
}

type gitHubCopilotDeviceTokenResponse struct {
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
	Description string `json:"error_description"`
	Interval    int    `json:"interval"`
}

func mustDecodeBase64(value string) string {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		panic(err)
	}
	return string(decoded)
}

func GitHubCopilotStaticHeaders() map[string]string {
	return map[string]string{
		"User-Agent":             "GitHubCopilotChat/0.35.0",
		"Editor-Version":         "vscode/1.107.0",
		"Editor-Plugin-Version":  "copilot-chat/0.35.0",
		"Copilot-Integration-Id": "vscode-chat",
	}
}

func BuildGitHubCopilotDynamicHeaders(messages []Message) map[string]string {
	headers := map[string]string{
		"X-Initiator":   gitHubCopilotInitiator(messages),
		"Openai-Intent": "conversation-edits",
	}

	if gitHubCopilotHasVisionInput(messages) {
		headers["Copilot-Vision-Request"] = "true"
	}

	return headers
}

func GitHubCopilotUsesAnthropicMessages(model string) bool {
	lower := strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(lower, "claude") || strings.Contains(lower, "haiku") || strings.Contains(lower, "sonnet") || strings.Contains(lower, "opus")
}

func GitHubCopilotUsesOpenAIResponses(model string) bool {
	lower := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(lower, "gpt-5") || strings.HasPrefix(lower, "o1") || strings.HasPrefix(lower, "o3") || strings.HasPrefix(lower, "o4")
}

func gitHubCopilotInitiator(messages []Message) string {
	if len(messages) == 0 {
		return "user"
	}
	last := messages[len(messages)-1]
	if last.Role == RoleUser {
		return "user"
	}
	return "agent"
}

func gitHubCopilotHasVisionInput(messages []Message) bool {
	for _, message := range messages {
		if len(message.Images) > 0 {
			return true
		}
	}
	return false
}

func NormalizeGitHubCopilotDomain(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", nil
	}

	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Hostname() == "" {
		return "", fmt.Errorf("invalid GitHub Enterprise URL/domain")
	}

	return parsed.Hostname(), nil
}

func GetGitHubCopilotBaseURL(token, enterpriseDomain string) string {
	if host := gitHubCopilotProxyHost(token); host != "" {
		return "https://" + strings.TrimPrefix(host, "proxy.")
	}
	if strings.TrimSpace(enterpriseDomain) != "" {
		return "https://copilot-api." + strings.TrimSpace(enterpriseDomain)
	}
	return githubCopilotDefaultBaseURL
}

func gitHubCopilotProxyHost(token string) string {
	for _, part := range strings.Split(token, ";") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "proxy-ep=") {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(part, "proxy-ep="))
	}
	return ""
}

func StartGitHubCopilotDeviceFlow(ctx context.Context, enterpriseDomain string) (GitHubCopilotDeviceCode, error) {
	domain := enterpriseDomain
	if strings.TrimSpace(domain) == "" {
		domain = githubCopilotDefaultEnterpriseDomain
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("https://%s/login/device/code", domain),
		strings.NewReader(url.Values{
			"client_id": {githubCopilotClientID},
			"scope":     {githubCopilotDeviceFlowScope},
		}.Encode()),
	)
	if err != nil {
		return GitHubCopilotDeviceCode{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", GitHubCopilotStaticHeaders()["User-Agent"])

	response, err := newHTTPClient().Do(request)
	if err != nil {
		return GitHubCopilotDeviceCode{}, err
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusMultipleChoices {
		return GitHubCopilotDeviceCode{}, classifyOpenAICompatStatus(response.StatusCode, mustReadHTTPBody(response))
	}

	var payload gitHubCopilotDeviceCodeResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return GitHubCopilotDeviceCode{}, fmt.Errorf("decode GitHub Copilot device code response: %w", err)
	}
	if payload.DeviceCode == "" || payload.UserCode == "" || payload.VerificationURI == "" {
		return GitHubCopilotDeviceCode{}, errors.New("invalid GitHub Copilot device code response")
	}
	if payload.Interval <= 0 {
		payload.Interval = 5
	}

	return GitHubCopilotDeviceCode{
		DeviceCode:      payload.DeviceCode,
		UserCode:        payload.UserCode,
		VerificationURI: payload.VerificationURI,
		IntervalSeconds: payload.Interval,
		ExpiresIn:       payload.ExpiresIn,
	}, nil
}

func PollGitHubCopilotGitHubToken(
	ctx context.Context,
	enterpriseDomain string,
	deviceCode string,
	intervalSeconds int,
	expiresIn int,
) (string, error) {
	domain := enterpriseDomain
	if strings.TrimSpace(domain) == "" {
		domain = githubCopilotDefaultEnterpriseDomain
	}
	if intervalSeconds <= 0 {
		intervalSeconds = 5
	}

	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)
	pollInterval := time.Duration(intervalSeconds) * time.Second

	for time.Now().Before(deadline) {
		if err := sleepContext(ctx, pollInterval); err != nil {
			return "", err
		}

		request, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			fmt.Sprintf("https://%s/login/oauth/access_token", domain),
			strings.NewReader(url.Values{
				"client_id":   {githubCopilotClientID},
				"device_code": {deviceCode},
				"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			}.Encode()),
		)
		if err != nil {
			return "", err
		}
		request.Header.Set("Accept", "application/json")
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Header.Set("User-Agent", GitHubCopilotStaticHeaders()["User-Agent"])

		response, err := newHTTPClient().Do(request)
		if err != nil {
			return "", err
		}

		var payload gitHubCopilotDeviceTokenResponse
		decodeErr := json.NewDecoder(response.Body).Decode(&payload)
		response.Body.Close()
		if decodeErr != nil {
			return "", fmt.Errorf("decode GitHub Copilot device token response: %w", decodeErr)
		}

		if payload.AccessToken != "" {
			return payload.AccessToken, nil
		}

		switch payload.Error {
		case "authorization_pending", "":
			continue
		case "slow_down":
			if payload.Interval > 0 {
				pollInterval = time.Duration(payload.Interval) * time.Second
			} else {
				pollInterval += 5 * time.Second
			}
			continue
		default:
			suffix := ""
			if strings.TrimSpace(payload.Description) != "" {
				suffix = ": " + strings.TrimSpace(payload.Description)
			}
			return "", fmt.Errorf("GitHub Copilot device login failed: %s%s", payload.Error, suffix)
		}
	}

	return "", errors.New("GitHub Copilot device login timed out")
}

func RefreshGitHubCopilotToken(
	ctx context.Context,
	githubToken string,
	enterpriseDomain string,
) (GitHubCopilotCredentials, error) {
	if strings.TrimSpace(githubToken) == "" {
		return GitHubCopilotCredentials{}, errors.New("missing GitHub Copilot GitHub token")
	}

	domain := enterpriseDomain
	if strings.TrimSpace(domain) == "" {
		domain = githubCopilotDefaultEnterpriseDomain
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://api.%s/copilot_internal/v2/token", domain), nil)
	if err != nil {
		return GitHubCopilotCredentials{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+githubToken)
	for key, value := range GitHubCopilotStaticHeaders() {
		request.Header.Set(key, value)
	}

	response, err := newHTTPClient().Do(request)
	if err != nil {
		return GitHubCopilotCredentials{}, err
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusMultipleChoices {
		return GitHubCopilotCredentials{}, classifyOpenAICompatStatus(response.StatusCode, mustReadHTTPBody(response))
	}

	var payload gitHubCopilotTokenResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return GitHubCopilotCredentials{}, fmt.Errorf("decode GitHub Copilot token response: %w", err)
	}
	if payload.Token == "" || payload.ExpiresAt <= 0 {
		return GitHubCopilotCredentials{}, errors.New("invalid GitHub Copilot token response")
	}

	expiresAt := time.Unix(payload.ExpiresAt, 0).Add(-githubCopilotRefreshSkew)
	return GitHubCopilotCredentials{
		GitHubToken:      githubToken,
		AccessToken:      payload.Token,
		EnterpriseDomain: strings.TrimSpace(enterpriseDomain),
		ExpiresAt:        expiresAt,
	}, nil
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func mustReadHTTPBody(response *http.Response) []byte {
	if response == nil || response.Body == nil {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	return body
}
