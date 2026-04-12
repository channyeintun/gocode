# Progress

## Current Phase

Phase 1: Bug fixes (Streams B & C). Not yet started.

## Completed

- [x] Reviewed implementation-plan.md (subagent specialization).
- [x] Investigated OpenAI Responses tool input JSON decode error (Stream B root cause identified).
- [x] Investigated thinking messages shown in conversation (Stream C root cause identified).
- [x] Created plan.md covering all three work streams.
- [x] Created progress.md.

## In Progress

None.

## Pending

### Phase 1: Bug Fixes

- [x] **B1** Fix `decodeToolInput` error handling in `internal/api/openai_responses.go` (`handleOutputItemDone` and `buildOpenAIResponsesInput`).
- [x] **C1** Add `ReasoningContent` field to `Message` struct in `internal/api/client.go`.
- [x] **C2** Persist streamed thinking separately from assistant response text in the agent loop so it lands in `ReasoningContent` instead of `Content`.
- [x] **C3** Keep provider request builders reasoning-free by continuing to serialize only assistant `Content`, which now excludes stored thinking.
- [x] **C4** Update TUI to hide past thinking blocks while keeping live thinking visible during streaming.
- [x] Build, format, verify.

### Phase 2: Subagent Type Model (A1–A3)

- [ ] **A1** Add `search` and `execution` subagent type enum values and routing.
- [ ] **A2** Define tool allowlists per subagent type.
- [ ] **A3** Add per-type system prompts.

### Phase 3: Parent Guidance & Result Formatting (A4–A5)

- [ ] **A4** Update `agent` tool descriptions with use-case guidance.
- [ ] **A5** Add subagent-type-aware result postprocessing.

### Phase 4: Permissions, TUI, Docs, Tests (A6–A9)

- [ ] **A6** Tighten permission behavior per subagent type.
- [ ] **A7** TUI-friendly labels and summaries for new types.
- [ ] **A8** Documentation.
- [ ] **A9** Tests.

## Notes

- Stream B root cause: `decodeToolInput()` in `internal/api/anthropic.go` does strict `json.Unmarshal`. Called from `openai_responses.go:730` (handleOutputItemDone) and `:452` (buildOpenAIResponsesInput). Fails when accumulated tool arguments are incomplete JSON.
- Stream B implementation: `openai_responses.go` now prefers a valid final `output_item.done` arguments payload when the streamed buffer is incomplete, and degrades malformed historical tool-call inputs to `{}` instead of aborting request construction.
- Stream C root cause: thinking/reasoning content is accumulated into `Message.Content`. All message-building functions (`buildOpenAICompatMessages`, `buildOpenAIResponsesInput`, `buildAnthropicMessages`) re-send full Content including thinking on subsequent turns. No separate storage field exists.
- Stream C implementation: `internal/agent/loop.go` now persists thinking into `Message.ReasoningContent`, leaving `Message.Content` as user-visible assistant text only. The TUI keeps live thinking during streaming but drops it from completed assistant messages.

## Decisions

- Bug fixes (Phase 1) take priority over subagent work since they affect daily usability.
- Subagent: `explore` remains the default type for backward compatibility.
- Subagent: `search` should be workspace-only (no web_search/web_fetch).
- Subagent: `execution` should be terminal-focused, non-writing by default.
