# Progress

## Completed

- Added the `OpenAIResponsesAPI` client type and registered it in provider factory wiring.
- Added the `codex` provider preset with Responses API routing, `gpt-5.5`, `CODEX_ACCESS_TOKEN`, and Codex model limits.
- Migrated active GPT defaults and curated model selection from `gpt-5.4` to `gpt-5.5`.
- Added Codex Responses headers, account-id header support, and Codex payload behavior that omits `max_output_tokens`.
- Added Codex auth config storage, OAuth/device-flow token helpers, JWT account-id extraction, and a token refresher.
- Wired Codex into provider discovery, `/connect codex`, model switching, stored auth loading, and token refresh.

## In Progress

- Next: update `gpt-5.5` reasoning handling.

## Pending

- Update project documentation.
- Replace the TUI Silvery local dependency with an npm registry dependency.