# Tasks: Tier-3 Unclassified-Anomaly Trap

**Mission**: tier3-anomaly-trap-01KXF8KZ | **Branch**: `feat/tier3-anomaly-trap`
**Input**: plan.md, spec.md, research.md, data-model.md, contracts/anomaly-matrix.md

3 work packages, ownership-clean (no overlapping `owned_files`), mirroring the codex-read mission's core→wiring→validation shape. Post-plan Codex review (READY-WITH-CHANGES) is folded into the design these WPs implement.

## Subtask Index

| ID | Description | WP | Parallel |
|----|-------------|----|----------|
| T001 | `anomalyCandidate` + residual signal detector (top-level `exit_status`, crash sigs) | WP01 | |
| T002 | Signature-hash normalization + full-digest hash over (channel,tool,kind,token) | WP01 | [P] |
| T003 | Ignore registry (`ignoredAnomalySignatures`) | WP01 | [P] |
| T004 | `Anomaly`/`AnomalyEvidence` types + `Report.Anomalies` + `anomalyCandidates` on `TimelineEvent` | WP01 | |
| T005 | `buildAnomalies` aggregation (group by hash, sort, drop ignored) | WP01 | |
| T006 | Unit/golden tests for detector, hash, registry, aggregation (test-first) | WP01 | |
| T007 | Stash candidates at `parseFile` single-object JSON branch (residual-only + `!isArtifactKind`) | WP02 | |
| T008 | Stash candidates at `parseFile` scanner-loop branch (obj + text) | WP02 | |
| T009 | Wire `buildAnomalies` at both recompute sites; normalize `Anomalies` to `[]` | WP02 | |
| T010 | Integration tests: residual-only, non-artifact, narrative/read exclusion, segregation, both paths (test-first) | WP02 | |
| T011 | Assemble frozen corpus fixtures (`testdata/anomaly/**`) | WP03 | |
| T012 | Corpus additivity test (findings unchanged) + genuine-anomaly assertion | WP03 | |
| T013 | Report-contract design doc + promote/refine/ignore loop | WP03 | [P] |

## Work Packages

### WP01 — Anomaly module core (detector, types, hash, ignore registry)
- **Goal**: The self-contained Tier-3 module — a pure residual detector, the `Anomaly` types, the signature hasher, the ignore registry, and `buildAnomalies` — plus unit/golden tests. Foundational; no pipeline wiring yet.
- **Priority**: MVP. **Depends on**: none.
- **Independent test**: `go test ./internal/analyzer/ -run Anomaly` green; detector fires on P1–P5, stays silent on detector-level negatives; hash deterministic; grouping + ignore work.
- **Subtasks**: T001, T002, T003, T004, T005, T006
- **Owned files**: `internal/analyzer/anomaly.go`, `internal/analyzer/anomaly_test.go`, `internal/analyzer/types.go`
- **Est. prompt**: ~230 lines

### WP02 — Emission + report wiring
- **Goal**: Stash candidates at both `parseFile` append sites (residual-only + non-artifact, post-gate), wire `buildAnomalies` into both recompute paths, normalize the new field. Integration tests for the gate/segregation invariants.
- **Priority**: core. **Depends on**: WP01.
- **Independent test**: `go test ./internal/analyzer/` green; an `exit_status`/crash event yields an anomaly; a Tier-1/Tier-2 event and an artifact event yield none; `findings`/counts unchanged; mission-filtered reports carry anomalies.
- **Subtasks**: T007, T008, T009, T010
- **Owned files**: `internal/analyzer/analyzer.go`, `internal/analyzer/normalize.go`, `internal/analyzer/anomaly_wiring_test.go`
- **Est. prompt**: ~210 lines

### WP03 — Frozen-corpus validation + report-contract docs
- **Goal**: Committed corpus fixtures proving additivity (findings byte-identical) and a genuine anomaly set (no anomaly shares an event with a finding); the report-contract design doc documenting the `anomalies` schema and the promote/refine/ignore loop.
- **Priority**: validation. **Depends on**: WP01, WP02.
- **Independent test**: `go test ./internal/analyzer/ -run AnomalyCorpus` green; the design doc describes the additive schema + loop.
- **Subtasks**: T011, T012, T013
- **Owned files**: `internal/analyzer/anomaly_corpus_test.go`, `internal/analyzer/testdata/anomaly/**`, `docs/design/tier3-anomaly-trap.md`
- **Est. prompt**: ~180 lines

## Dependencies

- WP01 → (none)
- WP02 → WP01
- WP03 → WP01, WP02

## MVP

WP01 is the MVP core (the module is unit-testable in isolation). WP02 makes it live; WP03 proves it on a corpus and documents the contract.
