# Progress

## Active Task

- Completed: implemented structured debug logging and a live monitor flow for `/debug`.

## Notes

- Updated chan's runtime prompt in `chan/cmd/chan/engine.go` and `chan/internal/agent/planner.go` so the agent prefers subagent choreography for bounded exploration and execution, avoids polling non-background child agents, and treats implementation plans as durable artifacts that should be created or revised in place for substantial work.
- Expanded session-level safe auto-approve so `Allow Safe This Session` now auto-approves normal non-destructive, non-sensitive tool calls across the session instead of only read-only requests and safe bash commands.
- Reworked `chan/internal/debuglog` into a runtime-configurable JSONL logger with a versioned event envelope.
- Added centralized secret redaction and truncation for raw IPC and SSE payload logging.
- Switched IPC, SSE, and client debug wrappers to stay active so `/debug` can start capturing mid-session.
- Added `/debug`, `/debug status`, `/debug path`, and `/debug off` to the Go slash command layer.
- Added `chan debug-view --file <path>` as a standalone live monitor command.
- Added macOS Terminal popup launching for `/debug` using AppleScript.
- Updated the Bun launcher so installed `chan debug-view ...` forwards to the Go engine instead of opening the TUI.
- Rebound debug session output on `/clear` and `/resume` so logging follows the active session.
- Documented the new debug workflow and JSONL piping examples in `chan/README.md`.
- Updated `web/docs.html` with the new `/debug` and `chan debug-view` workflow details.
- Restored the animated gradient NDJSON connector in the architecture section styling for `web/index.html` via `web/styles.css`.
- Adjusted the architecture visual layout so the NDJSON connector line now physically touches and bridges both boxes.
- Added `ollama/gemma4:e4b` examples to the setup and usage documentation for local Ollama runs.
