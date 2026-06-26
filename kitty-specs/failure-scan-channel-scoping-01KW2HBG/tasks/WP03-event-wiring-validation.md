---
work_package_id: WP03
title: Event wiring, obj==nil model, ordering + validation
dependencies:
- WP01
- WP02
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-005
- FR-006
tracker_refs: []
planning_base_branch: fix/failure-scan-channel-scoping
merge_target_branch: fix/failure-scan-channel-scoping
branch_strategy: Planning artifacts for this mission were generated on fix/failure-scan-channel-scoping. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into fix/failure-scan-channel-scoping unless the human explicitly redirects the landing branch.
subtasks:
- T013
- T014
- T015
- T016
- T017
- T018
- T019
phase: Phase 3 - Integration & validation
assignee: ''
agent: "claude:opus:reviewer-renata:reviewer"
shell_pid: "52829"
history:
- at: '2026-06-26T18:05:00Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/analyzer
create_intent: []
execution_mode: code_change
model: ''
owned_files:
- internal/analyzer/analyzer.go
- internal/analyzer/analyzer_test.go
- internal/analyzer/types.go
role: implementer
tags: []
task_type: implement
---

# Work Package Prompt: WP03 – Event wiring, obj==nil model, ordering + validation

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

Wire the channel extraction into event construction, resolve the plain-text
(`obj == nil`) model, make the classification ordering explicit, and produce the
charter-required corpus evidence. This WP turns the foundation (WP01) + scoped rules
(WP02) into the observable issue-#4 behavior change.

**Done when:**
- `TimelineEvent` caches `outputText` / `diagnosticText`, built **once** per event via WP01's `channels.go`.
- The `obj == nil` model (§3d) is implemented: artifact/spec source kinds → `diagnostic`-only; transcript stray text → output-eligible; generic `.log`/`.txt` → explicitly unsupported (documented).
- Classification ordering is explicit (structural → §3a exclusion → per-pattern text rules → `generic_error` fallback) and `skipArtifactMessage` is applied as a single suppression gate **before aggregation**.
- The issue-#4 four-way reproduction (Contract B) classifies **only** the `stderr` line; `branch_worktree_confusion` in narrative still classifies (Contract C).
- Existing `analyzer_test.go` call sites are adapted to the new `classifyFailures` signature; the narrative-only `branch_worktree_confusion` test is preserved.
- Corpus FN/FP evidence (both directions) is captured for the PR (§7.6).
- `go build ./... && go vet ./... && go test ./...` is green.

## Context & Constraints

- **Authoritative design**: `docs/design/issue-4-failure-scan-channel-scoping.md` §3d, §5, §7. The §5 ordering and the §7 validation list are the contract.
- Mission docs: `plan.md` (IC-03/04/05), `quickstart.md` (corpus procedure), `contracts/channel-classification.md` (Contracts B, D, E), `research.md` (Decisions 4, 6).
- **Depends on WP01 + WP02**: build the cached strings with `channels.go`; call the reworked `classifyFailures(outputText, diagnosticText, …)` from WP02. Match WP02's documented signature (check its Activity Log).
- Existing code: `internal/analyzer/analyzer.go` — `eventFromJSONObject` (~`:265` builds `text = flattenJSON(obj)`), `eventFromText` (~`:256/310`, the `obj==nil` path), `skipArtifactMessage` (~`:285`, drops artifact-derived failures after classification, subject to `artifactFailureAllowed` exceptions like `review_rejected`). `internal/analyzer/types.go` — `TimelineEvent` struct + the source-`kind` detection.
- Constraints: ≤10% wall-clock overhead on the largest cached mission (NFR-002) — cache the strings, no per-rule re-walk; determinism (FR-006); report schema/`report.version` unchanged (NFR-003); no new deps (NFR-001).

## Branch Strategy

- **Strategy**: already-confirmed
- **Planning base branch**: fix/failure-scan-channel-scoping
- **Merge target branch**: fix/failure-scan-channel-scoping

> Execution worktrees are allocated per computed lane from `lanes.json`.

## Subtasks & Detailed Guidance

### Subtask T013 – Cache fields on `TimelineEvent`
- **Purpose**: Hold the two derived strings per event.
- **Steps**: Add unexported `outputText string` and `diagnosticText string` (or a small struct) to `TimelineEvent` in `types.go`. Do not change the serialized report schema (NFR-003) — these are in-memory only; ensure they are not emitted in JSON output (no exported tags / excluded from the report DTO).
- **Files**: `internal/analyzer/types.go`.

### Subtask T014 – Build & cache the strings once per event
- **Purpose**: Single extraction pass (NFR-002).
- **Steps**: In `eventFromJSONObject`, after `obj` is available, call `outputText(obj)` / `diagnosticText(obj)` once and store on the event. Replace the whole-event `flattenJSON` feed into the text rules with these cached strings (keep `flattenJSON` for any non-failure uses, e.g. the existing Command/summary extraction, unchanged).
- **Files**: `internal/analyzer/analyzer.go`.

### Subtask T015 – `obj == nil` plain-text model (§3d)
- **Purpose**: Stop artifact/spec prose from classifying while preserving genuine raw output logs.
- **Steps**: In `eventFromText` (nil path): if the event's source kind ∈ the artifact/spec kind set (`work_package`, `mission_artifact`, `mission_meta`, `mission_status_snapshot`; treat as "any artifact/spec kind") → put the line in `diagnosticText` only (empty `outputText`). Transcript-derived stray non-JSON lines → output-eligible. Generic standalone `.log`/`.txt` → unsupported for now (leave a code comment documenting the deferral; do not silently treat as output).
- **Files**: `internal/analyzer/analyzer.go`.

### Subtask T016 – Explicit ordering + single suppression gate (§5)
- **Purpose**: Make precedence deterministic and gate artifact suppression once.
- **Steps**: Order classification: (1) structural `obj` rules; (2) §3a exclusion already applied by extraction; (3) per-pattern text rules over the cached strings; (4) `generic_error` fallback (output). Apply `skipArtifactMessage` as a single gate that drops an artifact event's failures **before** they reach `findings` aggregation, honoring existing `artifactFailureAllowed` exceptions (e.g. `review_rejected`). Document that the same gate point will later also gate Tier-3 anomalies (separate PR).
- **Files**: `internal/analyzer/analyzer.go`.

### Subtask T017 – Acceptance four-way repro + obj==nil tests
- **Purpose**: Lock the issue-#4 behavior (Contracts B & D).
- **Steps**: In `analyzer_test.go`: (a) the four-way repro for `AssertionError` — assistant text / Edit / codex reasoning → not a failure; `toolUseResult.stderr` → `test_failure`. (b) `obj==nil`: artifact/spec line discussing an error → not a failure; transcript stray line with real output failure text → output-eligible. (c) Contract C: `branch_worktree_confusion` narrative still classifies.
- **Files**: `internal/analyzer/analyzer_test.go`.

### Subtask T018 – Adapt existing tests to the new signature
- **Purpose**: Keep the suite green; preserve the validated TP test.
- **Steps**: Update existing `analyzer_test.go` call sites for the new `classifyFailures` signature. **Preserve** the narrative-only `branch_worktree_confusion` test (~`analyzer_test.go:115`) — it must still pass.
- **Files**: `internal/analyzer/analyzer_test.go`.

### Subtask T019 – Corpus FN/FP sweep evidence (§7.6)
- **Purpose**: The charter gate for detection changes — both-directions evidence against real logs.
- **Steps**: Follow `quickstart.md`: build `main` and candidate binaries; run both over a representative sample of the ~233 locally cached missions; diff per-mission failure counts + by-id breakdown. Record: `generic_error`/`timeout`/`test_failure` narrative FPs **down** (e.g. agent-workspace `generic_error` −22); `branch_worktree_confusion` **unchanged** (finalize-inbox stays 10×, SC-002); no prior true failure becomes unreported (SC-003); `analyze` wall-clock within +10% (NFR-002). Also spot-check the `timeout ×7` drop (§7.7) to confirm they were narrative. Capture the output for the PR description.
- **Files**: evidence recorded in the PR / Activity Log (no source file; uses the built binaries + `quickstart.md` procedure).
- **Notes**: zsh does not word-split `$var` in for-loops — list mission slugs literally or use bash.

## Test Strategy

- Test-first for T017 (DIRECTIVE_034). Full suite: `go test ./...`.
- Acceptance smoke: feed the four-line issue-#4 repro through `analyze`; expect only the `stderr` line to classify.

## Risks & Mitigations

- **`skipArtifactMessage` ordering** → apply once before aggregation; cover with a test that an artifact event contributes no failure.
- **Existing-test breakage from the signature change** → T018 adapts them; run the whole package frequently.
- **Corpus harness footguns** → bash (not zsh) for the loop; cache-bust only if needed.
- **Performance regression** → verify the single-pass caching holds the +10% bound on the largest mission.

## Review Guidance

- The four-way repro is the headline acceptance: only `stderr` classifies.
- `branch_worktree_confusion` corpus count is unchanged (the no-regression invariant).
- Confirm the cached strings are not leaked into the serialized report (schema stable).
- Corpus evidence (both directions) is present in the PR.

## Activity Log

- 2026-06-26T18:05:00Z – system – Prompt created.
- 2026-06-26T19:57:31Z – claude:opus:implementer-ivan:implementer – shell_pid=18036 – Assigned agent via action command
- 2026-06-26T21:07:02Z – claude:opus:implementer-ivan:implementer – shell_pid=18036 – Wiring + obj==nil + ordering; corpus -46% FP, SC-003 holds, branch 10->2 accepted (8 were FPs). Commit 851c1da
- 2026-06-26T21:07:18Z – claude:opus:reviewer-renata:reviewer – shell_pid=43542 – Started review via action command
- 2026-06-26T21:10:21Z – user – shell_pid=43542 – Capstone verified: wiring, §3d (review_rejected-preserving deviation sound), ordering, acceptance + corpus
- 2026-06-26T21:15:22Z – user – shell_pid=43542 – Moved to planned
- 2026-06-26T21:15:27Z – claude:opus:implementer-ivan:implementer – shell_pid=46430 – Started implementation via action command
- 2026-06-26T21:20:12Z – claude:opus:implementer-ivan:implementer – shell_pid=46430 – Codex cycle-2: uniform artifact suppression + per-failure whitelist + dedup extraction
- 2026-06-26T21:21:51Z – claude:opus:reviewer-renata:reviewer – shell_pid=48953 – Started review via action command
- 2026-06-26T21:24:10Z – user – shell_pid=48953 – Cycle-2 Codex fixes verified: uniform artifact suppression (4 kinds) + per-failure whitelist + single extraction. Stale review-cycle-1.md gate overridden; cycle-2 review passed.
- 2026-06-26T21:26:30Z – user – shell_pid=48953 – Moved to planned
- 2026-06-26T21:26:35Z – claude:opus:implementer-ivan:implementer – shell_pid=50910 – Started implementation via action command
- 2026-06-26T21:30:22Z – claude:opus:implementer-ivan:implementer – shell_pid=50910 – Cycle 3: title-recompute after artifact filter + comment cleanup
- 2026-06-26T21:31:03Z – claude:opus:reviewer-renata:reviewer – shell_pid=52829 – Started review via action command
- 2026-06-26T21:32:46Z – user – shell_pid=52829 – Cycle 3 verified: title recompute after artifact filter + comment cleanup. Stale review-cycle-2.md gate overridden (addressed by cycle-3 commit 4d86584).
