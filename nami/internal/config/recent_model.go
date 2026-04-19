package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type RecentModelSelection struct {
	Model     string `json:"model,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

func RecentModelPath() string {
	return filepath.Join(ConfigDir(), "recent-model.json")
}

func LoadRecentModelSelection() (RecentModelSelection, error) {
	data, err := os.ReadFile(RecentModelPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return RecentModelSelection{}, nil
		}
		return RecentModelSelection{}, err
	}

	var recent RecentModelSelection
	if err := json.Unmarshal(data, &recent); err != nil {
		return RecentModelSelection{}, err
	}
	recent.Model = strings.TrimSpace(recent.Model)
	return recent, nil
}

func SaveRecentModelSelection(model string) error {
	model = strings.TrimSpace(model)
	if model == "" {
		return nil
	}
	if err := os.MkdirAll(ConfigDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(RecentModelSelection{
		Model:     model,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(RecentModelPath(), data, 0o644)
}
