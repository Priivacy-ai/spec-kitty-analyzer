---
work_package_id: WP03
title: Release + readiness workflow wiring
dependencies:
- WP02
requirement_refs:
- FR-006
- FR-007
- FR-009
- FR-010
- NFR-002
tracker_refs: []
planning_base_branch: feat/changelog-release-pipeline
merge_target_branch: feat/changelog-release-pipeline
branch_strategy: Planning artifacts for this mission were generated on feat/changelog-release-pipeline. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/changelog-release-pipeline unless the human explicitly redirects the landing branch.
subtasks:
- T012
- T013
- T014
- T015
- T016
- T017
agent: "claude"
shell_pid: "33423"
shell_pid_created_at: "1783992639.906285"
history:
- 2026-07-14 created (tasks phase)
agent_profile: implementer-ivan
authoritative_surface: .github/workflows/
create_intent:
- .github/workflows/release-readiness.yml
execution_mode: code_change
owned_files:
- .github/workflows/release.yml
- .github/workflows/release-readiness.yml
- .github/workflows/ci.yml
role: implementer
tags: []
---

## ⚡ Do This First: Load Agent Profile

```
/ad-hoc-profile-load implementer-ivan
```

Adopt its identity, governance scope, and boundaries before proceeding.

## Objective

Wire the `tools/release` program (WP02) into CI: make `release.yml` extract the tagged CHANGELOG
section as the Release body with a triple-consistency guard, add a `release-readiness.yml` PR gate,
and extend `ci.yml`'s cross-build smoke to cover `tools/release`.

## Context — READ THESE FIRST

- **Current `release.yml`**: tag-triggered (`v*`) + `workflow_dispatch`; builds six targets; stamps
  ldflags on tag builds only (has a dev-sentinel "footgun guard"); final `softprops/action-gh-release@v2`
  step runs **unconditionally** with `generate_release_notes: true`.
- **Current `ci.yml`**: has a "Release cross-build smoke" step building only `./cmd/spec-kitty-analyzer`
  for the six targets.
- **Research**: R3 (validate tag mode), R4 (triple-consistency: validate + awk binary read fail-closed
  + body_path always exists), R6 (fetch-depth:0), R11 (extend cross-build). Reference:
  `~/repos/spec-kitty/.github/workflows/release-readiness.yml`.
- **FR-006, FR-007, FR-009, FR-010, NFR-002.**
- **PR #30 overlap**: #30 bumps `actions/*` pins near the top of `release.yml`; keep these edits on the
  build-job steps + the final upload step (disjoint lines). After #30 merges, rebase; expect no/only
  trivial conflict.

### Subtasks

- **T012** — `release.yml`: add `fetch-depth: 0` to the `actions/checkout` step so `git tag --list`
  sees all tags (FR-010).
- **T013** — `release.yml`: ensure `dist/RELEASE_NOTES.md` **always exists** before the upload step.
  On tag builds (`GITHUB_REF_TYPE == tag`): after the build+footgun guard, run
  `go run ./tools/release validate --mode tag --tag "${GITHUB_REF_NAME}"` (fail build on error), then
  `go run ./tools/release extract "${GITHUB_REF_NAME#v}" > dist/RELEASE_NOTES.md`. On non-tag runs:
  `: > dist/RELEASE_NOTES.md` (empty) and skip validate/extract.
- **T014** — `release.yml`: triple-consistency guard (FR-009). Read
  `bin_ver="$(dist/spec-kitty-analyzer_linux_amd64/spec-kitty-analyzer version | awk '{print $2}')"`;
  **fail closed** if `bin_ver` is empty or not `^[0-9]+\.[0-9]+\.[0-9]`; assert
  `bin_ver == "${GITHUB_REF_NAME#v}"` (tag builds only). Then on the upload step set
  `body_path: dist/RELEASE_NOTES.md` and **remove** `generate_release_notes: true`.
- **T015** — new `.github/workflows/release-readiness.yml`: `on: pull_request` (paths:
  `CHANGELOG.md`, `tools/release/**`, `.github/workflows/release.yml`,
  `.github/workflows/release-readiness.yml`), `schedule` nightly cron, and `workflow_dispatch` with an
  optional `tag` input. `actions/checkout@v4` with `fetch-depth: 0`; `actions/setup-go@v5`
  (`go-version-file: go.mod`, cache); run `go run ./tools/release validate --mode branch` on
  PR/nightly, or `--mode tag --tag <input>` when a dispatch tag is given. Fail the job on validator
  failure; emit a short job summary. Scope down spec-kitty's version — no pyproject/uv.lock/metadata.
- **T016** — `ci.yml`: in the cross-build smoke loop, also
  `CGO_ENABLED=0 GOOS=.. GOARCH=.. go build -trimpath -o "$tmpdir/release_${os}_${arch}" ./tools/release`
  for each of the six targets (NFR-002 / R11).
- **T017** — Local verification: `go run ./tools/release validate --mode branch`,
  `... validate --mode tag --tag v0.3.0`, `... extract 0.3.0`; `actionlint`/`yamllint` if available;
  eyeball the tag-only gating and the always-exists body file.

## Branch Strategy

Base and merge target `feat/changelog-release-pipeline`; work in the lane from `lanes.json`.

## Definition of Done

- [ ] `release.yml`: `fetch-depth: 0`; `dist/RELEASE_NOTES.md` created in all paths; validate+extract
      + triple-consistency guard on tag builds; `body_path` set; `generate_release_notes` removed.
- [ ] Guard fails closed on empty/non-version binary output and on any bin/tag/changelog mismatch.
- [ ] `release-readiness.yml`: correct triggers, path filter (incl. `release.yml`), `fetch-depth: 0`,
      branch/tag/dispatch modes, fails on validation failure.
- [ ] `ci.yml` cross-build smoke also builds `./tools/release` for all six targets.
- [ ] Workflows are valid YAML and the referenced commands run locally (T017).

## Risks / Reviewer guidance

- Reviewer: confirm extraction/guard run on **tag builds only**; the body file exists on non-tag
  `workflow_dispatch`; `fetch-depth: 0` present in BOTH workflows; the awk read fails closed; the
  readiness path filter includes `release.yml`; edits stay disjoint from PR #30's action-pin lines.

## Activity Log

- 2026-07-14T01:30:51Z – claude – shell_pid=33423 – Assigned agent via action command
