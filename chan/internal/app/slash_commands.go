package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/channyeintun/chan/internal/agent"
	"github.com/channyeintun/chan/internal/api"
	artifactspkg "github.com/channyeintun/chan/internal/artifacts"
	commandspkg "github.com/channyeintun/chan/internal/commands"
	"github.com/channyeintun/chan/internal/config"
	costpkg "github.com/channyeintun/chan/internal/cost"
	"github.com/channyeintun/chan/internal/ipc"
	"github.com/channyeintun/chan/internal/session"
	"github.com/channyeintun/chan/internal/timing"
)

func handleSlashCommand(
	ctx context.Context,
	bridge *ipc.Bridge,
	router *ipc.MessageRouter,
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
	subagentModelID string,
	cwd string,
	messages []api.Message,
	client *api.LLMClient,
) (slashCommandState, error) {
	cmd := newSlashCommandContext(
		ctx,
		bridge,
		router,
		store,
		timingLogger,
		cfg,
		artifactManager,
		tracker,
		payload,
		sessionID,
		startedAt,
		mode,
		activeModelID,
		subagentModelID,
		cwd,
		messages,
		client,
	)

	handler, ok := lookupSlashCommandHandler(cmd.command)
	if !ok {
		if err := bridge.EmitError(fmt.Sprintf("unknown slash command: %s", payload.Command), true); err != nil {
			return cmd.state, err
		}
		if err := bridge.Emit(ipc.EventTurnComplete, ipc.TurnCompletePayload{StopReason: "end_turn"}); err != nil {
			return cmd.state, err
		}
		return cmd.state, nil
	}

	if err := handler.Handle(cmd); err != nil {
		return cmd.state, err
	}
	return cmd.state, nil
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

func gitHubCopilotPolicyModels(cfg config.Config) []string {
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

func defaultSessionSubagentModel(cfg config.Config, activeModelID string) string {
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

		activeProvider, _ := config.ParseModel(strings.TrimSpace(activeModelID))
		if strings.TrimSpace(provider) == "" && normalizeProvider(activeProvider) == "github-copilot" {
			return modelRef("github-copilot", model)
		}
		if strings.TrimSpace(provider) != "" {
			return modelRef(provider, model)
		}
		return model
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
