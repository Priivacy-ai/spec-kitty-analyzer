---
work_package_id: WP01
title: Anomaly module core — detector, types, hash, ignore registry
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-005
- FR-006
- FR-007
- FR-008
- NFR-002
tracker_refs: []
planning_base_branch: feat/tier3-anomaly-trap
merge_target_branch: feat/tier3-anomaly-trap
branch_strategy: Planning artifacts for this mission were generated on feat/tier3-anomaly-trap. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/tier3-anomaly-trap unless the human explicitly redirects the landing branch.
created_at: '2026-07-14T03:00:00Z'
subtasks:
- T001
- T002
- T003
- T004
- T005
- T006
phase: Phase 1 - Foundation
assignee: ''
agent: "claude"
shell_pid: "67449"
shell_pid_created_at: "1783999101.17327"
history:
- at: '2026-07-14T03:00:00Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/anomaly
create_intent:
- internal/analyzer/anomaly.go
- internal/analyzer/anomaly_test.go
execution_mode: code_change
model: ''
owned_files:
- internal/analyzer/anomaly.go
- internal/analyzer/anomaly_test.go
- internal/analyzer/types.go
role: implementer
tags: []
task_type: implement
---

# Work Package Prompt: WP01 – Anomaly module core

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile in the frontmatter and behave per its guidance before reading further.
- **Profile**: `implementer-ivan` · **Role**: `implementer` · **Agent/tool**: `claude`

---

## Objectives & Success Criteria

Build the **self-contained Tier-3 module** — pure, deterministic, and not yet wired into the pipeline (WP02 does that). Read `research.md` (D1–D5 + the folded Codex-review findings), `data-model.md`, and `contracts/anomaly-matrix.md` before coding.

**Done when:**
- `internal/analyzer/anomaly.go` exposes: the residual signal detector, the signature hasher, `ignoredAnomalySignatures`, and `buildAnomalies`.
- `internal/analyzer/types.go` gains `Anomaly`/`AnomalyEvidence`, the additive `Report.Anomalies` field, and the unexported `anomalyCandidates []anomalyCandidate` on `TimelineEvent`.
- `go build ./... && go vet ./... && gofmt -l internal/analyzer/anomaly.go internal/analyzer/types.go` clean.
- Stdlib only (`strings`, `regexp`, `crypto/sha256`, `encoding/hex`, `sort`). No new dependency.
- `go test ./internal/analyzer/ -run Anomaly` green.

## Context & Constraints

- **Authoritative design**: `research.md` (esp. the amended D1 = top-level-only `exit_status`; D2/D5 = residual-only; and folded findings H3/M2/M3). `data-model.md` (types, `anomalyCandidates` slice, signature-hash contract). `contracts/anomaly-matrix.md` (P/N/G rows).
- **Residual set (D1, tight — FR-008)**: (a) structured **top-level** `exit_status` with a non-zero numeric value; (b) output crash signatures `panic:`, `segmentation fault`, `core dumped`. Nothing else — everything else is already Tier-1 (`jsonHasError`, the `Traceback` fingerprint) or Tier-2 (`genericFailureSignals`).
- **Study, do NOT edit** (owned by other WPs): `internal/analyzer/analyzer.go` (WP02 — emission/wiring), `internal/analyzer/normalize.go` (WP02).
- **Determinism (NFR-002)**: no map iteration in hash input or output ordering; sort deterministically.

## Branch Strategy
- **Strategy**: already-confirmed · **Planning/merge target**: `feat/tier3-anomaly-trap`
> Execution worktrees are allocated per computed lane from `lanes.json`.

## Subtasks & Detailed Guidance

### Subtask T001 – `anomalyCandidate` + residual signal detector
- **Purpose**: Turn a decoded event into zero-or-more residual anomaly candidates (FR-001, FR-002; M2 = may return several).
- **Steps**:
  - Define `type anomalyCandidate struct { kind, channel, snippet string }`. Kinds: `structured_exit_status`, `crash_panic`, `crash_segfault`, `crash_core_dumped`. Channels: `structured`, `output`.
  - `func detectAnomalies(obj map[string]any, outputCh string) []anomalyCandidate`:
    - **Structured (top-level only — H3)**: read `obj["exit_status"]` directly (type-assert to the JSON number type; a float64 in Go's decoder). If present and non-zero → append `{kind: structured_exit_status, channel: structured, snippet: "exit_status=<n>"}`. **Do NOT** use `firstJSONNumberByKey` or any recursive walk (nondeterministic + would reach into post-#13-excluded content).
    - **Crash sigs (output channel)**: compile `regexp` (package-level `var`, `(?i)` where apt) for `panic:`, `segmentation fault`, `core dumped`. For each match in `outputCh`, append a candidate with the matched line as `snippet` (bounded, see below).
    - `obj` may be nil (text-line events) → only crash-sig detection runs.
  - Bound each snippet (e.g. ≤200 chars) and pass it through the existing scrubber used by findings evidence (match how `FindingEvidence` snippets are produced — reuse, don't reinvent).
- **Files**: `internal/analyzer/anomaly.go`
- **Notes**: Keep the crash-sig set exactly these three. `Traceback` is deliberately excluded (already Tier-1/Tier-2).

### Subtask T002 – Signature hash [P]
- **Purpose**: Group identical shapes across files/runs (FR-005) with a collision-safe key (M3).
- **Steps**:
  - `normalizeToken(s string) string`: lowercase; collapse digit runs to a single placeholder (e.g. `#`); collapse path-like / long hex runs to a placeholder. Deterministic, no allocations-order dependence.
  - `signatureHash(channel, tool, kind, token string) string`: `sha256(channel+"\x00"+tool+"\x00"+kind+"\x00"+normalizeToken(token))` rendered as the **full 64-char hex** (`encoding/hex`). This is the group key AND the ignore-registry key — no truncation.
- **Files**: `internal/analyzer/anomaly.go`
- **Notes**: `tool` is the event's `ToolName` (or `""`). Include it in the tuple per FR-005.

### Subtask T003 – Ignore registry [P]
- **Purpose**: Suppress confirmed-benign signatures (FR-006).
- **Steps**: `var ignoredAnomalySignatures = map[string]string{}` (hash → human reason); starts empty. A helper `isIgnoredSignature(hash string) bool`. Document that a maintainer pastes a report's full `signature_hash` here to suppress it (promote/refine/ignore loop; richer tooling is out of scope, C-007).
- **Files**: `internal/analyzer/anomaly.go`

### Subtask T004 – Types + report/event fields
- **Purpose**: The report record + the in-memory candidate stash (FR-007; data-model).
- **Steps**:
  - In `types.go` add `Anomaly` (`signature_hash`, `kind`, `channel`, `title`, `count`, `evidence`, `first_seq`, `last_seq`) and `AnomalyEvidence` (`source_path`, `seq`, `line`, `snippet`) mirroring `Finding`/`FindingEvidence`.
  - Add `Anomalies []Anomaly \`json:"anomalies"\`` to `Report` (additive; place after `Findings`). **No** schema-version field (D3).
  - Add unexported `anomalyCandidates []anomalyCandidate` to `TimelineEvent` (in-memory only; document it like the `outputCh`/`diagnosticCh` comment — never serialized).
- **Files**: `internal/analyzer/types.go`

### Subtask T005 – `buildAnomalies`
- **Purpose**: Aggregate stashed candidates into grouped, deterministic `[]Anomaly` (FR-007; NFR-002).
- **Steps**:
  - `func buildAnomalies(events []TimelineEvent) []Anomaly`: for each event, for each `anomalyCandidate`, compute `signatureHash(channel, event.ToolName, kind, snippet)`; skip if `isIgnoredSignature`. Group by hash: accumulate `count`, `first_seq`/`last_seq`, append `AnomalyEvidence` (cap evidence to first N, e.g. 5; `count` still totals all). Title = a human string per kind.
  - Sort groups by `(signature_hash, first_seq)`; sort evidence within a group by `seq`. No map-iteration in output.
- **Files**: `internal/analyzer/anomaly.go`

### Subtask T006 – Unit/golden tests (TEST-FIRST)
- **Purpose**: Drive T001–T005 test-first (DIRECTIVE_034/039; specification-by-example).
- **Steps**: Create `internal/analyzer/anomaly_test.go` (package `analyzer`). Author before production code (red→green):
  - Detector: P1 (`exit_status:2`→structured), P2 `panic:`, P3 `segmentation fault`, P4 `core dumped`, P5 (both exit_status+panic → two candidates). Detector-level negatives: `exit_status:0` → none; a bare `unexpected`/`failure` word → none; `Traceback (most recent call last):` alone → none (not in the residual set).
  - Hash: same shape (`panic ... [5]` vs `[9]`) → same hash (G1/normalizeToken); different channel/kind → different hash; determinism across two calls (G2).
  - `buildAnomalies`: grouping with count + first/last (G1); ignore-registry suppression by injecting a hash into a local map or asserting `isIgnoredSignature` path (G3).
- **Files**: `internal/analyzer/anomaly_test.go`
- **Notes**: These are pure-function tests (unit-test rules, not the black-box integration directive). Run red first.

## Test Strategy
- Test-first (T006), then implement to green. `go test ./internal/analyzer/ -run Anomaly -v`, then full `go test ./internal/analyzer/`.

## Risks & Mitigations
- **Over-broad triggers re-admit chatter (#4 class)** → keep the residual set to the three crash sigs + top-level `exit_status`; negative tests assert silence on Tier-1/Tier-2 shapes.
- **Non-determinism** → full-digest hash, sorted output, no map iteration.
- **Snippet leakage** → reuse the findings scrubber; bound length.

## Review Guidance
- Confirm `exit_status` read is **top-level-only**, never recursive (H3).
- Confirm an event can yield **multiple** candidates (P5).
- Confirm the hash is the full 64-char digest and includes `tool` (M3, FR-005).
- Confirm `buildAnomalies` output is deterministic and drops ignored hashes.
- Confirm `Report.Anomalies` is additive and no schema-version field was added.

## Activity Log
- 2026-07-14T03:00:00Z – system – Prompt created.
- 2026-07-14T03:18:28Z – claude – shell_pid=67449 – Assigned agent via action command
