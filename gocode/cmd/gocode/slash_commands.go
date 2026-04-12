package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/channyeintun/gocode/internal/agent"
	"github.com/channyeintun/gocode/internal/api"
	artifactspkg "github.com/channyeintun/gocode/internal/artifacts"
	"github.com/channyeintun/gocode/internal/compact"
	"github.com/channyeintun/gocode/internal/config"
	costpkg "github.com/channyeintun/gocode/internal/cost"
	"github.com/channyeintun/gocode/internal/ipc"
	"github.com/channyeintun/gocode/internal/session"
	"github.com/channyeintun/gocode/internal/timing"
)

func handleSlashCommand(
	ctx context.Context,
	bridge *ipc.Bridge,
	store *session.Store,
	timingLogger *timing.Logger,
	cfg config.Config,
	artifactManager *artifactspkg.Manager,
	tracker *costpkg.Tracker,
	payload ipc.SlashCommandPayload,
	sessionID string,
	startedAt time.Time,
	mode agent.ExecutionMode,
	activeModelID string,
	cwd string,
	messages []api.Message,
	client *api.LLMClient,
) (bool, string, time.Time, agent.ExecutionMode, string, string, []api.Message, error) {
	command := strings.ToLower(strings.TrimSpace(payload.Command))
	args := strings.TrimSpace(payload.Args)

	switch command {
	case "connect":
		providerName, enterpriseInput, err := parseConnectArgs(args)
		if err != nil {
			if emitErr := emitTextResponse(bridge, err.Error()); emitErr != nil {
				return false, sessionID, startedAt, mode, activeModelID, cwd, messages, emitErr
			}
			return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
		}

		switch providerName {
		case "github-copilot":
			persisted := config.Load()
			domain, err := api.NormalizeGitHubCopilotDomain(enterpriseInput)
			if err != nil {
				if emitErr := emitTextResponse(bridge, err.Error()); emitErr != nil {
					return false, sessionID, startedAt, mode, activeModelID, cwd, messages, emitErr
				}
				return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
			}

			copilotAuth := persisted.GitHubCopilot
			if strings.TrimSpace(domain) != "" {
				copilotAuth.EnterpriseDomain = domain
			}

			appendSlashResponse(bridge, "Connecting GitHub Copilot...\n\n")

			refreshCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()

			if strings.TrimSpace(copilotAuth.GitHubToken) != "" {
				appendSlashResponse(bridge, "Refreshing saved credentials...\n\n")
				refreshed, refreshErr := api.RefreshGitHubCopilotToken(refreshCtx, copilotAuth.GitHubToken, copilotAuth.EnterpriseDomain)
				if refreshErr == nil {
					copilotAuth.AccessToken = refreshed.AccessToken
					copilotAuth.ExpiresAtUnixMS = refreshed.ExpiresAt.UnixMilli()
				} else {
					appendSlashResponse(bridge, "Saved credentials could not be refreshed. Starting device login...\n\n")
					copilotAuth.AccessToken = ""
					copilotAuth.ExpiresAtUnixMS = 0
				}
			}

			if strings.TrimSpace(copilotAuth.AccessToken) == "" {
				device, err := api.StartGitHubCopilotDeviceFlow(refreshCtx, copilotAuth.EnterpriseDomain)
				if err != nil {
					if emitErr := emitTextResponse(bridge, fmt.Sprintf("GitHub Copilot connect failed: %v", err)); emitErr != nil {
						return false, sessionID, startedAt, mode, activeModelID, cwd, messages, emitErr
					}
					return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
				}

				browserMessage := ""
				if err := openBrowserURL(device.VerificationURI); err == nil {
					browserMessage = "Opened the browser automatically.\n"
				}
				appendSlashResponse(bridge, fmt.Sprintf("%sVisit: %s\nEnter code: %s\n\nWaiting for GitHub authorization...\n\n", browserMessage, device.VerificationURI, device.UserCode))

				githubToken, err := api.PollGitHubCopilotGitHubToken(
					refreshCtx,
					copilotAuth.EnterpriseDomain,
					device.DeviceCode,
					device.IntervalSeconds,
					device.ExpiresIn,
				)
				if err != nil {
					if emitErr := emitTextResponse(bridge, fmt.Sprintf("GitHub Copilot connect failed: %v", err)); emitErr != nil {
						return false, sessionID, startedAt, mode, activeModelID, cwd, messages, emitErr
					}
					return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
				}

				refreshed, err := api.RefreshGitHubCopilotToken(refreshCtx, githubToken, copilotAuth.EnterpriseDomain)
				if err != nil {
					if emitErr := emitTextResponse(bridge, fmt.Sprintf("GitHub Copilot token exchange failed: %v", err)); emitErr != nil {
						return false, sessionID, startedAt, mode, activeModelID, cwd, messages, emitErr
					}
					return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
				}

				copilotAuth.GitHubToken = githubToken
				copilotAuth.AccessToken = refreshed.AccessToken
				copilotAuth.ExpiresAtUnixMS = refreshed.ExpiresAt.UnixMilli()
			}

			persisted.GitHubCopilot = copilotAuth
			persisted.Model = modelRef("github-copilot", api.Presets["github-copilot"].DefaultModel)
			persisted.SubagentModel = modelRef("github-copilot", api.GitHubCopilotDefaultSubagentModel)
			if strings.TrimSpace(persisted.ReasoningEffort) == "" {
				persisted.ReasoningEffort = api.ReasoningEffortMedium
			}
			if err := config.Save(persisted); err != nil {
				if emitErr := emitTextResponse(bridge, fmt.Sprintf("save GitHub Copilot credentials: %v", err)); emitErr != nil {
					return false, sessionID, startedAt, mode, activeModelID, cwd, messages, emitErr
				}
				return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
			}

			nextClient, err := newLLMClient("github-copilot", api.Presets["github-copilot"].DefaultModel, persisted)
			if err != nil {
				if emitErr := emitTextResponse(bridge, fmt.Sprintf("initialize GitHub Copilot client: %v", err)); emitErr != nil {
					return false, sessionID, startedAt, mode, activeModelID, cwd, messages, emitErr
				}
				return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
			}

			*client = nextClient
			activeModelID = modelRef("github-copilot", nextClient.ModelID())
			if err := emitToolUseCapabilityNotice(bridge, activeModelID, *client, nil); err != nil {
				return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
			}
			if err := persistSessionState(store, sessionStateParams{
				SessionID: sessionID,
				CreatedAt: startedAt,
				Mode:      mode,
				Model:     activeModelID,
				CWD:       cwd,
				Branch:    agent.LoadTurnContext().GitBranch,
				Tracker:   tracker,
				Messages:  messages,
			}); err != nil {
				return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
			}
			if err := emitModelChanged(bridge, activeModelID, *client); err != nil {
				return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
			}
			if err := emitContextWindowUsage(bridge, *client, messages); err != nil {
				return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
			}
			if err := emitTextResponse(bridge, fmt.Sprintf("GitHub Copilot connected. Set main model to %s, subagent model to github-copilot/%s, and reasoning effort to %s.", activeModelID, api.GitHubCopilotDefaultSubagentModel, persisted.ReasoningEffort)); err != nil {
				return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
			}
			return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
		default:
			if err := emitTextResponse(bridge, fmt.Sprintf("unsupported connect provider: %s", providerName)); err != nil {
				return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
			}
			return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
		}
	case "plan", "plan-mode":
		mode = agent.ModePlan
		if err := persistSessionState(store, sessionStateParams{
			SessionID: sessionID,
			CreatedAt: startedAt,
			Mode:      mode,
			Model:     activeModelID,
			CWD:       cwd,
			Branch:    agent.LoadTurnContext().GitBranch,
			Tracker:   tracker,
			Messages:  messages,
		}); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		if err := bridge.Emit(ipc.EventModeChanged, ipc.ModeChangedPayload{Mode: string(mode)}); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
	case "fast":
		mode = agent.ModeFast
		if err := persistSessionState(store, sessionStateParams{
			SessionID: sessionID,
			CreatedAt: startedAt,
			Mode:      mode,
			Model:     activeModelID,
			CWD:       cwd,
			Branch:    agent.LoadTurnContext().GitBranch,
			Tracker:   tracker,
			Messages:  messages,
		}); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		if err := bridge.Emit(ipc.EventModeChanged, ipc.ModeChangedPayload{Mode: string(mode)}); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
	case "model":
		if args == "" {
			if err := emitTextResponse(bridge, fmt.Sprintf("Current model: %s", activeModelID)); err != nil {
				return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
			}
			return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
		}

		selectedModel := args
		if strings.EqualFold(strings.TrimSpace(args), "default") {
			selectedModel = cfg.Model
		}

		currentProvider, _ := config.ParseModel(activeModelID)
		provider, model := resolveModelSelection(selectedModel, currentProvider)
		nextClient, err := newLLMClient(provider, model, cfg)
		if err != nil {
			if emitErr := bridge.EmitError(fmt.Sprintf("switch model %q: %v", args, err), true); emitErr != nil {
				return false, sessionID, startedAt, mode, activeModelID, cwd, messages, emitErr
			}
			return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
		}

		*client = nextClient
		activeModelID = modelRef(provider, nextClient.ModelID())
		if err := emitToolUseCapabilityNotice(bridge, activeModelID, *client, nil); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		if err := persistSessionState(store, sessionStateParams{
			SessionID: sessionID,
			CreatedAt: startedAt,
			Mode:      mode,
			Model:     activeModelID,
			CWD:       cwd,
			Branch:    agent.LoadTurnContext().GitBranch,
			Tracker:   tracker,
			Messages:  messages,
		}); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		if err := emitModelChanged(bridge, activeModelID, *client); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		if err := emitContextWindowUsage(bridge, *client, messages); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		if err := emitTextResponse(bridge, fmt.Sprintf("Set model to %s", activeModelID)); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
	case "reasoning":
		persisted := config.Load()
		currentModelID := activeModelID
		if client != nil && *client != nil {
			currentModelID = strings.TrimSpace((*client).ModelID())
		}
		current := describeReasoningEffort(strings.TrimSpace(persisted.ReasoningEffort), currentModelID)
		if strings.TrimSpace(args) == "" {
			if err := emitTextResponse(bridge, fmt.Sprintf("Current reasoning effort: %s", current)); err != nil {
				return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
			}
			return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
		}

		nextEffort, clearSetting, err := parseReasoningArgs(args)
		if err != nil {
			if emitErr := emitTextResponse(bridge, err.Error()); emitErr != nil {
				return false, sessionID, startedAt, mode, activeModelID, cwd, messages, emitErr
			}
			return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
		}

		if clearSetting {
			persisted.ReasoningEffort = ""
		} else {
			persisted.ReasoningEffort = nextEffort
		}
		if err := config.Save(persisted); err != nil {
			if emitErr := emitTextResponse(bridge, fmt.Sprintf("save reasoning effort: %v", err)); emitErr != nil {
				return false, sessionID, startedAt, mode, activeModelID, cwd, messages, emitErr
			}
			return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
		}

		updated := describeReasoningEffort(strings.TrimSpace(persisted.ReasoningEffort), currentModelID)
		if err := emitTextResponse(bridge, fmt.Sprintf("Set reasoning effort to %s", updated)); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
	case "cost", "usage":
		if err := emitTextResponse(bridge, formatCostSummary(tracker.Snapshot(), activeModelID)); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
	case "compact":
		if len(messages) == 0 {
			if emitErr := bridge.EmitError("no messages to compact", true); emitErr != nil {
				return false, sessionID, startedAt, mode, activeModelID, cwd, messages, emitErr
			}
			return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
		}

		resolvedClient, nextModelID, err := ensureClientForSelection(activeModelID, cfg, *client)
		if err != nil {
			if emitErr := bridge.EmitError(fmt.Sprintf("initialize model %q: %v", activeModelID, err), true); emitErr != nil {
				return false, sessionID, startedAt, mode, activeModelID, cwd, messages, emitErr
			}
			return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
		}
		*client = resolvedClient
		activeModelID = nextModelID

		tokensBefore := compact.EstimateConversationTokens(messages)
		if err := bridge.Emit(ipc.EventCompactStart, ipc.CompactStartPayload{
			Strategy:     string(agent.CompactManual),
			TokensBefore: tokensBefore,
		}); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}

		result, err := compactWithMetrics(ctx, bridge, tracker, *client, timingLogger, sessionID, 0, string(agent.CompactManual), messages)
		if err != nil {
			if emitErr := bridge.EmitError(fmt.Sprintf("compact conversation: %v", err), true); emitErr != nil {
				return false, sessionID, startedAt, mode, activeModelID, cwd, messages, emitErr
			}
			return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
		}

		messages = result.Messages
		tokensAfter := compact.EstimateConversationTokens(messages)
		if err := persistSessionState(store, sessionStateParams{
			SessionID: sessionID,
			CreatedAt: startedAt,
			Mode:      mode,
			Model:     activeModelID,
			CWD:       cwd,
			Branch:    agent.LoadTurnContext().GitBranch,
			Tracker:   tracker,
			Messages:  messages,
		}); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		if err := bridge.Emit(ipc.EventCompactEnd, ipc.CompactEndPayload{TokensAfter: tokensAfter}); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		if err := emitContextWindowUsage(bridge, *client, messages); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		if err := emitTextResponse(bridge, fmt.Sprintf("Compacted conversation with %s. Tokens %d -> %d.", result.Strategy, tokensBefore, tokensAfter)); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
	case "resume":
		var targetID string
		if args != "" {
			targetID = args
		} else {
			meta, err := store.LatestResumeCandidate(sessionID)
			if err != nil {
				if emitErr := bridge.EmitError(err.Error(), true); emitErr != nil {
					return false, sessionID, startedAt, mode, activeModelID, cwd, messages, emitErr
				}
				return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
			}
			targetID = meta.SessionID
		}

		restored, err := store.Restore(targetID)
		if err != nil {
			if emitErr := bridge.EmitError(fmt.Sprintf("restore session %q: %v", targetID, err), true); emitErr != nil {
				return false, sessionID, startedAt, mode, activeModelID, cwd, messages, emitErr
			}
			return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
		}

		messages = append(messages[:0], restored.Messages...)
		sessionID = restored.Metadata.SessionID
		if !restored.Metadata.CreatedAt.IsZero() {
			startedAt = restored.Metadata.CreatedAt
		}
		mode = parseExecutionMode(restored.Metadata.Mode)

		if restored.Metadata.Model != "" {
			provider, model := config.ParseModel(restored.Metadata.Model)
			provider = normalizeProvider(provider)
			restoredClient, err := newLLMClient(provider, model, cfg)
			if err != nil {
				*client = nil
				activeModelID = modelRef(provider, model)
				if emitErr := bridge.EmitError(fmt.Sprintf("restore model %q: %v", restored.Metadata.Model, err), true); emitErr != nil {
					return false, sessionID, startedAt, mode, activeModelID, cwd, messages, emitErr
				}
				return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
			}
			*client = restoredClient
			activeModelID = modelRef(provider, restoredClient.ModelID())
			if err := emitToolUseCapabilityNotice(bridge, activeModelID, *client, nil); err != nil {
				return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
			}
		}

		if restored.Metadata.CWD != "" {
			if err := os.Chdir(restored.Metadata.CWD); err == nil {
				cwd = restored.Metadata.CWD
			}
		}

		if err := persistSessionState(store, sessionStateParams{
			SessionID: sessionID,
			CreatedAt: startedAt,
			Mode:      mode,
			Model:     activeModelID,
			CWD:       cwd,
			Branch:    agent.LoadTurnContext().GitBranch,
			Tracker:   tracker,
			Messages:  messages,
		}); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}

		if err := bridge.Emit(ipc.EventSessionRestored, ipc.SessionRestoredPayload{
			SessionID: sessionID,
			Mode:      string(mode),
		}); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		if err := emitSessionUpdated(bridge, sessionID, restored.Metadata.Title); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		if err := emitModelChanged(bridge, activeModelID, *client); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		if err := emitContextWindowUsage(bridge, *client, messages); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		if err := bridge.Emit(ipc.EventModeChanged, ipc.ModeChangedPayload{Mode: string(mode)}); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		if err := emitSessionArtifacts(ctx, bridge, artifactManager, sessionID); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		if err := emitTextResponse(bridge, fmt.Sprintf("Resumed session %s with %d messages.", sessionID, len(messages))); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
	case "clear":
		messages = messages[:0]
		newID, err := newSessionID()
		if err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		sessionID = newID
		startedAt = time.Now()
		if err := persistSessionState(store, sessionStateParams{
			SessionID: sessionID,
			CreatedAt: startedAt,
			Mode:      mode,
			Model:     activeModelID,
			CWD:       cwd,
			Branch:    agent.LoadTurnContext().GitBranch,
			Tracker:   tracker,
			Messages:  messages,
		}); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		if err := emitSessionUpdated(bridge, sessionID, ""); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		if err := emitContextWindowUsage(bridge, *client, messages); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		if err := emitTextResponse(bridge, "Conversation cleared. New session started."); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
	case "help":
		helpText := formatHelpText()
		if err := emitTextResponse(bridge, helpText); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
	case "status":
		statusText := formatStatusText(sessionID, startedAt, mode, activeModelID, cwd, len(messages), tracker)
		if err := emitTextResponse(bridge, statusText); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
	case "sessions":
		sessions, err := store.ListSessions()
		if err != nil {
			if emitErr := bridge.EmitError(fmt.Sprintf("list sessions: %v", err), true); emitErr != nil {
				return false, sessionID, startedAt, mode, activeModelID, cwd, messages, emitErr
			}
			return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
		}
		if err := emitTextResponse(bridge, formatSessionList(sessions, sessionID)); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
	case "diff":
		diffOutput := gitDiff(args)
		if strings.TrimSpace(diffOutput) == "" {
			diffOutput = "No changes detected."
		}
		if err := emitTextResponse(bridge, diffOutput); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
	default:
		if err := bridge.EmitError(fmt.Sprintf("unknown slash command: %s", payload.Command), true); err != nil {
			return false, sessionID, startedAt, mode, activeModelID, cwd, messages, err
		}
		return true, sessionID, startedAt, mode, activeModelID, cwd, messages, nil
	}
}

func emitTextResponse(bridge *ipc.Bridge, text string) error {
	if strings.TrimSpace(text) != "" {
		if err := bridge.Emit(ipc.EventTokenDelta, ipc.TokenDeltaPayload{Text: text}); err != nil {
			return err
		}
	}
	return bridge.Emit(ipc.EventTurnComplete, ipc.TurnCompletePayload{StopReason: "end_turn"})
}

func appendSlashResponse(bridge *ipc.Bridge, text string) {
	if bridge == nil || strings.TrimSpace(text) == "" {
		return
	}
	_ = bridge.Emit(ipc.EventTokenDelta, ipc.TokenDeltaPayload{Text: text})
}

func parseConnectArgs(args string) (string, string, error) {
	parts := strings.Fields(args)
	switch len(parts) {
	case 0:
		return "github-copilot", "", nil
	case 1:
		if strings.EqualFold(parts[0], "github-copilot") {
			return "github-copilot", "", nil
		}
		return "", "", fmt.Errorf("usage: /connect [github-copilot [enterprise-domain]]")
	case 2:
		if !strings.EqualFold(parts[0], "github-copilot") {
			return "", "", fmt.Errorf("usage: /connect [github-copilot [enterprise-domain]]")
		}
		return "github-copilot", parts[1], nil
	default:
		return "", "", fmt.Errorf("usage: /connect [github-copilot [enterprise-domain]]")
	}
}

func parseReasoningArgs(args string) (string, bool, error) {
	selection := strings.ToLower(strings.TrimSpace(args))
	if selection == "default" {
		return "", true, nil
	}
	normalized, ok := api.NormalizeReasoningEffort(selection)
	if !ok {
		return "", false, fmt.Errorf("usage: /reasoning [low|medium|high|xhigh|default]")
	}
	return normalized, false, nil
}

func describeReasoningEffort(configured string, modelID string) string {
	effective := api.ClampReasoningEffort(modelID, configured)
	if effective == "" {
		effective = api.DefaultReasoningEffort(modelID)
		if effective == "" {
			if configured == "" {
				return "default"
			}
			return configured + " (saved for supported models)"
		}
		if configured == "" {
			return effective + " (default)"
		}
	}
	if configured != "" && configured != effective {
		return fmt.Sprintf("%s (clamped from %s for %s)", effective, configured, modelID)
	}
	return effective
}

func openBrowserURL(target string) error {
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("empty browser URL")
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}

func emitSessionArtifacts(ctx context.Context, bridge *ipc.Bridge, artifactManager *artifactspkg.Manager, sessionID string) error {
	if artifactManager == nil || strings.TrimSpace(sessionID) == "" {
		return nil
	}

	artifacts, err := artifactManager.LoadSessionArtifacts(ctx, sessionID)
	if err != nil {
		if warning, ok := err.(*artifactspkg.ArtifactLoadWarning); ok {
			if emitErr := bridge.Emit(ipc.EventError, ipc.ErrorPayload{Message: warning.Error(), Recoverable: true}); emitErr != nil {
				return emitErr
			}
		} else {
			return err
		}
	}

	for index := len(artifacts) - 1; index >= 0; index-- {
		artifact := artifacts[index]
		if err := emitArtifactCreated(bridge, artifact.Artifact); err != nil {
			return err
		}
		if err := emitArtifactUpdated(bridge, artifact.Artifact, artifact.Content); err != nil {
			return err
		}
	}

	for _, artifact := range artifacts {
		if artifact.Artifact.Kind == artifactspkg.KindImplementationPlan && strings.TrimSpace(artifact.Content) != "" {
			return emitArtifactFocused(bridge, artifact.Artifact)
		}
	}

	return nil
}

func formatCostSummary(snapshot costpkg.TrackerSnapshot, activeModelID string) string {
	return fmt.Sprintf(
		"Model: %s\nTotal cost: $%.4f\nInput tokens: %d\nOutput tokens: %d\nAPI duration: %s\nTool duration: %s",
		activeModelID,
		snapshot.TotalCostUSD,
		snapshot.TotalInputTokens,
		snapshot.TotalOutputTokens,
		snapshot.TotalAPIDuration.Round(time.Millisecond),
		snapshot.TotalToolDuration.Round(time.Millisecond),
	)
}

func formatHelpText() string {
	return `Available slash commands:

	/connect       Connect GitHub Copilot and switch to a Copilot model
  /plan          Switch to plan mode (read-only until approved)
  /fast          Switch to fast mode (direct execution)
  /model [name]  Show or switch the active model
	/reasoning     Show or set GPT-5 reasoning effort: low, medium, high, xhigh, or default
  /cost          Show token usage and cost breakdown
  /usage         Alias for /cost
  /compact       Compact the conversation to save context
  /resume [id]   Resume a previous session
  /clear         Clear conversation and start a new session
  /status        Show current session status
  /sessions      List recent sessions
  /diff [args]   Show git diff (optionally with args like --staged)
  /help          Show this help message`
}

func formatStatusText(sessionID string, startedAt time.Time, mode agent.ExecutionMode, model string, cwd string, msgCount int, tracker *costpkg.Tracker) string {
	elapsed := time.Since(startedAt).Round(time.Second)
	snap := tracker.Snapshot()
	reasoning := describeReasoningEffort(strings.TrimSpace(config.Load().ReasoningEffort), model)
	return fmt.Sprintf(
		"Session: %s\nStarted: %s (%s ago)\nMode: %s\nModel: %s\nReasoning: %s\nCWD: %s\nMessages: %d\nCost: $%.4f\nTokens: %d in / %d out",
		sessionID,
		startedAt.Format(time.RFC3339),
		elapsed,
		string(mode),
		model,
		reasoning,
		cwd,
		msgCount,
		snap.TotalCostUSD,
		snap.TotalInputTokens,
		snap.TotalOutputTokens,
	)
}

func formatSessionList(sessions []session.Metadata, currentID string) string {
	if len(sessions) == 0 {
		return "No sessions found."
	}
	var b strings.Builder
	b.WriteString("Recent sessions:\n\n")
	shown := 0
	for _, meta := range sessions {
		if shown >= 20 {
			break
		}
		marker := "  "
		if meta.SessionID == currentID {
			marker = "* "
		}
		title := meta.Title
		if title == "" {
			title = "(untitled)"
		}
		b.WriteString(fmt.Sprintf("%s%s  %s  %s  %s  $%.4f\n",
			marker,
			meta.SessionID[:8],
			meta.UpdatedAt.Format("2006-01-02 15:04"),
			meta.Model,
			title,
			meta.TotalCostUSD,
		))
		shown++
	}
	return strings.TrimSpace(b.String())
}

func gitDiff(args string) string {
	parts := []string{"diff", "--stat"}
	if strings.TrimSpace(args) != "" {
		parts = strings.Fields("diff " + args)
	}
	cmd := exec.Command("git", parts...)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("git diff error: %v", err)
	}
	result := strings.TrimSpace(string(out))
	if len(result) > 5000 {
		result = result[:5000] + "\n[truncated]"
	}
	return result
}
