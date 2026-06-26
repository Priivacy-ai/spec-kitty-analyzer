---
work_package_id: WP02
title: Per-pattern scope + rule reclassification
dependencies:
- WP01
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-006
tracker_refs: []
planning_base_branch: fix/failure-scan-channel-scoping
merge_target_branch: fix/failure-scan-channel-scoping
branch_strategy: already-confirmed
subtasks:
- T007
- T008
- T009
- T010
- T011
- T012
phase: Phase 2 - Rules
assignee: ''
agent: claude
history:
- at: '2026-06-26T18:05:00Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/fingerprints
create_intent: []
execution_mode: code_change
model: ''
owned_files:
- internal/analyzer/fingerprints.go
- internal/analyzer/fingerprints_test.go
role: implementer
tags: []
task_type: implement
---

# Work Package Prompt: WP02 – Per-pattern scope + rule reclassification

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile specified in the frontmatter, and behave according to its guidance before parsing the rest of this prompt.

- **Profile**: `implementer-ivan`
- **Role**: `implementer`
- **Agent/tool**: `claude`

If no profile is specified, run `spec-kitty agent profile list` and select the best match for this work package's `task_type` and `authoritative_surface`.

---

## Markdown Formatting

Wrap HTML/XML tags in backticks. Use language identifiers in code blocks.

---

## Objectives & Success Criteria

Make failure detection **channel-scoped per regex pattern**. Each text pattern declares
an explicit `output` | `diagnostic` scope (no default), mixed rules are split per §4,
and `classifyFailures` scans each pattern only against the text for its scope.

**Done when:**
- The pattern type carries a required `scope`; `failureRules` is expressed with scoped patterns.
- The §4 taxonomy is applied: only `branch_worktree_confusion` patterns are `diagnostic`; `merge_conflict` / `worktree_linkage_broken` are split (distinctive parts `output`, generic prose NOT promoted); `reviewer_failed` / `stale_agent` are `output`; everything else `output`.
- `classifyFailures` accepts the cached `outputText` / `diagnosticText` (provided by WP03) and matches each pattern against the string for its scope; `genericFailureSignal` runs through the `output` text.
- A build-failing test asserts no text pattern is left unscoped (§7.1).
- The output-scoped narrative negative (§7.2b) and the Class-B regression guard (§7.2) pass.
- `go build ./... && go vet ./... && go test ./internal/analyzer/` is green.

## Context & Constraints

- **Authoritative design**: `docs/design/issue-4-failure-scan-channel-scoping.md` §3b, §4, §5, §6. The §4 taxonomy table is the contract for which pattern gets which scope.
- Mission docs: `plan.md` (IC-02), `data-model.md` (Pattern/FailureRule refined), `contracts/channel-classification.md` (Contracts A & C), `research.md` (Decisions 1–3).
- **Depends on WP01**: consume `outputText` / `diagnosticText` from `internal/analyzer/channels.go`. Do not re-implement extraction.
- Existing code: `internal/analyzer/fingerprints.go` currently has `failureRules` (text-pattern loop ~`:405`), `classifyFailures` (~`:345`), `genericFailureSignal` (~`:427`), `jsonLooksLikeSourceRead` (~`:395`), and the `reviewer_failed`/`stale_agent` regexes (~`:163/174`). The structural `obj != nil` rules (`runtime_blocked`, `guard_failure` field, `review_rejected`, etc.) are **out of scope** — do not change them (§4 "Genuinely structural").
- Constraints: no new deps (NFR-001); determinism (FR-006); schema unchanged (NFR-003).

## Branch Strategy

- **Strategy**: already-confirmed
- **Planning base branch**: fix/failure-scan-channel-scoping
- **Merge target branch**: fix/failure-scan-channel-scoping

> Execution worktrees are allocated per computed lane from `lanes.json`.

## Subtasks & Detailed Guidance

### Subtask T007 – Explicit `scope` on the pattern type
- **Purpose**: Carry channel scope per pattern, not per rule (§3b).
- **Steps**: Introduce a `scope` enum/type (`scopeOutput`, `scopeDiagnostic`) with **no usable zero value** (or a sentinel `scopeUnset` that the build test rejects). Refactor the pattern representation so each regex in `failureRules` declares its scope. One rule may hold both `output` and `diagnostic` patterns (`[]scopedPattern`).
- **Files**: `internal/analyzer/fingerprints.go`.

### Subtask T008 – Apply the §4 taxonomy
- **Purpose**: Assign the correct scope to every text pattern.
- **Steps**: Per §4:
  - `branch_worktree_confusion` patterns → `diagnostic` (the only diagnostic rule today).
  - `merge_conflict`: `CONFLICT`, `Automatic merge failed`, `merge conflict`, `rebase.*conflict` → `output`.
  - `worktree_linkage_broken`: `\.git/worktrees`, `worktree (broken|corrupt)`, `detached worktree references` → `output`.
  - `reviewer_failed`, `stale_agent` → `output` (they are text rules, not structural).
  - All remaining text rules (`test_failure`, `timeout`, `generic_error`, `merge_operation_failed`, `encoding_error`, `permission_denied`, `typer_usage_error`, `wrong_cli_surface`, `config_yaml_invalid`, `skill_surface_missing`, `manifest_drift`, `sync_*`, `tracker_binding_missing`, `dirty_worktree_ref_advance`, `ref_advance_non_fast_forward`, `circular_dependencies`, `no_code_commits`, `runtime_not_initialized`, `namespace_package_import`, text variants of `missing_artifact`/`guard_failure`) → `output`.
- **Files**: `internal/analyzer/fingerprints.go`.
- **Notes**: Do NOT broaden a generic pattern to `diagnostic` to gain recall — tighten first (distinctiveness principle). That is out of scope here.

### Subtask T009 – Rework `classifyFailures` for per-scope matching
- **Purpose**: Run each pattern against its scope's cached text.
- **Steps**: Change `classifyFailures` to accept the cached `outputText` and `diagnosticText` (computed in WP03). For each scoped pattern: match `output` patterns against `outputText`, `diagnostic` patterns against `diagnosticText`. Route `genericFailureSignal` through `outputText`. Drop the old whole-`text` scan and fold `jsonLooksLikeSourceRead`'s role into the WP01 exclusion (the file-read content no longer reaches `outputText`). Preserve the structural `obj != nil` block unchanged.
- **Files**: `internal/analyzer/fingerprints.go`.
- **Notes**: Keep the function signature explicit so WP03's call site is a clean adapter. Coordinate the exact signature with WP03 (it passes the two strings).

### Subtask T010 – Explicit-scope build test (§7.1)
- **Purpose**: No silent FN debt from an unscoped pattern.
- **Steps**: In `internal/analyzer/fingerprints_test.go`, iterate every pattern in `failureRules` and fail if any has `scopeUnset`/no declared scope.
- **Files**: `internal/analyzer/fingerprints_test.go`.

### Subtask T011 – Output-scoped narrative negative (§7.2b)
- **Purpose**: Prove generic merge prose no longer classifies.
- **Steps**: Feed narrative-only text matching `merge_operation_failed` ("the merge failed because…") and `merge_conflict` ("resolve the merge conflict next") via `diagnosticText` with empty `outputText`; assert NO failure classifies (these patterns are `output`-scoped).
- **Files**: `internal/analyzer/fingerprints_test.go`.

### Subtask T012 – Class-B regression guard (§7.2)
- **Purpose**: Prove the `branch_worktree_confusion` detection survives at the rule level.
- **Steps**: Feed its distinctive signature as narrative (in `diagnosticText`, empty `outputText`); assert it DOES classify. Conversely, feed the same signature only via excluded channels (simulated by absence from both cached strings) and assert it does not.
- **Files**: `internal/analyzer/fingerprints_test.go`.

## Test Strategy

- Test-first (DIRECTIVE_034). Tests in `internal/analyzer/fingerprints_test.go`.
- Run: `go test ./internal/analyzer/ -run TestFingerprint`.

## Risks & Mitigations

- **`branch_worktree_confusion` regression** (the FN the blanket prototype caused) → T012 guards it explicitly; it must stay `diagnostic`.
- **Mis-split of mixed rules** → follow §4 exactly; when unsure, keep `output` (never promote a generic pattern).
- **Signature drift with WP03** → define the `classifyFailures` signature here and document it in the Activity Log so WP03's adapter matches.

## Review Guidance

- Every text pattern has an explicit scope; the build test enforces it.
- Only `branch_worktree_confusion` is `diagnostic`. Confirm against §4.
- Structural `obj != nil` rules are untouched.

## Activity Log

- 2026-06-26T18:05:00Z – system – Prompt created.
