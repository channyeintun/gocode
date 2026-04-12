# Release Note V2

Date: 2026-04-12

Scope: completion of the enhancement roadmap focused on file-tool robustness and subagent orchestration across Phases 1 through 4.

## Highlights

- Completed the full enhancement execution baseline from `enhancement.md`, with all four phases now marked shipped in `progress.md`.
- Hardened the file-tool surface so create, overwrite, exact edit, multi-replace, and patch-grade structural edits each have clearer semantics, stronger failure handling, and safer approvals.
- Added earlier feedback and recovery loops around file mutations, including structured edit-failure kinds, explicit recovery hints, stable diff previews, file-history coverage, and post-edit diagnostics when local checkers are available.
- Strengthened child-agent orchestration with stable invocation lineage, structured child metadata, direct TUI attribution, and hook-aware lifecycle control that can block child completion with explicit reasons.

## Shipped Changes

### File Semantics and Safety

- Split file creation, overwrite, and in-place edit behavior so `create_file`, `file_write`, and `file_edit` no longer blur intent.
- Hardened `file_read` with binary or image-like detection plus better continuation guidance for partial ranged reads.
- Escalated sensitive local targets such as `.env*`, `.git/*`, selected dotfiles, editor config, and lockfiles to stronger approval handling without weakening cwd containment.
- Kept direct-write diff previews and file-history coverage aligned across create, overwrite, exact edit, multi-replace, and patch-based changes.

### Edit Engine Hardening

- Added `apply_patch` as a dedicated patch-grade edit path for multi-hunk and multi-file structural edits.
- Standardized machine-readable edit failure kinds across the edit family, including `no_match`, `multiple_matches`, `content_mismatch`, `invalid_range`, and invalid-patch cases.
- Surfaced structured recovery hints through Go tool output, IPC payloads, and the TUI so failed edits explain how to retry instead of returning flat strings.
- Made the edit ladder explicit in runtime guidance and docs so structural changes route to `apply_patch` instead of overusing exact replacement tools.
- Added post-edit diagnostics for file-mutating tools when obvious local Go or TypeScript checkers are available.

### Child-Agent Lineage

- Added stable `invocation_id` values for child runs and reused them across sync results, background launches, transcript paths, result files, and status checks.
- Extended `agent`, `agent_status`, and `agent_stop` with structured child metadata covering lifecycle, status messaging, transcript and result paths, and tool-list context.
- Updated the TUI to consume structured child metadata directly instead of inferring child state from mixed `agent_id` fields and summary strings.

### Child Lifecycle Hooks

- Aligned child stop behavior with the shared query-loop stop-control contract instead of maintaining a separate child-only completion path.
- Added optional child start hooks via the existing hook runner and injected returned context into the delegated child prompt before the first child iteration.
- Added child stop-hook blocking so local policy can prevent early completion and return an explicit reason.
- Persisted blocked-stop feedback into child transcript flow and surfaced stop-block reason and count metadata in child status updates and final results.

## User-Visible Notes

- File mutation failures now explain whether the fix is to reread context, narrow the edit, refresh line ranges, or switch to `apply_patch`.
- File mutation cards in the TUI can now show post-edit diagnostics directly after the diff preview.
- Background child agents now expose stable lineage and richer status details without forcing the user to open raw transcripts.
- Child lifecycle hooks can now inject extra local context before a child starts or prevent it from stopping early with a visible reason.

## Commits Included

- `15103d3` Harden file tool semantics
- `5f3fc6a` Escalate sensitive file approvals
- `d86f052` Split file create and overwrite tools
- `3e3688a` Classify edit failures with recovery hints
- `88ecb69` Add apply_patch edit tool
- `796066c` Clarify file edit tool ladder
- `76235bd` Run post-edit diagnostics for file tools
- `f0a8ee9` Track child invocation lineage
- `808dd13` Add child lifecycle hooks

## Validation

- `go build ./...` passed for the Go engine during the enhancement slices.
- `bun run build` passed for the TUI during the enhancement slices.
- `progress.md` now reports the full four-phase enhancement roadmap as completed.
