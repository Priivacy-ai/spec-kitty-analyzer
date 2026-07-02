# Quickstart & Validation: Scope failure detection to real channels

## Build & unit tests

```bash
cd /Users/kentgale/repos/spec-kitty-analyzer
go build ./...
go vet ./...
go test ./...            # incl. internal/analyzer table-driven + golden + regression tests
```

Local green `build && vet && test` is the merge gate (charter Quality Gates).

## Corpus validation evidence (REQUIRED in the PR — charter gate for detection changes)

Detection changes must record both-directions corpus evidence (FP reduction with no FN
regression), validated against real Spec Kitty logs, not only fixtures.

1. Build `main` and the candidate branch binaries:
   ```bash
   go build -o /tmp/ska-main ./cmd/spec-kitty-analyzer            # on main
   go build -o /tmp/ska-cand ./cmd/spec-kitty-analyzer            # on fix/failure-scan-channel-scoping
   ```
2. Run both over a representative sample of the ~233 locally cached missions and diff
   per-mission failure-fingerprint counts and the by-id breakdown. (Proven harness from
   the design session — note: zsh does not word-split `$var` in for-loops; list mission
   slugs literally or use bash.)
3. Required outcomes to record in the PR:
   - `generic_error` / `timeout` / `test_failure` narrative FPs **down** (e.g. the
     agent-workspace mission's `generic_error` −22 baseline).
   - `branch_worktree_confusion` preserves genuine narrative detections (finalize-inbox 10→2: 2 genuine detections retained, 8 baseline FPs dropped) — SC-002.
   - No event that was a *true* failure before becomes unreported after — SC-003.
   - `analyze` wall-clock on the largest cached mission within +10% — NFR-002.

## Acceptance smoke (issue #4 four-way repro)

Feed the four-line repro from issue #4 through `analyze`; expect only the `stderr`
line to classify as `test_failure` (Contract B). Confirm `branch_worktree_confusion`
in narrative still classifies (Contract C).

## Out of scope (do not add here)

Anomaly trap (Tier 3), versioning/CHANGELOG, structural-rule changes.
