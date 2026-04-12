# GitHub Copilot Connect Plan

## Goal Description

Enable `gocode` to use GitHub Copilot models through a simple slash-command login flow, with the smallest practical implementation that works in the existing Go engine and TUI architecture.

## Proposed Changes

### [MODIFY] `gocode/internal/config/config.go`

- Add persisted GitHub Copilot credential fields to `Config`.
- Store the long-lived GitHub device-flow token, short-lived Copilot access token, expiry timestamp, and optional enterprise domain.

### [NEW] `gocode/internal/api/github_copilot.go`

- Port the GitHub Copilot device-code OAuth flow from `pi-mono` into Go.
- Implement:
  - device-code start
  - polling for GitHub access token
  - Copilot token refresh
  - Copilot base URL derivation from `proxy-ep`
  - helper methods for token freshness and provider headers

### [MODIFY] `gocode/internal/api/provider_config.go`

- Register a `github-copilot` OpenAI-compatible provider preset.
- Set Copilot-specific defaults: base URL, default model, and capabilities.

### [MODIFY] `gocode/internal/api/openai_compat.go`

- Inject Copilot-specific headers for the OpenAI-compatible transport when the provider is `github-copilot`.
- Preserve normal behavior for all other providers.

### [MODIFY] `gocode/cmd/gocode/engine.go`

- Teach client creation to resolve GitHub Copilot credentials from config.
- Refresh expired Copilot access tokens automatically before creating the client.
- Persist refreshed tokens back to config when needed.

### [MODIFY] `gocode/cmd/gocode/slash_commands.go`

- Add `/connect` and `/connect github-copilot [enterprise-domain]`.
- Stream the device-login URL and user code to the existing TUI response area.
- Optionally open the verification URL in the default browser.
- Persist credentials, switch to a Copilot default model, and report the result.
- Update `/help` text to document the new command.

## User Review Required

- The login flow will be implemented as a device-code flow handled inside the Go engine, not a browser callback server.
- `/connect` will default to GitHub Copilot unless another provider is added later.
- The first connected model will switch to a Copilot default automatically so the connection is immediately usable.

## Open Questions

- None blocking. The initial implementation will target GitHub.com with optional enterprise domain support via an argument.

## Verification Plan

- Build the Go code with `go build ./...`.
- Run `gofmt -w` on changed Go files.
- Rebuild the local TUI/engine bundle used by `gocode`.
- Do not add tests.
