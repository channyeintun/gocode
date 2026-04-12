# GitHub Copilot Connect Plan

## Goal Description

Enable `gocode` to use GitHub Copilot models through a simple slash-command login flow, with the smallest practical implementation that works in the existing Go engine and TUI architecture.

---

# AOP Debugger Monitoring Tool Plan

## Goal

A zero-source-change runtime debugger that intercepts function calls, SSE events,
goroutine activity, and IPC bridge traffic inside the `gocode` engine, writing
structured logs to a file so an agent (or human) can diagnose issues like the
empty-response bug without reasoning through source code.

Activated by `GOCODE_DEBUG=1` or `--debug`. All logs go to a single file
(`~/.config/gocode/debug.log`, rotated per session).

## Design Principles

- **Non-invasive**: existing function signatures and types do not change.
- **Compile-time wrappers**: thin decorator types implement existing interfaces
  (`LLMClient`, `io.Reader`) and delegate to the real implementation while
  logging every call.
- **Single log sink**: a shared `debuglog.Logger` backed by a mutex-protected
  file writer. JSONL format so logs are grep-friendly and machine-parseable.
- **Conditional activation**: wrappers are only inserted when the debug flag is
  set. Normal builds carry zero overhead beyond a single `if debugEnabled` check
  at startup.

## Proposed Changes

### [NEW] `gocode/internal/debuglog/logger.go`

Core debug logger.

- `var Enabled bool` — set once at startup from env/flag.
- `func Init(sessionDir string)` — opens `debug.log` in the session directory.
- `func Log(category, event string, fields map[string]any)` — appends one JSONL
  line with timestamp, goroutine ID, category, event name, and arbitrary fields.
- `func Close()` — flushes and closes the file.
- Categories: `sse`, `responses`, `anthropic`, `ipc`, `agent`, `goroutine`.

### [NEW] `gocode/internal/debuglog/sse_reader.go`

SSE stream interceptor — wraps `io.Reader`.

- `type SSEReaderProxy struct { inner io.Reader }` — implements `io.Reader`.
- Every `Read` call logs the raw bytes before returning them.
- Inserted by the Responses/Anthropic/Gemini clients when `debuglog.Enabled`.
- Captures the exact SSE frames the provider sends, including event names and
  data payloads, so dropped or malformed events are visible in the log.

### [NEW] `gocode/internal/debuglog/client_proxy.go`

LLMClient interceptor — wraps `api.LLMClient`.

- `type ClientProxy struct { inner api.LLMClient }` — implements `LLMClient`.
- `Stream()`: logs the `ModelRequest` (system prompt length, message count, tool
  count, reasoning effort), then wraps the returned iterator so every
  `ModelEvent` is logged before being yielded (event type, text length, tool
  call name, stop reason).
- `ModelID()`, `Capabilities()`: pass-through with a log entry.
- `Warmup()`: logs start/end/error.
- Inserted in `engine.go` → `newLLMClient` when `debuglog.Enabled`.

### [NEW] `gocode/internal/debuglog/bridge_proxy.go`

IPC bridge interceptor — wraps `ipc.Bridge` emit methods.

- Logs every `Emit` / `EmitEvent` / `EmitError` call with event type and
  payload summary (truncated to 500 chars).
- Logs every inbound message from the TUI (message type, payload size).
- Inserted in `engine.go` → `runStdioEngine` when `debuglog.Enabled`.

### [NEW] `gocode/internal/debuglog/goroutine.go`

Goroutine & channel snapshot helper.

- `func SnapshotGoroutines() string` — calls `runtime.Stack` and returns a
  formatted goroutine dump (filtered to gocode frames).
- `func LogGoroutineCount()` — logs `runtime.NumGoroutine()`.
- Called at key lifecycle points: session start, query start, query end,
  subagent start/end, stream open/close.

### [MODIFY] `gocode/cmd/gocode/engine.go`

- At the top of `runStdioEngine`, check `GOCODE_DEBUG` env var and call
  `debuglog.Init(sessionDir)`.
- Wrap `client` with `debuglog.NewClientProxy(client)` when enabled.
- Wrap `bridge` emit path when enabled.
- Call `debuglog.Close()` on shutdown.

### [MODIFY] `gocode/internal/api/openai_responses.go`

- In `openStream`, when `debuglog.Enabled`, wrap `resp.Body` with
  `debuglog.NewSSEReaderProxy(resp.Body)` before passing to `readSSE`.

### [MODIFY] `gocode/internal/api/anthropic.go`

- Same SSE body wrapper as above.

### [MODIFY] `gocode/internal/api/gemini.go`

- Same SSE body wrapper as above.

## Log Format (JSONL)

```json
{"ts":"2026-04-12T17:48:01.123Z","gid":1,"cat":"sse","evt":"read","bytes":1024,"raw":"event: response.output_text.delta\\ndata: {...}"}
{"ts":"2026-04-12T17:48:01.124Z","gid":1,"cat":"responses","evt":"handle_event","type":"response.output_text.delta","delta_len":42}
{"ts":"2026-04-12T17:48:01.200Z","gid":1,"cat":"responses","evt":"emit","model_event":"token","text_len":42}
{"ts":"2026-04-12T17:48:02.000Z","gid":1,"cat":"responses","evt":"stream_eof","sent_stop":false,"saw_content":true,"saw_tool":false}
{"ts":"2026-04-12T17:48:02.001Z","gid":1,"cat":"agent","evt":"turn_result","assistant_text_len":0,"tool_calls":0,"stop_reason":"end_turn"}
{"ts":"2026-04-12T17:48:02.002Z","gid":1,"cat":"ipc","evt":"emit","type":"turn_complete","stop_reason":"end_turn"}
{"ts":"2026-04-12T17:48:02.003Z","gid":1,"cat":"goroutine","evt":"snapshot","count":12}
```

## What This Would Have Caught

For the empty-response bug:

1. **SSE reader log** would show the exact raw bytes from Copilot — whether text
   deltas were actually sent or the stream was truly empty.
2. **Responses handler log** would show which event types were received and
   whether any fell through to `default: return nil`.
3. **Client proxy log** would show the `ModelEvent` sequence yielded to the
   agent loop — confirming whether tokens were emitted before the stop.
4. **Agent turn result log** would show `assistant_text_len: 0` immediately,
   instead of requiring source-code reasoning to discover it.
5. **IPC bridge log** would show the `turn_complete` payload sent to the TUI,
   confirming the empty turn.

## Implementation Order

1. `debuglog/logger.go` — core logger, JSONL writer, env flag
2. `debuglog/sse_reader.go` — raw SSE interceptor
3. `debuglog/client_proxy.go` — LLMClient wrapper
4. `debuglog/goroutine.go` — goroutine snapshots
5. `debuglog/bridge_proxy.go` — IPC emit interceptor
6. Wire into `engine.go` and the three SSE-consuming clients
7. Verify with `GOCODE_DEBUG=1 gocode` and inspect `debug.log`

## Constraints

- No tests.
- No changes to existing function signatures or types.
- Debug mode must not alter runtime behavior — only observe.
- Log file must not grow unbounded: truncate at session start, cap at 50 MB.
- Sensitive data (API keys, full message content) must be redacted or truncated.

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

## Reference-Alignment Follow-Up

Align the existing GitHub Copilot implementation more closely with the proven
`opencode` and `pi-mono` behavior so Copilot support is treated as a deliberate
provider integration instead of a sequence of isolated compatibility fixes.

### [MODIFY] `gocode/internal/api/github_copilot.go`

- Improve device-flow polling to match reference behavior more closely:
  - apply interval multipliers during polling
  - handle repeated `slow_down` responses more defensibly
  - surface a clearer timeout message when clock drift is likely
- Add post-login model policy enablement helpers for Copilot models that require
  explicit acceptance before use.
- Add runtime `/models` discovery with short request timeouts and caching so
  Copilot model capabilities can be derived from the provider response instead
  of only hardcoded presets.

### [MODIFY] `gocode/cmd/gocode/slash_commands.go`

- After a successful GitHub Copilot login, best-effort enable the connected
  Copilot models so default GPT-5.4 and Claude Haiku 4.5 usage works more like
  the reference implementations.

### [MODIFY] `gocode/cmd/gocode/engine.go`

- Resolve GitHub Copilot model capabilities dynamically during client creation
  and apply them consistently to the active client so tool use, vision,
  reasoning, and token-budget behavior reflect the provider's current model
  metadata.
