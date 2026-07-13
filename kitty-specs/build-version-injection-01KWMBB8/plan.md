# Implementation Plan: Build Version & Metadata Injection

**Branch**: `feat/build-version-injection` | **Date**: 2026-07-03 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `kitty-specs/build-version-injection-01KWMBB8/spec.md`

## Summary

Replace the hand-edited `const Version` with build-time-injected package variables (`Version`, `Commit`, `BuildDate`), model them as a single `Build` value, and surface that value as a nested `build` object in the `version` command and in all JSON output. The release workflow injects real values via `-ldflags -X`; local builds fall back to sentinel defaults. The top-level `version` JSON field is removed (C1), a breaking output-schema change shipping in **0.3.0**.

## Technical Context

**Language/Version**: Go 1.25.0 (single module `github.com/priivacy-ai/spec-kitty-analyzer`)
**Primary Dependencies**: Standard library only (`encoding/json`, `flag`, `fmt`); no third-party runtime deps introduced. Build/CI: `go build`, `-ldflags -X`, GitHub Actions `release.yml`.
**Storage**: N/A (build metadata is compiled into the binary; no persistence)
**Testing**: `go test ./...` (existing suite, 6 test files) plus new unit coverage asserting: (a) sentinel defaults for an un-injected build, (b) `Build` value composition, (c) JSON emits a nested `build` object and no top-level `version` across all three emitters. **Existing tests reading `Version` directly must be migrated** to `Build` (`internal/query/query_test.go`, `internal/analyzer/analyzer_test.go`), and a new `missions` cmd test added.
**Target Platform**: 6 release targets — {linux, darwin, windows} × {amd64, arm64}
**Project Type**: single (Go CLI + internal packages)
**Performance Goals**: N/A (no runtime hot path affected; injection is compile-time)
**Constraints**: `-ldflags -X` symbol path MUST be the lowercase module path `github.com/priivacy-ai/spec-kitty-analyzer/internal/analyzer` (a wrong path silently no-ops — C-002); injection applied to BOTH the windows `.exe` and non-windows build invocations (C-003); stamping gated on `GITHUB_REF_TYPE == 'tag'` with sentinel fallback for manual dispatch (C-006); `build_date` in UTC ISO-8601 (C-004); `go test ./...` stays green (NFR-001).
**Scale/Scope**: ~4 source files touched (`internal/analyzer/types.go`, `cmd/spec-kitty-analyzer/main.go`, `internal/query/query.go`, `.github/workflows/release.yml`) + tests + a 0.3.0 breaking-change note.

## Charter Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Charter present (`.kittify/doctrine`); paradigms **deep-module-design** + **specification-by-example**. Relevant directives:

- **DIRECTIVE_001 (Architectural Integrity) / deep-module-design** — ✅ The `Build` struct is a single cohesive module for provenance (one type, one source of truth), rather than three loose fields threaded through call sites. Directly satisfied by the nested-`build` design.
- **DIRECTIVE_024 (Locality of Change)** — ✅ Change stays in the version/build surface: the analyzer types, the three JSON emitters, the `version` command, and the release workflow. No unrelated refactoring.
- **DIRECTIVE_003 (Decision Documentation)** — ✅ The C1 breaking-change decision and the 0.3.0 SemVer rationale are recorded in spec C-005 and this plan's research.
- **DIRECTIVE_025 (Boy Scout Rule)** — Any touched-file test/lint failure gets fixed in-scope; no domain-adjacent debt folding identified.
- **specification-by-example** — ✅ Contracts (`contracts/build-output.md`) express the JSON shape as concrete before/after examples; quickstart gives an executable verification.

No violations → Complexity Tracking not required.

## Project Structure

### Documentation (this mission)

```
kitty-specs/build-version-injection-01KWMBB8/
├── plan.md              # This file
├── research.md          # Phase 0 output — key technical decisions
├── data-model.md        # Phase 1 output — the Build entity
├── quickstart.md        # Phase 1 output — how to verify injection locally + in CI
├── contracts/
│   └── build-output.md  # Phase 1 output — JSON build-object contract (before/after)
└── tasks.md             # Phase 2 output (/spec-kitty.tasks — NOT created here)
```

### Source Code (repository root)

```
cmd/spec-kitty-analyzer/
└── main.go              # `version` command output (FR-001); missions JSON build object (FR-002, FR-005)
internal/analyzer/
├── types.go             # const Version -> vars Version/Commit/BuildDate + Build struct + CurrentBuild() (FR-003, FR-004, C-001)
└── analyzer.go          # Report construction — set Build instead of Version (FR-002, FR-005)
internal/query/
└── query.go             # QueryResult carries the Build object through (FR-002, FR-005)
.github/workflows/
└── release.yml          # -ldflags -X injection on both build lines (FR-003, C-001..C-004)
```

**Structure Decision**: Single Go module, no new packages. The `Build` type and its `CurrentBuild()` constructor live in `internal/analyzer` (the existing home of `Version`), keeping provenance ownership in one place; the three JSON emitters and the CLI consume it.

## Complexity Tracking

*No Charter Check violations — section intentionally empty.*

## Implementation Concern Map

> Concerns, not work packages. `/spec-kitty.tasks` decomposes these into WPs.

### IC-01 — Build provenance model (source of truth)

- **Purpose**: Replace `const Version` with injectable `var Version/Commit/BuildDate` (defaults `dev`/`none`/`unknown`) and a `Build` struct + `CurrentBuild()` constructor, so provenance is one cohesive value.
- **Relevant requirements**: FR-003, FR-004; C-001
- **Affected surfaces**: `internal/analyzer/types.go`
- **Sequencing/depends-on**: none (foundation)
- **Risks**: Changing `const`→`var` is safe (no const-context use); defaults must read as unambiguously non-release.

### IC-02 — JSON surfacing (nested `build`, top-level `version` removed)

- **Purpose**: Emit provenance as a nested `build` object across the `analyze`, `query`, and `missions` outputs; remove the top-level `version` field (breaking, C1).
- **Relevant requirements**: FR-002, FR-005
- **Affected surfaces**: `internal/analyzer/types.go` (Report), `internal/analyzer/analyzer.go`, `cmd/spec-kitty-analyzer/main.go` (missions struct), `internal/query/query.go`
- **Sequencing/depends-on**: IC-01
- **Risks**: Must catch ALL three emitters (missing one leaves an inconsistent schema); JSON field ordering/tags; this is the breaking surface — verify no residual top-level `version`.

### IC-03 — `version` command output

- **Purpose**: Print version, commit, and build date in one human-readable line.
- **Relevant requirements**: FR-001
- **Affected surfaces**: `cmd/spec-kitty-analyzer/main.go` (version case)
- **Sequencing/depends-on**: IC-01
- **Risks**: Low; keep format stable and greppable.

### IC-04 — Release-time injection (tag-gated)

- **Purpose**: Inject real version, short commit, and UTC ISO-8601 date via `-ldflags -X` on both the windows and non-windows build invocations — **only when the triggering ref is a tag**; non-tag runs leave the sentinels.
- **Relevant requirements**: FR-003; C-001, C-002, C-003, C-004, **C-006**
- **Affected surfaces**: `.github/workflows/release.yml`
- **Sequencing/depends-on**: IC-01 (symbols must exist)
- **Risks**:
  - **The lowercase module-path footgun (C-002)** — wrong path fails silently. Both build lines must be updated identically. Verify by grepping the injected output.
  - **Manual-dispatch stamping (C-006, Codex HIGH-1)** — `release.yml` also allows `workflow_dispatch`; `GITHUB_REF_NAME` is then a branch name. Gate the version/commit/date computation on `GITHUB_REF_TYPE == 'tag'` and fall back to sentinels otherwise, or a manual run brands the binary with a branch name (violates INV-2).

### IC-05 — Breaking-change documentation + tests

- **Purpose**: Document the removed top-level `version` field as a breaking change for the 0.3.0 release, and cover the new behavior with tests.
- **Relevant requirements**: FR-006; NFR-001, NFR-003
- **Affected surfaces**:
  - **Tests**: add sentinel-default + JSON-shape tests for all three emitters; **update existing tests that read `Version` directly** — `internal/query/query_test.go` (~L10) and `internal/analyzer/analyzer_test.go` (~L245) will fail to compile once `Version`→`Build` (Codex MEDIUM-3); add a `cmd/spec-kitty-analyzer` test for the `missions` JSON.
  - **Docs**: a curated `docs/releases/release-notes-0.3.0.md` draft carrying the `.version` → `.build.version` migration note (delivered to the published release via the `--notes-file` swap at release time — Codex HIGH-2); PR-body callout. Formal `CHANGELOG.md` adoption + notes automation is deferred to issue #20.
- **Sequencing/depends-on**: IC-02, IC-03
- **Risks**: Tests must assert the *absence* of top-level `version`, not just the presence of `build`, to actually guard the breaking contract. Release runbook must use `--notes-file`, not auto-notes, or the warning is dropped.
