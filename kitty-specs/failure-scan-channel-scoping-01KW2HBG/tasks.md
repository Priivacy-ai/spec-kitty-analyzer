# Tasks: Scope failure detection to real channels

**Mission:** failure-scan-channel-scoping-01KW2HBG
**Branch:** `fix/failure-scan-channel-scoping` (planning base = merge target)
**Design source:** `docs/design/issue-4-failure-scan-channel-scoping.md` (Codex-reviewed) + plan IC-01…IC-05.

All work is localized to `internal/analyzer/`. WPs are split by **file ownership**
(no two WPs touch the same file) and run **sequentially**: WP01 → WP02 → WP03.

## Subtask Index

| ID | Description | WP | Parallel |
|----|-------------|----|----------|
| T001 | Define channel model + extraction entry points in `channels.go` | WP01 | |
| T002 | Implement per-harness extraction matrix (§3c) → output/narrative/excluded | WP01 | [P] |
| T003 | Apply §3a universal exclusion (code-edit/file-read), generalizing `jsonLooksLikeSourceRead` | WP01 | |
| T004 | Recursive JSON re-decode of bare-string `toolUseResult`; log unmapped shapes (excluded by default) | WP01 | |
| T005 | Golden tests per harness shape in `channels_test.go` (§7.3) | WP01 | |
| T006 | Structural-vs-text ordering fixture: source-read obj also carrying `error`/`status` (§7.4) | WP01 | |
| T007 | Add explicit `scope` (output\|diagnostic) to the pattern type; `failureRules` use scoped patterns | WP02 | |
| T008 | Apply the §4 taxonomy: `branch_worktree_confusion`→diagnostic; split `merge_conflict`/`worktree_linkage_broken`; `reviewer_failed`/`stale_agent`→output; all else output | WP02 | |
| T009 | Rework `classifyFailures` to match each pattern against the cached output/diagnostic text for its scope; route `genericFailureSignal` through output | WP02 | |
| T010 | Explicit-scope build test — fail if any text pattern is unscoped (§7.1) | WP02 | |
| T011 | Output-scoped narrative negative test — `merge_*` prose must NOT classify (§7.2b) | WP02 | |
| T012 | Class-B regression guard at the rule level — `branch_worktree_confusion` narrative still classifies (§7.2) | WP02 | |
| T013 | `TimelineEvent`: add cached `outputText`/`diagnosticText` fields (`types.go`) | WP03 | |
| T014 | Build & cache the two channel strings once per event in event construction via `channels.go` (§5) | WP03 | |
| T015 | `obj == nil` plain-text model: artifact/spec kinds → diagnostic-only; transcript text → output-eligible; generic `.log` unsupported (§3d) | WP03 | |
| T016 | Explicit classification ordering + `skipArtifactMessage` single suppression gate before aggregation (§5) | WP03 | |
| T017 | Acceptance four-way repro (Contract B) + `obj==nil` tests (§7.5) in `analyzer_test.go` | WP03 | |
| T018 | Adapt existing `analyzer_test.go` call sites to the new `classifyFailures` signature; preserve the narrative-only `branch_worktree_confusion` test | WP03 | |
| T019 | Corpus FN/FP sweep (main vs candidate binary) per `quickstart.md`; record both-directions evidence for the PR (§7.6) | WP03 | |

---

## WP01 — Channel extraction module

**Goal:** Produce, for one event, the text belonging to each channel class
(`output`, `narrative`) with code-edit/file-read content excluded — the single source
the text rules will scan. Foundation; no behavior change to classification yet.
**Priority:** P1 (foundation). **Independent test:** golden tests assert correct
output/narrative/excluded routing for every harness shape, built and passing in isolation.

### Included subtasks
- [x] T001 Define channel model + extraction entry points in `channels.go` (WP01)
- [x] T002 Implement per-harness extraction matrix (§3c) (WP01)
- [x] T003 Apply §3a universal exclusion (code-edit/file-read) (WP01)
- [x] T004 Recursive JSON re-decode + unmapped-shape logging (WP01)
- [x] T005 Golden tests per harness shape in `channels_test.go` (WP01)
- [x] T006 Structural-vs-text ordering fixture (WP01)

**Implementation sketch:** new `internal/analyzer/channels.go` exposing
`outputText(obj) string` and `diagnosticText(obj) string` (diagnostic ⊇ output),
both driven by the §3c matrix with the §3a exclusion applied; golden-tested in
`channels_test.go`. **Dependencies:** none. **Risks:** harness-shape coverage;
JSON-string re-decode; unmapped shapes must default to excluded-from-output and be logged.
**Estimated prompt size:** ~320 lines.

## WP02 — Per-pattern scope + rule reclassification

**Goal:** Tag every failure regex with an explicit channel scope (no default), split
mixed rules per §4, and run each pattern only against its scope's text.
**Priority:** P1. **Independent test:** explicit-scope build test passes; the
output-scoped narrative negative and the Class-B regression guard pass.

### Included subtasks
- [x] T007 Add explicit `scope` to the pattern type; scoped `failureRules` (WP02)
- [x] T008 Apply the §4 taxonomy (WP02)
- [x] T009 Rework `classifyFailures` to scan per-scope cached text; route `genericFailureSignal` through output (WP02)
- [x] T010 Explicit-scope build test (§7.1) (WP02)
- [x] T011 Output-scoped narrative negative test (§7.2b) (WP02)
- [x] T012 Class-B regression guard (§7.2) (WP02)

**Implementation sketch:** in `internal/analyzer/fingerprints.go`, give each pattern a
required `scope`; convert the `failureRules` table; rework `classifyFailures` to accept
the cached `outputText`/`diagnosticText` (computed by WP03) and match each pattern
against the string for its scope; tests in `fingerprints_test.go`.
**Dependencies:** WP01. **Risks:** the `branch_worktree_confusion` regression; correct
split of `merge_conflict`/`worktree_linkage_broken`; build-failing test if any pattern
is left unscoped. **Estimated prompt size:** ~360 lines.

## WP03 — Event wiring, obj==nil model, ordering + validation

**Goal:** Compute/cache the channel strings per event, route plain-text correctly,
make the classification ordering explicit, and produce the charter-required corpus
evidence. **Priority:** P1. **Independent test:** the issue-#4 four-way reproduction
classifies only the stderr line; corpus sweep shows FP down + `branch_worktree_confusion`
unchanged.

### Included subtasks
- [ ] T013 `TimelineEvent` cached `outputText`/`diagnosticText` (`types.go`) (WP03)
- [ ] T014 Build & cache the two strings once per event via `channels.go` (WP03)
- [ ] T015 `obj == nil` plain-text model (§3d) (WP03)
- [ ] T016 Explicit ordering + `skipArtifactMessage` single suppression gate (§5) (WP03)
- [ ] T017 Acceptance four-way repro + `obj==nil` tests (WP03)
- [ ] T018 Adapt existing `analyzer_test.go` call sites; keep narrative-only test (WP03)
- [ ] T019 Corpus FN/FP sweep evidence for the PR (§7.6) (WP03)

**Implementation sketch:** in `internal/analyzer/analyzer.go` build the cached strings
during event construction (fields added in `types.go`), implement the `obj==nil`
artifact-vs-transcript routing, make the ordering explicit, and apply
`skipArtifactMessage` as a single suppression gate before aggregation; acceptance +
obj==nil tests in `analyzer_test.go`; corpus sweep per `quickstart.md`.
**Dependencies:** WP01, WP02. **Risks:** signature adaptation of existing tests;
`skipArtifactMessage` ordering; corpus harness (zsh word-split caveat).
**Estimated prompt size:** ~420 lines.

---

## Dependencies

- WP01 → (none)
- WP02 → WP01
- WP03 → WP01, WP02

## MVP scope

There is no partial MVP — the three WPs form one coherent fix and must all land for the
issue-#4 behavior change to be correct and non-regressing. WP01 is the foundation.

## Parallelization

None: strictly sequential by dependency (shared-subsystem change with file-split
ownership). Parallelism is intentionally traded for clean, independently-reviewable diffs.
