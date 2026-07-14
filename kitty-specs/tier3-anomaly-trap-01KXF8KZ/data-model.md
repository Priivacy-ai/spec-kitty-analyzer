# Data Model — Tier-3 Unclassified-Anomaly Trap

Phase 1 output. Types, fields, invariants, and the signature-hash contract.

## Report.Anomalies (additive top-level field)

```
Report {
  build       Build
  ...
  findings    []Finding
  anomalies   []Anomaly     // NEW — additive; JSON key "anomalies"; parallel to findings
  ...
}
```

- Additive only; absent/empty when no anomalies. **Not** counted in `Summary.FailureEvents` / `FailureModes`. **No** schema-version field (research.md D3).

## Anomaly (report record, grouped by signature hash)

| Field | JSON | Type | Meaning |
|-------|------|------|---------|
| SignatureHash | `signature_hash` | string | Short hex; the group key. Stable across files/runs for identical shapes. |
| Kind | `kind` | string | The residual signal kind: `structured_exit_status`, `crash_panic`, `crash_segfault`, `crash_core_dumped`. |
| Channel | `channel` | string | `output` or `structured`. |
| Title | `title` | string | Human label, e.g. "Unclassified anomaly: non-zero exit_status". |
| Count | `count` | int | Number of events sharing this signature hash. |
| Evidence | `evidence` | []AnomalyEvidence | Per-occurrence provenance (bounded). |
| FirstSeq | `first_seq` | int | Timeline seq of the first occurrence. |
| LastSeq | `last_seq` | int | Timeline seq of the last occurrence. |

## AnomalyEvidence (per-occurrence provenance)

| Field | JSON | Type | Meaning |
|-------|------|------|---------|
| SourcePath | `source_path` | string | File the anomaly came from. |
| Seq | `seq` | int | Timeline seq of the event. |
| Line | `line` | int | Source line (when known). |
| Snippet | `snippet` | string | Bounded excerpt of the matched signal (scrubbed, length-capped). |

Mirrors `Finding`/`FindingEvidence` plus `channel`, `kind`, `signature_hash`. Evidence list is capped (e.g. first N occurrences) to bound report size; `Count` still reflects the true total.

## anomalyCandidate (in-memory only — NOT serialized)

Unexported field on `TimelineEvent`, computed at event construction while `obj` is live:

```
type anomalyCandidate struct {
    kind    string   // structured_exit_status | crash_panic | crash_segfault | crash_core_dumped
    channel string   // output | structured
    snippet string   // bounded, scrubbed
    // signatureHash is derived at aggregation from (channel, kind, normalizedToken)
}
```

- Set only when the event is residual (no Tier-1/Tier-2 finding) and not artifact-suppressed. `nil` otherwise.
- `encoding/json` never serializes it (unexported), so the report schema is unaffected (mirrors the `outputCh`/`diagnosticCh` precedent).

## Signature hash contract (FR-005, NFR-002)

- **Input**: fixed-order tuple `(channel, kind, normalizedToken)`.
- **normalizedToken**: the matched signal canonicalized so incidental variation collapses — lowercased; runs of digits → a single placeholder; path-like/hex runs → a single placeholder. (E.g. `panic: runtime error: index out of range [5]` and `[9]` normalize to the same token.)
- **Hash**: `sha256(channel + "\x00" + kind + "\x00" + normalizedToken)`, rendered as a short hex prefix (e.g. first 12 hex chars).
- **Determinism**: no map iteration in the hash input; identical shapes across files/runs → identical hash → one group.

## Invariants

- **INV-1 (segregation)**: an `Anomaly` never appears in `Findings` and never increments any failure/failure-mode counter.
- **INV-2 (residual-only)**: an event contributes an anomaly candidate only when it produced no Tier-1 and no Tier-2 finding.
- **INV-3 (channel discipline)**: candidates are derived only from the post-#13 output/structured channels; narrative, codex-read, and file/edit content never reach the detector.
- **INV-4 (determinism)**: `buildAnomalies` output is stably ordered — sort by `(signature_hash, first_seq)`; evidence within a group sorted by `seq`.
- **INV-5 (ignore)**: a candidate whose signature hash is in `ignoredAnomalySignatures` is dropped before grouping.
