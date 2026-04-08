# Go CLI Agent — Architecture & Patterns to Adopt

## Verified Feature Mapping (from conversation → actual source files)

| Your feature | Source file(s) verified |
|---|---|
| Compact conversation | `services/compact/compact.ts`, `autoCompact.ts`, `microCompact.ts` |
| Multi-model support | `utils/model/providers.ts`, `utils/model/model.ts` |
| Skill system | `skills/loadSkillsDir.ts`, `skills/bundled/` |
| Tool executor | `tools/BashTool/`, `tools/GrepTool/`, `tools/FileReadTool/` etc. |
| Web research | `tools/WebSearchTool/`, `tools/WebFetchTool/` |
| Chain of thought | `utils/thinking.ts` (`ultrathink` + adaptive thinking budget) |
| Context injection | `context.ts` (`getSystemContext`, `getGitStatus`) |
| Commands | `commands.ts`, slash-command parsing in `utils/slashCommandParsing.ts` |
| Hooks | `utils/hooks.ts`, `query/stopHooks.ts` |
| Session restore | `utils/sessionRestore.ts`, `utils/sessionStorage.ts`, `utils/plans.ts` |
| Artifact storage | `artifact/service.go`, `internal/artifact/artifacts.go` |

---

## Architecture: Ink Frontend + Go Engine (Architecture 1)

The CLI uses a **split-process architecture**: a minimal React Ink frontend (Node.js) owns the terminal and renders the TUI, while the Go engine runs as a long-lived subprocess communicating via NDJSON over stdio.

```
┌────────────────────────────────┐
│  Node.js process (main)        │
│  React Ink TUI (~700-800 LOC)  │
│    <App>              shell + NDJSON reader + event dispatch
│    <Input>            text input + slash command detection
│    <StreamOutput>     render token deltas as markdown
│    <StatusBar>        mode, cost, model name
│    <PlanPanel>        render implementation-plan artifact
│    <PermissionPrompt> yes/no/always for tool approval
│    <ToolProgress>     spinner + tool name while executing
│         │                      │
│    stdin/stdout NDJSON          │
│         │                      │
│  ┌──────▼──────────────┐       │
│  │ Go binary (child)    │       │
│  │ agentcli-engine      │       │
│  │  streams events out  │       │
│  │  reads commands in   │       │
│  └──────────────────────┘       │
└────────────────────────────────┘
```

**Why Architecture 1 (Ink parent, Go child):**
- Ink needs terminal ownership (raw mode, cursor, alternate screen buffer)
- Go subprocess starts in ~5ms, communicates cleanly over stdio pipes
- Each side is independently testable: mock NDJSON for Ink, pipe events for Go
- Future UI swap (web, electron) only requires replacing the Ink layer

**Dependency:** Requires Node.js on the user's machine. The CLI checks for `node` at startup.

---

## Go Engine Structure

```
cmd/agentcli/main.go       ← cobra entrypoint (starts engine in stdio mode)
internal/
  agent/
        loop.go                ← phase-based query state machine
        query_stream.go        ← iter.Seq2-based streaming query interface
        modes.go               ← planning/fast execution profiles
        planner.go             ← plan creation + plan enforcement before writes
        token_budget.go        ← continuation budget + diminishing returns logic
    context_inject.go      ← env context injection
  api/
    client.go              ← LLMClient interface + provider registry
    anthropic.go           ← Anthropic Messages API (native, streaming)
    openai_compat.go       ← OpenAI-compatible API (covers GPT, Qwen, GLM, DeepSeek, Mistral, etc.)
    gemini.go              ← Google Gemini API (native, streaming)
    ollama.go              ← Ollama local models (Gemma 4, Llama, etc.)
    provider_config.go     ← provider presets + model capability registry
    retry.go               ← retry/fallback policy per error class
  ipc/
    protocol.go            ← StreamEvent/ClientMessage types + NDJSON codec
    bridge.go              ← stdin reader + stdout writer for Ink ↔ Go
  tools/
    interface.go           ← Tool interface + PermissionLevel
    registry.go            ← Tool registry
        orchestration.go       ← concurrency classification + batch planning
        streaming_executor.go  ← launch read tools before full model turn completes
        budgeting.go           ← tool result size limits + disk spillover
    bash.go
    file_read.go
    file_write.go
    file_edit.go
    glob.go
    grep.go                ← rg wrapper
    web_search.go
    web_fetch.go
    git.go
  compact/
    pipeline.go            ← CompactionPipeline
    tool_truncate.go       ← strategy 1 (microcompact)
    sliding_window.go      ← strategy 2
    summarize.go           ← strategy 3 (calls LLM or local model)
    tokens.go              ← token estimation
  localmodel/
    runner.go              ← local model interface (ollama/llama.cpp)
    router.go              ← decides local vs. remote per task
  skills/
    loader.go              ← load from ~/.config/agentcli/agents/*.md & .agents/
    frontmatter.go         ← YAML frontmatter parser
  permissions/
    gating.go              ← ask/allow/deny per tool
    bash_rules.go          ← command-level rules
  cost/
    tracker.go             ← per-model token/cost/duration accounting
  artifacts/
        service.go             ← artifact storage interface + metadata model
        store.go               ← local filesystem artifact store
        types.go               ← artifact kinds, scopes, and references
        manager.go             ← plan/task/walkthrough artifact lifecycle
  hooks/
    runner.go              ← lifecycle hook execution
    types.go               ← hook payloads + responses
  session/
    store.go               ← transcript + session metadata persistence
    restore.go             ← resume conversation/todos/model state
  utils/
    tokens.go              ← token counting
    messages.go            ← message normalization for API
  config/
    config.go
```

## Ink Frontend Structure

```
tui/
  package.json             ← ink, react, ink-markdown dependencies
  tsconfig.json
  src/
    index.tsx              ← entry: spawn Go engine, wire stdio
    App.tsx                ← top-level layout + NDJSON event dispatch
    components/
      Input.tsx            ← text input + slash command detection + Tab mode toggle
      StreamOutput.tsx     ← render token deltas as streaming markdown
      StatusBar.tsx        ← mode indicator, cost, model name, token count
      PlanPanel.tsx        ← render implementation-plan artifact above output
      PermissionPrompt.tsx ← yes/no/always for tool approval requests
      ToolProgress.tsx     ← spinner + tool name while executing
      ArtifactView.tsx     ← render artifact content (task-list, walkthrough, etc.)
    hooks/
      useEngine.ts         ← spawn Go child process, manage NDJSON stream
      useEvents.ts         ← parse StreamEvents into React state
    protocol/
      types.ts             ← StreamEvent + ClientMessage type definitions (mirrors Go)
      codec.ts             ← NDJSON line parser + serializer
```

---

## Key Patterns to Adopt (Clean-Room Rewrite)

### 1. The Query Engine — streaming, cancellable, testable

The biggest architectural takeaway is not "use a loop". It is "treat the agent turn as a streaming process". TypeScript uses an async generator; the closest Go equivalent for this project should be an ADK-style `iter.Seq2` stream with `context.Context` cancellation.

```go
// internal/agent/query_stream.go
func QueryStream(ctx context.Context, req QueryRequest, deps QueryDeps) iter.Seq2[StreamEvent, error] {
    return func(yield func(StreamEvent, error) bool) {
        defer deps.Cleanup()

        state := NewQueryState(req)
        for state.ShouldContinue() {
            if err := runIteration(ctx, state, deps, yield); err != nil {
                yield(StreamEvent{}, err)
                return
            }
        }
    }
}
```

Why this matters in Go:
- The TUI, tests, and future sub-agents can all consume the same `StreamEvent` interface
- Cancellation is native: `ctx.Done()` stops iteration and triggers cleanup naturally
- Pull-based consumption is explicit, which is closer to the async-generator mental model than a raw background goroutine
- Streaming output becomes first-class instead of bolted onto a blocking request function
- This matches the shape ADK already uses in `runner.go`, which is a good sign the pattern is robust in production Go

Key insights verified in source and worth adopting:
- Context injection happens **once per user turn**, not inside the tool-call loop
- Compaction fires **before** each LLM call (both first and subsequent)
- Error recovery belongs **inside** the query engine, not in an outer `for {}` + `try/catch` equivalent
- `autoCompact.ts` triggers at `effectiveContextWindow - AUTOCOMPACT_BUFFER_TOKENS` (13,000 token buffer)

### 1A. Five phases per iteration

Each iteration should be explicit in code, not implicit in control flow:

1. **Setup** — inject fresh turn context, enforce tool result budgets, compact if nearing threshold, validate token counts
2. **Model invocation** — call the provider through a streaming interface and emit token/tool events as they arrive
3. **Recovery** — handle `prompt_too_long`, max-output exhaustion, overloaded/rate-limit errors, then retry with a defined policy
4. **Tool execution** — finish any tool calls not already launched by the streaming executor and stream their results back to the UI
5. **Continuation** — inspect stop reason, max turn count, user abort, hook stop requests, and mode policy before looping again

That phase split is what keeps retries, compaction, and tool execution from becoming a pile of ad-hoc conditionals.

### 1B. Dependency injection via `QueryDeps`

The query engine should be a state machine with injected effects:

```go
// internal/agent/loop.go
type QueryDeps struct {
    CallModel            func(context.Context, ModelRequest) (iter.Seq2[ModelEvent, error], error)
    ExecuteToolBatch     func(context.Context, ToolBatch) ([]ToolResult, error)
    CompactMessages      func(context.Context, []Message, CompactReason) ([]Message, error)
    ApplyResultBudget    func([]Message) []Message
    EmitTelemetry        func(StreamEvent)
    Cleanup              func()
    Clock                func() time.Time
}
```

This is the difference between a demo harness and a testable engine. It lets you unit test cancellation, compaction retries, tool failures, and mode switching without touching the real API.

### 1C. Planning mode and fast mode must be execution profiles with command parity

Claude Code's UX distinction is worth copying, but it should live in engine policy as well as in the prompt. Implement two execution profiles and let the Ink frontend switch them with `Tab` (which sends a `mode_toggle` message over the IPC protocol).

```go
// internal/agent/modes.go
type ExecutionMode string

const (
    ModePlan ExecutionMode = "plan"
    ModeFast ExecutionMode = "fast"
)

type ExecutionProfile struct {
    RequirePlanBeforeWrite bool
    PreferFastModel        bool
    MaxParallelReadTools   int
    ToolSummaryVerbosity   string
    ShowPlanPanel          bool
}
```

**Planning mode** should:
- create/update an explicit task plan before the first mutating action
- keep the default tool posture read-heavy first, mutate second
- surface a visible plan panel in the TUI
- ask for confirmation at major boundary crossings if permission rules require it

**Fast mode** should:
- skip mandatory upfront planning for low-risk tasks
- prefer the fastest eligible model/profile for exploration and short turns
- aggressively parallelize read-only tools
- keep streamed reasoning and summaries terse

**Ink frontend behavior**:
- `Tab` toggles `ModePlan` ↔ `ModeFast` (sends `mode_toggle` via IPC)
- the current mode is always visible in `<StatusBar>`
- toggling mode emits a `StreamEvent{Type: EventModeChanged}` so both Ink and engine stay in sync

**Command parity**:
- implement `/plan` and `/fast` as first-class slash commands that switch the same underlying mode state
- `Tab` is an Ink shortcut for those commands, not a separate mode implementation
- planning mode should couple to permission posture as well as prompting: writes require stricter confirmation and an upfront plan refresh
- fast mode should keep the same safety rules for destructive actions, but reduce ceremony for low-risk reads and exploration

This should be implemented before the UI is considered complete. If mode is only a prompt instruction, it will drift; if it is an execution profile, it becomes reliable.

### 1D. Continuation logic should consider budget and diminishing returns

The sourcecode adds a useful control that is missing from the current plan: continuation should stop when the agent is no longer adding enough value, not just when it hits a raw turn cap.

```go
// internal/agent/token_budget.go
type ContinuationTracker struct {
    ContinuationCount int
    RecentTokenDeltas []int
    MaxBudgetTokens   int
}
```

Adopt these rules:
- stop when the turn budget reaches roughly 90% of the configured ceiling
- stop when there have been at least 3 continuations and the last two added fewer than ~500 tokens each
- disable auto-continuation for sub-agents or helper tasks by default
- emit a status-line hint when the agent is continuing due to budget still being available

This will avoid long, low-yield tails where the model keeps going but stops producing meaningful progress.

---

### 2. Compaction Pipeline — `services/compact/`

Implement **three core strategies** (verified from source):

**Strategy A — Tool Result Truncation** (`microCompact.ts`)
Run first, zero API calls. Compactable tools: `FileRead`, shell tools, `Grep`, `Glob`, `WebSearch`, `WebFetch`, `FileEdit`, `FileWrite`. Truncates old tool results to `[Old tool result content cleared]`. This alone recovers 30-50% of space in code-heavy sessions.

**Strategy B — Summarization** (`compact.ts` + `prompt.ts`)
**Most valuable reference.** The compaction prompt in `services/compact/prompt.ts` is a production-tested string constant with 9 sections: Primary Request, Technical Concepts, Files/Code, Errors/Fixes, Problem Solving, All User Messages, Pending Tasks, Current Work, Optional Next Step. Adapt this format for your `summarize.go`.

**Strategy C — Partial Compaction** (`compact.ts`)
When full summarization would still overflow, use `getPartialCompactPrompt` to scope summarization to recent messages only, preserving earlier retained context intact. Prevents summary-of-summary recursion.

**Optional (advanced)** — The source also implements:
- **Selective Retention** (`compact.ts`): Score messages by importance, drop low-score ones
- **Hierarchical Compaction** (`compact.ts`): Maintain multiple memory layers (hot/warm/cold)
- **Snip compaction** (`snipCompact.ts`): a lighter pre-summary retention pass before microcompact/autocompact

Start with A+B+C, add selective retention if needed.

If you expect long interactive sessions, add a pre-summary compaction tier in phase 2:
- Tier 0: snip/retention pass over older messages
- Tier 1: microcompact old tool results
- Tier 2: context collapse — extract and store summary messages offline, replacing them with a short reference in the transcript (this is a distinct mechanism from summarization; it keeps summaries retrievable without them consuming context)
- Tier 3: full summarization/partial compaction

That ordering matters because each cheaper tier can often avoid triggering the next, more expensive one. The actual source cascade runs: snip → microcompact → context collapse → autocompact. Match that order.

**Threshold logic** (adopted from `autoCompact.ts`):
```go
const AutocompactBufferTokens = 13_000        // ~1.8% of 200k window
const WarningThresholdBufferTokens = 20_000   // warn user before hard threshold
const MaxConsecutiveFailures = 3              // circuit breaker

func autocompactThreshold(contextWindow int) int {
    return contextWindow - AutocompactBufferTokens
}
```

**Token counting** — Implement rough estimation (verified from `utils/tokenBudget.ts`):
```go
// Approximate: ~4 chars per token (cl100k encoding)
func estimateTokens(text string) int {
    return len(text) / 4
}
```
For production, integrate a library like `pkoukk/tiktoken-go` for exact counting.

### 2A. Tool result budgeting is mandatory

This is separate from compaction. Compaction handles long conversations; budgeting prevents any single tool result from polluting context.

Adopt these rules:
- every tool declares `MaxResultSizeChars`
- oversized results are written to a temp artifact/log file
- the conversation only keeps a short preview plus the saved path reference
- `ApplyResultBudget()` runs before **every** model call, not just after Bash

```go
// internal/tools/budgeting.go
type ResultBudget struct {
    MaxChars      int
    PreviewChars  int
    SpillDir      string
}
```

Real users will absolutely produce megabytes of shell output. If this is not designed in from day one, the agent will degrade under normal usage.

### 2B. Artifacts should be a first-class subsystem, not just spill files

Two different ideas are worth combining here:
- **ADK-style artifacts**: versioned, scoped storage for named outputs
- **Antigravity-style artifacts**: user-facing work products like plans, walkthroughs, and task lists

For this project, treat artifacts as both a storage primitive and a product primitive.

**Storage layer**:
```go
// internal/artifacts/service.go
type Service interface {
    Save(context.Context, SaveRequest) (ArtifactVersion, error)
    Load(context.Context, LoadRequest) (Artifact, error)
    List(context.Context, ListRequest) ([]ArtifactRef, error)
    Delete(context.Context, DeleteRequest) error
    Versions(context.Context, VersionsRequest) ([]ArtifactVersion, error)
}
```

**Product layer artifact kinds**:
- `task-list`
- `implementation-plan`
- `walkthrough`
- `tool-log`
- `search-report`
- `diff-preview`
- `diagram`
- `screenshot`
- `codegen-output`
- `compact-summary`
- `knowledge-item`

**Scopes**:
- `session`: outputs tied to the current run
- `user`: durable knowledge and reusable artifacts across sessions

**Key rule**:
- oversized tool output should spill into artifacts by default, but artifacts are not limited to spillover; plans, walkthroughs, and reviewable work products should also be artifact-backed

This is one of the strongest additions you can make for agentic coding because it keeps context thin while preserving the agent's actual work products.

### 2C. Artifact UX should mirror planning and async work

Antigravity's stronger idea is that artifacts are the agent's async communication layer. Copy that behavior.

Recommended artifact-driven UX:
- planning mode produces a visible `implementation-plan` artifact before major writes
- task progress is mirrored into a live `task-list` artifact
- completed work emits a `walkthrough` artifact summarizing changes, validation, and next steps
- browser or UI work can attach screenshots/recordings as artifacts instead of bloating chat
- important artifacts can request review/feedback before the engine proceeds

This is a better model than treating plans and summaries as throwaway assistant text.

---

### 3. Bash Validation — `tools/BashTool/bashSecurity.ts` + `destructiveCommandWarning.ts`

Two separate concerns, both worth adopting:

**Security validation** (inject into `bash.go`):
- Blocklist of ZSH dangerous commands: `zmodload`, `emulate`, `sysopen`, `syswrite`, `zpty`, `ztcp`, `zsocket`, `zf_rm`, `zf_mv`, `zf_chmod`
- Command substitution patterns to reject: `$()`, `${}`, `<()`, `>()`, `=cmd` (Zsh equals expansion)
- Block `IFS` injection, `HEREDOC_IN_SUBSTITUTION`, unicode whitespace in commands

**Destructive command warnings** (UI hint only, do not block):
- Git: `reset --hard`, `push --force`, `clean -f`, `checkout .`, `commit --amend`, `--no-verify`
- Files: `rm -rf`, `rm -f`
- DB: `DROP TABLE`, `TRUNCATE`, unbounded `DELETE FROM`
- Infra: `kubectl delete`, `terraform destroy`

Adapt the regex patterns from `destructiveCommandWarning.ts` — they are precise and production-tested.

---

### 4. Tool Permission System — `Tool.ts`

The source uses three rule lists, not a simple level enum:

```go
// internal/permissions/gating.go
type PermissionContext struct {
    Mode             string // "default", "bypassPermissions", "autoApprove"
    AlwaysAllowRules []Rule
    AlwaysDenyRules  []Rule
    AlwaysAskRules   []Rule
}
```

Auto-approve reads, prompt for writes/executes — but the actual classifier (`bashPermissions.ts`) checks command-level rules, not just tool-level. **Caveat:** The per-command bash classifier in the public source is a stub (it's Anthropic-internal). For your Go port: implement it as **regex-based per-command rules** for bash (e.g. `Bash(git diff:*)` allow, `Bash(rm:*)` ask). This is the practical equivalent you can build from scratch — match parsed command names against an allowlist/denylist rather than relying on a classifier.

### 4A. Tool orchestration should classify concurrency dynamically per invocation

The actual source does not use a static enum. Each tool implements `IsConcurrencySafe(input)` and the executor evaluates it **per call** with parsed input. This is critical because `Bash(git diff)` is parallel-safe but `Bash(git push)` is not — a static classification cannot capture this.

```go
// internal/tools/orchestration.go

// Each tool implements this on its Tool interface
type ConcurrencyClassifier interface {
    IsConcurrencySafe(input ToolInput) bool
}

// The executor partitions consecutive tool calls into batches:
// - Group consecutive concurrency-safe calls into one concurrent batch
// - Each non-safe call gets its own serial batch
// - Max concurrency is configurable (default 10, env AGENTCLI_MAX_TOOL_CONCURRENCY)
```

Default concurrency safety:
- `Glob`, `Grep`, `FileRead`, `WebSearch`, `WebFetch` → always safe
- `Bash` → safe only if parsed command matches a read-only allowlist (`git diff`, `git status`, `git log`, `ls`, `cat`, `find`, `rg`, `wc`, `head`, `tail`, etc.)
- `FileWrite`, `FileEdit` → never safe

The executor should partition tool calls into batches, run safe reads concurrently, and preserve original result order when streaming results back. That gives you latency reduction without file corruption risk.

### 4B. Streaming tool execution should overlap model generation

Do not wait for the model to finish its full response before starting tools. As soon as a tool call's JSON is complete in the stream, enqueue it for execution.

Benefits:
- hides 1-5 seconds of tool latency in multi-tool turns
- makes the UI feel continuously alive
- reduces end-to-end turn time on search-heavy workflows

Failure handling requirements:
- a failed tool in a parallel batch cancels only its siblings via a per-batch `context.CancelFunc` (the source calls this `siblingAbortController`), not the whole query
- if the stream collapses and you fall back to non-streaming, mark unfinished queued tools with synthetic error results
- result ordering to the user/model stays deterministic even if execution order is not

---

### 5. Skill System — `skills/loadSkillsDir.ts`

Skills in the source are **markdown files with YAML frontmatter** loaded from:
- `~/.config/agentcli/agents/` (user-global, platform-generic)
- `.agents/` in project root (project-local)

Frontmatter fields that matter:
```yaml
---
name: git-workflow
description: Git operations and PR workflow guidance
allowed-tools: Bash, FileRead  # optional tool restriction
argument-hint: branch name      # optional
---
Your skill prompt content here...
```

The skill is injected as a system prompt section when invoked via `/skill-name` command or auto-detected on startup. Implement the same two-directory discovery pattern in `skills/loader.go`.

### 5A. Slash commands should be a real control plane

The sourcecode is command-heavy in a useful way. Do not treat slash commands as a thin UI nicety. They should be a control plane for the session.

Minimum command set worth adopting in the Go CLI MVP:
- `/plan` and `/fast` to switch execution mode
- `/compact` to force compaction
- `/model` to switch model/provider at runtime
- `/resume` to restore a previous session
- `/permissions` to inspect or change permission posture
- `/cost` or `/usage` to inspect budget consumption

This aligns the Ink frontend, headless `--stdio` mode, and future automation around one interaction model.

### 5B. Cost tracking is a required subsystem, not just a UI command

The source has a full cost tracker (`cost-tracker.ts`, `costHook.ts`) that feeds `/cost` and session metadata. Without it, `/cost` has nothing to display, local model savings are invisible, and session-level cost attribution is impossible.

```go
// internal/cost/tracker.go
type CostTracker struct {
    mu                sync.Mutex
    TotalCostUSD      float64
    TotalInputTokens  int
    TotalOutputTokens int
    TotalCacheReadTokens    int
    TotalCacheCreationTokens int
    TotalAPIDuration  time.Duration
    TotalToolDuration time.Duration
    TotalLinesAdded   int
    TotalLinesRemoved int
    ModelUsage        map[string]*ModelUsageEntry // per-model breakdown
}

type ModelUsageEntry struct {
    ModelName      string
    InputTokens    int
    OutputTokens   int
    CacheReadTokens int
    CostUSD        float64
    CallCount      int
    ContextWindow  int
}
```

Requirements:
- updated after every model call and tool execution
- persisted with session metadata for resume
- surfaced in `<StatusBar>` (running total) and via `/cost` command
- local model calls tracked separately so savings are visible
- saved on session end for historical usage reporting

### 5B. Hooks are worth building early

One of the higher-leverage additions from sourcecode is the hook lifecycle. This is a clean extension point for local policy without bloating the main engine.

Suggested MVP hooks:
- `session_start`, `session_end`
- `pre_tool_use`, `post_tool_use`
- `permission_request`
- `pre_compact`, `post_compact`
- `stop`, `stop_failure`

Design notes:
- shell-script hooks in `~/.config/agentcli/hooks/`
- JSON or plain-text responses
- permission hooks can recommend allow/deny/ask, but the core engine remains authoritative
- stop hooks can attach messages that feed back into the next loop iteration

This is one of the few extensibility features that pays off immediately for power users.

### 5C. Artifacts and hooks should integrate cleanly

The hook system should be able to create or enrich artifacts.

Useful examples:
- `post_tool_use` saves a large tool log as a `tool-log` artifact
- `post_compact` saves the generated summary as a `compact-summary` artifact
- `stop` emits a `walkthrough` artifact when a task finishes
- `permission_request` can attach a `diff-preview` artifact for review before approval

That gives you durable outputs without having to overload the conversation transcript.

---

### 6. Context Injection — `context.ts`

Two layers, different cache strategies:

**Layer 1 — System context** (cached once per session via `getSystemContext`):
```go
// internal/agent/context_inject.go
type SystemContext struct {
    MainBranch string // default branch for PRs — stable per session
    GitUser    string // git config user.name — stable per session
}
```

**Layer 2 — Environment context** (refreshed every user turn):
```go
type TurnContext struct {
    CurrentDir    string // pwd — may change via cd in bash tool
    GitBranch     string // current branch — may change via checkout
    GitStatus     string // git status --short (truncated at 2000 chars)
    RecentLog     string // git log --oneline -n 5
}
```

The `getUserContext()` also loads AGENTS.md-style config files from ancestor directories up to home (~/.config/agentcli/AGENTS.md). These are loaded once at startup and on `/refresh` command.

---

### 7. Multi-Model Support — universal provider architecture

The source only supports Claude (Anthropic, Bedrock, Vertex). Your Go build should support **all major providers** from day one via a three-client architecture:

**Key insight:** Most providers now expose OpenAI-compatible chat/completions endpoints. You only need native clients for Anthropic (different API format) and Gemini (different API format + grounding). Everything else goes through `OpenAICompatClient`.

```go
// internal/api/client.go
type LLMClient interface {
    // Stream returns model events as they arrive (tokens, tool calls, stop reasons)
    Stream(ctx context.Context, req ModelRequest) (iter.Seq2[ModelEvent, error], error)
    // ModelID returns the active model identifier
    ModelID() string
    // Capabilities reports what this model supports (tool use, extended thinking, vision, etc.)
    Capabilities() ModelCapabilities
}

type ModelCapabilities struct {
    SupportsToolUse         bool
    SupportsExtendedThinking bool
    SupportsVision          bool
    SupportsJsonMode        bool
    MaxContextWindow        int
    MaxOutputTokens         int
}
```

**Three concrete clients cover all providers:**

| Client | File | Covers | API Format |
|---|---|---|---|
| `AnthropicClient` | `anthropic.go` | Claude (direct, Bedrock, Vertex) | Anthropic Messages API |
| `GeminiClient` | `gemini.go` | Gemini 2.5 Pro/Flash, Gemini 2.0 | Google Generative AI API |
| `OpenAICompatClient` | `openai_compat.go` | GPT-4o/o3, Qwen 3, GLM-4, DeepSeek V3/R1, Mistral Large, Grok, Llama via Groq/Together, **any OpenAI-compatible endpoint** | OpenAI Chat Completions API |

Plus `OllamaClient` in `ollama.go` for local models (Gemma 4 E4B, Llama 3, Qwen local, etc.) — used for both internal tasks and as a primary model if the user chooses.

**Provider presets** (in `provider_config.go`):

```go
// internal/api/provider_config.go
type ProviderPreset struct {
    Name          string            // "openai", "anthropic", "gemini", "deepseek", "qwen", etc.
    ClientType    ClientType        // AnthropicAPI, GeminiAPI, OpenAICompatAPI, OllamaAPI
    BaseURL       string            // default endpoint
    EnvKeyVar     string            // env var for API key (e.g. "OPENAI_API_KEY")
    DefaultModel  string            // e.g. "gpt-4o", "claude-sonnet-4-20250514", "gemini-2.5-pro"
    Capabilities  ModelCapabilities // model-specific caps
}

var Presets = map[string]ProviderPreset{
    "anthropic": {
        ClientType: AnthropicAPI,
        BaseURL:    "https://api.anthropic.com",
        EnvKeyVar:  "ANTHROPIC_API_KEY",
        DefaultModel: "claude-sonnet-4-20250514",
    },
    "openai": {
        ClientType: OpenAICompatAPI,
        BaseURL:    "https://api.openai.com/v1",
        EnvKeyVar:  "OPENAI_API_KEY",
        DefaultModel: "gpt-4o",
    },
    "gemini": {
        ClientType: GeminiAPI,
        BaseURL:    "https://generativelanguage.googleapis.com/v1beta",
        EnvKeyVar:  "GEMINI_API_KEY",
        DefaultModel: "gemini-2.5-pro",
    },
    "deepseek": {
        ClientType: OpenAICompatAPI,
        BaseURL:    "https://api.deepseek.com/v1",
        EnvKeyVar:  "DEEPSEEK_API_KEY",
        DefaultModel: "deepseek-chat",
    },
    "qwen": {
        ClientType: OpenAICompatAPI,
        BaseURL:    "https://dashscope.aliyuncs.com/compatible-mode/v1",
        EnvKeyVar:  "DASHSCOPE_API_KEY",
        DefaultModel: "qwen3-235b-a22b",
    },
    "glm": {
        ClientType: OpenAICompatAPI,
        BaseURL:    "https://open.bigmodel.cn/api/paas/v4",
        EnvKeyVar:  "GLM_API_KEY",
        DefaultModel: "glm-4-plus",
    },
    "mistral": {
        ClientType: OpenAICompatAPI,
        BaseURL:    "https://api.mistral.ai/v1",
        EnvKeyVar:  "MISTRAL_API_KEY",
        DefaultModel: "mistral-large-latest",
    },
    "groq": {
        ClientType: OpenAICompatAPI,
        BaseURL:    "https://api.groq.com/openai/v1",
        EnvKeyVar:  "GROQ_API_KEY",
        DefaultModel: "llama-4-maverick-17b-128e",
    },
    "ollama": {
        ClientType: OllamaAPI,
        BaseURL:    "http://localhost:11434",
        DefaultModel: "gemma4-e4b",
    },
    // Users can add custom presets in config
}
```

**Model selection priority:** `--model provider/model` flag → `AGENTCLI_MODEL` env → config file → default (anthropic/claude-sonnet-4-20250514)

**Model string format:** `provider/model-name` (e.g. `openai/gpt-4o`, `deepseek/deepseek-chat`, `ollama/gemma4-e4b`, `gemini/gemini-2.5-pro`). If no provider prefix, infer from model name or use default provider.

**Custom endpoint support:** For self-hosted or proxy setups:
```bash
# Any OpenAI-compatible endpoint
AGENTCLI_BASE_URL=https://my-proxy.example.com/v1 \
AGENTCLI_API_KEY=sk-... \
agentcli --model custom/my-model
```

**Runtime switching:** `/model deepseek/deepseek-chat` switches the active model mid-session without restart. The engine re-initializes the client and emits `EventModeChanged` with the new model info.

**Capability-aware behavior:**
- If model lacks `SupportsToolUse`, fall back to prompt-based tool invocation (JSON in system prompt) — but warn the user this is degraded
- If model lacks `SupportsExtendedThinking`, skip `ultrathink` even if user requests it
- If model has a smaller context window, adjust autocompact threshold accordingly
- Tool result budgets should scale with `MaxOutputTokens`

---

### 8. Chain of Thought — `utils/thinking.ts`

Two mechanisms verified in source:

1. **Extended thinking** — API-level `budgetTokens` parameter, triggered by `ultrathink` keyword in user input
2. **System prompt CoT enforcement** — the main prompt explicitly instructs the model to "think before acting, verify assumptions with a tool call before writing code, prefer reading files over assuming content"

For multi-model support (non-Claude models don't have extended thinking API), implement option 2 only: a system prompt section that enforces CoT reasoning regardless of provider.

---

### 9. Local On-Device Model for Token Savings — `getSmallFastModel()` pattern

**Source precedent:** The source already uses a two-tier model strategy. `getSmallFastModel()` (Haiku) is called for cheap internal tasks that don't need the main model's full reasoning. Verified uses:

| Internal Task | Source file | Why it's cheap |
|---|---|---|
| Token counting/estimation | `services/tokenEstimation.ts` | Short input, numeric output |
| API key verification | `services/api/claude.ts` | 1-token response, throwaway |
| Away/session summary | `services/awaySummary.ts` | 30-message window, short output |
| Compaction summarization | `services/compact/compact.ts` | Long input but structured output |
| Bash command classification | `bashPermissions.ts` | Short input, boolean-ish output |

**Your advantage:** Replace all of these with a local on-device model like **Gemma 4 E4B** — zero API cost.

#### Where to apply Gemma 4 E4B in your Go project

**Tier 1 — High-value local tasks (implement first):**

| Task | File | Savings | Feasibility |
|---|---|---|---|
| Compaction summarization | `compact/summarize.go` | **Highest** — every compaction currently costs 1 API call | Good — structured prompt, Gemma handles summarization well |
| Selective retention scoring | `compact/pipeline.go` | High — scores each message for importance | Excellent — classification task, perfect for small models |
| Session title generation | `agent/loop.go` | Medium — 1 call per session | Excellent — trivial task |

**Tier 2 — Medium-value local tasks:**

| Task | File | Savings | Feasibility |
|---|---|---|---|
| User intent detection | `agent/loop.go` | Medium — enhances context | Good — frustration/urgency/continuation detection |
| Bash command risk scoring | `permissions/bash_rules.go` | Medium — 1 call per bash execution | Good — classification into safe/risky/dangerous |
| Context relevance filtering | `agent/context_inject.go` | Medium — reduces what gets injected | Moderate — needs to understand code semantics |

**Tier 3 — Lower-value but still useful:**

| Task | File | Savings | Feasibility |
|---|---|---|---|
| Tool result summarization before truncation | `compact/tool_truncate.go` | Low-medium — summarize before clearing | Good — replaces blind truncation with smart truncation |
| Commit message generation | `tools/git.go` | Low — occasional | Excellent — Gemma handles this well |

#### Architecture: Model Router

```go
// internal/localmodel/router.go
type TaskType int

const (
    TaskCompaction    TaskType = iota  // → prefer local
    TaskScoring                        // → prefer local
    TaskTitleGen                       // → prefer local
    TaskIntentDetect                   // → prefer local
    TaskMainReasoning                  // → always remote (main model)
)

type ModelRouter struct {
    localModel  LocalModel     // Gemma 4 E4B via ollama
    remoteModel LLMClient      // Claude/OpenAI API
    localAvail  bool           // is local model running?
}

func (r *ModelRouter) Route(task TaskType, messages []Message) (LLMClient, error) {
    // Fall back to remote if local model unavailable
    if !r.localAvail {
        return r.remoteModel, nil
    }
    switch task {
    case TaskCompaction, TaskScoring, TaskTitleGen, TaskIntentDetect:
        return r.localModel, nil
    default:
        return r.remoteModel, nil
    }
}
```

#### Local Model Integration via Ollama

```go
// internal/localmodel/runner.go
type LocalModel struct {
    baseURL   string // default: http://localhost:11434
    modelName string // default: gemma4-e4b
}

func (m *LocalModel) Query(prompt string, maxTokens int) (string, error) {
    // POST to Ollama /api/generate endpoint
    // Same interface as remote LLMClient but no streaming needed
}

func DetectLocalModel() (*LocalModel, bool) {
    // Check if ollama is running: GET http://localhost:11434/api/tags
    // Look for gemma4-e4b or similar small model
    // Return (nil, false) if unavailable — graceful fallback
}
```

#### Token savings estimate

For a typical 2-hour coding session with 5 compaction cycles:
- **Without local model:** ~5 summarization API calls × ~4k output tokens = ~20k tokens ($0.30)
- **With Gemma 4 E4B:** 0 API tokens for summarization, scoring, titles = **~$0 for internal tasks**
- **Over a month (20 work days):** saves ~400k tokens (~$6) per developer

The real value is not just cost — it's **latency**. Local inference on Gemma 4 E4B (~4B params) runs in 1-3 seconds on Apple Silicon vs. 3-8 seconds for an API roundtrip. Compaction feels instant.

#### Fallback behavior
Always graceful: if `ollama` is not running or the model isn't pulled, fall back to the remote API silently. The user should never be blocked by the local model being unavailable.

---

### 10. Ripgrep Integration — `utils/ripgrep.ts`

The source uses a priority chain: system `rg` → bundled vendor binary → embedded. For Go, simplify:
1. Check if `rg` is on PATH via `exec.LookPath`
2. Fall back to `grep -r` if not found

This is your `grep.go` tool — do not bundle ripgrep, just shell out to it.

### 11. Planning UX details to copy directly

The feature ask here is concrete enough to lock into the plan now:

- `Tab` switches between **Plan** and **Fast** mode in the Ink `<Input>` component
- Plan mode shows a `<PlanPanel>` above streamed output
- Fast mode hides the `<PlanPanel>` by default and prioritizes immediate execution
- the selected mode persists for the session until toggled again
- `/plan` explicitly asks the agent to refresh the current plan even while in Fast mode
- `/fast` and `/plan-mode` should exist as command aliases so keyboard and command workflows both work
- the persisted session restores the last active mode so resume behavior matches the prior session
- the current implementation plan should also exist as an artifact that can be reviewed, revised, and resumed

This is worth building even in MVP because it changes how the agent behaves, not just how it looks.

### 12. Session persistence should restore more than messages

The source snapshot suggests a more complete resume model than a plain transcript replay. That is worth adopting in a simplified form.

Recommended restoration layers:
1. conversation transcript/messages
2. todo/plan state
3. model override and active execution mode
4. session metadata like cwd, branch, and cost counters
5. artifact references for plans, walkthroughs, logs, and compact summaries

For the Go build, you do not need the full attribution/file-history machinery immediately. But you should persist enough state that `/resume` feels like a true continuation, not a chat import.

### 13. Suggested artifact storage layout

Keep the first version local and simple.

```text
.agentcli/
    sessions/
        <session-id>/
            transcript.ndjson
            artifacts/
                tool-log/
                implementation-plan/
                walkthrough/
                compact-summary/
    users/
        <user-id>/
            artifacts/
                knowledge-item/
                repo-summary/
```

Each artifact should include:
- stable id
- artifact kind
- scope
- mime type
- source tool or producer
- created time
- version
- metadata map
- content path or blob handle

This is enough for CLI-first workflows now and a richer UI later.

### 14. IPC Protocol — the stability contract between Ink and Go

The NDJSON protocol over stdio is the single most important interface in the system. If this is right, swapping Ink for a web UI or a different terminal renderer is mechanical.

**Go → Ink (stdout): `StreamEvent`**

Each line is a JSON object with a `type` discriminator:

```go
// internal/ipc/protocol.go
type StreamEvent struct {
    Type    EventType       `json:"type"`
    Payload json.RawMessage `json:"payload,omitempty"`
}

type EventType string

const (
    // Model output
    EventTokenDelta       EventType = "token_delta"        // {"text": "..."}
    EventThinkingDelta    EventType = "thinking_delta"     // {"text": "..."}
    EventTurnComplete     EventType = "turn_complete"      // {"stop_reason": "end_turn"}

    // Tool lifecycle
    EventToolStart        EventType = "tool_start"         // {"tool_id": "...", "name": "Bash", "input": "..."}
    EventToolProgress     EventType = "tool_progress"      // {"tool_id": "...", "bytes_read": 1024}
    EventToolResult       EventType = "tool_result"        // {"tool_id": "...", "output": "...", "truncated": false}
    EventToolError        EventType = "tool_error"         // {"tool_id": "...", "error": "..."}

    // Permission
    EventPermissionRequest EventType = "permission_request" // {"request_id": "...", "tool": "Bash", "command": "rm -rf node_modules", "risk": "destructive"}

    // Session state
    EventModeChanged      EventType = "mode_changed"       // {"mode": "plan"}
    EventCostUpdate       EventType = "cost_update"        // {"total_usd": 0.12, "input_tokens": 5000, ...}
    EventCompactStart     EventType = "compact_start"      // {"strategy": "summarize", "tokens_before": 180000}
    EventCompactEnd       EventType = "compact_end"        // {"tokens_after": 45000}

    // Artifacts
    EventArtifactCreated  EventType = "artifact_created"   // {"id": "...", "kind": "implementation-plan", "title": "..."}
    EventArtifactUpdated  EventType = "artifact_updated"   // {"id": "...", "content": "..."}

    // Engine status
    EventReady            EventType = "ready"              // {} — engine initialized
    EventError            EventType = "error"              // {"message": "...", "recoverable": true}
    EventSessionRestored  EventType = "session_restored"   // {"session_id": "...", "mode": "plan", ...}
)
```

**Ink → Go (stdin): `ClientMessage`**

```go
type ClientMessage struct {
    Type    ClientMessageType `json:"type"`
    Payload json.RawMessage   `json:"payload,omitempty"`
}

type ClientMessageType string

const (
    MsgUserInput          ClientMessageType = "user_input"          // {"text": "..."}
    MsgSlashCommand       ClientMessageType = "slash_command"       // {"command": "plan", "args": "..."}
    MsgPermissionResponse ClientMessageType = "permission_response" // {"request_id": "...", "decision": "allow"|"deny"|"always_allow"}
    MsgCancel             ClientMessageType = "cancel"              // {} — Ctrl+C
    MsgModeToggle         ClientMessageType = "mode_toggle"         // {} — Tab key
    MsgShutdown           ClientMessageType = "shutdown"            // {} — clean exit
)
```

**Design rules:**
- One JSON object per line, no multi-line payloads
- Go writes to stdout only; stderr is reserved for fatal panics and debug logging
- Event ordering is authoritative: Ink renders events in the order received
- Permission requests are blocking: Go pauses the query loop until Ink sends a `permission_response`
- `cancel` triggers `context.CancelFunc` in Go — the engine drains in-flight tools and emits `turn_complete`
- The protocol is versioned via a `ready` event payload: `{"protocol_version": 1}` — Ink checks this on startup

**Testing the protocol independently:**
- Go engine: `agentcli-engine --stdio` reads/writes NDJSON directly — pipe test events with `echo '{...}' | agentcli-engine --stdio`
- Ink frontend: `MOCK_ENGINE=1 npm start` reads events from a fixture file and renders them — no Go binary needed

---

## What NOT to Build (verified dead weight for your scope)

| Feature | Source location | Why skip |
|---|---|---|
| KAIROS background agent | `feature('KAIROS')` gated everywhere | You said no |
| Coordinator/Subagent mode | `coordinator/` dir, `feature('COORDINATOR_MODE')` | You said no |
| Undercover mode | `utils/undercover.ts` | Anthropic-internal only |
| BUDDY tamagotchi | `buddy/` dir | Not minimal |
| MCP server orchestration | `services/mcp/` | You said no |
| LSP integration | `tools/LSPTool/` | Not in your feature list |
| Full Ink component library | `ink.ts`, all `*.tsx` files | Write minimal ~700-800 LOC Ink frontend from scratch |
| 3,167-line `print.ts` | `cli/print.ts` | SDK mode only |

---

## Effort Estimate (solo Go developer)

| Component | Source reference | Effort |
|---|---|---|
| LLM API client + streaming | `services/api/claude.ts` | 3–5 days |
| Dispatch loop + context injection | `QueryEngine.ts`, `context.ts` | 3–5 days |
| Tool executor (7 tools) | `tools/BashTool/`, `FileReadTool/`, etc. | 6–8 days |
| Bash validation blocklists | `bashSecurity.ts`, `destructiveCommandWarning.ts` | 2–3 days |
| Permission gating | `Tool.ts`, `bashPermissions.ts` | 2–3 days |
| Compaction pipeline (3 strategies) | `compact/` dir | 1–2 weeks |
| Compaction prompt (adapt from reference) | `services/compact/prompt.ts` | 1 day |
| Skill system (markdown loader) | `skills/loadSkillsDir.ts` | 3–4 days |
| IPC protocol + NDJSON bridge | New — `internal/ipc/` | 2–3 days |
| Ink TUI (~700-800 LOC) | Minimal from scratch, source for UX reference | 1 week |
| Token counting + message normalization | `utils/tokens.ts`, `utils/messages.ts` | 2 days |
| Error handling + retry logic | `services/api/errors.ts` | 1–2 days |
| Artifact subsystem | `artifact/service.go`, `internal/artifact/artifacts.go` | 3–5 days |
| Local model router (Gemma 4 E4B) | `getSmallFastModel()` pattern | 3–4 days *(phase 2)* |
| Multi-model provider interface (Anthropic + Gemini + OpenAI-compat + Ollama) | `utils/model/providers.ts` | 3–5 days *(phase 2)* |
| Commands + config | `commands.ts` | 3–4 days |
| Hooks system | `utils/hooks.ts`, `query/stopHooks.ts` | 2–3 days |
| Session persistence + resume | `utils/sessionRestore.ts`, `utils/sessionStorage.ts` | 3–4 days |
| Cost tracker | `cost-tracker.ts`, `costHook.ts` | 1–2 days |
| **MVP (Generic provider-agnostic)** | | **~8–10 weeks solo** |
| **Phase 2a (local model)** | Ollama + Gemma 4 E4B for internal tasks | **+3–4 days** |
| **Phase 2b (multi-model support)** | Anthropic + Gemini + OpenAI-compat (GPT, Qwen, GLM, DeepSeek, Mistral, Groq) + Ollama | **+1–2 weeks** |

---

## Recommended Build Order

### Week 1–2 (MVP Core)
1. **`internal/api/`** — Claude streaming client with tool use + retry wrapper (day 1–5)
2. **`internal/agent/query_stream.go`** — `iter.Seq2`-based event stream with mocked tools (day 5–7)  
    **Checkpoint:** Can stream tokens, cancel cleanly, and mock-execute tools ✓
3. **`internal/tools/`** — real tool execution for 5 essential tools plus concurrency classification (day 7–11):
   - `bash.go`, `file_read.go`, `file_write.go`, `file_edit.go`, `glob.go`
   - Add `git.go` and `web_search.go` by end of week 2
4. **`internal/utils/tokens.go`** — token counting + message normalization (day 11–12)
5. **`internal/tools/budgeting.go`** — per-tool result budgets + spillover files (day 12)
6. **`internal/artifacts/`** — artifact service, local storage, and references for spillover + plans (day 12–14)

### Week 3 (Security & Awareness)
7. **`internal/permissions/`** — permission gating + bash validation (day 14–17)
8. **`internal/agent/context_inject.go`** — git status + working directory injection (day 17–18)
9. **`internal/cost/tracker.go`** — wire into API client and tool executor from day one; accumulates per-model tokens, cost, and duration (day 18–19)

### Week 4–5 (Compaction — your differentiator)
9. **`internal/compact/`** — full 3-strategy pipeline (day 18–30)
   - Strategy A (tool truncation): day 1
   - Strategy B (summarization): day 2–3 (mostly adapted prompt)
   - Strategy C (partial): day 4
   - Threshold logic + tests: day 5

### Week 6 (Interface & Configuration)
10. **`internal/ipc/`** — NDJSON protocol types + stdio bridge (day 30–32)
    - Define `StreamEvent` and `ClientMessage` types in Go
    - Implement `bridge.go`: stdin line reader → `ClientMessage`, `StreamEvent` → stdout writer
    - Add `--stdio` flag to `cmd/agentcli/main.go` for engine-only mode
    - **Checkpoint:** Can pipe `echo '{"type":"user_input","payload":{"text":"hello"}}' | agentcli-engine --stdio` and get streaming NDJSON back ✓
11. **`tui/`** — Ink frontend (~700-800 LOC) wired to Go engine via NDJSON (day 32–38)
    - `<App>` → spawn Go child, NDJSON reader/writer
    - `<Input>` → text input + slash command detection + `Tab` mode toggle
    - `<StreamOutput>` → render token deltas as streaming markdown
    - `<StatusBar>` → mode indicator, cost, model name
    - `<PlanPanel>` → render implementation-plan artifact
    - `<PermissionPrompt>` → yes/no/always for tool approval
    - `<ToolProgress>` → spinner + tool name while executing
12. **`internal/agent/modes.go` + `planner.go`** — planning/fast execution profiles + plan refresh command (day 38–39)
13. **`internal/skills/`** — markdown skill loader (day 39–41)  
   - Loads `~/.config/agentcli/agents/*.md` and `.agents/*.md` with YAML frontmatter
14. **`internal/config/`** — CLI flags + AGENTS.md config file parsing (day 41–43)
15. **`internal/hooks/`** — lifecycle hooks for tool use, permission requests, compaction, and artifact emission (day 43–45)
16. **`internal/session/`** — transcript persistence + resume of mode/model/todo/artifact state (day 45–47)

### Phase 2a (Post-MVP, ~3–4 days) — Local Model for Token Savings
17. **`internal/localmodel/runner.go`** — Ollama/Gemma 4 E4B integration
    - Auto-detect if ollama is running at startup
    - Implement `LocalModel` satisfying `LLMClient` interface
18. **`internal/localmodel/router.go`** — Task-based model router
    - Route compaction, scoring, title gen → local model
    - Route main reasoning, tool execution → remote API
    - Graceful fallback if local unavailable
19. **Wire local model into `compact/summarize.go`** — biggest savings  
    **Checkpoint:** Compaction runs offline with zero API cost ✓

### Phase 2b (Post-MVP, ~1–2 weeks) — Universal Multi-Model Support
20. **`internal/api/client.go`** — finalize `LLMClient` interface with `Capabilities()` (day 1)
21. **`internal/api/openai_compat.go`** — OpenAI-compatible client (day 2–4)
    - Covers: GPT-4o/o3, Qwen 3, GLM-4, DeepSeek V3/R1, Mistral Large, Grok, Groq, Together, any compatible endpoint
    - Streaming SSE parser for chat/completions
    - Tool use via OpenAI function calling format
22. **`internal/api/gemini.go`** — Google Gemini native client (day 5–7)
    - Streaming via `generateContent` with `streamGenerateContent`
    - Tool use via Gemini function declarations format
23. **`internal/api/provider_config.go`** — provider presets + model capability registry (day 8)
    - Built-in presets for all major providers
    - Custom endpoint support via env vars and config file
    - `/model provider/model-name` runtime switching
24. **Capability-aware engine adjustments** (day 9–10)
    - Adjust autocompact threshold per model's context window
    - Skip extended thinking for models that don't support it
    - Warn on models without native tool use

**Checkpoint:** Can switch between Claude, GPT-4o, Gemini 2.5 Pro, DeepSeek, and Qwen mid-session with `/model` ✓

**After week 1:** You have a working streaming chat loop that can read/write files.  
**After week 5:** Full agent with compaction, budgeting, and orchestration — production-grade feature set.  
**After week 6:** Ink TUI rendering Go engine output via NDJSON — full interactive experience.

---

## Critical Architecture Notes

### Message Normalization (`utils/messages.ts`)
The source normalizes messages before every API call:
- Consolidate consecutive assistant/user messages
- Ensure tool results are paired with tool calls
- Strip trailing whitespace

This is essential for correct API semantics — don't skip it.

### Error Handling (`services/api/errors.ts`)
The source categorizes API errors:
- `prompt_too_long` → trigger compaction immediately
- `rate_limit` → exponential backoff (1s, 2s, 4s)
- `overloaded` → retry with delay

Start with simple retry logic: 3 attempts with exponential backoff.

### Context Injection — The Secret Sauce
Two-layer injection: session-stable data (main branch, git user) cached once + turn-level data (branch, status, cwd) refreshed on each user message. See section 6 for the full struct breakdown.

This is why the source feels "aware" of your project state. Most agents inject once at startup and never update. The key insight: refresh the volatile parts (status, branch, cwd) every turn.

### Key Reference Assets

**1. Compaction prompt** (`services/compact/prompt.ts`)  
Adapt the 9-section summary format — it's production-tested.

**2. ZSH dangerous commands** (`tools/BashTool/bashSecurity.ts`)  
```
zmodload, emulate, sysopen, sysread, syswrite, zpty, ztcp, zsocket,
zf_rm, zf_mv, zf_chmod, zf_mkdir, zf_chown, mapfile
```

**3. Destructive command patterns** (`tools/BashTool/destructiveCommandWarning.ts`)  
Adapt the regex patterns for: git reset, rm -rf, DROP TABLE, kubectl delete, terraform destroy, etc.

**4. Token thresholds** (`services/compact/autoCompact.ts`)  
```
AutocompactBufferTokens = 13_000
WarningThresholdBufferTokens = 20_000
ManualCompactBufferTokens = 3_000
MaxConsecutiveAutocompactFailures = 3
```

---

## Core Insight

> **The real magic is:** Dispatch loop + per-turn context injection + compaction before LLM call. This pattern is what separates a toy agent from a production tool. Most open-source CLI agents miss the per-turn context refresh — they inject once at startup. The source refreshes every turn.

> **Compaction is structural, not optional.** It's not a long-context luxury — it's the difference between working for 5 minutes vs. working for hours. Allocate a full week to it (week 4–5) and get it right.

> **Local model (Gemma 4 E4B) is your unfair advantage.** The source pays for Haiku API calls for every internal task. You can do them for free on-device. Compaction summaries, scoring, session titles — all run locally in 1-3 seconds on Apple Silicon. This is something the original source code doesn't do, and it makes your Go port cheaper and faster than the original for long sessions.

> **The extra lift from this new analysis is:** make the runtime a streaming state machine, classify tools by concurrency, budget tool output before every model call, treat Plan/Fast as real execution modes with `Tab` as the primary switch, and decouple the UI via NDJSON IPC so Ink renders while Go thinks. Those five choices will have a larger effect on perceived quality than adding more tools.
