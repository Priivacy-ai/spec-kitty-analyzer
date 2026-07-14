# Feature Specification: Curated CHANGELOG & Release Notes Pipeline

**Mission**: changelog-release-pipeline-01KXF0ZK
**Mission type**: software-dev
**Status**: Draft
**Created**: 2026-07-14
**Related issue**: #20 · Charter directive: DIR-013

## Summary

Adopt the release-notes mechanism spec-kitty uses on itself — a hand-curated
`CHANGELOG.md`, a validator that gates release-metadata changes, and an extractor that
feeds the GitHub Release body — **scoped** to this single-maintainer Go CLI whose version
source of truth is the git tag (ldflags-injected, shipped in #23). Today the analyzer has
no `CHANGELOG.md`; release notes are hand-pasted onto the GitHub Release after tagging and
the workflow's `generate_release_notes: true` auto-list is manually discarded. This mission
replaces that manual, error-prone step with an in-repo, CI-validated, automatically-extracted
pipeline.

## User Scenarios & Testing

### Primary scenario — preparing and shipping a release
1. **Maintainer** lands feature/fix PRs; each PR that changes shipped behavior adds an entry
   under `## [Unreleased]` in `CHANGELOG.md`.
2. When ready to release, the maintainer renames `## [Unreleased]` to `## [X.Y.Z] - <date>`
   (and opens a fresh empty `## [Unreleased]`), on a release-prep PR.
3. CI (`release-readiness`) runs the validator in **branch mode**: the top released section is
   well-formed SemVer, populated, and strictly greater than the latest existing tag. The PR
   cannot merge if any check fails.
4. After merge, the maintainer tags `vX.Y.Z` and pushes it.
5. The `release` workflow builds artifacts, runs the extractor for the tag's version, and
   publishes the extracted `## [X.Y.Z]` section as the GitHub Release body — no manual paste.
6. Before publishing, the workflow asserts **triple consistency**: the built binary's
   self-reported `build.version` equals the tag version equals the CHANGELOG section that was
   extracted. If any of the three disagree, the release build fails rather than shipping a
   mislabeled release.

### Exception — release without a changelog entry
- The maintainer tags `vX.Y.Z` but `CHANGELOG.md` has no populated `## [X.Y.Z]` section. The
  release workflow runs `validate --mode tag` before publishing; with no populated matching
  section the validator **fails and the release build stops** (FR-009 / SC-006) — no Release is
  published. (The `extract` command's default-body fallback exists only as a defense-in-depth
  safety net for the body text; it never substitutes for the failing gate.)

### Exception — routine PR between releases (no monotonic gate)
- A normal PR adds an entry under `## [Unreleased]` without promoting a new version. Here the
  top **released** section equals the latest tag (e.g. `[0.2.0]` with tag `v0.2.0`). Branch-mode
  validation MUST treat this as an inter-release state: it checks structure and that released
  sections stay populated, but MUST NOT fail for "version does not advance" — the monotonic gate
  applies only when a new version is actually being prepared (top released version > latest tag).

### Exception — tag/version mismatch
- The pushed tag `vX.Y.Z` does not match the top released heading in `CHANGELOG.md`. The
  validator in tag mode fails with a clear message naming both values.

### Edge cases
- `## [Unreleased]` is never treated as "the version being released" for tag parity — only a
  concrete `## [X.Y.Z]` heading is.
- Link-reference lines (`[X.Y.Z]: https://…/compare/…`) at the bottom must not be mistaken for
  version headings.
- A version heading whose body is empty (only whitespace before the next heading) counts as
  **not populated** and fails validation when that section is the version being released/prepared.
- A bracketed heading (`## [...]`) whose content is neither `Unreleased` nor a valid version
  (e.g. `## [0.3]`, `## [v0.3.0]`) is a **hard validation error**, not a silently-skipped line —
  otherwise a typo'd heading could hide the real top section.
- Prerelease headings in the compact form (`X.Y.ZrcN`, `X.Y.ZaN`, `X.Y.ZbN`) are accepted by the
  parser even though the release flow ships stable versions only for now. (The dotted `-rc.N`
  spelling is NOT accepted — the reference tool uses compact only, and accepting both would create
  a parity-comparison ambiguity for no benefit while prereleases are out of scope.)
- A `workflow_dispatch` (non-tag) run of the `release` workflow must not attempt extraction, must
  not stamp a version, and must not fail the unconditional upload step on a missing body file —
  the Release-body file MUST always exist (empty on non-tag builds).

## Domain Language

| Term | Canonical meaning | Avoid |
|------|-------------------|-------|
| **CHANGELOG entry / section** | The block under a single `## [X.Y.Z] - <date>` (or `## [Unreleased]`) heading in `CHANGELOG.md` | "release notes file" |
| **Extractor** | The `tools/release extract <version>` command that prints one section for the GitHub Release body | — |
| **Validator** | The `tools/release validate --mode branch\|tag` command that checks release-metadata readiness | "linter" |
| **Version SSOT** | The git tag (`vX.Y.Z`), injected into the binary via ldflags — there is no in-repo version file | "version constant" |
| **Branch mode / Tag mode** | Validator modes: branch = PR/local readiness; tag = release-time parity | — |

## Requirements

### Functional Requirements

| ID | Requirement | Status |
|----|-------------|--------|
| FR-001 | The repository MUST contain a root `CHANGELOG.md` in Keep a Changelog format with SemVer, containing an `## [Unreleased]` section and dated `## [X.Y.Z] - <date>` sections seeded for the already-released `0.2.0`, `0.1.1`, and `0.1.0`, plus a populated section for the unreleased `0.3.0`. | Draft |
| FR-002 | The `0.3.0` section MUST be sourced from the curated in-repo notes (`docs/releases/release-notes-0.3.0.md`), including its breaking-change notice (top-level `version` → `build.version`). The `0.2.0` section MUST be sourced from the curated `v0.2.0` GitHub Release body. | Draft |
| FR-003 | A standalone Go program under `tools/release` (package main, not shipped in the `spec-kitty-analyzer` binary) MUST provide an `extract <version>` subcommand that prints the `CHANGELOG.md` section for that version to stdout, and a safe default message when no populated section exists. | Draft |
| FR-004 | The same program MUST provide a `validate --mode branch\|tag [--tag vX.Y.Z]` subcommand. Both modes check: (a) the top released section heading is well-formed SemVer and (b) that section is populated. **Branch mode** additionally checks version progression *only when a new version is being prepared* — if the top released version is greater than the latest existing `v*.*.*` tag it MUST be strictly greater (release-prep); if it equals the latest tag it is an accepted inter-release state (no monotonic failure); if it is less than the latest tag that is an error (changelog behind tags). **Tag mode** additionally checks (c) `--tag` (default `$GITHUB_REF_NAME`) matches the top released heading and its section is populated, and (d) the version is strictly greater than the latest tag **excluding the tag under release** from the comparison set. | Draft |
| FR-005 | The validator and extractor MUST parse stable (`X.Y.Z`) and compact-prerelease (`X.Y.ZrcN`, `X.Y.ZaN`, `X.Y.ZbN`) version forms (mirroring the reference tool; the dotted `-rc.N` spelling is NOT accepted). They MUST NOT misclassify `## [Unreleased]` or bottom link-reference lines as released version headings, and MUST raise a hard error (not silently skip) on a bracketed `## [...]` heading whose content is neither `Unreleased` nor a valid version. | Draft |
| FR-006 | The `release` workflow MUST, on tag-triggered builds only, run the extractor for the tag's version and publish the extracted section as the GitHub Release body (`body_path`), and MUST remove `generate_release_notes: true`. The Release-body file referenced by `body_path` MUST always exist so the unconditional upload step never fails — on non-tag `workflow_dispatch` runs it is created empty and no extraction runs. | Draft |
| FR-007 | A `release-readiness` GitHub Actions workflow MUST run the validator in branch mode on pull requests that touch release-control paths (`CHANGELOG.md`, `tools/release/**`, `.github/workflows/release.yml`, `.github/workflows/release-readiness.yml`), on a nightly schedule, and on manual dispatch (tag mode when a tag input is supplied), failing the check when validation fails. The checkout MUST fetch all tags (`fetch-depth: 0`) so the tag set the validator reads is complete. | Draft |
| FR-008 | A root `RELEASE_CHECKLIST.md` MUST document the scoped, tag-as-SSOT release procedure: update the changelog, run the validator locally, merge, tag and push, and confirm the extracted Release body. | Draft |
| FR-009 | On tag-triggered builds, the `release` workflow MUST assert triple consistency before publishing: the built binary's self-reported `build.version` == the tag version (`${GITHUB_REF_NAME#v}`) == the version whose CHANGELOG section was extracted. Any disagreement MUST fail the build. This extends the existing dev-sentinel footgun guard to leverage the analyzer's ldflags-injected build provenance. The binary version is read from the `version` command's output (field 2); the guard MUST fail closed if that read yields an empty or non-version-shaped value (defends against the `version` output format drifting). | Draft |
| FR-010 | The `release.yml` checkout MUST fetch all tags (`fetch-depth: 0`) so the tag-mode `validate` monotonic check reads the complete tag set; a shallow checkout that omits tags MUST NOT be relied upon. | Draft |

### Non-Functional Requirements

| ID | Requirement | Threshold | Status |
|----|-------------|-----------|--------|
| NFR-001 | The `tools/release` program MUST introduce no new CI language/runtime beyond Go already used by both workflows, and MUST NOT add third-party Go module dependencies (standard library only). | 0 new runtimes; 0 new `require` entries in `go.mod` | Draft |
| NFR-002 | `tools/release` MUST compile cleanly under `go build ./...` on the CI runner and MUST be pure portable Go (no build tags, cgo, or GOOS-specific code) so it is platform-agnostic. The existing cross-build smoke step MUST be extended to also build `./tools/release` for the six targets (linux/darwin/windows × amd64/arm64) so the claim is verified, not assumed. | 6/6 targets build; `go build ./...` clean | Draft |
| NFR-003 | The extractor and validator core logic MUST be covered by `go test` table tests exercising parsing, extraction, populated/empty detection, monotonicity, tag parity, prerelease forms, and the missing-section default. | ≥ 1 test per FR-003/004/005 behavior; `go test ./tools/release` passes | Draft |
| NFR-004 | Validator failures MUST print an actionable message naming the offending value(s) and exit non-zero; successes exit zero with a concise summary. | Non-zero exit on every failure path | Draft |

### Constraints

| ID | Constraint | Status |
|----|------------|--------|
| C-001 | Version SSOT remains the git tag (ldflags). No in-repo version file is introduced or synced; branch-mode "version being prepared" is derived from the top populated `## [X.Y.Z]` heading. | Draft |
| C-002 | Scope excludes spec-kitty-ecosystem checks with no analogue here: pyproject/`.kittify/metadata.yaml`/`uv.lock` version-sync, shared-package drift, SaaS consumer-contract, PyPI publish + `--pre` semantics, migration-target coverage. | Draft |
| C-003 | The PR gate fires only on release-metadata paths, NOT on every `internal/`/`cmd/` source PR. A source-touching changelog gate is explicitly deferred (issue #20 item 3). | Draft |
| C-004 | The release flow ships stable `0.x` versions only for now; prerelease parsing is supported but prerelease publishing is out of scope. | Draft |
| C-005 | `release.yml` edits must remain compatible with the concurrently-open PR #30 (Node-24 action bumps, different lines); rebase after #30 merges rather than conflicting. | Draft |

## Success Criteria

- SC-001: A maintainer can produce a GitHub Release whose body is the curated changelog
  section with **zero** manual copy-paste after pushing a tag.
- SC-002: A release-prep pull request that forgets or leaves empty the version's changelog
  entry is blocked by CI before merge.
- SC-003: A tag that does not match the changelog's top released version is reported as a
  failure at validation time, not discovered after publish.
- SC-004: The `0.3.0` release (the first through this pipeline) publishes with its
  breaking-change notice intact, extracted automatically from `CHANGELOG.md`.
- SC-005: No new CI runtime is introduced and no third-party Go dependency is added.
- SC-006: A tag whose value disagrees with the built binary's `build.version` or the extracted
  CHANGELOG section fails the release build before any Release is published.

## Key Entities

- **`CHANGELOG.md`** — the curated source of record for human-facing release notes; one
  section per version plus `[Unreleased]`.
- **`tools/release`** — the standalone Go program exposing `extract` and `validate`.
- **`release-readiness.yml`** — the PR/nightly/dispatch gate running the validator.
- **`release.yml`** — the tag-triggered build+publish workflow, now extraction-driven.
- **`RELEASE_CHECKLIST.md`** — the human runbook for the scoped release procedure.

## Assumptions

- The git tag remains the version SSOT (settled by shipped code in #23/#19/#21).
- Prerelease publishing, and a source-touching changelog PR-gate, are deferred, not rejected —
  the design keeps the door open (prerelease parsing present; readiness workflow structured to
  add path filters later).
- Dates for the seeded released sections come from the corresponding tag/commit dates.
- The mission lands before `v0.3.0` is cut, so `0.3.0` debuts the pipeline.

## Out of Scope

- Publishing to any package registry (PyPI, Homebrew, etc.).
- Auto-generating changelog entries from commits (entries are authored by the PR author).
- A changelog PR-gate keyed on `internal/`/`cmd/` source changes (deferred).
- Prerelease (`rc`/`alpha`/`beta`) release publishing.
