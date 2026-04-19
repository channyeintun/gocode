---
name: go-coding-philosophy
description: Guidance for Golang work that keeps code obvious, package-oriented, explicit about errors, and composable across commands and runtimes.
keywords: golang, go philosophy, go code, idiomatic go, go.mod, .go, go package, go error handling, go refactor
argument-hint: Use for Go or Golang coding tasks, package structure decisions, readability refactors, naming cleanup, and explicit error handling changes.
---
Apply when task is Go or Golang code.

Core philosophy:
- Obvious over clever. Boring and clear beats clever and opaque.
- Hard to read = bad Go.
- Simple if statements over abstractions. `readFile` not `executeIOStrategy`.

Package structure:
- Beginners write scripts. Professionals design packages.
- Core logic independent of CLI, transport, storage, UI.
- `cmd/` → entry points. `internal/` → app logic. `pkg/` → reusable components.
- One function, one job. One package, one responsibility.
- Easier to test, reuse, debug.

Error handling:
- Always check errors.
- "What can go wrong here?" → handle it immediately.
- Return meaningful context. Never swallow failures.

Composability:
- Small focused functions. Coherent packages. Think LEGO.
- I/O at the edges. Core logic reusable by CLI, API, any runtime.
- Building blocks over tangled orchestration.