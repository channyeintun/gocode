# Implementation Plan — Fix Critical Issues

Fixes ordered by severity and dependency. Each task is self-contained and can be committed independently.

---

## Phase 1: Critical Bugs (High Severity)

### Task 1: Fix Gemini `FunctionResponse.Name` Using ToolCallID Instead of Function Name

**File:** `internal/api/gemini.go`

**Problem:** `geminiFunctionResponsePart()` passes `result.ToolCallID` as the Gemini `FunctionResponse.Name`, but Gemini expects the actual function name. Tool results fail to correlate with their calls.

**Fix:**
- Thread the tool/function name through to `geminiFunctionResponsePart()`.
- In `convertGeminiMessage()` for `RoleTool`, find the matching tool call name from the ToolCallID.
- Alternative: Change the caller in `convertGeminiMessage` to pass the function name explicitly. This requires building a `toolCallID → toolName` map from prior messages in `buildGeminiContents`.

**Scope:** ~30 lines changed in `gemini.go`.

---

### Task 2: Fix Race Condition in Title Generation Goroutine

**File:** `cmd/gocode/main.go`

**Problem:** The title goroutine reads `api.Message` structs whose inner pointer fields (`ToolCalls`, `ToolResult`, `Images`) may be concurrently modified when the main loop compacts or appends messages.

**Fix:**
- Deep-copy messages before passing to the goroutine: clone each message's `ToolCalls` slice, `ToolResult` pointer, and `Images` slice.
- Add a `DeepCopy([]api.Message)` helper in `internal/api/client.go`.

**Scope:** ~25 lines — new helper + one call site change.

---

### Task 3: Fix Race Condition on Global Mutable State

**Files:** `internal/tools/file_history.go`, `internal/tools/session_artifacts.go`

**Problem:** `SetGlobalFileHistory` and `SetGlobalSessionArtifacts` write package-level variables without synchronization while tool goroutines read them.

**Fix:**
- Protect both globals with a `sync.RWMutex`.
- Wrap the getter functions with `RLock`/`RUnlock`.
- Wrap the setter functions with `Lock`/`Unlock`.

**Scope:** ~20 lines per file.

---

### Task 4: Add Timeouts to `http.Client` in All API Clients

**Files:** `internal/api/anthropic.go`, `internal/api/gemini.go`, `internal/api/ollama.go`, `internal/api/openai_compat.go`

**Problem:** `&http.Client{}` with no timeout can block indefinitely.

**Fix:**
- Set a transport-level timeout: `&http.Client{Timeout: 5 * time.Minute}` (streaming responses need a long timeout, but not infinite).
- Alternatively, configure a custom `http.Transport` with `TLSHandshakeTimeout: 10s`, `ResponseHeaderTimeout: 30s`, and rely on context cancellation for the body read.
- Use a shared helper `func newHTTPClient() *http.Client` to keep it DRY.

**Scope:** ~15 lines — new helper + 4 call site changes.

---

## Phase 2: Medium Severity Bugs and Security

### Task 5: Fix Child Agent Cost Double-Counting

**File:** `internal/cost/tracker.go`

**Problem:** `RecordChildAgentSnapshot` calls `mergeSnapshotLocked` which already merges `snapshot.ChildAgent*` fields, then adds `snapshot.TotalCostUSD` to `ChildAgentCostUSD` again.

**Fix:**
- In `RecordChildAgentSnapshot`, after `mergeSnapshotLocked`, only add the child's *own* costs (excluding its nested children) to the child agent subtotals:
  ```go
  ownCost := snapshot.TotalCostUSD - snapshot.ChildAgentCostUSD
  t.ChildAgentCostUSD += ownCost
  ```
- Or restructure: don't let `mergeSnapshotLocked` touch `ChildAgent*` fields, and handle them separately.

**Scope:** ~10 lines.

---

### Task 6: Consolidate Duplicate Bash Security Rules

**Files:** `internal/tools/bash.go`, `internal/permissions/bash_rules.go`

**Problem:** Same regexes defined in two places.

**Fix:**
- Remove the duplicate definitions from `internal/tools/bash.go`.
- Import and use the canonical versions from `internal/permissions/bash_rules.go`.
- Update `validateBashSecurity()` and `checkDestructive()` in `bash.go` to call `permissions.ValidateBashSecurity()` and `permissions.CheckDestructive()`.

**Scope:** ~40 lines removed, ~10 lines changed.

---

### Task 7: Resolve Symlinks in `resolveToolPath`

**File:** `internal/tools/path_resolution.go`

**Problem:** Symlinks inside the working directory can point outside it, bypassing the traversal check.

**Fix:**
- After resolving the absolute path, call `filepath.EvalSymlinks()` to get the real path.
- Re-check the `filepath.Rel` escape condition against the real path.
- Handle the case where the symlink target doesn't exist yet (for `create_file`).

**Scope:** ~10 lines.

---

### Task 8: Validate `base_url` Against Known Provider Domains

**File:** `internal/api/anthropic.go`, `internal/api/gemini.go`, etc.

**Problem:** A user-controlled `base_url` override causes API keys to be sent to arbitrary servers.

**Fix:**
- Add a warning log when `base_url` is overridden from the default.
- In warmup functions, skip warmup if `base_url` doesn't match the provider's default domain.
- Optionally: require explicit opt-in (e.g., `GOCODE_ALLOW_CUSTOM_BASE_URL=1`) before API keys are sent to non-default endpoints.

**Scope:** ~20 lines.

---

### Task 9: Validate Permission Mode on Load

**File:** `internal/config/config.go`

**Problem:** Invalid `GOCODE_PERMISSION_MODE` values are silently accepted.

**Fix:**
- Add validation in `Load()` that warns on stderr and falls back to `"default"` if the value isn't one of the known modes.

**Scope:** ~10 lines.

---

## Phase 3: Code Quality

### Task 10: Remove `max()` Redefinition and `minInt` Helper

**Files:** `internal/tools/file_read.go`, `internal/agent/output_budget.go` (and any other files defining `minInt`)

**Problem:** Go 1.21+ builtins `min()` and `max()` exist. Custom definitions shadow them.

**Fix:**
- Remove the custom `max()` from `file_read.go`.
- Replace `minInt()` calls with `min()` builtin across the codebase.
- Search for other shadowed builtins.

**Scope:** ~5-10 lines per file.

---

### Task 11: Expand Bash Security Validation

**File:** `internal/permissions/bash_rules.go`

**Problem:** The blocklist misses `eval`, `source`, `exec`, backtick substitution.

**Fix:**
- Add patterns for `eval`, `source`/`.`, `exec`, and backtick command substitution.
- Consider whether the login shell flag (`-l`) should be removed to prevent `.zshrc` alias execution.

**Scope:** ~15 lines.

---

## Phase 4: Structural Improvements (Optional, Larger Scope)

### Task 12: Extract Engine Loop from `main.go`

**Problem:** `cmd/gocode/main.go` is ~2400 lines.

**Fix:**
- Extract `runStdioEngine` and its helpers into a new `cmd/gocode/engine.go`.
- Extract `executeToolCalls` and permission handling into `cmd/gocode/tool_executor.go`.
- Extract slash command handling into `cmd/gocode/slash_commands.go`.
- Extract artifact/session helpers into `cmd/gocode/session_helpers.go`.

**Scope:** Large — file reorganization only, no logic changes.

---

### Task 13: Add Foundational Tests

**Problem:** Zero test coverage.

**Fix (incremental):**
- `internal/api/retry_test.go` — test retry logic and backoff.
- `internal/compact/tokens_test.go` — test token estimation and thresholds.
- `internal/tools/path_resolution_test.go` — test path traversal prevention.
- `internal/tools/bash_test.go` — test security validation patterns.
- `internal/permissions/gating_test.go` — test permission decision logic.
- `internal/agent/token_budget_test.go` — test continuation tracker.

**Scope:** Large — but each test file is independent.

---

## Execution Order

| Order | Task | Severity | Effort |
|-------|------|----------|--------|
| 1 | Task 1 — Fix Gemini FunctionResponse | High | Small |
| 2 | Task 2 — Fix title goroutine race | High | Small |
| 3 | Task 3 — Fix global state race | High | Small |
| 4 | Task 4 — Add http.Client timeouts | High | Small |
| 5 | Task 5 — Fix cost double-counting | Medium | Small |
| 6 | Task 6 — Consolidate bash security rules | Medium | Small |
| 7 | Task 7 — Resolve symlinks in path resolution | Medium | Small |
| 8 | Task 8 — Validate base_url for API keys | Medium | Small |
| 9 | Task 9 — Validate permission mode | Low | Tiny |
| 10 | Task 10 — Remove builtin shadows | Low | Tiny |
| 11 | Task 11 — Expand bash security | Medium | Small |
| 12 | Task 12 — Extract main.go (optional) | Medium | Large |
| 13 | Task 13 — Add foundational tests (optional) | High | Large |
