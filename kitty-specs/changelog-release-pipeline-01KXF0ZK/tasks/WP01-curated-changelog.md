---
work_package_id: WP01
title: Curated CHANGELOG.md
dependencies: []
requirement_refs:
- FR-001
- FR-002
tracker_refs: []
planning_base_branch: feat/changelog-release-pipeline
merge_target_branch: feat/changelog-release-pipeline
branch_strategy: Planning artifacts for this mission were generated on feat/changelog-release-pipeline. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/changelog-release-pipeline unless the human explicitly redirects the landing branch.
subtasks:
- T001
- T002
- T003
- T004
agent: "claude"
shell_pid: "28991"
shell_pid_created_at: "1783991940.541283"
history:
- 2026-07-14 created (tasks phase)
agent_profile: implementer-ivan
authoritative_surface: CHANGELOG.md
create_intent:
- CHANGELOG.md
execution_mode: code_change
owned_files:
- CHANGELOG.md
role: implementer
tags: []
---

## ⚡ Do This First: Load Agent Profile

Before reading anything else, load your agent profile:

```
/ad-hoc-profile-load implementer-ivan
```

Adopt its identity, governance scope, and boundaries for this work package.

## Objective

Add a root `CHANGELOG.md` (Keep a Changelog + SemVer) that is the human-facing source of record for
release notes and the input the `tools/release` extractor (WP02) reads. Seed it with the existing
releases and the unreleased 0.3.0 from their real curated sources.

## Context

- Fulfills **FR-001, FR-002** and charter **DIR-013**. See `spec.md`, and `research.md` R1 (heading
  grammar) and R7 (seed data).
- The heading grammar is a hard contract with the WP02 parser. Use exactly:
  - `## [Unreleased]` (kept at top, empty group scaffolding OK)
  - `## [X.Y.Z] - YYYY-MM-DD` for released sections
  - Bottom link-reference lines: `[X.Y.Z]: https://github.com/priivacy-ai/spec-kitty-analyzer/compare/vPREV...vX.Y.Z`
  - Group changes under `### Added` / `### Changed` / `### Fixed` (NOT `## `) so they are never
    mistaken for version headings.
- Sources of truth (do not invent content):
  - `0.3.0` ← `docs/releases/release-notes-0.3.0.md` (in-repo). MUST include the ⚠️ breaking-change
    notice: top-level `version` removed from JSON → now `build.version`.
  - `0.2.0` ← the curated `v0.2.0` GitHub Release body (Improved/Added/Internal/Known limitations).
    Retrieve with `gh release view v0.2.0 --json body -q .body`.
  - `0.1.1` / `0.1.0` ← reconstruct concisely from `git log v0.1.0` / `v0.1.0..v0.1.1` (initial
    analyzer + early fixes). These predate curated notes — keep short and factual.
- Tag dates (verified): `0.1.0` = 2026-06-20, `0.1.1` = 2026-06-20, `0.2.0` = 2026-07-03. The
  `0.3.0` heading date is a placeholder set when v0.3.0 is cut — use the current date or `- UNRELEASED`
  is NOT allowed (it must be a real dated heading once promoted; for now, since 0.3.0 is the top
  released section being prepared, date it with the mission date `2026-07-14` and note it will be
  refreshed at tag time in the RELEASE_CHECKLIST).

### Subtasks

- **T001** — CHANGELOG.md skeleton: the Keep a Changelog preamble (title + "format based on Keep a
  Changelog, adheres to Semantic Versioning" with links), `## [Unreleased]`, then the version
  sections in descending order, then the bottom link-reference block.
- **T002** — `## [0.3.0] - 2026-07-14`: port `docs/releases/release-notes-0.3.0.md` into
  `### Added` (build provenance #19/#21) and a clearly-marked breaking-change block under
  `### Changed` (top-level `version` → `build.version`, before/after JSON, migration note).
- **T003** — `## [0.2.0] - 2026-07-03`: port the curated v0.2.0 Release body (precision improvements
  #4/#11/#2/#5, recall additions #11A/#11F/#6, internal CI, known limitations).
- **T004** — `## [0.1.1] - 2026-06-20` and `## [0.1.0] - 2026-06-20`: concise factual entries from
  git history (0.1.0 = initial analyzer + install; 0.1.1 = early fixes). Add all bottom link refs.

## Branch Strategy

Planning base and final merge target are both `feat/changelog-release-pipeline` (then PR → `main`).
Execution worktree is allocated per the computed lane in `lanes.json`; work in that lane.

## Definition of Done

- [ ] `CHANGELOG.md` exists at repo root, valid Keep a Changelog structure.
- [ ] Sections `[Unreleased]`, `[0.3.0]`, `[0.2.0]`, `[0.1.1]`, `[0.1.0]` present, each populated.
- [ ] `[0.3.0]` includes the breaking-change notice verbatim in intent from the 0.3.0 notes.
- [ ] Bottom link-reference lines present for every released version.
- [ ] Headings match the grammar in research R1 exactly (verified by eye against the WP02 parser).

## Risks / Reviewer guidance

- Reviewer: confirm no `## ` group headings inside a version section (would confuse the parser);
  confirm dates match the tags; confirm the 0.3.0 breaking notice is intact; confirm content is
  sourced (not invented). Cross-check `tools/release extract 0.3.0` once WP02 exists.

## Activity Log

- 2026-07-14T01:15:46Z – claude – shell_pid=27715 – Assigned agent via action command
- 2026-07-14T01:18:04Z – claude – shell_pid=27715 – CHANGELOG.md authored (seed 0.1.0-0.3.0 + Unreleased), committed in lane-a 60c503c
- 2026-07-14T01:19:04Z – claude – shell_pid=28991 – Started review via action command
