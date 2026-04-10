# TUI Parity Plan

## Goal

Bring `go-cli/tui` as close as possible to the interaction model and visual behavior used in `sourcecode`, with priority on:

- main prompt input
- permission prompt input
- markdown rendering
- syntax highlighting
- transcript/message layout
- status/footer behavior

## Remaining Work

No planned parity work remains in this pass.

## Post-Parity Stabilization

Current follow-up fixes focus on:

1. Queue prompt submissions that arrive during an active turn instead of clearing the live response.
2. Keep the live assistant status visible across tool execution and permission waits.
3. Make Esc interruption reliable while a turn is active.

### Deferred Follow-up

1. Full scroll/fullscreen primitives and a true virtualized transcript list remain deferred because the upstream implementation depends on custom renderer internals that stock Ink does not expose. The anchored render cap plus transcript paging/jump controls are the accepted local substitute for now.

## Out of Scope

The following sourcecode features are explicitly excluded from this parity effort:

- **Voice input** — waveform animation, voice recording integration
- **Vim mode** — `useTextInput` supports vim keybindings but this requires a keybinding subsystem we don't have
- **Feature flag system** — compile-time `feature()` gates
- **Plugin system** — third-party plugin initialization and lifecycle
- **Multi-screen architecture** — separate screens (Doctor, ResumeConversation, AssistantSessionChooser, etc.)
- **Modal/dialog overlays** — dialog launcher system for multiple overlay types
- **Coordinator mode** — separate app flowpath for coordinator sessions
- **Analytics/telemetry** — event tracking in permission prompts and other interactions
- **MDM/keychain** — enterprise prefetch and credential management
- **Suggestion dropdown** — inline suggestion/autocomplete dropdown in prompt input

These can be revisited individually if needed later.

## Risks

- permission amendment/feedback parity requires engine protocol expansion
- block-oriented messages would be a significant refactor of both engine emitters and TUI reducers
- virtual transcript list depends on scroll/fullscreen primitives that Ink does not natively provide

## Definition of Done

- remaining protocol evaluation items are resolved (implemented or explicitly deferred)
- virtual transcript list is either implemented or the anchored render cap is confirmed as sufficient
- parity roadmap is complete for this pass, with any renderer-level fullscreen work explicitly deferred
