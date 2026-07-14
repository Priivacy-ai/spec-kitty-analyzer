# Tier-3 Unclassified-Anomaly Trap

**Mission**: tier3-anomaly-trap-01KXF8KZ
**Type**: software-dev
**Purpose (TL;DR)**: Add a segregated Tier-3 "unclassified anomaly" channel so the analyzer surfaces novel failure modes without inflating confirmed-failure counts.

## Overview

The analyzer classifies harness session logs into failure fingerprints. The precision work in #4 (per-pattern channel scoping) and the #11 fast-follows (companion tokens, source gating) pushed hard toward specificity: the more precisely the tool looks for *known* conditions, the more *useful* its output — but the more it becomes structurally **blind to failure modes no rule anticipates**. Left alone, precision buys accuracy at the cost of recall against the unknown.

This mission adds the deliberate, deterministic counterweight: a **Tier-3 unclassified-anomaly trap**. It re-captures output/structured signals that clearly indicate a problem but match **no** existing fingerprint, reports them **segregated** (never counted as confirmed failures), and feeds a **promote → refine → ignore** self-improvement loop so the anomaly stream shrinks over time and its volume becomes a health / early-warning metric.

The tiered model this makes explicit:

- **Tier 1** — specific fingerprints (`failureRules`): confirmed failures.
- **Tier 2** — `generic_error`: known generic distress in the output channel with no specific rule (the existing output-only fallback).
- **Tier 3 (new)** — **unclassified anomaly**: an output/structured signal that suggests a problem but matches neither a Tier-1 rule nor the Tier-2 fallback. Reported for triage, **never** counted as a confirmed failure and **never** contributing to `findings` / failure counts.

Tier-3 is residual-only by construction: it fires for an event **only if** no Tier-1 or Tier-2 finding already fired for that event. Because much of the intuitive trigger surface is already consumed by Tier-1 (`jsonHasError` → `json_error_event`) and the Tier-2 fallback, the genuine net-new surface is small; its exact membership is re-derived against current fingerprint coverage during planning. The feature's primary value is the **framework** — segregated anomaly channel, stable signature-hash grouping, an ignore registry, and the self-improvement loop — not a large new trigger set.

Sequencing dependency #13 is **resolved**: it shipped (`d4aae87`), and the codex read-content false-positive class is fixed in the channel layer via a per-file correlation-id → command registry that keeps file-read/inspection content out of both the output and diagnostic channels. Tier-3 scans the **post-#13** channel strings and inherits that discipline for free; it does not re-introduce read-content scanning and does not duplicate a codex-read guard.

Addresses #15; the last remaining #11 fast-follow item (E); the recall-preserving counterpart to #4.

## User Scenarios & Testing

**Primary actor**: the analyzer classifying harness session logs; and the maintainer triaging the resulting report to grow detection coverage.

### Scenario 1 — a genuinely-uncovered structured signal surfaces (recall)
1. An event carries a structured failure indicator that no Tier-1 rule covers (e.g. a non-zero `exit_status`, an uncovered status value) and produces no Tier-1/Tier-2 finding.
2. The analyzer emits a **Tier-3 anomaly** for that event, with provenance and a signature hash.
3. The anomaly appears in the segregated anomaly section — never in `findings`, never in failure counts.

### Scenario 2 — an unfingerprinted crash signature in output (recall)
1. A command's output contains a strong crash signature not already fingerprinted (e.g. `panic:`, `segmentation fault`, `core dumped`) and no Tier-1/Tier-2 finding fired.
2. The analyzer emits a Tier-3 anomaly for the event.

### Scenario 3 — already a confirmed failure → no anomaly (residual-only, no double-count)
1. An event carries a non-zero `exit_code` / `error` field already consumed by Tier-1 `json_error_event` (or matched by any Tier-1 rule, or the Tier-2 `generic_error` fallback).
2. The analyzer produces the existing finding and emits **no** Tier-3 anomaly for that event.

### Scenario 4 — generic distress word alone → no anomaly (precision preserved)
1. Output/narrative contains a bare generic word (`unexpected`, `aborted`, `unhandled`, `failure`) with no structured indicator and no strong crash signature.
2. The analyzer emits **no** anomaly — these are exactly the benign-chatter tokens that would reopen the #4 false-positive class.

### Scenario 5 — narrative and read content never mint anomalies (precision preserved)
1. Failure-like tokens appear only in the narrative channel, or in codex read/inspection or file-read/code-edit content excluded by the post-#13 channel discipline (§3a).
2. The analyzer emits **no** anomaly — Tier-3 scans only the output/structured channels that survive the existing exclusions.

### Scenario 6 — same shape groups; ignore registry suppresses (self-improvement loop)
1. The same anomaly shape occurs across multiple files/runs.
2. All occurrences share one signature hash and are grouped with a count and first/last occurrence.
3. A maintainer adds that hash to the ignore registry; on the next run the anomaly is suppressed.

### Scenario 7 — artifact/spec events never mint anomalies
1. An artifact or spec message that the single suppression gate already drops flows through classification.
2. Anomaly emission routes through the **same** gate, so no anomaly is minted for it.

### Rule / invariant that must always hold
- **Segregation is absolute.** A Tier-3 anomaly never enters `findings` and never contributes to any failure count or failure-mode total.
- **Residual-only.** No anomaly is emitted for an event that already produced a Tier-1 or Tier-2 finding.
- **No benign chatter.** Generic distress words alone, narrative-channel content, and excluded read/edit content never trigger an anomaly.

## Domain Language

| Term | Meaning | Avoid |
|------|---------|-------|
| Tier 1 | Specific fingerprint match → confirmed failure (`failureRules`). | — |
| Tier 2 | Generic output-channel distress fallback (`generic_error`). | — |
| Tier 3 / anomaly | Residual, segregated unclassified signal; never a confirmed failure. | Do not call it a "failure" or "finding". |
| Signature hash | Channel/tool/token-normalized identifier that groups identical anomaly shapes across files and runs. | — |
| Ignore registry | Checked-in list of confirmed-benign signature hashes that suppress anomalies. | — |

## Requirements

### Functional Requirements

| ID | Requirement | Status |
|----|-------------|--------|
| FR-001 | Tier-3 emits an anomaly when a **structured failure indicator that no Tier-1 rule covers** appears in the output/structured channel and no Tier-1/Tier-2 finding fired for the event (e.g. a non-zero `exit_status`, or another structured distress signal confirmed uncovered during planning). | Draft |
| FR-002 | Tier-3 emits an anomaly when a **strong crash signature not already fingerprinted** appears in the output channel and no Tier-1/Tier-2 finding fired (candidate set: `panic:`, `segmentation fault`, `core dumped`, and stack-trace shapes not already covered by an existing rule). | Draft |
| FR-003 | **Residual-only**: a Tier-3 anomaly is emitted for an event only when classification produced neither a Tier-1 finding nor the Tier-2 `generic_error` finding for that event. | Draft |
| FR-004 | Tier-3 does **not** emit an anomaly on: a bare generic distress word with no structured/crash signal; narrative-channel content; codex read/inspection or file-read/code-edit content excluded by the post-#13 channel discipline; or any event dropped by the single artifact/spec suppression gate. | Draft |
| FR-005 | Each anomaly carries provenance: source path, seq/line, channel (output vs structured), the matched signal kind, and a bounded snippet — plus a **signature hash normalized by channel/tool/token** so identical shapes group across files and runs. | Draft |
| FR-006 | An **ignore registry** (a checked-in list of confirmed-benign signature hashes) suppresses listed anomalies from the report. | Draft |
| FR-007 | Anomalies are **segregated** in the report as a new top-level collection (parallel to `findings`), not merged into `findings` and not counted in any failure/failure-mode summary total; anomalies are grouped by signature hash with a count and first/last occurrence, in deterministic order. | Draft |
| FR-008 | The exact Tier-3 trigger set is **re-derived against current fingerprint coverage during planning** and documented, so Tier-3 covers only genuinely-uncovered residual signals rather than re-detecting what Tier-1/Tier-2 already catch. | Draft |

### Non-Functional Requirements

| ID | Requirement | Threshold / Measure | Status |
|----|-------------|---------------------|--------|
| NFR-001 | Existing confirmed-failure output is unchanged (Tier-3 is additive/segregated). | `findings` and all failure/failure-mode counts are identical base vs candidate, verified by a back-to-back run over one frozen corpus snapshot. | Draft |
| NFR-002 | Anomaly detection is deterministic. | Same input log → identical anomalies and ordering across repeated runs; no map-iteration nondeterminism. | Draft |
| NFR-003 | The existing test suite stays green. | `go test ./...` exits 0. | Draft |
| NFR-004 | On the frozen corpus, the anomaly set is dominated by genuine unclassified signals (each promotable or ignorable), not benign chatter, and double-counts zero Tier-1/Tier-2 events. | Manual inspection of the anomaly set + an automated check that no anomaly shares an event with a Tier-1/Tier-2 finding. | Draft |

### Constraints

| ID | Constraint | Status |
|----|-----------|--------|
| C-001 | Reuses the existing §3c cached channel strings (`outputCh`) and the decoded structured object fields; Tier-3 is a **distinct feature** and does not change Tier-1/Tier-2 semantics. | Draft |
| C-002 | Anomaly emission routes through the **single** artifact/spec suppression gate (`skipArtifactMessage`), so artifact/spec events never mint anomalies. | Draft |
| C-003 | Tier-3 consumes the **post-#13** channel model; it must never re-introduce scanning of codex read/inspection content and must not duplicate a codex-read guard. | Draft |
| C-004 | Structured-signal evaluation happens while the decoded JSON object is in scope (during event construction) and the Tier-3 candidate is cached on `TimelineEvent` as an **unexported, in-memory** signal for later aggregation; raw JSON is **not** serialized into the report. | Draft |
| C-005 | Report-version signaling for the additive anomalies schema is decided during planning, reconciling the #23 removal of the top-level report `version` field (options: reintroduce an explicit report schema version, carry it in build metadata, or treat the addition as a purely additive contract-doc change); the report contract/docs are updated accordingly. | Draft |
| C-006 | Validation uses a **frozen, representative** corpus (not the live full `~/.claude`/`~/.codex`) plus golden channel-matrix cases; corpus diffs run base and candidate binaries **back-to-back** with separate caches to avoid the live-session-in-corpus confound. | Draft |
| C-007 | Triage tooling beyond the ignore registry (dashboards, promotion automation) is **out of scope** for v1. | Draft |

## Success Criteria

| ID | Criterion |
|----|-----------|
| SC-001 | A maintainer reviewing a report sees a segregated anomalies section listing genuinely-unclassified distress signals, none of which is double-counted as a confirmed failure. |
| SC-002 | Confirmed-failure output (`findings` and failure counts) is identical to base on the frozen corpus. |
| SC-003 | Every surfaced anomaly is actionable: promotable to a Tier-1 fingerprint, refinable into an existing rule, or ignorable via the registry. |
| SC-004 | Adding a signature hash to the ignore registry suppresses that anomaly on the next run. |

## Key Entities

- **Anomaly / AnomalyEvidence** — the segregated report record, mirroring `Finding`/`FindingEvidence` plus the extra provenance fields: channel (output vs structured), signal kind, signature hash, bounded snippet, count, and first/last occurrence.
- **Signature hash** — a channel/tool/token-normalized identifier that groups identical anomaly shapes across files and runs; the grouping and ignore-registry key.
- **Ignore registry** — a checked-in list of confirmed-benign signature hashes that suppress matching anomalies.
- **Structured-signal candidate** — the unexported, in-memory Tier-3 candidate computed during event construction (while the decoded object is available) and consulted by the anomaly-aggregation pass.

## Assumptions

- The genuine net-new trigger surface is small: most of the issue's intuitive trigger list is already consumed by Tier-1 (`jsonHasError`) or Tier-2; planning re-derives the residual set against current code (FR-008).
- #13 has shipped; Tier-3 scans post-#13 channel strings and needs no codex-read guard of its own.
- The frozen validation corpus is assembled during planning, sized for practical back-to-back before/after diffing.
- The report-version signaling approach (C-005) is a plan-level design decision; the spec does not pre-commit to one.
