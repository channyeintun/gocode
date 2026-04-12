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

---
