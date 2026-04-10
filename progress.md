# Progress

## Working Rules

- Follow [plan.md](/Users/channyeintun/Documents/go-code/plan.md) as the execution baseline.
- Reference `sourcecode/` first for every feature or behavior change.
- Do not add tests.
- After each completed task: update this file, run formatting, and create a git commit.

## Current Status

| Phase                                      | Status      | Notes                                                                                                                                                                                                                                                                                                                                                        |
| ------------------------------------------ | ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1. Layout and prompt foundation            | completed   | Cursor-aware editing, multiline entry, wrapped-line navigation, prompt footer groundwork, and clipboard image paste are in place. The image path now includes the required TUI and engine protocol expansion.                                                                                                                                                |
| 2. Permission UX parity                    | completed   | The permission prompt now uses a selectable flow with arrow-key focus, Enter confirmation, direct shortcut keys, and Esc mapped into the existing deny path. Amendment feedback is still blocked by the current engine payload shape.                                                                                                                        |
| 3. Markdown and syntax highlighting parity | completed   | `marked-terminal` has been replaced with a token-based markdown renderer that caches lexer output, renders tables through a dedicated component, highlights fenced code blocks, and uses a stable-prefix path for streaming assistant output.                                                                                                                |
| 4. Transcript/message-row parity           | completed   | The transcript now uses a shared row wrapper, distinct row renderers, continuation-aware message labels, assistant row metadata stamped from reducer state, and an anchored long-session render cap modeled on the upstream non-virtual fallback path. A real virtual list still depends on scroll/fullscreen primitives that the current TUI does not have. |
| 5a. Status line parity                     | completed   | The boxed top bar has been replaced with a lighter inline status line that shows workspace/session context, formatted model display, cost and token counts, and a best-effort context percentage derived from model presets. Exact session titles and exact live context usage still need protocol/state follow-up.                                          |
| 5b. Prompt footer parity                   | not started | Waiting for Phase 5a completion.                                                                                                                                                                                                                                                                                                                             |
| 6. Protocol follow-up                      | not started | Only if parity requires engine changes.                                                                                                                                                                                                                                                                                                                      |

## Task Log

### 2026-04-10

- Completed: reset `progress.md` back to the current parity plan only after stale unrelated history reappeared.
- Completed: referenced `sourcecode/hooks/useTextInput.ts`, `sourcecode/hooks/useArrowKeyHistory.tsx`, `sourcecode/components/TextInput.tsx`, and `sourcecode/utils/Cursor.ts` before continuing Phase 1 prompt work.
- Completed: landed the first Phase 1 slice with cursor-aware editing, multiline input via Shift+Enter or Meta+Enter, word and line movement, and a bordered prompt container.
- Completed: added wrapped-line aware prompt rendering and vertical cursor movement based on the current terminal width.
- Completed: added a fuller prompt-adjacent footer with mode, activity, wrapped-input state, and shortcut hints, based on upstream `PromptInputFooter` and `PromptInputFooterLeftSide` structure.
- Completed: added clipboard image paste support with inline `[Image #N]` references, prompt attachment tracking, and image-aware submit handling based on the upstream prompt flow.
- Completed: expanded the `user_input` payload and Go IPC bridge to carry image attachments, including a larger NDJSON line limit so base64 image payloads fit through stdio.
- Completed: reject image input on non-vision models and serialize image blocks for Anthropic, OpenAI-compatible, and Gemini providers.
- Completed: replaced the static `y/n/a/s` permission box with a selectable permission prompt modeled on the upstream flow, including focusable options, direct shortcuts, Enter confirmation, and an explicit Esc cancel path.
- Completed: kept the Phase 2 implementation TUI-only because the current permission payload still exposes only `tool`, `command`, and `risk` plus the decision callback.
- Documented gap: upstream-style amendment or feedback input is still not wired because the Go engine has no permission-response field for feedback text.
- Completed: replaced the `marked-terminal` path with a token-based markdown renderer modeled on the upstream lexer-and-format pipeline.
- Completed: added module-level markdown token caching, dedicated table rendering, fenced code syntax highlighting, and a streaming renderer that preserves the stable prefix during live assistant output.
- Completed: removed the old `marked-terminal` dependency and its type package from the TUI package manifest.
- Completed: introduced a shared transcript row wrapper and split `StreamOutput` into distinct renderers for user text, assistant text, live thinking, live assistant output, tool rows, and grouped read/search rows.
- Completed: moved tool and grouped tool output onto the same row rhythm as message content so the transcript layout is no longer composed from a single coarse `StreamOutput` block.
- Completed: stamped transcript messages with creation metadata in the reducer and surfaced assistant model/time metadata plus continuation-aware labels through the row renderers.
- Completed: ported the upstream slice-anchor idea into `StreamOutput` so long sessions stop re-rendering the entire transcript while keeping the visible window stable across appended rows and regrouping changes.
- Completed: treated the anchored transcript cap as the Phase 4 endpoint for the current local architecture, since the TUI still lacks the scroll/fullscreen primitives required for a real `VirtualMessageList` port.
- Completed: replaced the boxed status bar with a lighter inline status line that keeps mode, model, cost, token counts, workspace context, and resumed-session context visible in a more upstream-like shape.
- Documented gap: the status line uses a best-effort context percentage from model window presets because the protocol does not currently expose exact live context usage or a real session title.
- Next: continue with Phase 5b by moving prompt hints and mode indicators into a dedicated footer layer beneath the input.
