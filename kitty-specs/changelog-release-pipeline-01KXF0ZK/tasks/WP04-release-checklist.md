---
work_package_id: WP04
title: RELEASE_CHECKLIST.md
dependencies:
- WP03
requirement_refs:
- FR-008
tracker_refs: []
planning_base_branch: feat/changelog-release-pipeline
merge_target_branch: feat/changelog-release-pipeline
branch_strategy: Planning artifacts for this mission were generated on feat/changelog-release-pipeline. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/changelog-release-pipeline unless the human explicitly redirects the landing branch.
subtasks:
- T018
- T019
- T020
agent: "claude"
shell_pid: "37079"
shell_pid_created_at: "1783993123.149862"
history:
- 2026-07-14 created (tasks phase)
agent_profile: implementer-ivan
authoritative_surface: RELEASE_CHECKLIST.md
create_intent:
- RELEASE_CHECKLIST.md
execution_mode: code_change
owned_files:
- RELEASE_CHECKLIST.md
role: implementer
tags: []
---

## ⚡ Do This First: Load Agent Profile

```
/ad-hoc-profile-load implementer-ivan
```

Adopt its identity, governance scope, and boundaries before proceeding.

## Objective

Write `RELEASE_CHECKLIST.md`: the maintainer runbook for the scoped, **tag-as-SSOT** release
procedure this mission establishes. Every command must match the shipped `tools/release` CLI and the
workflows exactly (no drift).

## Context

- **FR-008.** Model after `quickstart.md` (this mission) and `~/repos/spec-kitty/scripts/release/README.md`
  (scoped down — no pyproject bump, no PyPI, no uv.lock, no SaaS/shared-package checks).
- Version SSOT is the git tag; there is **no in-repo version file to bump** — the release step is
  "promote Unreleased → dated version heading, validate, merge, tag, push". The workflow does the rest
  (extract Release body + triple-consistency guard).

### Subtasks

- **T018** — Author `RELEASE_CHECKLIST.md`: (1) update `CHANGELOG.md` — rename `## [Unreleased]` to
  `## [X.Y.Z] - YYYY-MM-DD`, open a fresh empty `## [Unreleased]`, add the bottom link ref; (2)
  `go run ./tools/release validate --mode branch` locally; (3) preview with
  `go run ./tools/release extract X.Y.Z`; (4) open release-prep PR (readiness gate runs); (5) after
  merge, `git tag vX.Y.Z -m "Release X.Y.Z" && git push origin vX.Y.Z`; (6) confirm the published
  Release body came from the extractor and the triple-consistency guard passed. Include a short
  "what the automation guarantees" note (binary build.version == tag == CHANGELOG section).
- **T019** — Cross-check every command/flag against the shipped `tools/release` (`--mode`, `--tag`,
  subcommand names, exit-code meaning) and the workflow trigger names. Fix any drift.
- **T020** — If a natural spot exists, add a one-line pointer from `README.md` to
  `RELEASE_CHECKLIST.md` (Releases/Contributing area). If none fits cleanly, skip and note why (do
  NOT force an edit — `README.md` is not in this WP's owned_files; if you touch it, record a one-line
  rationale per the out-of-map-edit rule, or leave it to a follow-up).

## Branch Strategy

Base and merge target `feat/changelog-release-pipeline`; work in the lane from `lanes.json`.

## Definition of Done

- [ ] `RELEASE_CHECKLIST.md` exists at repo root and documents the full scoped release flow.
- [ ] Every command matches the shipped CLI and workflows (verified in T019).
- [ ] No pyproject/PyPI/uv.lock/SaaS steps (those are out of scope for this repo).

## Risks / Reviewer guidance

- Reviewer: run each command in the checklist mentally/really against the shipped tooling; confirm no
  invented flags; confirm the tag-as-SSOT framing (no version-file bump step).

## Activity Log

- 2026-07-14T01:36:34Z – claude – shell_pid=35910 – Assigned agent via action command
- 2026-07-14T01:38:28Z – claude – shell_pid=35910 – RELEASE_CHECKLIST.md done; commands+messages cross-checked vs shipped CLI (T019); T020 README pointer skipped (out-of-owned-files, noted)
- 2026-07-14T01:38:46Z – claude – shell_pid=37079 – Started review via action command
