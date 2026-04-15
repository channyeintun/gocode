# Progress

## Current

- Phase 3 provider-context behavior is confirmed; Phase 5 subagent model selection is next.

## Completed

- Phase 1 display cleanup completed.
	- Added a shared TUI helper for stripping provider prefixes.
	- Switched StatusBar, ModelSelectionPrompt, ResumeSelectionPrompt, StreamingAssistantMessage, and AssistantTextMessage to the shared helper.
- Phase 2 provider inference updates completed.
	- Added provider hints to curated model presets.
	- Carried provider hints through the model-selection IPC flow.
	- Kept GitHub Copilot as the routing authority when the active provider is Copilot.

## Next

- Add a /subagent model picker that reuses the model selection flow.