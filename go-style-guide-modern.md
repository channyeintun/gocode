# Go Style Guide

This guide defines the default conventions for Go code in this workspace.
The goal is simple: code should be easy to read, easy to debug, and hard to misuse.

## Principles

- Prefer obvious code over clever code.
- Design packages, not scripts.
- Keep I/O at the edges and core logic reusable.
- Handle errors explicitly and close to the failure point.
- Favor simple control flow over abstraction-heavy designs.
- Reach for the standard library first.

## Package Design

- Use `cmd/` for thin binary entrypoints.
- Use `internal/` for module-private application code.
- Use `pkg/` only for code intentionally meant for reuse outside the module.
- Give each package one clear responsibility.
- Avoid dependency cycles by keeping higher-level packages dependent on lower-level ones, not the reverse.
- Keep transport, storage, CLI, and UI concerns out of core domain packages.

## APIs and Types

- Export the smallest API surface that solves the problem.
- Accept interfaces where behavior is consumed, not where values are created.
- Prefer concrete return types unless callers benefit from an interface.
- Accept `context.Context` as the first parameter after the receiver when work can block, wait, or be canceled.
- Do not use pointers to interfaces.
- Use pointer receivers for methods that mutate state or where copying would be misleading.
- Choose receiver kinds deliberately: value receivers work on values and pointers, but pointer receivers require pointers or addressable values.
- Verify interface conformance at compile time when a type is expected to satisfy a specific public contract.

```go
type Runner interface {
    Run(context.Context) error
}

type Worker struct{}

var _ Runner = (*Worker)(nil)
```

## Naming

- Use names that describe behavior, not implementation details.
- Avoid package-name stutter such as `cache.CacheClient`.
- Use short receiver names that stay consistent for a type.
- Use `ErrName` for exported sentinel errors and `errName` for internal sentinels.
- Use the `Error` suffix for custom error types.
- Avoid built-in names such as `string`, `error`, `map`, or `len` for local variables.

## Constants and Enums

- Start enums at 1 unless the zero value is the intended default.
- Use the zero value only when it has clear semantics for callers.
- Keep unrelated constants in separate groups even when they share a type.

## Errors

- Check every error immediately.
- Add context where the operation becomes clearer, not at every line.
- Use `errors.New` for stable sentinel errors.
- Use custom error types when callers need structured details.
- Use `errors.Is` for sentinel matching.
- Prefer `errors.AsType[T]` in Go 1.26+ for extracting concrete error types.
- Use `errors.As` when you need the older pointer-target form or when matching a non-error interface.
- Use `%w` only when exposing the wrapped error is part of the contract.
- Use `%v` when you want to keep the text but not the matching behavior.
- Handle each error once. Do not log an error and then return that same error unchanged.

### Choosing an Error Shape

- Use a sentinel error when callers only need to branch on meaning.
- Use a custom type when callers need fields such as a path, operation, or code.
- Return the original error unchanged when it already provides enough meaning.
- Use `errors.Join` only when multiple independent failures all matter.

```go
var ErrClosed = errors.New("closed")

type ParseError struct {
    Field string
}

func (e *ParseError) Error() string {
    return fmt.Sprintf("parse %q", e.Field)
}

func Load(path string) error {
    if path == "" {
        return &ParseError{Field: "path"}
    }
    return nil
}

func Run() error {
    if err := Load(""); err != nil {
        if parseErr, ok := errors.AsType[*ParseError](err); ok {
            return fmt.Errorf("invalid input %q: %w", parseErr.Field, err)
        }
        return fmt.Errorf("load config: %w", err)
    }
    return nil
}
```

### Wrapping Rules

- Keep error context short and operation-oriented: `read config`, `dial upstream`, `decode payload`.
- Avoid stacked noise such as `failed to` on every layer.
- If callers should not match the underlying error, do not wrap it with `%w`.

## Resource Cleanup

- Use `defer` to clean up files, locks, tickers, spans, and similar resources.
- Prefer `defer` for readability unless profiling shows the call is on a true nanosecond-scale hot path.
- Keep cleanup next to acquisition so ownership is obvious.

## Assertions, Panics, and Exit

- Use the comma-ok form for type assertions unless failure is truly impossible by construction.
- Avoid panics in production paths. Return errors and let callers decide how to respond.
- Reserve `panic` for irrecoverable programmer bugs or deliberate `Must...` initialization at package scope.
- Call `os.Exit` or `log.Fatal` only from `main` or the outermost command boundary.
- Prefer a single exit path in `main`, often by delegating work to a `run()` function that returns an error or exit code.

## Concurrency and Shared State

- Start a goroutine only when ownership and shutdown are clear.
- Every goroutine must have a predictable stop condition or an explicit stop signal and a way to wait for exit.
- Prefer structured coordination with `errgroup` or an explicitly owned `sync.WaitGroup`.
- Do not fire-and-forget goroutines in library code.
- Prefer unbuffered channels or a channel buffer of 1 unless a larger buffer has a demonstrated reason.
- Keep `sync.Mutex` and `sync.RWMutex` as value fields.
- Do not embed mutexes in exported structs.
- Do not start background goroutines in `init()`.
- Keep shared mutable state small and easy to reason about.

## Atomics

- Prefer typed atomics from `sync/atomic` in new code.
- Use `atomic.Bool`, `atomic.Int64`, `atomic.Uint64`, and `atomic.Pointer[T]` instead of ad hoc unsafe patterns.
- If a package already standardizes on another atomic wrapper, keep that package internally consistent instead of mixing styles.

## Initialization and Data Ownership

- Copy slices and maps at package boundaries when later mutation would be surprising or unsafe.
- Return nil slices when there is no data unless an API or wire format requires an empty slice.
- Check slice emptiness with `len(s) == 0`, not with `s == nil`.
- Prefer `var items []T` when building a slice incrementally from its zero value.
- Use map and slice literals for fixed data.
- Use `make` when populating incrementally or when a capacity hint is known.
- Use `make(map[K]V)` for empty maps built programmatically and map literals for fixed initial contents.
- Use `&T{}` instead of `new(T)` for struct pointers.
- Use field names in struct literals when the type is declared elsewhere.
- Omit zero-value struct fields unless they add useful meaning in that context.
- Use `var state T` when you want the zero value of a struct and are not setting any fields yet.

## Structs, Embedding, and Tags

- Keep methods on the type that owns the state and invariants.
- Avoid embedding public types in public structs just to save a field name.
- If you embed a type, put embedded fields first and separate them from regular fields with a blank line.
- Do not embed types purely for convenience.
- Prefer explicit fields over promoted methods when API clarity matters.
- Avoid embedding types that leak internals, widen the API unexpectedly, or break a useful zero value.
- Add field tags for marshaled structs whenever field names or omission behavior matter.

## Control Flow and Local Style

- Prefer early returns to deep nesting.
- Remove unnecessary `else` blocks after a `return`, `break`, or `continue`.
- Keep variable scope as small as practical.
- Use `:=` for local values with obvious types.
- Use `var` when you need the zero value, an explicit type, or a package-level declaration.
- Group related declarations together.
- Group imports, constants, variables, and types when they are part of the same idea.
- Keep top-level variables declared with `var`; omit the explicit type unless the desired type is broader than the expression.
- Narrow scope with `if err := ...; err != nil {}` when that improves clarity.
- Avoid naked boolean arguments when the meaning is unclear; prefer small custom types, named constants, or inline parameter comments.
- Keep long lines readable rather than enforcing arbitrary wrapping.
- Be consistent within a file and package before chasing global rules.

## Imports and Package Names

- Let `goimports` manage import grouping.
- Keep two import groups by default: standard library first, everything else second.
- Keep package names short, lower-case, and descriptive.
- Avoid names that are too generic at call sites, such as `util`, `common`, or `helpers`, unless the package truly owns that role.
- Use import aliases sparingly and only to avoid collision or improve clarity.

## Interfaces and Generics

- Define small interfaces at the consumer side.
- Do not create interfaces for a single concrete type unless they decouple a real boundary.
- Use generics when they clearly remove duplicated logic across real call sites.
- Prefer simple type parameters and constraints over generic frameworks.
- If the generic version is harder to read than two explicit functions, write the two functions.

## Configuration and Globals

- Prefer explicit construction over package-level mutable state.
- Avoid mutable globals.
- Avoid `init()` for non-trivial setup.
- Keep `init()` deterministic when it cannot be avoided.
- Avoid I/O, environment reads, and hidden runtime behavior inside `init()`.
- Do initialization in constructors or explicit setup functions so errors can be returned normally.

## Time, Strings, and Formatting

- Use the `time` package for timestamps, durations, deadlines, and comparisons.
- Pass `time.Time` and `time.Duration` instead of raw integers when units matter.
- When an external representation cannot use `time.Duration`, include the unit in the field name.
- When an external representation cannot use `time.Time`, prefer RFC 3339 strings unless the API contract requires something else.
- Prefer `strconv` over `fmt` in performance-sensitive conversion paths.
- Prefer raw string literals when they make escaped content easier to read.
- Keep reusable `Printf` format strings as constants so tooling can analyze them.
- Name custom `Printf`-style functions with a trailing `f` so `go vet` can validate format strings.

## Performance

- Measure before optimizing.
- Provide slice and map capacity hints when the final size is known or easy to estimate.
- Avoid repeated string-to-byte and byte-to-string conversions in hot paths.
- Prefer the clearest correct code until profiling shows a real bottleneck.

## API Construction Patterns

- Prefer plain constructors for small APIs.
- Use option structs when there are several related optional settings and defaults are easy to express as data.
- Use functional options when options must evolve without frequent breaking changes and the call sites stay readable.
- Do not hide core required inputs inside optional configuration.

## Tooling

- Format code with `gofmt` or `goimports`.
- Run `go vet` regularly.
- Run `staticcheck` for broader static analysis.
- Use `revive` when the repository wants style-oriented linting.
- Use `golangci-lint` when one runner for multiple linters is helpful.
- Do not add `golint` to new workflows. It is deprecated.
- When upgrading Go versions, run `go fix ./...` and review the proposed modernizers.

## Current Go Guidance

- Prefer `errors.AsType[T]` over `errors.As` for concrete typed error extraction in Go 1.26+.
- Prefer standard library helpers such as `slices`, `maps`, and `cmp` when they make the code simpler and clearer.
- Prefer typed atomics from `sync/atomic` for new code.
- Keep code compatible with the module's declared Go version even if a newer feature exists.
- Treat new language features as readability tools, not reasons to make code denser.

## Review Checklist

- Is the package boundary clear?
- Is the control flow easy to follow?
- Are errors explicit, contextual, and handled once?
- Is shared state minimal and obviously synchronized?
- Does the public API expose only what callers need?
- Did we prefer the standard library where it already solves the problem well?