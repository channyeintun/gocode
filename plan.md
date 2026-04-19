# Runtime-Neutral Launcher Plan

## Objective

Remove the Bun-only runtime requirement from released Nami artifacts.

Target result:

- release archives do not ship a Bun-specific launcher wrapper
- `nami` can start when the user has any supported JavaScript runtime installed
- the supported runtime set is `node`, `bun`, or `deno`
- installers and docs describe a runtime-agnostic launcher story instead of requiring Bun specifically

## Current State

- release artifacts ship a Bun launcher bundle as `nami`
- installers explicitly require Bun
- docs still frame Bun as the runtime requirement for installed releases
- the engine binary is already separate, so the launcher is the main friction point

## Proposed Runtime Policy

Launcher runtime resolution order:

1. `node`
2. `bun`
3. `deno`

Design rules:

- release artifacts should contain a portable JavaScript entrypoint, not a Bun-bundled executable wrapper
- platform launchers should be thin shims that locate an available runtime and execute the shared JS entrypoint
- the shared JS entrypoint should stick to standard Node-compatible APIs already supported by Node, Bun, and Deno's Node compatibility layer
- installers should validate "one supported runtime exists" instead of hard-failing on missing Bun

## Workstreams

### 1. Docs And Messaging

Goal:

- stop presenting Windows as undocumented in the docs site
- prepare user-facing copy for a runtime-neutral release story

Tasks:

- update `web/docs.html` with current Windows install instructions
- update `README.md`, `nami/install.sh`, and `nami/install.ps1` once runtime-neutral releases exist
- replace "Bun required" wording with "Node, Bun, or Deno required" after the launcher migration lands

Status:

- `web/docs.html` updated in this pass

### 2. Portable Launcher Artifact

Goal:

- replace the Bun-specific release launcher with a portable JS launcher asset

Tasks:

- split the runtime-neutral launcher source into a release file such as `nami.js` or `nami.mjs`
- remove the `#!/usr/bin/env bun` assumption from the shipped launcher artifact
- keep the launcher limited to APIs available through Node-compatible runtimes
- preserve current behaviors like engine lookup, env forwarding, and `debug-view`

Primary files:

- `nami/tui/bin/nami.js`
- `nami/tui/Makefile`

### 3. Platform Shims

Goal:

- keep `nami` ergonomic on Unix and Windows without baking in a single JS runtime

Tasks:

- add a POSIX `nami` shim that detects `node`, `bun`, then `deno`
- update the Windows `nami.cmd` launcher to do the same runtime detection
- ensure Deno invocation includes the permissions needed to run the engine and read the installed files
- keep the JS launcher path resolution stable when installed next to `nami-engine` or `nami-engine.exe`

Primary files:

- `nami/tui/bin/nami`
- `nami/tui/bin/nami.cmd`

### 4. Installer Changes

Goal:

- installers should no longer require Bun specifically

Tasks:

- change `install.sh` to validate the presence of any supported runtime
- change `install.ps1` to validate the presence of any supported runtime
- improve the error message to list supported runtimes in detection order
- install the shared JS launcher plus platform shim instead of a Bun-bundled launcher

Primary files:

- `nami/install.sh`
- `nami/install.ps1`

### 5. Release Packaging

Goal:

- release archives should contain runtime-neutral launcher assets only

Tasks:

- stop bundling the Bun wrapper into release tarballs and zip files
- package these launcher assets instead:
  - shared JS entrypoint
  - POSIX `nami` shim where applicable
  - Windows `nami.cmd` shim where applicable
  - `nami-engine` or `nami-engine.exe`
- keep any Bun-specific local-dev build path separate from release packaging if it still helps development

Primary files:

- `nami/tui/Makefile`

### 6. Validation Matrix

Minimum verification matrix:

- macOS: `node`, `bun`
- Linux: `node`, `bun`, `deno`
- Windows: `node`, `bun`, `deno`

Minimum scenarios:

1. `nami --help`
2. normal TUI launch
3. `debug-view`
4. engine lookup when installed next to the launcher assets
5. install into a path containing spaces
6. Windows launch through `nami.cmd`

## Rollout Order

1. sync user-facing docs with current Windows install support
2. introduce the portable JS launcher artifact
3. add runtime-detection shims for Unix and Windows
4. switch installers to runtime-neutral validation
5. switch release packaging away from Bun-wrapped assets
6. validate across Node, Bun, and Deno on all supported platforms

## Definition Of Done

This work is complete when all of the following are true:

1. release archives no longer contain a Bun-specific launcher wrapper
2. installed Nami runs with `node`, `bun`, or `deno`
3. install scripts validate any supported runtime, not Bun only
4. Windows and Unix launch flows both work with the shared JS launcher
5. docs describe the runtime-neutral requirement accurately