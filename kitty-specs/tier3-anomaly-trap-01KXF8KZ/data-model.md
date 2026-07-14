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
| SignatureHash | `signature_hash` | string | Full 64-char sha256 hex; the group key and the ignore-registry key. Stable across files/runs for identical shapes. A maintainer pastes this value directly into `ignoredAnomalySignatures`. |
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

## anomalyCandidates (in-memory only — NOT serialized)

Unexported **slice** field on `TimelineEvent` — an event may carry more than one signal, e.g. both `exit_status` and a `panic:` (M2). Stashed at the post-gate append site in `parseFile`, using the in-scope top-level `obj` and the finalized failures:

```
type anomalyCandidate struct {
    kind    string   // structured_exit_status | crash_panic | crash_segfault | crash_core_dumped
    channel string   // output | structured
    snippet string   // bounded, scrubbed
    // signatureHash is derived at aggregation from (channel, tool, kind, normalizedToken)
}
// on TimelineEvent:
anomalyCandidates []anomalyCandidate
```

- Populated only when the event is **kept**, `!isArtifactKind(kind)` (H1), and the finalized `len(event.Failures)==0` (residual-only; H2). Empty/nil otherwise.
- **Structured read is top-level-only**: `obj["exit_status"]` as a direct numeric access, non-zero — never a recursive walk (H3), so it stays deterministic and within the post-#13 channel exclusion. Nested/embedded `exit_status` is a non-goal (M1).
- `encoding/json` never serializes it (unexported), so the report schema is unaffected (mirrors the `outputCh`/`diagnosticCh` precedent).

## Signature hash contract (FR-005, NFR-002)

- **Input**: fixed-order tuple `(channel, tool, kind, normalizedToken)` — `tool` added per FR-005 (M3); `tool` is `event.ToolName` or `""`.
- **normalizedToken**: the matched signal canonicalized so incidental variation collapses — lowercased; runs of digits → a single placeholder; path-like/hex runs → a single placeholder. (E.g. `panic: runtime error: index out of range [5]` and `[9]` normalize to the same token.)
- **Hash**: `sha256(channel + "\x00" + tool + "\x00" + kind + "\x00" + normalizedToken)`, rendered as the **full 64-char hex digest**. The report's `signature_hash`, the group key, and the ignore-registry key are all this same full digest — no truncated prefix (M3), so a collision cannot suppress an unrelated anomaly and a maintainer can paste the reported value straight into the registry.
- **Determinism**: no map iteration in the hash input; identical shapes across files/runs → identical hash → one group.

## Invariants

- **INV-1 (segregation)**: an `Anomaly` never appears in `Findings` and never increments any failure/failure-mode counter.
- **INV-2 (residual-only)**: an event contributes an anomaly candidate only when it produced no Tier-1 and no Tier-2 finding.
- **INV-3 (channel discipline)**: candidates are derived only from the post-#13 output/structured channels; narrative, codex-read, and file/edit content never reach the detector.
- **INV-4 (determinism)**: `buildAnomalies` output is stably ordered — sort by `(signature_hash, first_seq)`; evidence within a group sorted by `seq`.
- **INV-5 (ignore)**: a candidate whose signature hash is in `ignoredAnomalySignatures` is dropped before grouping.
- **INV-6 (non-artifact)**: anomalies are never minted from artifact/spec kinds — emission requires `!isArtifactKind(kind)` in addition to residual-only (H1).
- **INV-7 (normalize)**: `Report.Anomalies` normalizes to `[]` (never `null`) via `normalizeReport`, matching the other report slices (L1).
