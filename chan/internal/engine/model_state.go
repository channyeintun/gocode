package engine

import (
	"sync"

	"github.com/channyeintun/chan/internal/api"
)

type ActiveModelState struct {
	mu      sync.RWMutex
	client  api.LLMClient
	modelID string
}

type ActiveSubagentModelState struct {
	mu      sync.RWMutex
	modelID string
}

func NewActiveModelState(client api.LLMClient, modelID string) *ActiveModelState {
	return &ActiveModelState{client: client, modelID: modelID}
}

func (s *ActiveModelState) Get() (api.LLMClient, string) {
	if s == nil {
		return nil, ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client, s.modelID
}

func (s *ActiveModelState) Set(client api.LLMClient, modelID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.client = client
	s.modelID = modelID
}

func NewActiveSubagentModelState(modelID string) *ActiveSubagentModelState {
	return &ActiveSubagentModelState{modelID: modelID}
}

func (s *ActiveSubagentModelState) Get() string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.modelID
}

func (s *ActiveSubagentModelState) Set(modelID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modelID = modelID
}
