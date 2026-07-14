# Implementation Plan: Curated CHANGELOG & Release Notes Pipeline

**Branch**: `feat/changelog-release-pipeline` | **Date**: 2026-07-14 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `kitty-specs/changelog-release-pipeline-01KXF0ZK/spec.md`

## Summary

Adopt spec-kitty's curated-CHANGELOG release-notes mechanism — a hand-authored `CHANGELOG.md`,
a validator that gates release-metadata changes, and an extractor that feeds the GitHub Release
body — scoped to this single-maintainer Go CLI whose version source of truth is the git tag
(ldflags-injected via #23). Technical approach: a standalone `tools/release` Go program (stdlib
only) exposing `extract` and `validate` subcommands over `CHANGELOG.md`; a `release-readiness`
GitHub Actions workflow that runs the validator on release-metadata PRs, nightly, and on manual
dispatch; and an edit to `release.yml` that extracts the tagged section as the Release body and
asserts triple consistency (binary `build.version` == tag == extracted section) before publishing.

## Technical Context

**Language/Version**: Go 1.25.0 (module `github.com/priivacy-ai/spec-kitty-analyzer`)
**Primary Dependencies**: Go standard library only (`os`, `os/exec` for `git tag`, `regexp`,
`bufio`/`strings`, `flag`); no third-party modules — `go.mod` gains 0 `require` entries.
**Storage**: Files — `CHANGELOG.md` at repo root; git tags as the version registry (read via
`git tag --list 'v*.*.*'`).
**Testing**: `go test ./tools/release` — table-driven unit tests over parsing/extraction/
validation; runs under the existing `ci.yml` `go test -race ./...`.
**Target Platform**: CI runners (ubuntu-latest) + maintainer workstation; the program must
cross-compile on the six existing GOOS/GOARCH targets since `go build ./...` and the cross-build
smoke enumerate it.
**Project Type**: single (Go CLI repo + a new standalone `tools/` program + GitHub Actions).
**Performance Goals**: N/A — release-time / PR-time tooling; runs in well under a second on a
changelog of tens of KB. No hot path.
**Constraints**: No new CI language/runtime (Go already set up in both workflows); stdlib-only;
`tools/release` must not import `internal/analyzer` (keep maintainer infra decoupled from product
code and out of the shipped binary); edits to `release.yml` must stay on lines disjoint from the
concurrently-open PR #30 (Node-24 action bumps) so the branch rebases cleanly.
**Scale/Scope**: One `CHANGELOG.md`; ~4 seeded version sections + Unreleased; ~4–6 small Go
source files + tests; 1 new workflow; 1 edited workflow; 1 checklist doc.

## Charter Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **DIR-013 (comprehensive customer-facing documentation, maintain a CHANGELOG to spec-kitty's
  standards)** — this mission directly fulfills it: introduces the Keep a Changelog `CHANGELOG.md`
  and the RELEASE_CHECKLIST runbook. ✅ Advances the directive.
- **DIR-012 (privacy / local-first / no exfiltration)** — no analogue risk: `tools/release`
  reads only `CHANGELOG.md` and local git tags, writes only stdout / a Release body; ingests no
  user logs, performs no redaction-relevant work, and makes no network calls. ✅ No conflict.
- **Operating standard (evidence-based, enterprise-grade, no guesses)** — the design mirrors a
  proven mechanism (spec-kitty's), is `go test`-covered, and is validated against the real repo
  (actual tags, actual curated 0.2.0/0.3.0 notes). ✅ Consistent.
- **Detection-precision directives** — not applicable; this mission touches no detection logic,
  patterns, or corpora.

No charter violations. No Complexity Tracking entries required.

## Project Structure

### Documentation (this mission)

```
kitty-specs/changelog-release-pipeline-01KXF0ZK/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output (entities: version, changelog section, tag)
├── quickstart.md        # Phase 1 output (how to run extract/validate locally + release flow)
├── contracts/           # Phase 1 output (CLI contracts for extract + validate)
└── tasks/               # WP files (created later by /spec-kitty.tasks)
```

### Source Code (repository root)

```
CHANGELOG.md                         # NEW — Keep a Changelog; Unreleased + 0.3.0/0.2.0/0.1.1/0.1.0
RELEASE_CHECKLIST.md                 # NEW — scoped tag-as-SSOT release runbook

tools/release/                       # NEW — standalone Go program (package main), not shipped
├── main.go                          #   CLI dispatch: extract | validate; flag parsing; exit codes
├── changelog.go                     #   heading parse, section extract, populated/empty, top version
├── version.go                       #   SemVer parse (stable+prerelease), compare, tag<->version parity
├── git.go                           #   discover release tags via `git tag --list 'v*.*.*'`
├── changelog_test.go                #   table tests: parse/extract/populated/missing-default
├── version_test.go                  #   table tests: parse/compare/monotonic/parity/prerelease
└── main_test.go                     #   subcommand dispatch + exit-code behavior (optional)

.github/workflows/
├── release.yml                      # EDIT — add extractor step + triple-consistency guard;
│                                    #        set body_path; remove generate_release_notes
├── release-readiness.yml            # NEW — validator gate on PRs (metadata paths) + nightly + dispatch
└── ci.yml                           # EDIT — extend cross-build smoke to also build ./tools/release
                                     #        (six targets); go test ./... already covers its tests
```

> Post-plan Codex review (folded into research R2/R3/R4/R10/R11 and spec FR-004–FR-010) tightened
> four things the initial plan got wrong or vague: branch-mode monotonicity must be **state-aware**
> (else routine post-release PRs fail); `fetch-depth: 0` is required in both workflows; the
> `body_path` file must exist on non-tag runs; and the cross-build smoke must be **extended** to
> cover `tools/release` (it did not). Details in each artifact.

**Structure Decision**: Single Go module. Release tooling is isolated in a standalone
`tools/release` `package main` so it (a) is not linked into the shipped `spec-kitty-analyzer`
binary, (b) mirrors spec-kitty's standalone scripts, and (c) is still compiled and tested by the
existing `go build ./...` / `go test ./...` / cross-build-smoke gates. `CHANGELOG.md` and
`RELEASE_CHECKLIST.md` live at repo root (conventional, and where tooling + humans expect them).

## Implementation Concern Map

### IC-01 — Curated CHANGELOG content

- **Purpose**: Author the source-of-record `CHANGELOG.md` (Keep a Changelog + SemVer) seeded with
  the existing releases and the unreleased 0.3.0, so the extractor has real sections to serve.
- **Relevant requirements**: FR-001, FR-002; C-001, C-004.
- **Affected surfaces**: `CHANGELOG.md` (new).
- **Sequencing/depends-on**: none (content authoring; independent of the Go code). The heading
  format it establishes is the contract the parser in IC-02 must match, so agree the exact heading
  grammar (`## [X.Y.Z] - YYYY-MM-DD`, `## [Unreleased]`, bottom link refs) in research first.
- **Risks**: Getting the 0.3.0 breaking-change notice faithfully transcribed from
  `docs/releases/release-notes-0.3.0.md`; correct historical dates from tags; heading grammar
  drift vs the parser.

### IC-02 — Extractor + validator (Go `tools/release`)

- **Purpose**: Implement the `extract <version>` and `validate --mode branch|tag` commands and
  their SemVer/changelog/git-tag core, stdlib-only, `go test`-covered.
- **Relevant requirements**: FR-003, FR-004, FR-005; NFR-001, NFR-002, NFR-003, NFR-004; C-001.
- **Affected surfaces**: `tools/release/*.go`.
- **Sequencing/depends-on**: shares the heading grammar with IC-01 (fix grammar in research);
  otherwise independent. Provides the binary consumed by IC-03.
- **Risks**: Heading-regex false matches (Unreleased, link-ref lines) AND the inverse — a malformed
  `## [...]` heading must **error, not silently skip** (Codex R5); **state-aware** branch
  monotonicity (`V==T` inter-release must pass — Codex R4); tag-mode self-exclusion + parity;
  compact-only prerelease grammar (Codex R9); `extract` must never call git; `validate` degrades
  with a clear error outside a git tree; cross-build smoke must be extended to cover `tools/release`
  (Codex R6) since `go build ./...` only covers the runner platform.

### IC-03 — Release + readiness workflow wiring

- **Purpose**: Wire the extractor into `release.yml` (Release body via `body_path`, drop
  `generate_release_notes`, add the triple-consistency guard) and add `release-readiness.yml`
  (validator on release-metadata PRs + nightly cron + dispatch tag mode).
- **Relevant requirements**: FR-006, FR-007, FR-009; C-002, C-003, C-005; SC-001, SC-002,
  SC-003, SC-006.
- **Affected surfaces**: `.github/workflows/release.yml` (edit), `.github/workflows/release-readiness.yml` (new).
- **Sequencing/depends-on**: depends on IC-02 (needs the built commands). Must land on release.yml
  lines disjoint from PR #30.
- **Risks**: `body_path` must reference a file that **always exists** — create `RELEASE_NOTES.md`
  empty on non-tag runs since the upload step is unconditional (Codex R3); extraction + triple-check
  run on tag builds only; both `release.yml` and `release-readiness.yml` need `fetch-depth: 0` for a
  complete tag set (Codex R2); the `awk` binary-version read must fail closed on empty/non-version
  output (Codex R7); readiness path filter must include `release.yml` (Codex R8); PR #30 overlap.

### IC-04 — Release runbook

- **Purpose**: Document the scoped, tag-as-SSOT release procedure end to end.
- **Relevant requirements**: FR-008.
- **Affected surfaces**: `RELEASE_CHECKLIST.md` (new).
- **Sequencing/depends-on**: depends on IC-01–IC-03 being designed (documents their usage); content
  can be finalized last.
- **Risks**: Drift from the actual commands/flags — must cite the real `tools/release` invocations
  and the validator's modes verbatim.
