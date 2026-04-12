# Implementation Plan

## Objective

Three work streams for `gocode`:

1. **Subagent specialization** — Add Search and Execution subagent types alongside the existing Explore and General-Purpose modes, with dedicated tool allowlists, system prompts, result formatting, and TUI labels.
2. **Bug fix: OpenAI Responses tool input JSON decode error** — Fix `decode tool input JSON: unexpected end of JSON input` when the OpenAI Responses API returns incomplete or malformed tool call arguments.
3. **Bug fix: Previous thinking messages shown in conversation** — Stop previous turns' thinking/reasoning content from being re-sent to the API and displayed in the conversation history.

---

## Stream A: Subagent Specialization

### Current State

- `agent`, `agent_status`, and `agent_stop` tools already exist.
- Two subagent types exist: `explore` (read-only) and `general-purpose` (broader tools).
- Background child-agent lifecycle is already rendered in the TUI.
- Missing: dedicated `search` and `execution` types, specialized prompts, structured return formats, parent-agent steering.

### A1. Subagent type model and runtime routing

Files: `internal/tools/agent.go`, `cmd/gocode/subagent_runtime.go`, `internal/ipc/protocol.go`, `tui/src/protocol/types.ts`

- Add `search` and `execution` to the `subagent_type` enum.
- Update validation, schema, and runtime dispatch.
- Route each type to its own tool allowlist and system prompt.
- Keep `explore` as the default for backward compatibility.

### A2. Specialized tool allowlists

Files: `cmd/gocode/subagent_runtime.go`

- **Explore** — keep current read-only tool set.
- **Search** — `think`, `list_dir`, `file_read`, `glob`, `grep`, `go_definition`, `go_references`, `symbol_search`, `project_overview`, `dependency_overview`, `git`. Exclude `web_search`/`web_fetch` to stay workspace-focused.
- **Execution** — `bash`, `list_commands`, `command_status`, `send_command_input`, `stop_command`, `forget_command`, and optionally `list_dir`/`file_read` for minimal context. No file-write tools.
- **General-purpose** — retain current broader set.

### A3. Subagent system prompts

Files: `cmd/gocode/subagent_runtime.go`

- **Explore** — strengthen with parallel read-only encouragement, file-path-based reporting.
- **Search** — iterative search, compact file-path + line-range references, `<final_answer>` block contract.
- **Execution** — terminal-focused task, compact command summaries, `<final_answer>` block contract.
- **General-purpose** — document as fallback for broader delegated work.

### A4. Parent-facing `agent` tool descriptions

Files: `internal/tools/agent.go`, optionally `cmd/gocode/engine.go`

- Extend `subagent_type` enum description with use-case guidance.
- Add examples: explore=research, search=code discovery, execution=terminal tasks, general-purpose=fallback.

### A5. Child result formatting

Files: `cmd/gocode/subagent_runtime.go`, `tui/src/components/ToolProgress.tsx`, `tui/src/hooks/useEvents.ts`

- Search: parse/preserve `<final_answer>` reference block, normalize path/line results.
- Execution: compact command summaries, no raw terminal dumps.
- Explore: keep concise findings.

### A6. Permission and safety behavior

Files: `cmd/gocode/subagent_runtime.go`, `internal/permissions/clone.go`, `internal/permissions/executor.go`, `internal/permissions/gating.go`

- `explore` and `search` remain read-only.
- `execution` may execute commands but no file-write by default.
- `general-purpose` remains broad.
- Execution tools must auto-approve under cloned policy or fail predictably.

### A7. TUI surfacing

Files: `tui/src/components/BackgroundAgentsPanel.tsx`, `tui/src/components/ToolProgress.tsx`, `tui/src/hooks/useEvents.ts`

- Friendly labels for `search` and `execution`.
- Distinct summaries for research vs search-references vs terminal-execution.
- Status copy tuned for each behavior.

### A8. Documentation

Files: `README.md`, optionally `docs/`

- Document each subagent type, when to use them, current limitations.

### A9. Tests

Files: new tests in `cmd/gocode/`, `internal/tools/`

- Schema validation for new `subagent_type` values.
- Tool allowlist correctness per mode.
- System prompt content per type.
- Read-only enforcement for explore/search.
- Background-agent payloads preserve new types end-to-end.

---

## Stream B: Fix OpenAI Responses Tool Input JSON Decode Error

### Root Cause

`decodeToolInput()` in `internal/api/anthropic.go` (shared helper) does a strict `json.Unmarshal` on tool call arguments. When the OpenAI Responses API streams tool arguments and the final accumulated string is incomplete or malformed JSON, this call fails with `unexpected end of JSON input`.

The call sites are:
- `internal/api/openai_responses.go:730` — `handleOutputItemDone()` validates tool arguments on stream completion.
- `internal/api/openai_responses.go:452` — `buildOpenAIResponsesInput()` validates tool calls in message history.

### Planned Fix

- In `handleOutputItemDone()`, handle the `decodeToolInput` error gracefully — log a warning and either skip the malformed tool call or surface a user-visible error without crashing the stream.
- Ensure `buildOpenAIResponsesInput()` tolerates previously-stored tool calls whose arguments may not have been fully received.
- Consider adding a fallback: if the accumulated arguments string fails to parse, attempt recovery (e.g., treat as raw string or return a descriptive error to the model).

### Files

- `internal/api/openai_responses.go`
- `internal/api/anthropic.go` (shared `decodeToolInput()` function)

---

## Stream C: Fix Previous Thinking Messages Shown in Conversation

### Root Cause

When models return thinking/reasoning content (via `reasoning_delta`, `reasoning` fields, or Anthropic thinking blocks), it is accumulated into `Message.Content` alongside the actual response text. On subsequent turns, `buildOpenAICompatMessages()`, `buildOpenAIResponsesInput()`, and `buildAnthropicMessages()` send the full `Message.Content` back to the API — including the thinking content. This causes:

1. Massive context bloat from previous thinking being re-sent every turn.
2. Previous thinking being visible in the conversation transcript in the TUI.

### Planned Fix

- Add a separate field (e.g., `ReasoningContent string`) to the `Message` struct in `internal/api/client.go` to store thinking content separately from `Content`.
- Update streaming handlers to write thinking/reasoning output to `ReasoningContent` instead of `Content`.
- Update all message-building functions (`buildOpenAICompatMessages`, `buildOpenAIResponsesInput`, `buildAnthropicMessages`) to exclude `ReasoningContent` from the messages sent to the API.
- Update the TUI: thinking blocks should render as collapsible/hidden in past turns, only shown expanded for the current in-progress response.

### Files

- `internal/api/client.go` — add `ReasoningContent` field to `Message`.
- `internal/api/openai_compat.go` — separate thinking from content in streaming; exclude from `buildOpenAICompatMessages()`.
- `internal/api/openai_responses.go` — separate thinking; exclude from `buildOpenAIResponsesInput()`.
- `internal/api/anthropic.go` — separate thinking; exclude from `buildAnthropicMessages()`.
- `tui/src/hooks/useEvents.ts` — ensure past thinking blocks are not expanded.
- `tui/src/components/` — update rendering to collapse past thinking.

---

## Implementation Phases

### Phase 1: Bug fixes (Streams B & C)

Priority: fix the two bugs first since they affect daily usability.

1. Fix OpenAI Responses tool input JSON decode error (Stream B).
2. Fix thinking messages in conversation history (Stream C).
3. Build, format, test.

### Phase 2: Subagent type model (Stream A, tasks A1–A3)

1. Add `search` and `execution` subagent types.
2. Define tool allowlists per type.
3. Add specialized system prompts.

### Phase 3: Parent guidance and result formatting (Stream A, tasks A4–A5)

1. Update `agent` tool descriptions.
2. Add result postprocessing per type.

### Phase 4: Permissions, TUI, docs, tests (Stream A, tasks A6–A9)

1. Tighten permission behavior.
2. Update TUI labels and summaries.
3. Documentation.
4. Tests.

## Open Questions

- Should `execution` allow `list_dir`/`file_read` for context, or stay terminal-only?
- Should the parent system prompt explicitly steer toward `search`/`execution`?
- Should `<final_answer>` blocks be parsed at runtime or initially returned raw?
- Single `agent` tool vs future dedicated `search_subagent`/`execution_subagent` aliases?
- Config knob for per-mode model selection, or single `SubagentModel` for phase 1?

## Verification Plan

- Build verification for TUI protocol type changes.
- Manual flows: explore, search, execution, background agents, plan mode unchanged.
- Verify tool input JSON errors no longer crash the stream.
- Verify previous thinking is not re-sent to API or displayed in history.
