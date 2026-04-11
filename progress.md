# Progress

## Working Rules

- Follow `plan.md` as the active execution baseline.
- Use `enhancement.md` as the primary design reference and use targeted reads from `gocode/` and `microsoft/vscode-copilot-chat` only to confirm concrete implementation decisions.
- Do not add tests.
- Do not weaken cwd containment, deterministic exact-edit behavior, or file-history rollback support for the sake of parity.
- Do not plan teams, swarm orchestration, remote execution, browser automation, or unrelated product lines.
- After each completed task: update this file, run formatting, and create a git commit.

## Current Status

| Workstream                               | Status      | Scope | Notes                                                                                                                                                   |
| ---------------------------------------- | ----------- | ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Enhancement planning baseline            | completed   | S     | 2026-04-12 created a new execution baseline from `enhancement.md` focused on file-tool robustness and subagent orchestration.                           |
| Phase 1 file semantics and safety        | completed   | L     | Create, overwrite, and edit intent are now split; read hardening, high-risk file approvals, stable diff previews, and file-history coverage are landed. |
| Phase 2 edit engine hardening            | completed   | L     | The edit ladder, structured failures, patch-grade edits, and post-edit diagnostics are now surfaced across file-mutating tools.                        |
| Phase 3 subagent lineage and metadata    | completed   | M     | Stable invocation ids, structured child metadata, and stronger TUI attribution are now landed for child runs.                                            |
| Phase 4 child lifecycle and policy hooks | completed   | M     | Child runs now share the loop stop-control contract, honor start/stop hooks, and surface block-stop reasons in status and transcript state.            |

## Completion Dashboard

This section is the canonical phase tracker. A phase is only complete when its `Remaining to Finish` list is empty and its exit criteria are all satisfied.

### Phase 1: File Semantics and Safety Hardening

**Status:** completed

**Landed**

- `file_write` now requires explicit `overwrite=true` before replacing an existing file.
- `file_edit` no longer acts as an implicit create path and now requires editing an existing file with a non-empty `old_string`.
- Added `create_file` as a dedicated create-only path, and narrowed `file_write` to overwrite-only behavior for existing files.
- `file_read` now rejects likely binary or image-like files and adds explicit continuation guidance for partial ranged reads.
- Sensitive local file targets such as `.env*`, `.git/*`, `.vscode/*.json`, selected dotfiles, and lockfiles now escalate to `high` risk in permission prompts.
- High-risk file targets now surface an explicit policy reason in the permission prompt and no longer grant persistent `always_allow` rules.
- Direct write tools now share stable diff-preview behavior and file-history tracking across create, overwrite, exact edit, and multi-replace flows.
- README tool descriptions and runtime/UI labels now match the stricter create, overwrite, edit, and read semantics.

**Remaining to Finish**

- None.

**Exit Criteria Check**

- [x] Create, overwrite, and edit intent are no longer conflated.
- [x] `file_read` safely distinguishes text reads from binary-like content and guides continued reads.
- [x] Higher-risk local files require stronger approval than ordinary source files.
- [x] File-history behavior still covers every direct write path.

### Phase 2: Edit Engine Hardening

**Status:** completed

**Landed**

- `file_edit` and `multi_replace_file_content` now classify edit failures with explicit kinds such as `no_match`, `multiple_matches`, `content_mismatch`, and `invalid_range`.
- Recovery hints now travel through Go tool output, IPC payloads, and the TUI so edit failures surface actionable next steps instead of flat strings.
- `multi_replace_file_content` now renders as a first-class file mutation in the TUI, including structured file-update failures.
- Added `apply_patch` as a dedicated patch-grade edit tool for structured multi-hunk and multi-file text edits.
- Permission summaries, risk checks, compaction rules, subagent allowlists, runtime tool guidance, and TUI rendering now recognize `apply_patch` as part of the file-edit ladder.
- The runtime prompt, tool descriptions, recovery hints, and README docs now define an explicit edit ladder across `file_edit`, `multi_replace_file_content`, `apply_patch`, `file_write`, and `create_file`.
- File-mutating tools now run post-edit diagnostics when an obvious local checker is available and surface the results directly in the file-mutation UI.

**Remaining to Finish**

- None.

**Exit Criteria Check**

- [x] `gocode` has a dedicated patch-grade edit path for larger structural changes.
- [x] File-edit failures are machine-distinguishable and actionable.
- [x] The edit ladder is explicit enough that structural edits do not overuse exact replacement tools.
- [x] Post-edit diagnostics catch broken edits earlier when the environment can provide them.

### Phase 3: Subagent Lineage and Metadata

**Status:** completed

**Landed**

- Stable `invocation_id` values are now assigned before every child run and reused across sync results, background launches, status checks, transcripts, and result paths.
- `agent`, `agent_status`, and `agent_stop` now carry structured child metadata alongside the existing flat fields so lifecycle, lineage, transcript, result-path, and tool-list state stay machine-readable.
- The TUI background-agent state now consumes structured child metadata directly and renders child runs by stable invocation lineage instead of inferring everything from mixed `agent_id` and summary strings.

**Remaining to Finish**

- None.

**Exit Criteria Check**

- [x] Every child run is traceable from parent launch through completion or cancellation.
- [x] Background and sync child runs expose enough metadata to debug them without opening raw transcripts by default.
- [x] Child-agent lifecycle and cost data remain attributable without ambiguity.

### Phase 4: Shared Child Lifecycle and Policy Hooks

**Status:** completed

**Landed**

- Child runs now use the shared query-loop stop-control contract so hook-driven stop blocking happens inside the same iteration model as the main agent loop.
- Subagents now fire optional child start hooks through the existing hook runner and inject any returned context into the delegated child prompt before the first iteration.
- Child stop hooks can now block completion with explicit reasons, which are persisted back into the child transcript as follow-up context and surfaced in child metadata.
- Background child status updates and final child results now carry stop-block reason and count metadata so blocked stops stay visible without opening raw transcripts.

**Remaining to Finish**

- None.

**Exit Criteria Check**

- [x] Parent and child execution share the same core lifecycle concepts instead of drifting.
- [x] Child agents can be prevented from stopping early through explicit hook policy.
- [x] Child stop-block reasons remain visible and actionable.
- [x] Child isolation remains clear enough that parent context and artifact ownership are not polluted.

## Task Log

### 2026-04-12

- Completed: replaced the deleted planning files with a new enhancement execution baseline derived from `enhancement.md`.
- Completed: narrowed the roadmap to two workstreams only: file-tool robustness and subagent orchestration.
- Completed: translated the enhancement research into four implementation phases with explicit guardrails and exit criteria.
- Note: this remains a planning-only task. No runtime, tool, or TUI implementation work has started yet.
- Completed: started Phase 1 by making `file_write` require explicit overwrite permission for existing files, restricting `file_edit` to existing-file edits with non-empty `old_string`, and hardening `file_read` with binary detection plus ranged-read continuation hints.
- Completed: updated the README tool table so the user-facing file-tool descriptions match the new create, overwrite, edit, and read behavior.
- Completed: added high-risk file approval rules for sensitive project-local paths, surfaced policy reasons in the permission prompt, and prevented persistent `always_allow` approval on those targets while keeping cwd containment unchanged.
- Completed: finished Phase 1 by adding a dedicated `create_file` tool, narrowing `file_write` to overwrite-only behavior, updating tool labels/prompts/docs to reflect the split, and confirming diff-preview plus file-history coverage across the direct write tool surface.
- Completed: started Phase 2 by standardizing edit failure kinds and recovery hints across `file_edit` and `multi_replace_file_content`, and by surfacing those structured failures in the TUI.
- Completed: added a dedicated `apply_patch` tool for structured multi-file edits and wired it through permission summaries, risk assessment, compaction, runtime tool guidance, and TUI file-mutation rendering.
- Completed: defined the runtime edit ladder explicitly in the system prompt, tool descriptions, recovery hints, and README guidance so structural edits have a clear path to `apply_patch`.
- Completed: finished Phase 2 by adding post-edit diagnostics for file-mutating tools and surfacing those diagnostics in the file-mutation UI when local Go or TypeScript checkers are available.
- Completed: finished Phase 3 by assigning stable child invocation ids, returning structured child metadata from the `agent` flow, and updating the TUI background-agent state to consume that metadata directly.
- Note: a dedicated per-child active-tool field remains optional future polish, but the current lifecycle, transcript, result-path, tool-list, and cost metadata already satisfies the Phase 3 debugging and attribution bar.
- Completed: finished Phase 4 by aligning child stop control with the shared query loop, adding child start/stop hooks, and surfacing stop-block reasons in both child status metadata and the persisted transcript.

## Next Planning Baseline

See `enhancement.md` for the underlying research and decision rationale. See `plan.md` for the active execution sequence and phase scope.
