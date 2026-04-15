# MCP Integration Plan

## Goal

Add MCP client support to Chan so the agent can discover and call external MCP tools from configured servers, with a minimal first release that is stable, reviewable, and aligned with the current engine startup and permission model.

## Scope

Phase 1 should support external MCP tools only.

- Support user-level and project-level MCP server configuration.
- Support explicit per-server `stdio`, `sse`, `http`, and `ws` transport choices.
- Load configured MCP servers during engine startup and register discovered tools into the existing tool registry before the ready event is emitted.
- Expose MCP tools to the model with stable, namespaced tool names.
- Route MCP tool execution through the existing permission gate using a conservative mapping.
- Keep the initial release startup-loaded only; changed MCP config is reflected on the next fresh session.

Out of scope for Phase 1:

- MCP resources and prompts surfaced as first-class model context.
- OAuth or other interactive auth flows beyond static headers or env-backed secrets.
- Running Chan itself as an MCP server.
- Full TUI management screens for MCP server lifecycle.

Non-goal:

- Chan stays an MCP client only. Do not add any follow-up phase for exposing Chan itself as an MCP server.

## Design Constraints

- Chan currently builds its tool registry inside `RunStdioEngine` and emits tool definitions once via the ready event, so MCP tools must be available before that step in the first implementation.
- The current permission system is based on existing tool permission levels, so MCP integration should map external tools onto that model instead of introducing a second approval framework.
- The implementation should keep SDK-specific types behind a narrow internal wrapper so the rest of the engine deals with Chan-owned interfaces.
- MCP server config should declare one transport explicitly. Phase 1 should not guess or auto-fallback between SSE, HTTP, and WS for a single server entry.

## Proposed Changes

### 1. Configuration and dependency model

- [MODIFY] `chan/go.mod`
  - Add `github.com/modelcontextprotocol/go-sdk`.
- [MODIFY] `chan/internal/config/config.go`
  - Extend `Config` with an `MCP` section for user config.
  - Add config-loading support for a project-level override file and merge it with the existing user config.
  - Keep secrets out of persisted config where possible by allowing env-backed values for tokens, headers, and command arguments.
- [NEW] `chan/internal/config/mcp.go`
  - Define the MCP config schema and merge rules using a discriminated union keyed by `transport`.

Recommended config shape:

```json
{
  "mcp": {
    "servers": {
      "github": {
        "transport": "stdio",
        "command": "github-mcp-server",
        "args": ["stdio"],
        "env": {
          "GITHUB_TOKEN": "$GITHUB_TOKEN"
        },
        "enabled": true,
        "trust": false,
        "exclude_tools": []
      },
      "docs": {
        "transport": "http",
        "url": "http://127.0.0.1:8787/mcp",
        "headers": {
          "Authorization": "$DOCS_MCP_TOKEN"
        },
        "enabled": true
      },
      "search": {
        "transport": "sse",
        "url": "http://127.0.0.1:8811/sse",
        "enabled": true
      },
      "browser": {
        "transport": "ws",
        "url": "ws://127.0.0.1:9000/mcp",
        "enabled": false
      }
    }
  }
}
```

Project config should override or augment user config for the current workspace. The canonical first version should be a repo-local `.chan/mcp.json` file that contains only the MCP section and is merged on top of the global config.

### 2. MCP runtime wrapper

- [NEW] `chan/internal/mcp/config.go`
  - Convert merged config into runtime server definitions.
- [NEW] `chan/internal/mcp/client.go`
  - Wrap the MCP SDK client/session lifecycle behind Chan-owned interfaces.
- [NEW] `chan/internal/mcp/manager.go`
  - Own server startup, connection, tool discovery, and shutdown.
- [NEW] `chan/internal/mcp/transports/stdio.go`
  - Start and supervise `stdio` servers.
- [NEW] `chan/internal/mcp/transports/sse.go`
  - Connect to SSE servers.
- [NEW] `chan/internal/mcp/transports/http.go`
  - Connect to streamable HTTP servers.
- [NEW] `chan/internal/mcp/transports/ws.go`
  - Connect to WebSocket servers.

Manager responsibilities:

- Start enabled servers during engine startup.
- Perform MCP initialization and list tools.
- Retain server metadata needed for execution, status reporting, and shutdown.
- Return Chan-friendly tool descriptors without exposing SDK types outside the package.
- Cleanly close sessions and child processes on engine shutdown.

Transport rules:

- `stdio` entries use `command`, `args`, `env`, and optional startup timeout.
- `sse`, `http`, and `ws` entries use `url`, optional headers, and optional auth-related env expansion.
- Each server entry chooses exactly one transport. Switching transport means changing config, not runtime probing.
- Use a discriminated schema per transport instead of one loose shared struct. It is clearer, easier to validate, and avoids irrelevant keys on a server entry.

### 3. Tool registration and execution

- [MODIFY] `chan/internal/tools/registry.go`
  - Add a registration path for dynamically discovered MCP tools.
- [NEW] `chan/internal/tools/mcp_tool.go`
  - Implement a `Tool` adapter for MCP-discovered tools.
- [MODIFY] `chan/internal/engine/engine.go`
  - Create the MCP manager after config load and before `EmitReady`.
  - Register MCP tools into the main registry before the model sees the tool list.
  - Shut the MCP manager down when the engine exits.

Tool naming rules:

- Use a stable namespace such as `mcp__<server>__<tool>`.
- Preserve the original MCP tool name in adapter metadata for logs and status.
- Sanitize invalid characters once during registration rather than at call time.
- Export all discovered tools by default. Phase 1 filtering should stay simple: support `exclude_tools`, and add narrower allow-list behavior later only if needed.

Execution rules:

- Convert model input directly into MCP tool-call arguments.
- Preserve structured JSON results when possible.
- Apply the same output budgeting used for built-in tools so large MCP responses do not bypass transcript and artifact safeguards.

### 4. Permission mapping and trust policy

- [MODIFY] `chan/internal/permissions/gating.go`
  - Add an MCP-aware risk assessment path.
- [MODIFY] `chan/internal/permissions/executor.go`
  - Surface MCP tool approval requests with enough server and tool metadata for the UI.

Initial permission policy:

- Default all MCP tools to ask-for-approval unless the server is explicitly marked trusted.
- Allow a trusted server to downgrade explicitly configured read-style MCP tools to read-only approval semantics.
- Keep unknown or obviously mutating tools in ask mode even for trusted servers unless the config mapping is explicit.

The first release should prefer false positives over silent unsafe execution. A simple, defensible starting point is:

- trusted + explicitly configured read-only mapping -> `PermissionReadOnly`
- trusted + explicitly configured mutating mapping -> `PermissionWrite` or `PermissionExecute`
- untrusted or unknown mapping -> `PermissionExecute`

Permission mapping should live entirely in config for Phase 1. Do not infer read/write behavior from MCP annotations.

### 5. Provider-facing tool schema handling

- [MODIFY] provider tool-schema sanitization paths under `chan/internal/api/`
  - Ensure dynamically registered MCP tool schemas are normalized the same way built-in tools are.

This work matters because MCP tools are not static. The model-facing schema path must accept runtime-added definitions without assuming a compile-time tool set.

### 6. Commands and observability

- [MODIFY] slash-command catalog under `chan/internal/engine/`
  - Add a lightweight command for MCP status or reload after the core integration works.
- [NEW] optional session/status output for connected MCP servers
  - Emit enough information to explain which MCP servers loaded, which failed, and which tools were registered.

Do not make command UX a blocker for Phase 1. Startup logs plus a simple status command are enough.
Do not require reload support in Phase 1. Startup-time loading is sufficient for the first release.

## Delivery Phases

### Phase 1: working MCP tool execution

- Add MCP config schema and merging.
- Add runtime manager and transport wrappers.
- Load servers at startup.
- Register MCP tools before ready emission.
- Execute MCP tools through the existing tool runner.
- Gate calls conservatively through the permission system.

Exit criteria:

- A configured `stdio` server exposes tools in a fresh session.
- A configured `sse` server exposes tools in a fresh session.
- A configured streamable HTTP server exposes tools in a fresh session.
- A configured `ws` server exposes tools in a fresh session.
- The model can call at least one MCP tool successfully.
- Failed MCP servers degrade gracefully without breaking the rest of the session.

### Phase 2: ergonomics and safer policy defaults

- Add a dedicated MCP status or reload command.
- Improve permission mapping with config-driven trust and include or exclude lists.
- Improve engine and TUI status visibility for MCP load failures and tool origin.

### Phase 3: broader MCP surface

- Evaluate MCP resources and prompts.
- Evaluate OAuth flows where static tokens are insufficient.

## Decisions

1. Use `.chan/mcp.json` as the canonical project-level MCP config file.
2. Keep trusted-server permission mapping entirely in config.
3. Startup-time loading is enough for Phase 1; reload can wait.
4. Export all discovered tools by default.
5. Use a discriminated union keyed by `transport` for the MCP config schema.
6. Keep Phase 1 filtering simple with `exclude_tools`; only add allow-listing later if real usage justifies it.

## Verification Plan

1. Manually verify config parsing and user/project MCP config merge behavior with a global config plus a repo-local `.chan/mcp.json` override.
2. Manually verify tool-name sanitization and schema normalization with representative MCP tools that include unusual names and nested JSON schemas.
3. Manually verify permission behavior for trusted read-style tools, trusted mutating tools, and untrusted tools.
4. Manually verify a small fixture MCP server over `stdio` end to end in a fresh session.
5. Manually verify a fixture SSE server end to end in a fresh session.
6. Manually verify a fixture streamable HTTP server end to end in a fresh session.
7. Manually verify a fixture WebSocket server end to end in a fresh session.
8. Manually verify that all discovered tools are exported by default and that `exclude_tools` hides configured tools correctly.
9. Manually verify that startup failure for one MCP server does not prevent built-in tools or other MCP servers from loading.
10. Manually verify that oversized MCP responses still respect tool output budgeting and do not reintroduce transcript or TUI memory issues.

## Recommended Implementation Order

1. Add config types and merged config loading.
2. Add the internal MCP manager and transport wrappers.
3. Add the MCP tool adapter and dynamic registry wiring.
4. Wire startup loading into the engine before ready emission.
5. Add conservative permission mapping.
6. Add one status or reload command after the core flow is working.