# Fix Progress

Tracking fixes per plan.md.

---

## Task 1 — Create GitHub Copilot integration plan and tracker

**Files**: `plan.md`, `progress.md`

Created a new implementation plan for GitHub Copilot support in `gocode` before
making any code changes. The plan covers the minimal end-to-end path:

- persist Copilot credentials in config
- add a Go port of the device-code OAuth flow
- register a `github-copilot` provider preset
- inject Copilot-specific HTTP headers into the OpenAI-compatible client
- add a `/connect` slash command that logs in and switches to a Copilot model

Verification and execution constraints were recorded up front: no tests, format
after each completed task, and commit after each completed task.

Implementation completed in the same task:

- added persisted GitHub Copilot auth fields to `gocode/internal/config/config.go`
- added a new `gocode/internal/api/github_copilot.go` helper with:
  - device-code login start
  - device-code polling
  - Copilot token refresh
  - base URL derivation from the Copilot token
  - required Copilot static and dynamic headers
- registered a new `github-copilot` OpenAI-compatible provider preset
- updated the OpenAI-compatible client to send Copilot-specific headers only for
  the Copilot provider
- updated engine client creation to load, refresh, and persist Copilot
  credentials automatically
- added `/connect` in `gocode/cmd/gocode/slash_commands.go`
  - defaults to GitHub Copilot
  - supports optional enterprise domain via `/connect github-copilot <domain>`
  - prints the verification URL and device code into the TUI transcript
  - attempts to open the browser automatically
  - persists credentials and switches the active model to `github-copilot/gpt-4o`
- updated `/help` to show the new `/connect` command

Verification completed:

- ran `gofmt -w` on all changed Go files
- ran `go build ./...`
- ran `make release-local` in `gocode/tui`

## Task 2 — Set GitHub Copilot model defaults to GPT-5.4 and Haiku 4.5

**Files**: `gocode/internal/api/github_copilot.go`, `gocode/internal/config/config.go`, `gocode/internal/api/provider_config.go`, `gocode/cmd/gocode/slash_commands.go`, `gocode/cmd/gocode/subagent_runtime.go`, `progress.md`

Updated the GitHub Copilot integration so:

- the main Copilot model default is now `github-copilot/gpt-5.4`
- `/connect` persists `github-copilot/claude-haiku-4.5` as the subagent model
- Copilot subagents no longer inherit the main model; they resolve a dedicated
  child client using the saved subagent model

This keeps the primary interactive session on GPT-5.4 while routing subagents to
Claude Haiku 4.5 automatically when the active provider is GitHub Copilot.

## Task 3 — Include time, pwd, OS, and branch in environment prompt context

**Files**: `gocode/internal/agent/context_inject.go`, `progress.md`

Expanded the environment block injected into the system prompt so the model now
sees these fields explicitly on every turn:

- current time in RFC3339 format
- present working directory as `pwd`
- OS name
- architecture
- current git branch with a fallback when not on a branch

`gocode` already included git status, recent commits, and a working-directory
listing, so this change keeps the existing context and makes the high-signal
environment details explicit and easier for the model to use reliably.

## Task 4 — Document GitHub Copilot /connect usage in the README

**Files**: `README.md`, `gocode/README.md`, `progress.md`

Added a dedicated GitHub Copilot setup section to both user-facing README files.
The docs now explain:

- that GitHub Copilot uses `/connect` instead of a static API key
- what happens during the device-login flow
- how to use GitHub Enterprise with `/connect github-copilot <domain>`
- that the main model becomes `github-copilot/gpt-5.4`
- that the subagent model becomes `github-copilot/claude-haiku-4.5`
- that future launches can use the saved Copilot connection directly

The slash-command table was also updated to include `/connect`.

## Task 5 — Route GitHub Copilot models to the correct API protocol

**Files**: `gocode/internal/api/openai_responses.go`, `gocode/internal/api/anthropic.go`, `gocode/internal/api/github_copilot.go`, `gocode/internal/api/openai_compat.go`, `gocode/cmd/gocode/engine.go`, `progress.md`

Fixed the post-connect GitHub Copilot runtime failure where a normal prompt would
fail with `OpenAI-compatible request failed` even though `/connect` had already
completed successfully.

Root cause:

- `gocode` was sending every GitHub Copilot model through the OpenAI-compatible
  `/chat/completions` path
- Copilot does not use one protocol for every model family
- the selected main model `github-copilot/gpt-5.4` expects the OpenAI
  Responses API
- the selected subagent model `github-copilot/claude-haiku-4.5` expects the
  Anthropic Messages API with Copilot bearer-auth headers

Implementation completed:

- added a new `openai_responses.go` client that streams Copilot/OpenAI
  Responses events from `/responses`
- added Copilot model-family detection helpers in
  `gocode/internal/api/github_copilot.go`
- updated engine client creation so GitHub Copilot now routes by model:
  - GPT-5 and `o*` families use the Responses client
  - Claude families use the Anthropic Messages client
  - legacy Copilot-compatible models can still fall back to chat completions
- updated the Anthropic client so Copilot Claude models use bearer auth and
  Copilot headers instead of Anthropic API-key auth
- improved network-level provider errors so the underlying transport failure is
  included in the surfaced message instead of only showing a generic provider
  label

Verification completed:

- ran `gofmt -w` on all changed Go files
- ran `go build ./...`
- ran `make release-local` in `gocode/tui`

---
