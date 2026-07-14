# Implementation Plan: Tier-3 Unclassified-Anomaly Trap

**Branch**: `feat/tier3-anomaly-trap` | **Date**: 2026-07-14 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `kitty-specs/tier3-anomaly-trap-01KXF8KZ/spec.md`

## Summary

Add a deterministic, **residual-only** Tier-3 anomaly trap to the analyzer. During per-event classification, when an event produces **no** Tier-1 fingerprint and **no** Tier-2 `generic_error` finding, evaluate a tight set of **genuinely-uncovered** distress signals — a structured `exit_status`≠0 and the output crash signatures `panic:` / `segmentation fault` / `core dumped` — over the **post-#13** output/structured channels only. Matches are emitted as **segregated** `Report.Anomalies`, never as `findings` and never counted in failure totals, each carrying provenance and a channel/kind/token-normalized **signature hash** for grouping. A checked-in **ignore registry** of confirmed-benign hashes suppresses known noise, closing the promote → refine → ignore self-improvement loop. The additive `Anomalies` field ships with **no report schema-version bump** (see research.md D3, reconciling #23's removal of the top-level `version`).

## Technical Context

**Language/Version**: Go 1.25.0 (module `github.com/priivacy-ai/spec-kitty-analyzer`)
**Primary Dependencies**: Standard library only (`encoding/json`, `strings`, `regexp`, `crypto/sha256`). No new dependency.
**Storage**: N/A (in-memory per-file processing)
**Testing**: `go test ./...`; golden **anomaly-matrix** cases (positive + negative per FR); frozen-corpus base-vs-candidate diff proving zero `findings` delta (NFR-001) and a genuine anomaly set (NFR-004).
**Target Platform**: single Go CLI + internal packages
**Project Type**: single
**Performance Goals**: N/A — anomaly evaluation is O(1) per event, computed inline with existing per-event work; no second extraction walk.
**Constraints**: residual-only (no anomaly when Tier-1/Tier-2 fired) (FR-003); output/structured channels only, post-#13 (never narrative, read, or edit content) (FR-004, C-003); segregated — not in `Findings`, not in failure counts (FR-007); deterministic, stable ordering (NFR-002); **no report schema-version field** — `Anomalies` is a purely additive top-level array (C-005, research.md D3).
**Scale/Scope**: ~1 new source file (`internal/analyzer/anomaly.go`) + small edits to `types.go`, `analyzer.go`, `summary.go` + tests + a frozen validation corpus.

## Charter Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Charter present; paradigms **deep-module-design** + **specification-by-example**.
- **DIRECTIVE_001 / deep-module-design** — ✅ Tier-3 is a self-contained module (`anomaly.go`): a pure signal detector + a pure signature hasher + an aggregation pass. It consumes the already-computed channel strings and decoded object; the failure rules and channel layer are untouched.
- **DIRECTIVE_024 (Locality of Change)** — ✅ additive: one new file plus a stash-and-aggregate seam at the existing per-event construction point and the existing `buildFindings` call site. No change to Tier-1/Tier-2 semantics.
- **DIRECTIVE_003 (Decision Documentation)** — ✅ the four material decisions (residual trigger set, structured-signal retention, report-version signaling, ignore-registry form) are recorded in `research.md`.
- **DIRECTIVE_025 (Boy Scout Rule)** — touched-file fixes folded in-scope; none identified beyond the change.
- **specification-by-example** — ✅ `contracts/anomaly-matrix.md` expresses every trigger/suppression decision as a concrete input→outcome example; golden tests mirror it.

No violations → Complexity Tracking omitted.

## Project Structure

### Documentation (this mission)

```
kitty-specs/tier3-anomaly-trap-01KXF8KZ/
├── plan.md              # This file
├── research.md          # Phase 0 — the 4 design decisions + residual-coverage matrix
├── data-model.md        # Phase 1 — Anomaly/AnomalyEvidence, candidate, signature hash
├── quickstart.md        # Phase 1 — validation (frozen corpus + golden matrix)
├── contracts/
│   └── anomaly-matrix.md  # Phase 1 — input shape → anomaly / no-anomaly (residual matrix)
└── tasks.md             # Phase 2 (/spec-kitty.tasks — NOT created here)
```

### Source Code (repository root)

```
internal/analyzer/
├── anomaly.go       # NEW: residual signal detector (exit_status, panic/segfault/core dumped),
│                    #      signature-hash normalization, ignore registry, buildAnomalies aggregation
├── types.go         # Anomaly/AnomalyEvidence types; Report.Anomalies field; unexported
│                    #      anomalyCandidate cached on TimelineEvent (in-memory only)
├── analyzer.go      # stash an anomaly candidate at event construction (obj live) — residual-only,
│                    #      routed through the single skipArtifactMessage gate; call buildAnomalies
├── summary.go       # buildAnomalies lives alongside buildFindings (or in anomaly.go, called here)
└── *_test.go        # anomaly_test.go golden matrix + corpus test
```

**Structure Decision**: No new packages. One new file `anomaly.go` holds the whole Tier-3 module (detector + hasher + ignore registry + aggregator), keeping the concern encapsulated (deep-module-design). The only shared-type change is the additive `Report.Anomalies` field and an unexported `anomalyCandidate` field on `TimelineEvent` (never serialized). The report JSON gains exactly one additive top-level key, `anomalies`.

## Complexity Tracking

*No Charter Check violations — section intentionally empty.*

## Implementation Concern Map

> Concerns, not work packages. `/spec-kitty.tasks` decomposes these into WPs.

### IC-01 — Residual signal detector

- **Purpose**: Pure functions that, given the post-#13 output-channel string and the decoded structured object, detect the tight residual set and return **zero or more** anomaly signals `{kind, channel, snippet}` (an event may carry both an `exit_status` and a crash sig — M2): (a) a **top-level** structured `exit_status` with a non-zero numeric value; (b) an output crash signature in `{panic:, segmentation fault, core dumped}`. Deliberately excludes everything already covered by Tier-1 (`jsonHasError`, the `Traceback` fingerprint) and Tier-2 (`genericFailureSignals`, which already catches the *text* forms `exit code|exit status|returncode|return code` + N).
- **Relevant requirements**: FR-001, FR-002, FR-008
- **Affected surfaces**: `internal/analyzer/anomaly.go` (new). **Structured read is top-level-only** — `obj["exit_status"]` as a direct, type-asserted numeric access — **not** `firstJSONNumberByKey`/any recursive walk (H3: recursion is nondeterministic and would reach into post-#13-excluded read/edit content). Crash sig scans `event.outputCh`.
- **Sequencing/depends-on**: none (pure, foundational)
- **Risks**: over-broad triggers re-admit benign chatter (the #4 FP class) — keep the set minimal and re-confirm non-overlap with Tier-1/Tier-2 in tests (FR-008 is a test obligation, not just prose). Embedded/nested `exit_status` (inside a re-decoded tool-result string or codex envelope) is an explicit non-goal (M1; recall-safe).

### IC-02 — Residual-only emission through the single gate

- **Purpose**: At the **post-gate append site** in `parseFile` (both the single-object JSON branch ~L290 and the scanner-loop branch ~L335), for a **kept** event, stash anomaly candidate(s) **only when both**: (i) `!isArtifactKind(kind)` (H1 — the artifact gate is not blanket; an artifact JSON event with `Kind!="message"` and no failures survives `skipArtifactMessage`, so anomalies must be independently barred from artifact kinds); and (ii) the **finalized** `len(event.Failures)==0` (⇒ neither Tier-1 nor Tier-2, since `generic_error` is appended into that same slice). Computing post-gate uses the finalized failures (H2 — the gate can mutate `event.Failures`). Stash a **slice** `anomalyCandidates` (M2) on `TimelineEvent`.
- **Relevant requirements**: FR-003, FR-004; C-002, C-003, C-004
- **Affected surfaces**: `internal/analyzer/analyzer.go` (`parseFile` append sites), `internal/analyzer/types.go` (unexported `anomalyCandidates []anomalyCandidate` field)
- **Sequencing/depends-on**: IC-01
- **Risks**: must add candidates at **both** append sites (the single-object branch returns early and doesn't share the loop path); consumes the post-#13 channel strings + top-level `obj` only (never narrative/read content).

### IC-03 — Anomaly types, provenance, and signature hash

- **Purpose**: Add `Anomaly`/`AnomalyEvidence` types mirroring `Finding`/`FindingEvidence` plus channel, signal kind, signature hash, snippet, count, first/last occurrence. Compute a **signature hash** = sha256 over `(channel, kind, normalized-token)` where the token is canonicalized (lowercased; digits and path-like runs collapsed) so identical shapes group across files/runs; emit a short hex prefix.
- **Relevant requirements**: FR-005
- **Affected surfaces**: `internal/analyzer/types.go` (types), `internal/analyzer/anomaly.go` (hashing/normalization)
- **Sequencing/depends-on**: IC-01
- **Risks**: normalization must be deterministic and stable — no map iteration, no timestamps; hash inputs fixed-order.

### IC-04 — Ignore registry

- **Purpose**: A checked-in Go allowlist `ignoredAnomalySignatures map[string]string` (hash → benign reason), consistent with how `genericFailureSignals` / read-allowlists already live in code. `buildAnomalies` drops any candidate whose signature hash is listed. Promotion/refine/ignore is a code edit + rebuild for v1 (dashboards/automation out of scope, C-007).
- **Relevant requirements**: FR-006
- **Affected surfaces**: `internal/analyzer/anomaly.go`
- **Sequencing/depends-on**: IC-03 (keys on the signature hash)
- **Risks**: registry starts empty; a suppression test must add a hash and prove suppression.

### IC-05 — Aggregation + segregated report wiring

- **Purpose**: `buildAnomalies(events)` collects stashed candidates, drops ignored hashes, groups by signature hash with count + first/last occurrence, sorts deterministically, and returns `[]Anomaly`. Wire `report.Anomalies = buildAnomalies(report.Timeline)` at **both** `buildFindings` call sites — `Analyze` (analyzer.go:69) **and** `filterReportByMission` (analyzer.go:117) — so mission-filtered reports rebuild anomalies from their filtered timeline (M4). Add `Anomalies` to `normalizeReport` so it marshals as `[]`, not `null` (L1). Assert (in code + tests) that anomalies never enter `Findings` and never touch `Summary` failure/failure-mode counts. Update the report-contract docs; **no schema-version field** (C-005/D3).
- **Relevant requirements**: FR-007; C-001, C-005
- **Affected surfaces**: `internal/analyzer/summary.go` (or `anomaly.go`), `internal/analyzer/analyzer.go` (both call sites), `internal/analyzer/normalize.go` (`normalizeReport`), `internal/analyzer/types.go` (`Report.Anomalies`), report-contract docs
- **Sequencing/depends-on**: IC-02, IC-03, IC-04
- **Risks**: ordering nondeterminism → sort by `(signature_hash, first_seq)`, evidence by `seq`; ensure empty normalizes to `[]` consistently (L1).

### IC-06 — Golden matrix + frozen-corpus validation

- **Purpose**: Golden anomaly-matrix cases for every FR (positive + negative): residual `exit_status`/crash sig fires; a Tier-1 `json_error_event` event and a Tier-2 `generic_error` event each produce **no** anomaly (residual-only, no double-count); a bare generic word produces no anomaly; narrative-only and codex-read content produce no anomaly; the ignore registry suppresses a listed hash; identical shapes across two files share a hash and group; determinism across repeated runs. Plus a **frozen-corpus** base-vs-candidate run proving `findings`/counts are byte-identical (NFR-001) and the anomaly set is genuine, not chatter (NFR-004).
- **Relevant requirements**: NFR-001, NFR-002, NFR-003, NFR-004; C-006
- **Affected surfaces**: `internal/analyzer/anomaly_test.go` (+ corpus fixtures)
- **Sequencing/depends-on**: IC-05
- **Risks**: corpus must be frozen + representative; back-to-back base/candidate with separate caches to avoid the live-session-in-corpus confound (C-006).

## Branch contract (repeat)

- Current branch at plan: `feat/tier3-anomaly-trap`
- Planning/base branch: `feat/tier3-anomaly-trap`
- Final merge target for the mission: `feat/tier3-anomaly-trap` (then PR → `main` for colleague review)
- `branch_matches_target`: true

**Next suggested command**: `/spec-kitty.tasks` (invoked explicitly).
