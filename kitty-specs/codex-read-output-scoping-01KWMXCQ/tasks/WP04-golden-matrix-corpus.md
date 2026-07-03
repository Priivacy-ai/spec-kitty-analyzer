---
work_package_id: WP04
title: Golden channel-matrix + frozen-corpus validation
dependencies:
- WP02
- WP03
requirement_refs:
- NFR-001
- NFR-002
- NFR-003
- NFR-004
tracker_refs: []
planning_base_branch: fix/codex-read-output-scoping
merge_target_branch: fix/codex-read-output-scoping
branch_strategy: Planning artifacts for this mission were generated on fix/codex-read-output-scoping. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into fix/codex-read-output-scoping unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-codex-read-output-scoping-01KWMXCQ
base_commit: 5e6ea6fe9f89cf9f67a3a9f7dd20d84d29c17f88
created_at: '2026-07-03T21:38:22Z'
subtasks:
- T015
- T016
- T017
- T018
- T019
phase: Phase 4 - Validation
assignee: ''
agent: claude
history:
- at: '2026-07-03T21:38:22Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/channels_test
create_intent:
- internal/analyzer/testdata/codex
execution_mode: code_change
model: ''
owned_files:
- internal/analyzer/channels_test.go
- internal/analyzer/analyzer_test.go
- internal/analyzer/testdata/codex/**
role: implementer
tags: []
task_type: implement
---

# Work Package Prompt: WP04 – Golden channel-matrix + frozen-corpus validation

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile specified in the frontmatter, and
behave according to its guidance before parsing the rest of this prompt.

- **Profile**: `implementer-ivan`
- **Role**: `implementer`
- **Agent/tool**: `claude`

If no profile is specified, run `spec-kitty agent profile list` and select the best match for this
work package's `task_type` and `authoritative_surface`.

---

## Markdown Formatting

Wrap HTML/XML tags in backticks. Use language identifiers in code blocks.

---

## Objectives & Success Criteria

Prove the mission with executable evidence: golden channel-matrix cases for every routing decision,
an analyzer-level assertion that read-content FPs are gone while real failures remain, and a
frozen-corpus before/after diff. This WP is the acceptance surface (SC-001/002/003, NFR-001–004).

**Done when:**
- `internal/analyzer/channels_test.go` has table-driven golden cases mirroring every row of
  `contracts/channel-matrix.md` (read exit-0 excluded; non-zero read header-only; real command
  scanned; compound scanned; read pipeline excluded; unknown call_id scanned; `callId` spelling).
- Payload-type mapping cases cover the new types (`function_call`, `task_started`, `user_message`,
  empty) AND assert the unchanged types still map (regression guard for R5).
- An `analyzer_test.go` case builds a two-line codex file (`function_call` read + `function_call_output`
  exit-0 whose content contains "exit code 2") and asserts **no** `typer_usage_error` finding — and a
  companion case with a real failing command still yields its finding (recall).
- The frozen-corpus before/after runbook is executed and its FP-down / TP-preserved result recorded.
- `go test ./...` exits 0; the emitted report JSON schema is unchanged (NFR-004).

## Context & Constraints

- **Authoritative contract**: `contracts/channel-matrix.md` (rows 1–7 + the payload-type table) and
  `quickstart.md` (the frozen-corpus commands). These ARE the test spec (specification-by-example).
- **Existing tests to extend/mirror** (you own these):
  - `internal/analyzer/channels_test.go` (595 lines) — the golden channel-routing tests from the
    Claude channel-scoping mission; add codex cases in the same table-driven idiom.
  - `internal/analyzer/analyzer_test.go` — analyzer-level end-to-end assertions.
- **From WP02/WP03**: `channelTextPairCtx(obj, ctx)`, `channelContext`, `newCodexCall`, the threaded
  `channelStringsForEvent`/`eventFromText` signatures (update any callers you own to pass `ctx`).
- **Constraints**: C-005 (frozen, representative corpus — NOT live `~/.codex` ≈ 298 MB); run base +
  candidate back-to-back in one job with **separate `--cache`** (live-session-in-corpus confound);
  NFR-003 (determinism — assert identical findings across repeated runs where practical).

## Branch Strategy

- **Strategy**: already-confirmed
- **Planning base branch**: fix/codex-read-output-scoping
- **Merge target branch**: fix/codex-read-output-scoping

> Execution worktrees are allocated per computed lane from `lanes.json`.

## Subtasks & Detailed Guidance

### Subtask T015 – Golden channel-matrix cases (rows 1–7)
- **Purpose**: Lock every routing decision (specification-by-example).
- **Steps**: In `channels_test.go`, add a table-driven test exercising `channelTextPairCtx` with a
  hand-built `channelContext`. One case per contract row:
  1. `git diff` (read) + exit-0 diff containing "error"/"exit code 2" → `output` empty, `narrative` empty.
  2. `cat missing` (read) + exit-1 "No such file" → `output` contains the status header, NOT the bulk.
  3. `go build ./...` (real) + exit-1 build errors → `output` contains the full output (scanned).
  4. `git diff && go build` (compound) → `output` contains the full output (not all-read → scanned).
  5. `rg foo | head` (read pipeline) + any → `output` empty, `narrative` empty.
  6. output with a call_id absent from the registry → `output` contains the full output (unknown → scan).
  7. `callId` (camelCase) read, exit 0 → excluded (id spelling recognized).
- **Files**: `internal/analyzer/channels_test.go`.
- **Notes**: Build each case's registry the way the prepass would (a `codexCall` with the right
  `isRead`), so the test pins the extraction contract independent of WP03's walk.

### Subtask T016 – Payload-type mapping cases  [P]
- **Purpose**: Pin FR-006 mapping and guard against R5 regression.
- **Steps**: Add cases asserting: `function_call` contributes no channel text; `task_started`
  excluded; `user_message` with prose → `narrative` (and confirm the **actual prose field** from a
  real corpus sample — resolve the WP02 `TODO(WP04)` here); empty `payload.type` excluded; and that
  `reasoning`/`message`/`agent_message`/`task_complete`/`token_count` still route exactly as before
  (copy representative existing assertions to prove no regression).
- **Files**: `internal/analyzer/channels_test.go`.
- **Notes**: If the corpus shows `user_message` prose under a different key than WP02 assumed, fix
  WP02's field via a one-line out-of-map edit with a recorded rationale, or file it back to WP02.

### Subtask T017 – Absence-of-FP + presence-of-real-failure (recall)
- **Purpose**: Prove the actual user-visible outcome (SC-001/SC-003 + NFR-001).
- **Steps**: In `analyzer_test.go`, construct a small in-memory codex `.jsonl` (two lines: a
  `function_call` `git diff` + a `function_call_output` exit-0 whose bulk contains "exit code 2" and
  "merge"), run the analyzer over it, and assert **no** `typer_usage_error` / `merge_operation_failed`
  finding. Add a companion file with a real failing `go build` and assert its failure IS reported.
- **Files**: `internal/analyzer/analyzer_test.go`.
- **Notes**: These assert the ABSENCE of read-content findings, not merely the presence of real ones —
  the core acceptance for this mission.

### Subtask T018 – Frozen-corpus before/after diff
- **Purpose**: Empirical FP-down / TP-preserved on representative real sessions (NFR-001/002, C-005).
- **Steps**:
  - Curate a small **frozen** set of codex session `.jsonl` files that exhibit read-content FPs
    (git diff/show, rg/grep, cat of source/docs) plus at least one genuine command failure. Stage it
    at `~/spec-kitty-analyzer-issue4-backup/catfood-findings/frozen-codex-corpus` (documented in
    `quickstart.md`). Do NOT commit the raw corpus if it contains private content; instead commit a
    tiny redacted representative subset under `internal/analyzer/testdata/codex/` if useful, and
    record the full-corpus counts in this WP's Activity Log.
  - Run the `quickstart.md` procedure: build base (current `main`) and candidate binaries, run each
    over the frozen corpus with **separate caches**, diff fingerprint counts + evidence sources.
  - Record: read-content FP rules (`typer_usage_error`, `merge_operation_failed`, …) down; zero
    true-positive failures lost.
- **Files**: `internal/analyzer/testdata/codex/**` (optional redacted fixtures); results in Activity Log.
- **Notes**: Back-to-back in one job (live-session-in-corpus confound). The baseline whole-`~/.codex`
  counts (typer_usage_error×16, merge_operation_failed×37, permission_denied×496) are context only —
  measure on the frozen set.

### Subtask T019 – Full suite green + schema unchanged
- **Purpose**: NFR-004 gate.
- **Steps**: Run `go test ./...` (must exit 0). Confirm the emitted report JSON has no new/renamed
  fields — diff a report produced by the candidate against one from base on the same input, or inspect
  the report structs (nothing serialized from `codexCall`/`channelContext`).
- **Files**: n/a (verification); record the result in the Activity Log.

## Test Strategy

- Golden, table-driven, specification-by-example (mirrors `contracts/channel-matrix.md`).
- Deterministic: identical input → identical findings (NFR-003).
- Run: `go test ./internal/analyzer/ -run 'Codex|Channel|ReadOutput' -v` then `go test ./...`.

## Risks & Mitigations

- **Corpus not representative** → curate sessions that actually exhibit read-content FPs; include a
  genuine failure for the recall assertion.
- **Private content in fixtures** → commit only a redacted minimal subset; keep the full corpus local.
- **Asserting presence only** → every routing case must also assert the ABSENCE side (excluded → empty).

## Review Guidance

- Verify every `contracts/channel-matrix.md` row has a golden case.
- Verify the absence-of-FP analyzer test genuinely runs the classifier end-to-end (through WP03's walk).
- Verify `go test ./...` green and no report-schema drift.
- Verify the frozen-corpus result is recorded with concrete before/after counts.

## Activity Log

- 2026-07-03T21:38:22Z – system – Prompt created.
