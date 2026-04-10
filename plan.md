# TUI Parity Plan

## Goal

Bring `gocode/tui` as close as possible to the interaction model and visual behavior used in `sourcecode`, with priority on:

- main prompt input
- permission prompt input
- markdown rendering
- syntax highlighting
- transcript/message layout
- status/footer behavior

## Remaining Work

### Phase 8: Agent Tool Enhancement (Antigravity Parity)
Expand the agent's core tool registry to match the advanced capabilities of comprehensive coding agents (like Antigravity) by adding:

1. **multi_replace_file_content**
   - **Purpose:** Safe, atomic edits for multiple non-contiguous block changes in a single file update without clunky sed scripts.
   - **Implementation Details:** 
     - **Input Schema:** Expects a `TargetFile` string, and a `ReplacementChunks` array. Each chunk contains: `StartLine`, `EndLine`, `TargetContent` (exact string matcher), and `ReplacementContent`.
     - **Engine Behavior:** The Go engine reads the file into memory and validates *all* chunks to guarantee `TargetContent` strictly matches the exact string between `StartLine` and `EndLine`. This prevents destructive edits. It then executes the replacements from bottom-to-top (highest line number to lowest) to preserve coordinate indices before writing atomically to disk.

2. **command_status** & **send_command_input** (Background Tasks)
   - **Purpose:** Background job orchestration for persistent REPLs, sending stdin interactively, and monitoring dev servers asynchronously without blocking the LLM agent's loop.
   - **Implementation Details:** 
     - **Input Schema (`send_command_input`):** `CommandId`, `Input` string (usually with an explicit `\n`), and `WaitMs` for buffering STDOUT output.
     - **Input Schema (`command_status`):** `CommandId` and `WaitDurationSeconds` to monitor completion.
     - **Engine Behavior:** The Go engine tracks long-running processes via a thread-safe map `map[string]*exec.Cmd` initialized with pseudo-terminals (PTYs). Asynchronous stdout/stderr streams are captured into ring buffers. `send_command_input` writes to the PTY's `stdin` descriptor, waits the specified MS, and returns the accumulated buffer delta back to the tool transcript.

3. **list_dir**
   - **Purpose:** Native, fast, structured deep directory listing support to avoid messy, truncated CLI wrapper parsing.
   - **Implementation Details:** 
     - **Input Schema:** `DirectoryPath` (absolute string path).
     - **Engine Behavior:** The Go engine traverses the file system using `os.ReadDir()`, gathering the node name, type flag (`isDir`), and `sizeBytes`. It returns a compact JSON or JSON-Lines array to the model. This avoids piping through generic `/bin/ls` formatting which is prone to LLM parsing errors and space-character bugs in bash context.

4. **Remove Claude-Specific Memory Artifacts**
   - **Purpose:** Remove legacy Claude-oriented repo and user memory file conventions so the engine no longer depends on `.claude` directories or `CLAUDE.md` files.
   - **Implementation Details:**
     - **Scope:** Remove discovery, loading, comments, docs, and ignore rules tied to `~/.claude/CLAUDE.md`, `CLAUDE.md`, `.claude/CLAUDE.md`, and `.claude/rules/*.md`.
     - **Engine Behavior:** Project/context loading should stop scanning Claude-specific paths and rely only on the repo's supported instruction and memory mechanisms.


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
