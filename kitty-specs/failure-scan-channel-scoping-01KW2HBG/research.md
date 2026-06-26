# Phase 0 Research: Scope failure detection to real channels

Research for this mission was conducted ahead of the mission as a standalone design
study (`docs/design/issue-4-failure-scan-channel-scoping.md`), including empirical
corpus measurement and two adversarial Codex review rounds. This file consolidates the
resulting decisions. **No `[NEEDS CLARIFICATION]` markers remain.**

## Decision 1 — Per-pattern channel scope (not per-rule, not a global switch)

- **Decision**: Tag each failure regex `output` or `diagnostic`; scan it only against
  that channel's text. Universally exclude code-edit and file-read content from all
  failure text rules.
- **Rationale**: A blanket "output-only" switch was prototyped (`proto/scan-output-channels-only`)
  and measured: it cut narrative false positives (`generic_error` −22 on one mission)
  but **dropped `branch_worktree_confusion` 10→0** — a false-negative regression on a
  validated true positive (it surfaced upstream #1716/#2046). Some signatures
  legitimately live in narrative; per-pattern scope distinguishes them.
- **Alternatives rejected**: (a) blanket output-only — causes the FN regression;
  (b) per-rule scope — too coarse, since `merge_conflict` / `worktree_linkage_broken`
  mix distinctive output signatures with generic prose.

## Decision 2 — Distinctiveness gate for `diagnostic`

- **Decision**: A pattern earns `diagnostic` (narrative-eligible) scope only if a prose
  match is overwhelmingly a *report of an observed condition*, not discussion of a
  problem class. Distinctiveness of the pattern, not the rule's topic, is the gate.
- **Rationale**: Settled in the Codex scope debate. `branch_worktree_confusion`'s
  patterns (`mission targets … branch`, `No auto-detection is performed.*branch`) pass;
  `merge_operation_failed`'s broad `merge … failed` windows fail (match routine
  discussion) and stay `output`. Today only `branch_worktree_confusion` is `diagnostic`.
- **Alternatives rejected**: classifying by topic/category (would wrongly promote
  generic merge/worktree prose).

## Decision 3 — Explicit, test-enforced scope (no implicit default)

- **Decision**: Every text pattern declares its scope; a unit test fails the build if
  any is unset.
- **Rationale**: An implicit `output` default is silent false-negative debt (Codex
  finding #3). Also corrects the v1 misfiling of `reviewer_failed`/`stale_agent` as
  structural — they are text rules and are `output`.

## Decision 4 — `obj == nil` plain-text model

- **Decision**: Artifact/spec source kinds (`work_package`, `mission_artifact`,
  `mission_meta`, `mission_status_snapshot`) → `diagnostic`-only; transcript stray text
  → output-eligible; generic standalone `.log`/`.txt` → explicitly unsupported for now.
- **Rationale**: Closes the live FP path on plain-text artifacts without suppressing
  genuine raw output logs (Codex finding #1).

## Decision 5 — Channel extraction via an explicit per-harness schema matrix

- **Decision**: Define and golden-test an extraction matrix per harness shape before
  migrating rules; recurse into JSON-string `toolUseResult`; handle `tool_result`
  content blocks and structured `error` objects; typed codex `payload.type` paths.
  Unmapped shapes default to excluded-from-output and are logged.
- **Rationale**: Today there is no harness-aware extraction (everything is flattened);
  a bare key allowlist is insufficient (Codex finding #4).

## Decision 6 — Anomaly trap deferred; precision tuning out of scope

- **Decision**: The Tier-3 "unexpected result" anomaly trap ships as a separate
  fast-follow mission; per-rule regex precision tuning is out of scope here.
- **Rationale**: Keeps #4 small and reviewable (DIRECTIVE_039); the trap adds machinery
  that deserves its own review.

## Performance note

Cost is held to one channel-extraction pass per event by caching `outputText` /
`diagnosticText` and matching patterns against the cached strings — avoiding
`O(rules × event size)` re-walks (Codex finding #6). Target: ≤10% overhead (NFR-002).
