package engine

import (
	"fmt"
	"strings"

	commandspkg "github.com/channyeintun/nami/internal/commands"
	"github.com/channyeintun/nami/internal/config"
)

type startupProviderSelection struct {
	Provider string
	Model    string
	Snapshot commandspkg.ProviderSnapshot
	Notice   string
}

func resolveStartupProviderSelection(cfg config.Config) startupProviderSelection {
	effectiveCfg := cfg
	originalSelection := strings.TrimSpace(cfg.Model)
	startupNotice := ""
	if shouldPreferRecentModel(cfg.ModelSource) {
		if recent, err := config.LoadRecentModelSelection(); err == nil {
			recentModel := strings.TrimSpace(recent.Model)
			if recentModel != "" && !strings.EqualFold(recentModel, originalSelection) {
				recentCfg := cfg
				recentCfg.Model = recentModel
				recentSnapshot := commandspkg.DiscoverProviderSnapshot(recentCfg)
				recentProvider, _ := commandspkg.ResolveModelSelection(recentModel)
				if status, ok := recentSnapshot.LookupProvider(recentProvider); ok && status.Usable {
					effectiveCfg.Model = recentModel
					startupNotice = fmt.Sprintf("Using recent successful model %s instead of %s.", recentModel, originalSelection)
				}
			}
		}
	}

	snapshot := commandspkg.DiscoverProviderSnapshot(effectiveCfg)
	provider, model := commandspkg.ResolveModelSelection(effectiveCfg.Model)

	if provider != "" {
		if status, ok := snapshot.LookupProvider(provider); ok && status.Usable {
			if strings.TrimSpace(model) == "" {
				model = status.DefaultModel
			}
			return startupProviderSelection{
				Provider: provider,
				Model:    model,
				Snapshot: snapshot,
				Notice:   startupNotice,
			}
		}
	}

	if fallback, ok := firstStartupFallbackProvider(snapshot); ok {
		selection := startupProviderSelection{
			Provider: fallback.ID,
			Model:    fallback.DefaultModel,
			Snapshot: snapshot,
			Notice:   startupNotice,
		}
		fallbackRef := modelRef(fallback.ID, fallback.DefaultModel)
		preferredSelection := strings.TrimSpace(effectiveCfg.Model)
		if preferredSelection != "" && !strings.EqualFold(preferredSelection, fallbackRef) {
			selection.Notice = appendStartupNotice(selection.Notice, fmt.Sprintf("Preferred model %s is unavailable; using %s instead. Run /providers for setup details.", preferredSelection, fallbackRef))
		}
		return selection
	}

	selection := startupProviderSelection{
		Provider: provider,
		Model:    model,
		Snapshot: snapshot,
		Notice:   startupNotice,
	}
	if current, ok := snapshot.LookupProvider(provider); ok && !current.Usable {
		selection.Notice = appendStartupNotice(selection.Notice, formatNoUsableProviderNotice(current))
	} else {
		selection.Notice = appendStartupNotice(selection.Notice, "No usable providers detected. Run /providers for setup guidance.")
	}
	return selection
}

func appendStartupNotice(existing string, next string) string {
	existing = strings.TrimSpace(existing)
	next = strings.TrimSpace(next)
	switch {
	case existing == "":
		return next
	case next == "":
		return existing
	default:
		return existing + " " + next
	}
}

func shouldPreferRecentModel(source string) bool {
	switch strings.TrimSpace(source) {
	case "", "default", "config":
		return true
	default:
		return false
	}
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
