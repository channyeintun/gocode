package engine

import (
	"fmt"
	"strings"

	commandspkg "github.com/channyeintun/chan/internal/commands"
	"github.com/channyeintun/chan/internal/config"
)

type startupProviderSelection struct {
	Provider string
	Model    string
	Snapshot commandspkg.ProviderSnapshot
	Notice   string
}

func resolveStartupProviderSelection(cfg config.Config) startupProviderSelection {
	snapshot := commandspkg.DiscoverProviderSnapshot(cfg)
	provider, model := commandspkg.ResolveModelSelection(cfg.Model)
	originalSelection := strings.TrimSpace(cfg.Model)

	if provider != "" {
		if status, ok := snapshot.LookupProvider(provider); ok && status.Usable {
			if strings.TrimSpace(model) == "" {
				model = status.DefaultModel
			}
			return startupProviderSelection{
				Provider: provider,
				Model:    model,
				Snapshot: snapshot,
			}
		}
	}

	if fallback, ok := firstStartupFallbackProvider(snapshot); ok {
		selection := startupProviderSelection{
			Provider: fallback.ID,
			Model:    fallback.DefaultModel,
			Snapshot: snapshot,
		}
		fallbackRef := modelRef(fallback.ID, fallback.DefaultModel)
		if originalSelection != "" && !strings.EqualFold(originalSelection, fallbackRef) {
			selection.Notice = fmt.Sprintf("Preferred model %s is unavailable; using %s instead. Run /providers for setup details.", originalSelection, fallbackRef)
		}
		return selection
	}

	selection := startupProviderSelection{
		Provider: provider,
		Model:    model,
		Snapshot: snapshot,
	}
	if current, ok := snapshot.LookupProvider(provider); ok && !current.Usable {
		selection.Notice = formatNoUsableProviderNotice(current)
	} else {
		selection.Notice = "No usable providers detected. Run /providers for setup guidance."
	}
	return selection
}

func firstStartupFallbackProvider(snapshot commandspkg.ProviderSnapshot) (commandspkg.ProviderStatus, bool) {
	for _, status := range snapshot.Providers {
		if !status.Usable {
			continue
		}
		if status.ID == "ollama" {
			continue
		}
		return status, true
	}
	return commandspkg.ProviderStatus{}, false
}

func formatNoUsableProviderNotice(status commandspkg.ProviderStatus) string {
	if strings.TrimSpace(status.SetupHint) == "" {
		return "No usable providers detected. Run /providers for setup guidance."
	}
	return fmt.Sprintf("No usable providers detected. %s needs setup: %s Run /providers for more details.", status.ID, status.SetupHint)
}
