# Contract — Anomaly Matrix (input shape → outcome)

Specification-by-example. Every row is a golden test case (`anomaly_test.go`). "Outcome" is anomaly vs no-anomaly and, when an anomaly, its `kind`/`channel`. All inputs are assumed to have passed the single `skipArtifactMessage` gate unless the row says otherwise.

## Positive — residual signals fire (FR-001, FR-002)

| # | Event shape (post-#13 channels) | Tier-1/Tier-2? | Outcome |
|---|--------------------------------|----------------|---------|
| P1 | structured `{"exit_status": 2}`, no `error`/`exit_code`/status field | none | anomaly `kind=structured_exit_status`, `channel=structured` |
| P2 | output contains `panic: runtime error: index out of range [7]`, no exit-status line | none | anomaly `kind=crash_panic`, `channel=output` |
| P3 | output contains `signal: segmentation fault` | none | anomaly `kind=crash_segfault`, `channel=output` |
| P4 | output contains `Aborted (core dumped)` | none | anomaly `kind=crash_core_dumped`, `channel=output` |

## Negative — residual-only, no double-count (FR-003)

| # | Event shape | Tier-1/Tier-2? | Outcome |
|---|-------------|----------------|---------|
| N1 | structured `{"exit_code": 1}` | Tier-1 `json_error_event` | **no anomaly** (Tier-1 fired) |
| N2 | structured `{"status": "failed"}` | Tier-1 `json_error_event` | **no anomaly** |
| N3 | output `Traceback (most recent call last):` | Tier-1 fingerprint / Tier-2 | **no anomaly** |
| N4 | output `command failed: exit status 2` | Tier-2 `generic_error` | **no anomaly** (Tier-2 fired; panic-with-exit-status also lands here) |
| N5 | output `pytest ... failed` | Tier-2 `generic_error` | **no anomaly** |

## Negative — no benign chatter / channel discipline (FR-004)

| # | Event shape | Outcome |
|---|-------------|---------|
| N6 | output contains only a bare word: `unexpected` / `aborted` / `unhandled` / `failure` (no crash sig, no structured signal) | **no anomaly** |
| N7 | `panic:` appears only in the **narrative** channel (assistant prose), not output | **no anomaly** |
| N8 | `exit_status: 2` / `panic:` appears inside **codex read/inspection** content (excluded by post-#13 channels) | **no anomaly** |
| N9 | `panic:` appears inside **file-read/code-edit** content (§3a excluded) | **no anomaly** |
| N10 | artifact/spec event dropped by `skipArtifactMessage` even though it carries `exit_status: 2` | **no anomaly** |

## Grouping, ignore, determinism (FR-005, FR-006, FR-007, NFR-002)

| # | Scenario | Outcome |
|---|----------|---------|
| G1 | `panic: ... [5]` in file A and `panic: ... [9]` in file B | one `Anomaly`, `count=2`, shared `signature_hash`; evidence sorted by seq |
| G2 | Same input analyzed twice | identical `anomalies` array (order + hashes) |
| G3 | Signature hash of P2 added to `ignoredAnomalySignatures` | P2 anomaly **suppressed** on next run |
| G4 | Any run producing anomalies | `findings` and all `Summary` failure counts identical to a base build with Tier-3 disabled (segregation) |

## Report shape (additive)

```jsonc
{
  "build": { ... },
  "findings": [ ... ],          // unchanged
  "anomalies": [                // NEW additive top-level key
    {
      "signature_hash": "9f2a1c4e7b03",
      "kind": "crash_panic",
      "channel": "output",
      "title": "Unclassified anomaly: panic in command output",
      "count": 2,
      "first_seq": 41,
      "last_seq": 88,
      "evidence": [
        { "source_path": "a.jsonl", "seq": 41, "line": 12, "snippet": "panic: runtime error: index out of range [N]" }
      ]
    }
  ]
}
```
- No `version`/schema-version field is added (research.md D3).
- `anomalies` is `[]`/omitted when there are none.
