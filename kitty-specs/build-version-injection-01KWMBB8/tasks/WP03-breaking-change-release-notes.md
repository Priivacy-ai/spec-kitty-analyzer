---
work_package_id: WP03
title: 0.3.0 breaking-change release notes draft
dependencies:
- WP01
requirement_refs:
- FR-006
tracker_refs:
- '#19'
- '#21'
planning_base_branch: feat/build-version-injection
merge_target_branch: feat/build-version-injection
branch_strategy: Planning artifacts for this mission were generated on feat/build-version-injection. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/build-version-injection unless the human explicitly redirects the landing branch.
subtasks:
- T012
- T013
- T014
phase: Phase 2 - Release integration
agent: "codex"
shell_pid: "12430"
history:
- at: '2026-07-03T16:30:00Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: curator-carla
authoritative_surface: docs/releases/
create_intent:
- docs/releases/release-notes-0.3.0.md
execution_mode: code_change
model: ''
owned_files:
- docs/releases/release-notes-0.3.0.md
role: curator
tags: []
task_type: implement
---

# Work Package Prompt: WP03 – 0.3.0 breaking-change release notes draft

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile in the frontmatter, and behave per its guidance before parsing the rest of this prompt.

- **Profile**: `curator-carla`
- **Role**: `curator`
- **Agent/tool**: `claude`

If no profile is specified, run `spec-kitty agent profile list` and select the best match.

---

## Markdown Formatting

Wrap HTML/XML tags in backticks. Use language identifiers in code blocks.

## Objective

Produce the curated release-notes draft at `docs/releases/release-notes-0.3.0.md` that documents the removed top-level `version` field as a **breaking change** with clear consumer migration guidance. This is the artifact the 0.3.0 release runbook swaps into the published GitHub Release via `gh release edit --notes-file` (the proven 0.2.0 mechanism) — NOT the workflow's auto-generated notes, which would drop the warning (FR-006, research R7).

**Out of scope**: introducing `CHANGELOG.md` or notes-delivery automation (that is issue #20).

## Branch Strategy

- Planning/base + merge target: `feat/build-version-injection`. This is a `planning_artifact` WP.

## Context

- Authoritative source for the JSON shapes: `contracts/build-output.md` in this mission dir. Match it exactly.
- Spec requirements: FR-005 (top-level `version` removed), FR-006 (published notes document the break), C-005 (target 0.3.0).
- Prior art for tone/structure: the 0.2.0 release notes (`~/spec-kitty-analyzer-issue4-backup/catfood-findings/release-notes-0.2.0.md`) — same sectioned style.

## Subtasks

### T012 — Draft the release notes

**Create** `docs/releases/release-notes-0.3.0.md` with:
1. A one-line summary: binaries now self-report version + commit + build date; local builds report `dev`.
2. An **### Added** section: `version` command and JSON now expose a structured `build` object (`build.version`, `build.commit`, `build.build_date`); release builds stamp real values via ldflags (closes #19, #21).
3. A prominent **### ⚠️ Breaking change** section: the top-level `version` field is **removed** from `analyze` / `query` / `missions` JSON output; consumers must read `.build.version`.

**Validation**: the three sections exist; issue refs present.

### T013 — Before/after JSON snippet

**Steps**:
1. Embed a concrete before/after block copied/adapted from `contracts/build-output.md`:
   ```json
   // before (≤0.2.x):   { "version": "0.2.0", ... }
   // after  (0.3.0):     { "build": { "version": "0.3.0", "commit": "a1b2c3d", "build_date": "..." }, ... }
   ```
2. Add a short migration line: replace any read of top-level `.version` with `.build.version`.

**Validation**: snippet matches the contract shape exactly (nested `build`, no top-level `version`).

### T014 — Accuracy cross-check + release reminder

**Steps**:
1. Diff the draft's claims against `contracts/build-output.md` and spec FR-005/FR-006 — field names, key order, sentinel values must match.
2. Add a short maintainer note at the bottom: *"Release runbook: publish this via `gh release edit v0.3.0 --notes-file …`; do not rely on auto-generated notes (they omit this warning)."*

**Validation**: no discrepancy between the draft and the contract; the runbook reminder is present.

## Definition of Done

- [ ] `release-notes-0.3.0.md` exists with a clear ⚠️ Breaking-change section naming `.version` → `.build.version`.
- [ ] A before/after JSON example consistent with `contracts/build-output.md`.
- [ ] Consumer migration guidance + the `--notes-file` release reminder included.
- [ ] No `CHANGELOG.md` created (deferred to #20).

## Reviewer guidance

- Confirm the JSON examples exactly match the final contract (no drift).
- Confirm the breaking change is unmissable (its own headed section, not a footnote).

## Activity Log

- 2026-07-03T17:36:02Z – claude – shell_pid=10100 – WP03 complete: docs/releases/release-notes-0.3.0.md with breaking-change section, before/after JSON matching contract, migration + --notes-file reminder.
- 2026-07-03T17:36:22Z – codex – shell_pid=10768 – Started review via action command
- 2026-07-03T17:38:16Z – user – shell_pid=10768 – Moved to planned
- 2026-07-03T17:38:35Z – claude – shell_pid=11918 – Started implementation via action command
- 2026-07-03T17:39:12Z – claude – shell_pid=11918 – Cycle-2: JSON examples now valid + contract-matching (codex cycle-1 blocker fixed).
- 2026-07-03T17:39:35Z – codex – shell_pid=12430 – Started review via action command
- 2026-07-03T17:40:53Z – user – shell_pid=12430 – Codex cycle-2 APPROVE: JSON examples valid + contract-matching; all other DoD items confirmed.
