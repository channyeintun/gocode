# Completed Go Refactor Tree

This is the current Go layout after the refactor.

The result is:

- the command source directory is now `chan/cmd/chan-engine`
- `package main` is reduced to entrypoint wiring only
- the runtime cluster moved into `chan/internal/app`
- lower-coupling pieces remain split into focused internal packages

## Current Tree

```text
chan/
|-- cmd/
|   \-- chan-engine/
|       |-- debug.go
|       \-- main.go
|-- internal/
|   |-- agent/
|   |-- api/
|   |-- app/
|   |   |-- engine.go
|   |   |-- session_helpers.go
|   |   |-- slash_command_handlers.go
|   |   |-- slash_commands.go
|   |   |-- subagent_background.go
|   |   |-- subagent_runtime.go
|   |   \-- tool_executor.go
|   |-- artifacts/
|   |-- bashsecurity/
|   |-- clientdebug/
|   |   \-- client_proxy.go
|   |-- commands/
|   |   \-- catalog.go
|   |-- compact/
|   |-- config/
|   |-- cost/
|   |-- debuglog/
|   |   \-- monitor.go
|   |-- engine/
|   |   \-- model_state.go
|   |-- hooks/
|   |-- ipc/
|   |-- localmodel/
|   |-- memory/
|   |   \-- recall.go
|   |-- permissions/
|   |-- session/
|   |-- skills/
|   |-- timing/
|   |-- tools/
|   \-- utils/
|-- go.mod
|-- go.sum
|-- install.sh
\-- Makefile
```

## File Moves

```text
chan/cmd/chan/main.go
  -> chan/cmd/chan-engine/main.go

chan/cmd/chan/debug_monitor.go
  -> chan/cmd/chan-engine/debug.go
  -> chan/internal/debuglog/monitor.go

chan/cmd/chan/debug_proxy.go
  -> chan/internal/clientdebug/client_proxy.go

chan/cmd/chan/engine.go
  -> chan/internal/app/engine.go

chan/cmd/chan/memory_recall.go
  -> chan/internal/memory/recall.go

chan/cmd/chan/model_state.go
  -> chan/internal/engine/model_state.go

chan/cmd/chan/session_helpers.go
  -> chan/internal/app/session_helpers.go

chan/cmd/chan/slash_commands.go
  -> chan/internal/app/slash_commands.go
  -> chan/internal/commands/catalog.go

chan/cmd/chan/slash_command_handlers.go
  -> chan/internal/app/slash_command_handlers.go

chan/cmd/chan/subagent_background.go
  -> chan/internal/app/subagent_background.go

chan/cmd/chan/subagent_runtime.go
  -> chan/internal/app/subagent_runtime.go

chan/cmd/chan/tool_executor.go
  -> chan/internal/app/tool_executor.go
```

## Thin Command Package

The command package is now only:

```text
chan/cmd/chan-engine/
  debug.go   # Cobra debug-view command wiring
  main.go    # CLI flags, signal handling, TUI launch, stdio entrypoint
```

Everything else is outside `package main`.
