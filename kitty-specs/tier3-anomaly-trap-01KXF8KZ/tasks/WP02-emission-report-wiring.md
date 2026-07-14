---
work_package_id: WP02
title: Emission + report wiring (residual-only, non-artifact, both recompute paths)
dependencies:
- WP01
requirement_refs:
- FR-003
- FR-004
- FR-007
- NFR-001
- NFR-003
tracker_refs: []
planning_base_branch: feat/tier3-anomaly-trap
merge_target_branch: feat/tier3-anomaly-trap
branch_strategy: Planning artifacts for this mission were generated on feat/tier3-anomaly-trap. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/tier3-anomaly-trap unless the human explicitly redirects the landing branch.
created_at: '2026-07-14T03:00:00Z'
subtasks:
- T007
- T008
- T009
- T010
phase: Phase 2 - Wiring
assignee: ''
agent: "codex"
shell_pid: "78873"
shell_pid_created_at: "1784000844.968028"
history:
- at: '2026-07-14T03:00:00Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/analyzer
create_intent:
- internal/analyzer/anomaly_wiring_test.go
execution_mode: code_change
model: ''
owned_files:
- internal/analyzer/analyzer.go
- internal/analyzer/normalize.go
- internal/analyzer/anomaly_wiring_test.go
role: implementer
tags: []
task_type: implement
---

# Work Package Prompt: WP02 – Emission + report wiring

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the frontmatter profile and behave per its guidance first.
- **Profile**: `implementer-ivan` · **Role**: `implementer` · **Agent/tool**: `claude`

---

## Objectives & Success Criteria

Make Tier-3 live: stash candidates at the **post-gate append sites** in `parseFile`, wire `buildAnomalies` into **both** report recompute paths, and normalize the new field. Consumes WP01's `detectAnomalies`/`buildAnomalies`/types.

**Done when:**
- `parseFile` stashes `event.anomalyCandidates` for kept events at **both** append sites, guarded by `!isArtifactKind(kind)` **and** finalized `len(event.Failures)==0`.
- `Analyze` and `filterReportByMission` both set `report.Anomalies = buildAnomalies(report.Timeline)`.
- `normalizeReport` normalizes `Anomalies` to `[]` (never `null`).
- `go build ./... && go vet ./... && gofmt -l internal/analyzer/analyzer.go internal/analyzer/normalize.go` clean.
- `go test ./internal/analyzer/` green; `findings`/failure counts unchanged vs before this WP.

## Context & Constraints

- **Authoritative design**: `research.md` D2/D5 + folded H1/H2/M4/L1; `contracts/anomaly-matrix.md` (N1–N10, G4).
- **H1 (artifact gate is NOT blanket)**: `skipArtifactMessage` returns *false* (keeps) for an artifact-kind event with `Kind != "message"` and no failures — so anomaly emission MUST independently require `!isArtifactKind(kind)`. Do not rely on the gate alone.
- **H2 (timing)**: compute candidates **after** the gate decision at the append site, using the finalized `event.Failures`. The gate can mutate `Failures`.
- **Residual-only (FR-003)**: `len(event.Failures)==0` ⇒ neither Tier-1 nor Tier-2 (`generic_error` is appended into that same slice).
- **Channel discipline (FR-004, C-003)**: structured reads use the in-scope **top-level** `obj` (JSON branches only); crash-sig reads use `event.outputCh` — which already excludes narrative + codex-read + file/edit content post-#13. Never scan raw/narrative text.
- **Study, do NOT edit** (WP01): `internal/analyzer/anomaly.go`, `internal/analyzer/types.go`.
- **Two recompute paths (M4)**: `analyzer.go` calls `buildFindings` in `Analyze` (~L69) and `filterReportByMission` (~L117). Wire anomalies at both.

## Branch Strategy
- **Strategy**: already-confirmed · **Planning/merge target**: `feat/tier3-anomaly-trap`
> Execution worktrees are allocated per computed lane from `lanes.json`.

## Subtasks & Detailed Guidance

### Subtask T007 – Stash at the single-object JSON branch
- **Purpose**: The `.json` whole-file branch (~L287–293) returns early; it needs its own stash.
- **Steps**: After `event := eventFromJSONObjectCtx(...)` and the `!skipArtifactMessage(kind, &event)` check passes (event is kept), and if `!isArtifactKind(kind) && len(event.Failures)==0`, set `event.anomalyCandidates = detectAnomalies(obj, event.outputCh)` before returning it.
- **Files**: `internal/analyzer/analyzer.go`
- **Notes**: `obj` is in scope here. Factor the "should-stash + detect" logic into one small helper so both append sites call it identically (avoid drift).

### Subtask T008 – Stash at the scanner-loop branch
- **Purpose**: The per-line loop (~L320–337) handles both JSON-object lines and text lines.
- **Steps**: In the loop, after the `!skipArtifactMessage(kind, &event)` check passes and before `events = append(...)`, call the same helper: if `!isArtifactKind(kind) && len(event.Failures)==0`, set candidates via `detectAnomalies(obj, event.outputCh)` where `obj` is the decoded object for a JSON line, or `nil` for a text line (crash-sig-only).
- **Files**: `internal/analyzer/analyzer.go`
- **Notes**: For the text branch, `obj` is nil → detector runs crash-sig-only over `outputCh`. Confirm the decoded `obj` from the line's `decodeJSONObject` is threaded to the helper.

### Subtask T009 – Wire `buildAnomalies` + normalize
- **Purpose**: Populate `Report.Anomalies` on both paths (M4) and normalize (L1).
- **Steps**:
  - In `Analyze`, right after `report.Findings = buildFindings(...)`, add `report.Anomalies = buildAnomalies(report.Timeline)`.
  - Same in `filterReportByMission` (after its `buildFindings`) — the filtered timeline yields mission-scoped anomalies.
  - In `normalize.go`, extend `normalizeReport` so a nil `Anomalies` becomes `[]Anomaly{}` (match how other slices are normalized). Do the same for any nested nil evidence slice if the normalizer handles those.
- **Files**: `internal/analyzer/analyzer.go`, `internal/analyzer/normalize.go`

### Subtask T010 – Integration tests (TEST-FIRST)
- **Purpose**: Lock the gate/segregation invariants (FR-003, FR-004, FR-007, NFR-001).
- **Steps**: Create `internal/analyzer/anomaly_wiring_test.go`. Drive through `Analyze`/`parseFile` with small fixtures:
  - N1/N2 residual-only: a `{"exit_code":1}` event and a `{"status":"failed"}` event → a finding, **no** anomaly.
  - N4 Tier-2: `command failed: exit status 2` → `generic_error`, no anomaly.
  - P1/P2: a top-level `{"exit_status":2}` event and a `panic:`-in-tool-output event → an anomaly each; assert absent from `Findings` and `Summary` counts unchanged (G4 segregation).
  - N10 non-artifact: an artifact-kind event (e.g. a `work_package`/mission-status snapshot) carrying `exit_status:2` → **no** anomaly.
  - N7–N9: `panic:` only in narrative / codex-read content → no anomaly (outputCh excludes it).
  - M4: a mission-filtered report (`AnalyzeMission`) still carries the mission's anomalies.
  - L1: empty case marshals `"anomalies": []`, not `null`.
- **Files**: `internal/analyzer/anomaly_wiring_test.go`
- **Notes**: Reuse existing test fixture helpers/patterns in the package. Author red first.

## Test Strategy
- Test-first (T010). Run `go test ./internal/analyzer/ -run 'Anomaly|Wiring' -v`, then full `go test ./...`.
- **Additivity check**: run the suite; confirm no existing findings/summary assertions change.

## Risks & Mitigations
- **Forgetting one append site** → the shared helper (T007) is called at both; a test exercises both the whole-file `.json` branch and the per-line branch.
- **Pre-gate staleness (H2)** → compute strictly after the gate check, using finalized `Failures`.
- **Artifact leak (H1)** → explicit `!isArtifactKind(kind)`; N10 test guards it.
- **`null` vs `[]` (L1)** → normalize + an explicit empty-report test.

## Review Guidance
- Confirm both append sites stash via the same helper and the `!isArtifactKind && len(Failures)==0` guard.
- Confirm anomalies are built in BOTH `Analyze` and `filterReportByMission`.
- Confirm `findings`/`summary` are untouched (segregation) and `anomalies` normalizes to `[]`.

## Activity Log
- 2026-07-14T03:00:00Z – system – Prompt created.
- 2026-07-14T03:37:09Z – claude – shell_pid=74957 – Assigned agent via action command
- 2026-07-14T03:47:30Z – claude – shell_pid=74957 – Moved to for_review
- 2026-07-14T03:47:34Z – codex – shell_pid=78873 – Started review via action command
- 2026-07-14T03:47:42Z – user – shell_pid=78873 – Codex code review APPROVE, no findings; added .json-path test per non-blocking note. go build/vet/gofmt/test green.
