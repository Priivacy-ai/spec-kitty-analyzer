# Implementation Plan: Scope failure detection to real channels

**Branch**: `fix/failure-scan-channel-scoping` | **Date**: 2026-06-26 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `kitty-specs/failure-scan-channel-scoping-01KW2HBG/spec.md`

## Summary

Failure text-pattern rules currently match against the entire flattened event, so
narrative, code edits, and model reasoning that merely *discuss* or *implement* error
handling are reported as real failures (issue #4). The approach: scope each detection
**per regex pattern** to the channel where a genuine occurrence would appear —
`output` (real command/tool output + structured error fields) or `diagnostic`
(`output` plus narrative, reserved for *distinctive* signals), with code-edit and
file-read content universally excluded. The gate for narrative eligibility is
**distinctiveness, not topic**: a pattern earns `diagnostic` only if a prose match is
overwhelmingly a *report of an observed condition*, not discussion of a problem class.
Today only `branch_worktree_confusion` qualifies. The full design (Codex-reviewed) is
at `docs/design/issue-4-failure-scan-channel-scoping.md`.

This is a **structural intervention** (DIRECTIVE_040): the same false-positive class
produced prior point-fixes (#2, #5); channel-scoping closes the class rather than
tuning one more rule.

## Technical Context

**Language/Version**: Go 1.25.0 (per `go.mod`)
**Primary Dependencies**: Go standard library only (`regexp`, `encoding/json`, `strings`). No new third-party dependencies (existing `go-pdf/fpdf` untouched).
**Storage**: N/A — stateless log analysis; discovery cache schema (`cacheVersion`) unchanged.
**Testing**: `go test ./...` (table-driven unit tests in `internal/analyzer/*_test.go`); per-harness golden tests for channel extraction; a main-vs-candidate corpus FN/FP diff harness over the locally cached missions.
**Target Platform**: Cross-platform CLI (linux/darwin/windows × amd64/arm64).
**Project Type**: single (one Go module, `github.com/priivacy-ai/spec-kitty-analyzer`).
**Performance Goals**: ≤10% `analyze` wall-clock overhead on the largest cached mission (NFR-002); one channel-extraction pass per event (avoid `O(rules × event size)`).
**Constraints**: deterministic classification (FR-006); backward-compatible report schema, no `report.version` bump (NFR-003); no new deps (NFR-001).
**Scale/Scope**: ~233 cached missions; ~25 failure rules; transcripts up to ~13 MB.

## Charter Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Paradigms: **deep-module-design**, **specification-by-example**. Relevant directives
and how this plan satisfies them:

- **DIRECTIVE_040 (Recurring-Bug Structural Intervention)** — ✅ Primary driver. The
  narrative-false-positive class recurred (#2, #5 were point-fixes to the same family);
  channel-scoping is the structural fix that closes the class. Structural root cause
  identified (whole-event flatten feeding every rule); dormant masks enumerated (the
  per-rule taxonomy + the FN-regression on `branch_worktree_confusion`).
- **DIRECTIVE_001 / DIRECTIVE_024 (Separation of concerns / Locality)** — ✅ A single
  channel-extraction module (deep module, narrow interface: *text for a scope*) hides
  the harness-shape complexity; change stays within `internal/analyzer/`.
- **DIRECTIVE_034 / DIRECTIVE_036 (Test-First / Black-Box)** — ✅ Tests are authored
  first and assert on observable classification output through `classifyFailures` /
  `analyze`, not internal structure (specification-by-example: the spec's acceptance
  scenarios become tests).
- **DIRECTIVE_039 (Lynn Cole: boring/modular/reviewable, not over-abstracted)** — ✅
  Implementation is a plain per-pattern scope tag + two cached strings + an explicit
  extraction matrix. No new abstraction layers, no cleverness; the anomaly trap (which
  *would* add machinery) is deliberately deferred to a separate mission.
- **DIRECTIVE_003 (Decision Documentation)** — ✅ Design doc + Codex review/debate +
  `research.md` capture decisions and rationale.
- **DIRECTIVE_025 / DIRECTIVE_030 / DIRECTIVE_033** — ✅ Boy-scout touched code; local
  `go build && go vet && go test ./...` green before merge; stage only deliverables.

Quality Gate (charter): detection change ⇒ **corpus-validation evidence must be
recorded in the PR**, behavior-validated against real Spec Kitty logs (not only
fixtures). Captured as a Phase-1 deliverable (quickstart + the diff harness).

No charter conflicts identified. Re-check after Phase 1: no new gates triggered.

## Project Structure

### Documentation (this mission)

```
kitty-specs/failure-scan-channel-scoping-01KW2HBG/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output (incl. corpus-validation procedure)
└── contracts/
    └── channel-classification.md   # behavioral contract (not REST)
```

### Source Code (repository root)

```
internal/analyzer/
├── fingerprints.go      # failureRule/pattern types, failureRules table, classifyFailures, genericFailureSignal
├── json_helpers.go      # flattenJSON helpers
├── analyzer.go          # eventFromJSONObject / eventFromText, skipArtifactMessage (ordering)
└── analyzer_test.go     # table-driven unit + golden + regression tests
```

**Structure Decision**: Single Go module; all changes localized to
`internal/analyzer/`. No new packages unless the channel-extraction module reads more
cleanly as its own file within the package (a plain `channels.go` is acceptable; no new
import cycle, no exported surface change).

## Complexity Tracking

*No Charter Check violations. Section intentionally empty.*

## Implementation Concern Map

> Concerns are not work packages. `/spec-kitty.tasks` translates these into WPs.

### IC-01 — Channel extraction (per-harness schema matrix)

- **Purpose**: Produce, for one event, the text belonging to each channel class (`output`, `narrative`) with code-edit/file-read content excluded — the single source the text rules scan.
- **Relevant requirements**: FR-001, FR-004, FR-005.
- **Affected surfaces**: `internal/analyzer/channels.go` (generalized source-read/code-edit exclusion); `analyzer.go` event build.
- **Sequencing/depends-on**: none (foundation).
- **Risks**: harness-shape coverage (Claude message / `toolUseResult` string+struct / `tool_result` blocks / Edit-Write / Read / codex `payload` variants); JSON-string re-decode; unmapped shapes must default to excluded-from-output and be logged, never silently treated as output.

### IC-02 — Per-pattern scope + rule reclassification

- **Purpose**: Tag every failure regex with an explicit channel scope (no default), split mixed rules, and run each pattern only against its scope's text.
- **Relevant requirements**: FR-001, FR-002, FR-003, FR-006.
- **Affected surfaces**: `internal/analyzer/fingerprints.go` (pattern type carries scope; `failureRules` table; `classifyFailures`; `genericFailureSignal`).
- **Sequencing/depends-on**: IC-01.
- **Risks**: regression on `branch_worktree_confusion` (must stay `diagnostic`); correct split of `merge_conflict` / `worktree_linkage_broken`; a build-failing test if any pattern is left unscoped.

### IC-03 — `obj == nil` artifact-vs-transcript model

- **Purpose**: Classify plain-text (non-JSON) lines correctly — artifact/spec kinds as `diagnostic`-only; transcript text and `.log` command logs output-eligible; generic `.txt`/`.md`/`.yaml` unsupported.
- **Relevant requirements**: FR-004.
- **Affected surfaces**: `analyzer.go` (`eventFromText` nil path, source-kind detection, `skipArtifactMessage` interaction).
- **Sequencing/depends-on**: IC-01.
- **Risks**: interaction with existing `skipArtifactMessage` drop ordering (must gate both failures and, later, anomalies before aggregation).

### IC-04 — Caching & ordering

- **Purpose**: Compute `outputText`/`diagnosticText` once per event; define explicit classification ordering (structural → exclusion → text rules → generic fallback).
- **Relevant requirements**: NFR-002, FR-006.
- **Affected surfaces**: `analyzer.go`, `fingerprints.go`.
- **Sequencing/depends-on**: IC-01, IC-02, IC-03.
- **Risks**: avoid recomputation per rule; keep determinism.

### IC-05 — Tests & corpus validation (test-first)

- **Purpose**: Lock behavior with example-based tests and produce the charter-required corpus-validation evidence (both directions).
- **Relevant requirements**: FR-002, FR-003, SC-001..SC-004.
- **Affected surfaces**: `internal/analyzer/analyzer_test.go`; the main-vs-candidate corpus diff harness; `quickstart.md`.
- **Sequencing/depends-on**: authored FIRST per DIRECTIVE_034 for each concern; corpus sweep runs against the built binary.
- **Risks**: golden coverage of all harness shapes; the explicit-scope build test; the output-scoped-narrative negative (merge_* prose must not classify).
