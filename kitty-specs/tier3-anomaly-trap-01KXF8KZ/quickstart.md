# Quickstart — Validating the Tier-3 Anomaly Trap

How to build, test, and validate the feature.

## Build + unit tests

```bash
cd /Users/kentgale/repos/spec-kitty-analyzer
go build ./...
go vet ./...
gofmt -l internal/            # expect no output
go test ./...                 # expect PASS (includes anomaly_test.go golden matrix)
```

The golden matrix (`internal/analyzer/anomaly_test.go`) encodes every row of `contracts/anomaly-matrix.md`: positives P1–P4, residual-only negatives N1–N5, chatter/channel negatives N6–N10, and grouping/ignore/determinism G1–G4.

## Frozen-corpus validation (NFR-001, NFR-004; C-006)

Prove Tier-3 is additive (no change to confirmed failures) and that the anomaly set is genuine.

1. **Assemble a frozen, representative corpus** (defined during implementation) — a fixed snapshot of `~/.claude` + `~/.codex` sessions copied to a stable location. Do **not** analyze the live directories (they include this session's growing transcript → cross-run drift).
2. **Base vs candidate, back-to-back, separate caches** (avoids the live-session-in-corpus confound):

```bash
# base = current main build; candidate = feature build
git worktree add /tmp/tier3-base main && (cd /tmp/tier3-base && go build -o /tmp/analyzer-base ./cmd/...)
go build -o /tmp/analyzer-cand ./cmd/...

/tmp/analyzer-base analyze <frozen-corpus> --cache /tmp/cache-base --json > /tmp/base.json
/tmp/analyzer-cand analyze <frozen-corpus> --cache /tmp/cache-cand --json > /tmp/cand.json
```

3. **Assert additivity** — `findings` and all `summary` failure counts must be byte-identical:

```bash
jq '{findings, summary}' /tmp/base.json > /tmp/base.core.json
jq '{findings, summary}' /tmp/cand.json > /tmp/cand.core.json
diff /tmp/base.core.json /tmp/cand.core.json && echo "ADDITIVE OK (zero findings delta)"
```

4. **Inspect the anomaly set** — `jq '.anomalies' /tmp/cand.json`. Every group must be either **promotable** (a real failure mode worth a fingerprint) or **ignorable** (benign → add its `signature_hash` to `ignoredAnomalySignatures`). No group should be pure chatter. Confirm no anomaly shares an event (seq) with a Tier-1/Tier-2 finding.

## Expected outcome

- `go test ./...` green; report JSON gains only the additive `anomalies` key.
- Zero change to `findings`/failure counts on the frozen corpus.
- The anomaly section reads as an actionable triage queue, not noise.
