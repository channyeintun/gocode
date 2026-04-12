# Code Review — gocode CLI

## Summary

Full review of the Go CLI agentic coding tool. The project is well-structured overall with clean separation of concerns across packages. Below are the critical issues, bugs, and significant concerns discovered.

---

## Critical Issues

### 1. Race Condition: `titleMessages` Slice Shared with Goroutine

**File:** `cmd/gocode/main.go` (title generation goroutine)

The title generation goroutine copies the `messages` slice header but not the underlying array:

```go
titleMessages := append([]api.Message(nil), messages...)
```

This is correctly shallow-copied, **however** the `messages` slice continues to be mutated in the main loop (appending new messages, compaction replacements). Because `api.Message` contains pointer fields (`ToolResult *ToolResult`, `Images []ImageAttachment`, `ToolCalls []ToolCall`), the goroutine may read message structs whose inner slices/pointers are being concurrently modified by the main loop — particularly when compaction replaces or truncates messages. This is a **data race on the inner pointer fields**.

**Severity:** High — can cause subtle data corruption or panics under load.

---

### 2. Race Condition: Global Mutable State via `SetGlobalFileHistory` / `SetGlobalSessionArtifacts`

**File:** `internal/tools/file_history.go`, `internal/tools/session_artifacts.go`

These functions set package-level globals that are read by tool execution (which can run concurrently in goroutines):

```go
toolpkg.SetGlobalFileHistory(fileHistory)
toolpkg.SetGlobalSessionArtifacts(sessionID, artifactManager)
```

If a session restore or slash command swaps the session while a background agent or tool goroutine is still referencing the old globals, the tool reads stale or partially-written state. No synchronization protects these globals.

**Severity:** High — race condition during session switch with active background agents.

---

### 3. `http.Client` Without Timeout — Potential Resource Leak

**Files:** `internal/api/anthropic.go`, `internal/api/gemini.go`, `internal/api/ollama.go`, `internal/api/openai_compat.go`

All API clients construct `&http.Client{}` with **no timeout**:

```go
httpClient: &http.Client{},
```

If a server hangs or a network partition occurs and the context isn't cancelled, the goroutine will block indefinitely. While context cancellation on the request helps, the HTTP client's transport-level timeouts (TLS handshake, idle connection) are not bounded.

**Severity:** High — can cause goroutine leaks and eventual resource exhaustion in long-running sessions.

---

### 4. API Key Leaked in Warmup Headers

**File:** `internal/api/anthropic.go`, `internal/api/gemini.go`

The warmup functions send API keys as headers to real endpoints (HEAD requests):

```go
func (c *AnthropicClient) Warmup(ctx context.Context) error {
    return issueWarmupRequest(ctx, c.httpClient, http.MethodHead, c.baseURL+"/v1/messages", map[string]string{
        "x-api-key": c.apiKey,
    })
}
```

This is functionally correct for warming up the connection, but if `baseURL` is overridden by a user to a malicious endpoint, the API key is sent to an untrusted server. Currently `baseURL` can be set via `GOCODE_BASE_URL` env var or config file.

**Severity:** Medium — API key exfiltration via user-controlled `base_url` override.

---

### 5. Bash Command Injection: Incomplete Security Validation

**File:** `internal/tools/bash.go`

The bash tool validates a few dangerous patterns (ZSH builtins, process substitution, IFS injection), but the command string is passed directly to the shell via `sh -lc <command>`. The security checks are a blocklist approach which can be bypassed:

- `eval`, `source`, `exec` are not blocked
- Backtick command substitution is not checked (only `$()` in some contexts)
- Chaining with `;` or `&&` can embed undetected destructive commands after benign prefixes
- The `-l` (login shell) flag loads `.zshrc`/`.bashrc` which could contain malicious aliases

The `isParallelReadOnlyBashCommand` function does a more thorough parse, but the security validation in `validateBashSecurity` only checks three patterns.

**Severity:** Medium — the agent model is ultimately in control of commands, but insufficient guardrails if the model is manipulated via prompt injection from file contents.

---

### 6. Path Traversal: `resolveToolPath` Blocks Relative Escape but Not Symlink Escape

**File:** `internal/tools/path_resolution.go`

```go
func resolveToolPath(path string) (string, error) {
    // ... resolves relative to cwd, checks for "../" escape
}
```

The function checks `filepath.Rel` for `..` prefix, preventing relative path traversal. However, it does **not** resolve symlinks. A symlink within the working directory could point outside it, allowing reads/writes to arbitrary locations. Tools like `file_write`, `file_edit`, and `create_file` all use this function.

**Severity:** Medium — symlink-based directory escape for file operations.

---

## Bugs

### 7. Duplicate Bash Security Rules

**Files:** `internal/tools/bash.go` AND `internal/permissions/bash_rules.go`

The same regex patterns for dangerous commands, destructive patterns, and read-only detection are defined **twice** — once in `tools/bash.go` as package-level vars, and again in `permissions/bash_rules.go`. The `bash.go` tool uses its own copy while `permissions/bash_rules.go` exports the same patterns. If one is updated without the other, they silently diverge.

**Severity:** Medium — maintenance hazard that will lead to inconsistent security enforcement.

---

### 8. `RecordChildAgentSnapshot` Double-Counts Child Agent Costs

**File:** `internal/cost/tracker.go`

```go
func (t *Tracker) RecordChildAgentSnapshot(snapshot TrackerSnapshot) {
    t.mu.Lock()
    defer t.mu.Unlock()
    t.mergeSnapshotLocked(snapshot)           // adds to totals
    t.ChildAgentCostUSD += snapshot.TotalCostUSD  // also adds child subtotals
    t.ChildAgentInputTokens += snapshot.TotalInputTokens
    t.ChildAgentOutputTokens += snapshot.TotalOutputTokens
}
```

`mergeSnapshotLocked` already adds `snapshot.ChildAgentCostUSD` to `t.ChildAgentCostUSD`. Then the function adds `snapshot.TotalCostUSD` again. If the child snapshot itself has nested child agent costs, those get triple-counted.

**Severity:** Medium — inflated cost reporting for nested child agents.

---

### 9. `IPC Bridge.ReadMessage` Goroutine Leak

**File:** `internal/ipc/bridge.go`

When `ReadMessage` is called, it spawns a goroutine that blocks on `b.reader.Scan()`. If the context is cancelled before `Scan()` returns, the goroutine is abandoned (the `readCh` channel is consumed but the goroutine reading from stdin continues blocking). On the next call to `ReadMessage`, a new goroutine is spawned because `b.readCh` was set to nil.

The `readLoop` in `router.go` mitigates this for the main loop, but direct `ReadMessage` calls elsewhere could leak goroutines.

**Severity:** Low-Medium — goroutine leak on repeated context cancellation.

---

### 10. `minInt` Undefined in `output_budget.go`

**File:** `internal/agent/output_budget.go`

The file uses `minInt()` but never defines or imports it. If this compiles, there must be a definition elsewhere in the `agent` package, but there's no `import` or visible definition. Since Go 1.21+ has `min()` builtin, this relies on having a helper somewhere in the package.

**Severity:** Low — latent compilation issue if the helper is removed; should use `min()` builtin.

---

### 11. `max()` Redefined in `file_read.go`

**File:** `internal/tools/file_read.go`

```go
func max(a, b int) int {
```

Go 1.21+ provides a builtin `max()`. Redefining it shadows the builtin and will confuse contributors. Given `go 1.26.1` in `go.mod`, this is unnecessary.

**Severity:** Low — code quality issue.

---

### 12. Anthropic Tool Input: Conflicting `Initial` and `Builder` Data

**File:** `internal/api/anthropic.go`

In `anthropicToolUseState.inputJSON()`, if both `Builder.Len() > 0` and `Initial` are set, only `Builder` is used. The Anthropic API sends initial tool input in `content_block_start` and deltas in `content_block_delta`. If the initial block has partial JSON and deltas complete it, the builder only captures deltas, losing the initial fragment.

Looking more carefully: the builder appends delta `partial_json` strings, and `Initial` captures the `input` field from `content_block_start`. For Anthropic, the initial `input` is typically `{}` or empty for streaming, so the builder should capture the full input from deltas. But if the API sends a non-empty initial input and then also sends deltas, the initial portion is silently dropped.

**Severity:** Low — edge case with non-standard Anthropic streaming behavior.

---

### 13. Gemini: `FunctionResponse.Name` Uses `ToolCallID` Instead of Function Name

**File:** `internal/api/gemini.go`

```go
func geminiFunctionResponsePart(result ToolResult) (geminiPart, error) {
    return geminiPart{
        FunctionResponse: &geminiFunctionResponse{
            Name:     result.ToolCallID,  // BUG: should be the function name
            Response: response,
        },
    }, nil
}
```

Gemini's `functionResponse` requires the **function name**, not the tool call ID. The `ToolCallID` may be an opaque ID (like `toolu_xxx`), causing Gemini to fail to match the response to the original function call.

**Severity:** High for Gemini users — tool results will fail to correlate.

---

### 14. No Validation of `GOCODE_PERMISSION_MODE` Input

**File:** `internal/config/config.go`

The `GOCODE_PERMISSION_MODE` env var is accepted as-is without validating it's one of `"default"`, `"bypassPermissions"`, or `"autoApprove"`. An invalid value like `"bypassperms"` silently falls through to the default permission behavior, which could surprise users expecting permissions to be bypassed.

**Severity:** Low — silent misconfiguration.

---

## Design Concerns

### 15. `main.go` is ~2400 Lines — God File

The `cmd/gocode/main.go` file contains the entire engine loop, tool execution, permission handling, slash commands, artifact management, session persistence, model streaming, cost tracking, plan review gate, and more. This is a maintenance and testability concern.

**Severity:** Medium — hinders testability, increases merge conflicts.

---

### 16. No Tests Present

No test files (`*_test.go`) were found anywhere in the project. For a tool that executes shell commands, modifies files on disk, and manages API keys, this is a significant gap.

**Severity:** High — no automated verification of any behavior.

---

### 17. Hook Scripts Execute with No Sandboxing

**File:** `internal/hooks/runner.go`

Hook scripts are executed with `exec.CommandContext(ctx, script)` with the full environment of the parent process. The payload is passed via stdin as JSON. There's no sandbox, no PATH restriction, and no file permission check beyond the glob pattern.

**Severity:** Low-Medium — intentional design, but worth noting for security-conscious deployments.

---

### 18. Session Transcripts Contain Sensitive Data in Plaintext

**File:** `internal/session/store.go`

All conversation transcripts (including tool outputs, file contents, and potentially secrets found in code) are stored as plaintext NDJSON files in `~/.config/gocode/sessions/`. No encryption or access control beyond filesystem permissions.

**Severity:** Low — standard for CLI tools, but worth documenting.

---

## Summary Table

| # | Severity | Category | Description |
|---|----------|----------|-------------|
| 1 | High | Race condition | Title goroutine reads inner pointer fields of shared messages |
| 2 | High | Race condition | Global mutable state for file history / session artifacts |
| 3 | High | Resource leak | http.Client with no transport timeout |
| 4 | Medium | Security | API key sent to user-controlled base_url |
| 5 | Medium | Security | Bash command injection blocklist is incomplete |
| 6 | Medium | Security | Path resolution doesn't resolve symlinks |
| 7 | Medium | Maintenance | Duplicate bash security rules in two packages |
| 8 | Medium | Bug | Child agent cost double-counting |
| 9 | Low-Medium | Resource leak | Bridge ReadMessage goroutine leak |
| 10 | Low | Code quality | minInt helper instead of builtin min |
| 11 | Low | Code quality | max() redefinition shadows builtin |
| 12 | Low | Bug | Anthropic tool input initial fragment may be dropped |
| 13 | High | Bug | Gemini FunctionResponse uses ToolCallID instead of function name |
| 14 | Low | Validation | No validation of permission mode config |
| 15 | Medium | Design | main.go is a 2400-line god file |
| 16 | High | Testing | No test files anywhere in the project |
| 17 | Low-Medium | Security | Hook scripts run unsandboxed |
| 18 | Low | Security | Session transcripts stored in plaintext |
