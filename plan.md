# First-Class Output Artifacts Plan

## Goal

Make artifacts a primary output channel in `gocode`, closer to Antigravity's model where structured work products live as durable, reviewable documents instead of transient transcript text.

## Cleanup Note

This file replaces the completed TUI parity / Phase 8 document.

Completed parity and stabilization work remains recorded in `progress.md`. `plan.md` now tracks only the next artifact-focused workstream.

## Current Baseline

- the runtime already persists session markdown artifacts for implementation plans, task lists, walkthroughs, and oversized tool logs
- the TUI exposes a plan panel and artifact list, but those surfaces still render artifact bodies as plain text or truncated previews
- IPC only emits `artifact_created` and `artifact_updated`, which is not enough for focus, review, feedback, or version-aware presentation
- plan mode can save implementation plans, but artifact approval and revision are not first-class interactions
- long structured outputs are still transcript-first unless a specific tool persists them intentionally

## Phase 9: First-Class Output Artifacts (Antigravity Alignment)

1. **Artifact-First Presentation**
   - **Purpose:** Make structured outputs feel like first-class work products, not transcript spillover.
   - **Implementation Details:**
     - **TUI Rendering:** Replace plain-text rendering in the plan panel and artifact list with the shared markdown renderer so alerts, tables, fenced code, and diff blocks display consistently.
     - **Primary Surface:** Promote the most relevant artifact for the active turn into a dedicated primary panel instead of treating all artifacts as compact previews.
     - **Metadata:** Show artifact kind, scope, version, source, and draft/final status directly in the UI.

2. **Artifact Lifecycle & IPC**
   - **Purpose:** Give artifacts a real state machine rather than only create/update broadcasts.
   - **Implementation Details:**
     - **Protocol Expansion:** Extend IPC beyond `artifact_created` / `artifact_updated` with events for focus, review requested, feedback submitted, status changes, and version changes.
     - **Payload Shape:** Include artifact metadata and status in event payloads so the TUI does not infer lifecycle state from content.
     - **Versioning:** Expose version transitions intentionally so revised artifacts remain reviewable instead of being silently replaced.

3. **Reviewable Plans and Feedback Loops**
   - **Purpose:** Match Antigravity's workflow where plans and other key artifacts can be explicitly reviewed, revised, and approved.
   - **Implementation Details:**
     - **Plan Review Gate:** Allow implementation-plan artifacts to enter a review-required state before write execution proceeds.
     - **User Feedback:** Persist artifact-scoped review notes separately from permission feedback so "revise this artifact" becomes a first-class action.
     - **Revision Flow:** Support revise / approve semantics for plans and walkthroughs without forcing the user to manage everything through freeform chat.

4. **Artifact Output Routing**
   - **Purpose:** Route the right work products into artifacts automatically and consistently.
   - **Implementation Details:**
     - **Primary Artifact Types:** Keep implementation plans, task lists, walkthroughs, and tool logs as the first shipped set.
     - **Next Additions:** Start intentionally using existing artifact kinds such as `search-report`, `diff-preview`, `diagram`, and `compact-summary` where they improve readability.
     - **Oversized Results:** Prefer artifact spillover for long structured outputs instead of dumping large transcript blocks.

5. **Prompt & Runtime Contract**
   - **Purpose:** Make the model reliably treat artifacts as a deliberate output mechanism.
   - **Implementation Details:**
     - **System Prompt:** Tell the model when to answer inline, when to save or update an artifact, and how to structure artifact markdown for the TUI.
     - **Plan Mode Behavior:** Keep read-first planning, but shift from "save a plan and tell the user to switch to /fast" toward "save, review, revise, then execute when the user is ready."
     - **Tool Integration:** Ensure artifact-producing tools follow the same conventions for titles, metadata, status, and updates.

6. **Follow-on Task View Integration**
   - **Purpose:** Leave a clean path for Antigravity-style task-mode UI built on the artifact system.
   - **Implementation Details:**
     - **Task Status Surface:** Reuse artifact lifecycle primitives for task boundaries, execution summaries, and verification checkpoints.
     - **Non-Goal for First Slice:** Do not block initial artifact improvements on a full `task_boundary` / `notify_user` implementation.

## Out of Scope for This Slice

- general workflow-file execution
- knowledge-item retrieval and indexing
- subagent orchestration
- browser or media embedding beyond text-first markdown artifact support
- non-artifact TUI redesign unrelated to artifact review

## Risks

- richer artifact IPC touches both engine and TUI reducers
- review gating must avoid deadlocking plan mode or duplicating the existing permission flow
- fuller markdown support for artifacts may reveal renderer gaps that need staged rollout
- versioned artifact history can bloat session state if retention is not bounded

## Definition of Done

- artifact panels render full markdown content instead of plain-text previews for the supported first slice
- artifact events carry enough metadata for status, focus, and version-aware UI updates
- implementation plans can be explicitly reviewed and revised as artifacts before execution
- task lists, walkthroughs, and long structured outputs follow the same first-class artifact contract
- completed parity-era plan items are no longer the active execution baseline in this file
