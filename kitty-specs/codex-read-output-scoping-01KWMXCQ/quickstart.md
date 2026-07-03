# Quickstart — Validating Codex Read-Output Scoping

## Unit / golden matrix
```bash
go test ./internal/analyzer/ -run 'Codex|Channel|ReadOutput' -v
go test ./...   # full suite green (NFR-004)
```
Golden cases mirror `contracts/channel-matrix.md`: a `function_call`(read) + `function_call_output`(exit 0) yields empty output channel; a non-zero read keeps only the header; a real command scans fully; compound scans; call_id AND callId correlate.

## Frozen-corpus before/after (FP down, TP preserved)
```bash
# base = current main binary; cand = this branch's binary
go build -o /tmp/ska-base <main-checkout>/cmd/spec-kitty-analyzer   # or reuse a saved base
go build -o /tmp/ska-cand ./cmd/spec-kitty-analyzer
FROZEN=~/spec-kitty-analyzer-issue4-backup/catfood-findings/frozen-codex-corpus   # curated set (define during impl)
/tmp/ska-base analyze "$FROZEN" --cache-bust --cache /tmp/base.cache --out /tmp/base.json
/tmp/ska-cand analyze "$FROZEN" --cache-bust --cache /tmp/cand.cache --out /tmp/cand.json
# compare fingerprint counts; expect typer_usage_error / merge_operation_failed read-content FPs down,
# and NO real-failure fingerprint lost (diff must show FP-only reductions).
```
Run base+candidate **back-to-back in one job** with **separate caches** (live-session-in-corpus confound).

## Acceptance
- Read-content FPs eliminated/near-eliminated on the frozen corpus (SC-001).
- Zero true-positive failures lost (SC-002).
- No finding whose evidence is file/diff/doc content from a read command (SC-003).
