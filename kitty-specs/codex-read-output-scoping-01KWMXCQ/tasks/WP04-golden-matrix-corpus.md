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
- T018
- T019
phase: Phase 4 - Validation
assignee: ''
agent: "codex"
shell_pid: "13944"
history:
- at: '2026-07-03T21:38:22Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/corpus_codex_test
create_intent:
- internal/analyzer/corpus_codex_test.go
- internal/analyzer/testdata/codex
execution_mode: code_change
model: ''
owned_files:
- internal/analyzer/corpus_codex_test.go
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

Prove the mission on **real, representative data** and gate the whole suite. The per-routing golden
matrix (rows 1–7) and the analyzer integration case are authored test-first in WP02/WP03 (co-located
with the code they pin); this WP delivers the **frozen-corpus before/after diff** and a committed
**black-box fixture test** over a small redacted codex corpus, then confirms the full suite + schema.
This WP is the empirical acceptance surface (SC-001/002/003, NFR-001/002/004).

**Done when:**
- A small, redacted, committed codex fixture set under `internal/analyzer/testdata/codex/` exercises
  the analyzer end-to-end via its public API (black-box, DIRECTIVE_036) and asserts: no finding whose
  evidence is read/diff/doc content (SC-003), and a genuine command failure still reported (recall).
- The frozen-corpus before/after runbook (`quickstart.md`) is executed on the full local corpus and
  its FP-down / TP-preserved result recorded in this WP's Activity Log (NFR-001/002).
- `go test ./...` exits 0; the emitted report JSON schema is unchanged (NFR-004).
- The observable behavior change is documented (C1 — see T019).

## Context & Constraints

- **Authoritative contract**: `contracts/channel-matrix.md` (rows 1–7 + the payload-type table) and
  `quickstart.md` (the frozen-corpus commands). These ARE the test spec (specification-by-example).
- **Test ownership** (post-A1 remediation): the per-routing golden matrix lives in WP02
  (`channels_test.go`, T015/T016) and the analyzer integration case in WP03 (`analyzer_test.go`, T017),
  each authored test-first with its code. This WP owns a **new** `internal/analyzer/corpus_codex_test.go`
  (black-box over committed fixtures) + `internal/analyzer/testdata/codex/**` — study the existing
  `channels_test.go`/`analyzer_test.go` idiom but do NOT edit them (WP02/WP03 own them).
- **From WP02/WP03**: the analyzer's public entry point (the same one existing `analyzer_test.go`
  drives), `channelContext`, `newCodexCall` — invoke through the public API, not internals.
- **Constraints**: C-005 (frozen, representative corpus — NOT live `~/.codex` ≈ 298 MB); run base +
  candidate back-to-back in one job with **separate `--cache`** (live-session-in-corpus confound);
  NFR-003 (determinism — assert identical findings across repeated runs where practical).

## Branch Strategy

- **Strategy**: already-confirmed
- **Planning base branch**: fix/codex-read-output-scoping
- **Merge target branch**: fix/codex-read-output-scoping

> Execution worktrees are allocated per computed lane from `lanes.json`.

## Subtasks & Detailed Guidance

> **Prerequisite**: WP02 (T015/T016 golden matrix) and WP03 (T017 analyzer integration test) are
> already merged and green before this WP runs — those pin per-routing correctness. This WP adds the
> empirical corpus proof + a committed fixture test + the suite/schema gate.

### Subtask T018 – Frozen-corpus before/after diff + committed black-box fixture test
- **Purpose**: Empirical FP-down / TP-preserved on real sessions (NFR-001/002, C-005) plus a
  self-contained regression test that runs in CI-less `go test ./...`.
- **Steps**:
  - **Committed fixture test**: curate a tiny, **redacted** representative subset under
    `internal/analyzer/testdata/codex/` (a read `git diff`/`cat` whose content carries "exit code 2"/
    "merge", plus one genuine command failure). Add `internal/analyzer/corpus_codex_test.go` that runs
    the analyzer over that fixture directory through its **public API** (black-box, DIRECTIVE_036) and
    asserts: no read-content finding (SC-003) and the real failure IS reported (recall). This is the
    committed, deterministic proof (NFR-003).
  - **Full frozen-corpus diff** (local, not committed): assemble the larger frozen set at
    `~/spec-kitty-analyzer-issue4-backup/catfood-findings/frozen-codex-corpus` (git diff/show, rg/grep,
    cat of source/docs + a genuine failure). Run the `quickstart.md` procedure — base (`main`) vs
    candidate binaries, **separate caches**, back-to-back in one job — and diff fingerprint counts +
    evidence sources. Record read-content FP rules down (`typer_usage_error`, `merge_operation_failed`,
    …) and zero true-positive loss in this WP's Activity Log.
- **Files**: `internal/analyzer/corpus_codex_test.go`, `internal/analyzer/testdata/codex/**`; full-corpus
  counts in the Activity Log.
- **Notes**: Never commit private raw corpus content — redact fixtures. Baseline whole-`~/.codex` counts
  (typer_usage_error×16, merge_operation_failed×37, permission_denied×496) are context only; measure on
  the frozen set.

### Subtask T019 – Full suite green, schema unchanged, behavior documented (C1)
- **Purpose**: NFR-004 gate + charter documentation policy.
- **Steps**:
  - Run `go test ./...` (must exit 0). Confirm the emitted report JSON has no new/renamed fields — diff
    a candidate-vs-base report on the same input, or inspect the report structs (nothing serialized
    from `codexCall`/`channelContext`).
  - **C1 (documentation)**: record the observable behavior change (codex read/diff/doc content no
    longer produces failure findings) where the repo's docs live — a README/docs failure-mode note and
    a CHANGELOG entry for the eventual PR→main. If the CHANGELOG mechanism is not yet in place (analyzer
    #20 pending), capture the behavior delta in the PR body per the Quality-Gates policy and note the
    deferral here. Keep this proportional (DIRECTIVE_024) — a short, accurate note, not a doc rewrite.
- **Files**: docs/README/CHANGELOG as applicable (small, targeted); result recorded in the Activity Log.

## Test Strategy

- Black-box over committed fixtures (DIRECTIVE_036): drive the analyzer's public API, assert on emitted
  findings. Deterministic: identical input → identical findings (NFR-003).
- The full frozen-corpus diff is an evidence step (recorded), not a committed test.
- Run: `go test ./internal/analyzer/ -run 'Corpus|Codex' -v` then `go test ./...`.

## Risks & Mitigations

- **Corpus not representative** → curate sessions that actually exhibit read-content FPs; include a
  genuine failure for the recall assertion.
- **Private content in fixtures** → commit only a redacted minimal subset; keep the full corpus local.
- **Fixture test asserting presence only** → assert the ABSENCE side (read content → no finding) too.

## Review Guidance

- Verify the committed fixture test drives the analyzer through its public API (no internal imports).
- Verify the fixture test asserts BOTH absence-of-read-FP and presence-of-real-failure.
- Verify `go test ./...` green and no report-schema drift.
- Verify the frozen-corpus result is recorded with concrete before/after counts, and the C1 doc note exists.

## Activity Log

- 2026-07-03T21:38:22Z – system – Prompt created.
- 2026-07-03T23:17:29Z – claude – shell_pid=10089 – Fixture test (non-vacuous, black-box) + frozen-corpus diff (95->86, all FP, zero TP loss) + README C1 note; Codex APPROVE-WITH-NITS, determinism nit fixed. go test ./... green.
- 2026-07-03T23:17:32Z – codex – shell_pid=13944 – Started review via action command
- 2026-07-03T23:17:41Z – user – shell_pid=13944 – Codex APPROVE-WITH-NITS; determinism check strengthened to byte-identical findings. Corpus: -9 findings all verified FP, zero recall loss. NFR-004 schema unchanged.
