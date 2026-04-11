# Enhancement Research: VS Code Copilot Chat Patterns

Date: 2026-04-12

This document replaces the old mixed backlog. It is a research-only note and implementation plan seed.

## Scope

- Source repo: `microsoft/vscode-copilot-chat`
- Primary research target: `src/extension/tools`
- Primary research target: `src/extension/agents`
- For subagent orchestration, the actual control flow also extends into:
  - `src/extension/tools/node/searchSubagentTool.ts`
  - `src/extension/tools/node/executionSubagentTool.ts`
  - `src/extension/prompt/node/searchSubagentToolCallingLoop.ts`
  - `src/extension/prompt/node/executionSubagentToolCallingLoop.ts`
  - `src/extension/intents/node/toolCallingLoop.ts`

## Boundaries

- This is not a parity project.
- This is not a team/swarm/remote-agent roadmap.
- This is not an implementation task list yet.
- Only two areas are in scope:
  - file-related tools
  - subagent orchestration

## Current `gocode` baseline

`gocode` already has a stronger local baseline than the old enhancement note assumed.

- File tools already present: `file_read`, `file_write`, `file_edit`, `multi_replace_file_content`, `file_diff_preview`, `file_history`, `file_history_rewind`
- File-adjacent navigation already present: `glob`, `grep`, `go_definition`, `go_references`, `symbol_search`, `project_overview`
- Execution and batching already present: permission levels, schema validation, path resolution, ordered concurrent execution, streaming result delivery
- Child-agent surface already present: `agent`, `agent_status`, `agent_stop`

Conclusion: the best ideas to borrow are reliability and orchestration patterns, not raw tool-count parity.

## Best file-tool takeaways to adopt

### 1. Add a dedicated patch-grade edit tool

Reference files:

- `src/extension/tools/node/applyPatchTool.tsx`
- `src/extension/tools/node/applyPatch/parser.ts`
- `src/extension/prompts/node/panel/editCodePrompt2.tsx`

What is worth copying:

- A distinct tool for multi-hunk and multi-file edits
- A clear contract for when to use patch edits versus exact string replacement
- A patch format that is structured enough to validate before write time

Why it fits `gocode`:

- `file_edit` is good for exact replacements but brittle for larger dispersed edits
- `multi_replace_file_content` is still exact-match driven
- A patch tool would complement, not replace, the current edit family

Recommended direction:

- Add `apply_patch` as the large-edit path
- Keep `file_edit` for small exact replacements
- Keep `multi_replace_file_content` for repeated exact replacements

### 2. Add edit failure taxonomy and guided recovery

Reference files:

- `src/extension/tools/node/editFileToolUtils.tsx`
- `src/extension/tools/node/abstractReplaceStringTool.tsx`

What is worth copying:

- Distinct error classes for no match, multiple matches, and no-op edits
- Tool responses that tell the model what to do next instead of only failing
- A repair path that encourages reread or narrower edits rather than blind retries

Why it fits `gocode`:

- Current edit errors are accurate but mostly flat strings
- Richer failure classes would reduce retry churn and make automated recovery more reliable

Recommended direction:

- Standardize edit failure kinds across `file_edit`, `multi_replace_file_content`, and future `apply_patch`
- Return actionable recovery hints in tool output

### 3. Make file reads more chunk-aware and binary-aware

Reference files:

- `src/extension/tools/node/readFileTool.tsx`

What is worth copying:

- Explicit continuation hints when a read is partial or truncated
- A first-class chunking model for large files
- Binary handling instead of treating every file like text

Why it fits `gocode`:

- `file_read` already supports line ranges, which is good
- It still under-communicates what the model should do after a partial read
- Binary-awareness would reduce confusing output on non-text inputs

Recommended direction:

- Keep line-based reads
- Add continuation hints for large/partial reads
- Add binary detection and a safer fallback output mode

### 4. Add path-sensitive edit safety heuristics

Reference files:

- `src/extension/tools/node/editFileToolUtils.tsx`
- `src/extension/tools/node/createFileTool.tsx`

What is worth copying:

- Treating some paths as higher-risk than normal workspace files
- Consistent diff generation before confirmation
- Stronger guardrails around config, dotfiles, and user-home edits

Why it fits `gocode`:

- Current path resolution blocks cwd escape, which is necessary but not sufficient
- Risky local files should trigger stronger approval behavior than ordinary source files

Recommended direction:

- Add a risk tier for dotfiles, editor config, shell rc files, and user-home paths
- Ensure write approvals always include a stable diff preview

## Best subagent orchestration takeaways to adopt

### 1. Use named specialist subagents with fixed tool envelopes

Reference files:

- `src/extension/agents/vscode-node/agentTypes.ts`
- `src/extension/agents/vscode-node/exploreAgentProvider.ts`
- `src/extension/agents/vscode-node/planAgentProvider.ts`

What is worth copying:

- Agents declare role, tool set, and model policy separately from runtime loop code
- Explore-style agents stay read-heavy and cheap by design
- Tool access is part of the agent contract, not an afterthought

Why it fits `gocode`:

- `gocode` already has `explore` and `general-purpose`
- The next step should be stricter specialization, not more agent types

Recommended direction:

- Keep the agent set small
- Pin each subagent type to an explicit tool allowlist and model preference
- Do not add teams, swarm behavior, or remote orchestration

### 2. Propagate one stable invocation id across the whole child trajectory

Reference files:

- `src/extension/tools/node/searchSubagentTool.ts`
- `src/extension/tools/node/executionSubagentTool.ts`
- `src/extension/chatSessions/vscode-node/test/chatHistoryBuilder.spec.ts`

What is worth copying:

- One child invocation id links the parent tool call, child session, and child tool calls
- Nested tool activity stays attributable in logs and UI

Why it fits `gocode`:

- Current `agent` results are usable, but lineage should be first-class across background status, transcripts, and tool events

Recommended direction:

- Assign one invocation id at child launch
- Propagate it through transcript events, tool events, and status polling

### 3. Reuse the same loop abstraction for parent and child agents

Reference files:

- `src/extension/prompt/node/searchSubagentToolCallingLoop.ts`
- `src/extension/prompt/node/executionSubagentToolCallingLoop.ts`
- `src/extension/intents/node/toolCallingLoop.ts`

What is worth copying:

- Parent and child agents share one orchestration model
- Limits, hooks, telemetry, and prompt-building behavior stay aligned
- Child context remains isolated from parent budget accounting

Why it fits `gocode`:

- This reduces feature drift between the main agent loop and child-agent execution
- It keeps compaction, budgeting, and recovery behavior consistent

Recommended direction:

- Make child execution explicitly reuse the `internal/agent` loop contracts
- Keep child message history and token budgets isolated from the parent session

### 4. Add subagent start/stop hooks, including block-stop reasons

Reference files:

- `src/extension/intents/node/toolCallingLoop.ts`
- `src/platform/chat/common/chatHookService.ts`

What is worth copying:

- A start hook can inject additional context into the child run
- A stop hook can block completion if exit criteria are not met
- Block reasons are surfaced back into the next loop turn instead of disappearing

Why it fits `gocode`:

- This adds a policy surface without hardcoding workflow rules into the loop itself
- It gives child agents a cleaner definition of done

Recommended direction:

- Add optional subagent start and stop hooks
- Surface block reasons in transcript state and child status output
- Keep hooks local and synchronous in the first pass

### 5. Return structured child metadata, not only final text

Reference files:

- `src/extension/tools/node/searchSubagentTool.ts`
- `src/extension/tools/node/executionSubagentTool.ts`

What is worth copying:

- Child tools emit readable invocation and completion messages
- Tool metadata carries role, description, and invocation linkage
- The final textual summary is only one layer of the result

Why it fits `gocode`:

- `agent_status` and the TUI can present more useful state than running/done
- Background child work becomes debuggable without dumping full transcripts by default

Recommended direction:

- Extend child result payloads with phase, active tool, and last meaningful event
- Keep the final report concise but preserve structured metadata for the UI

## What not to copy

- agent management UI and wizard flows
- handoff buttons between agents
- teams, swarm, remote agents, or remote execution
- notebook-specific editing work
- broad tool parity for its own sake

## Recommended implementation order

1. Patch-grade file edits
   - add `apply_patch`
   - unify edit failure taxonomy
   - teach the model when to choose patch versus exact replace
2. Read and approval hardening
   - improve `file_read` continuation and binary handling
   - add risk-tiered edit approval with consistent diff previews
3. Subagent lineage
   - add invocation ids across child session, tool calls, and status APIs
   - surface structured child metadata to the TUI
4. Shared child lifecycle
   - align child execution with the main loop contracts
   - add start/stop hooks and block-stop reasons

## Likely `gocode` touch points

- `gocode/internal/tools/file_edit.go`
- `gocode/internal/tools/multi_replace_file_content.go`
- `gocode/internal/tools/file_read.go`
- `gocode/internal/tools/path_resolution.go`
- `gocode/internal/tools/validation.go`
- `gocode/internal/tools/agent.go`
- `gocode/internal/agent/loop.go`
- `gocode/internal/agent/query_stream.go`
- `gocode/internal/ipc/`
- `gocode/tui/src/`

## Decision

- Treat VS Code Copilot Chat as a source of patterns, not a parity target.
- The best imports for `gocode` are safer file-edit semantics and tighter subagent lifecycle plumbing.
- Do not start implementation from this document without picking one narrower slice first.
