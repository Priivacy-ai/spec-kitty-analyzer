---
work_package_id: WP03
title: Frozen-corpus validation + report-contract docs
dependencies:
- WP01
- WP02
requirement_refs:
- FR-006
- NFR-001
- NFR-004
tracker_refs: []
planning_base_branch: feat/tier3-anomaly-trap
merge_target_branch: feat/tier3-anomaly-trap
branch_strategy: Planning artifacts for this mission were generated on feat/tier3-anomaly-trap. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/tier3-anomaly-trap unless the human explicitly redirects the landing branch.
created_at: '2026-07-14T03:00:00Z'
subtasks:
- T011
- T012
- T013
phase: Phase 3 - Validation
assignee: ''
agent: claude
history:
- at: '2026-07-14T03:00:00Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/anomaly_corpus_test
create_intent:
- internal/analyzer/anomaly_corpus_test.go
- docs/design/tier3-anomaly-trap.md
execution_mode: code_change
model: ''
owned_files:
- internal/analyzer/anomaly_corpus_test.go
- internal/analyzer/testdata/anomaly/**
- docs/design/tier3-anomaly-trap.md
role: implementer
tags: []
task_type: implement
---

# Work Package Prompt: WP03 – Frozen-corpus validation + report-contract docs

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the frontmatter profile and behave per its guidance first.
- **Profile**: `implementer-ivan` · **Role**: `implementer` · **Agent/tool**: `claude`

---

## Objectives & Success Criteria

Prove Tier-3 is **additive** and its output is **genuine** on committed corpus fixtures, and document the report contract + the promote/refine/ignore loop. Depends on WP01 (module) + WP02 (wiring).

**Done when:**
- `internal/analyzer/testdata/anomaly/**` holds a small, frozen, representative set of session-log fixtures (some triggering anomalies, some triggering Tier-1/Tier-2 findings, some benign).
- `internal/analyzer/anomaly_corpus_test.go` asserts: (a) **additivity** — `Findings` + `Summary` failure counts are identical whether or not anomalies are computed (compare against golden expectations); (b) **no double-count** — no anomaly shares an event `seq` with any finding; (c) the anomaly set on the fixtures is exactly the expected, genuine set.
- `docs/design/tier3-anomaly-trap.md` documents the additive `anomalies` schema (no schema-version field), the tiered model, and the promote/refine/ignore loop.
- `go build ./... && go vet ./... && gofmt -l` clean; `go test ./internal/analyzer/ -run AnomalyCorpus` green; full `go test ./...` green.

## Context & Constraints

- **Authoritative design**: `quickstart.md` (frozen-corpus method), `spec.md` NFR-001/NFR-004/C-006, `research.md` D3 (report-version decision), `contracts/anomaly-matrix.md` (report shape).
- **C-006 (corpus method)**: fixtures are **committed + frozen** (deterministic, CI-safe) — NOT the live `~/.claude`/`~/.codex` (which drift with this session's transcript). The full-corpus back-to-back base/candidate diff with separate caches is a **manual** local step documented in `quickstart.md`; the automated test uses the committed fixtures.
- **Study, do NOT edit** (other WPs): `anomaly.go`, `types.go` (WP01); `analyzer.go`, `normalize.go` (WP02).
- **Report-version (D3)**: the doc must state the addition is purely additive with **no** schema-version field, reconciling #23's removal of the top-level `version`.

## Branch Strategy
- **Strategy**: already-confirmed · **Planning/merge target**: `feat/tier3-anomaly-trap`
> Execution worktrees are allocated per computed lane from `lanes.json`.

## Subtasks & Detailed Guidance

### Subtask T011 – Frozen corpus fixtures
- **Purpose**: A representative, deterministic sample exercising all outcome classes.
- **Steps**: Create `internal/analyzer/testdata/anomaly/` with a handful of `.jsonl`/`.json` fixtures modeled on real shapes (study existing `testdata/` + `corpus_codex_test.go` for the fixture idiom):
  - a top-level `exit_status:2` structured event (→ anomaly);
  - a tool-output `panic:` and a `segmentation fault` (→ anomalies);
  - a `{"exit_code":1}` and a `command failed: exit status 2` (→ findings, no anomaly);
  - an artifact-kind event carrying `exit_status:2` (→ neither);
  - `panic:` inside codex-read content (→ neither);
  - benign chatter (`unexpected`, `failure` bare words) (→ neither).
- **Files**: `internal/analyzer/testdata/anomaly/**`
- **Notes**: Keep it small and legible; each fixture's expected outcome is documented in the test.

### Subtask T012 – Corpus additivity + genuineness test
- **Purpose**: Lock NFR-001 (additive) and NFR-004 (genuine, no double-count).
- **Steps**: `internal/analyzer/anomaly_corpus_test.go`:
  - Run `Analyze` over the fixtures. Assert `Findings` + `Summary` failure/failure-mode counts equal a golden expectation derived independently of anomalies (additivity — Tier-3 changed nothing in the failure path).
  - Assert the set of `Anomalies` equals the expected genuine set (kinds + counts + which fixtures).
  - Assert **no** anomaly shares a `seq` with any finding (no double-count) — iterate both and cross-check.
- **Files**: `internal/analyzer/anomaly_corpus_test.go`
- **Notes**: Name tests `TestAnomalyCorpus*` so `-run AnomalyCorpus` selects them.

### Subtask T013 – Report-contract design doc [P]
- **Purpose**: Document the contract + self-improvement loop (FR-006 loop; D3).
- **Steps**: Create `docs/design/tier3-anomaly-trap.md` (mirror the style of `docs/design/issue-4-failure-scan-channel-scoping.md`): the tiered model (1/2/3); the residual trigger set + why it's tight; the additive `anomalies` schema with an example (full `signature_hash`); the explicit statement that **no** report schema-version field is added (D3, reconciling #23); and the promote → refine → ignore loop (how a maintainer reads a group, then either adds a fingerprint, refines a rule, or pastes the `signature_hash` into `ignoredAnomalySignatures`).
- **Files**: `docs/design/tier3-anomaly-trap.md`

## Test Strategy
- `go test ./internal/analyzer/ -run AnomalyCorpus -v`, then full `go test ./...`.
- Manual (documented in `quickstart.md`, not automated): full frozen `~/.claude`+`~/.codex` base-vs-candidate diff, separate caches, assert zero `findings` delta + inspect the anomaly set.

## Risks & Mitigations
- **Fixtures not representative** → cover every outcome class (anomaly / finding / neither) explicitly.
- **Flaky ordering** → rely on WP01's deterministic `buildAnomalies` ordering; assert exact slices.
- **Doc drift** → the doc's example must match the actual emitted JSON shape (full-digest hash, additive key).

## Review Guidance
- Confirm additivity is asserted against an anomaly-independent golden (not circular).
- Confirm the no-double-count cross-check exists.
- Confirm the doc states no schema-version field and describes the ignore loop with a real `signature_hash`.

## Activity Log
- 2026-07-14T03:00:00Z – system – Prompt created.
