# Tier-3 Unclassified-Anomaly Trap

Design + report-contract reference for the Tier-3 anomaly trap (issue #15). Companion
to `docs/design/issue-4-failure-scan-channel-scoping.md` (the channel model Tier-3
consumes).

## Why

The precision work in #4 (per-pattern channel scoping) and the #11 fast-follows made
detection sharper — but the sharper the tool looks for *known* failure conditions, the
more structurally blind it becomes to failure modes no rule anticipates. Tier-3 is the
deliberate, deterministic counterweight: it re-captures output/structured signals that
clearly indicate a problem but match **no** existing fingerprint, reports them
**segregated** (never a confirmed failure), and feeds a self-improvement loop so the
anomaly stream shrinks as distinctions fold back into Tier-1/Tier-2.

## The tiered model

- **Tier 1** — specific fingerprints (`failureRules` in `fingerprints.go`) and the
  structural `jsonHasError` check: confirmed failures.
- **Tier 2** — `generic_error`: a known generic distress signal in the output channel
  with no specific rule (`genericFailureSignals`, fires only when no Tier-1 rule did).
- **Tier 3** — **unclassified anomaly**: an output/structured signal that matches
  neither a Tier-1 rule nor the Tier-2 fallback. Reported for triage, **never** counted
  as a confirmed failure and **never** contributing to `findings` / failure counts.

Tier-3 is **residual-only**: it emits for an event only when that event produced no
Tier-1 and no Tier-2 finding (`len(event.Failures)==0`, since `generic_error` is
appended into the same slice).

## Residual trigger set (deliberately tight)

Re-derived against the current classifiers so it cannot re-admit the benign-chatter
false positives #4 closed. Everything already covered by Tier-1/Tier-2 is out of scope
by construction; the genuine residue is small:

| Kind | Channel | Trigger |
|------|---------|---------|
| `structured_exit_status` | structured | a **top-level** `exit_status` with a non-zero numeric value |
| `crash_panic` | output | `panic:` in the output channel |
| `crash_segfault` | output | `segmentation fault` in the output channel |
| `crash_core_dumped` | output | `core dumped` in the output channel |

Notes:
- `exit_status` is the one structured indicator `jsonHasError` does not cover
  (`exit_code`/`returncode`/`return_code` are Tier-1), and it never reaches the output
  channel in a Tier-2-catchable text form. It is read **top-level only** — never a
  recursive walk — so detection is deterministic and cannot reach into content the
  post-#13 channel model excluded (codex read/inspection, file/edit content).
- `Traceback (most recent call last):` is **excluded** — it is already Tier-1/Tier-2.
- A crash message that also contains Tier-2 text wins the residual-only rule: e.g. a Go
  `panic: runtime error: …` trips Tier-2 `generic_error` (the "error:" signal) and is
  reported as a **finding**, not a Tier-3 anomaly. That is correct by design.

## Channel + gate discipline

- Tier-3 scans only the **post-#13** `outputCh` (crash sigs) and the top-level decoded
  object (`exit_status`). Narrative, codex-read, and file/edit content are already
  excluded upstream, so they can never mint an anomaly.
- Emission runs at the single `parseFile` post-gate append site and additionally
  requires `!isArtifactKind(kind)` — because `skipArtifactMessage` keeps an artifact
  JSON event with `Kind != "message"` and no failures, so artifact/spec events are
  barred from minting anomalies independently of that gate.

## Report contract (additive)

Tier-3 adds one top-level array to the report JSON, `anomalies`, parallel to
`findings`. It is **purely additive**: absent consumers are unaffected; it is
normalized to `[]` (never `null`). There is **no** report schema-version field —
`#23` deliberately removed the top-level `version` field (the report now leads with a
nested `build` object), and adding an optional array is backward-compatible, so no
version mechanism is reintroduced.

```jsonc
{
  "build": { ... },
  "findings": [ ... ],          // unchanged by Tier-3
  "anomalies": [
    {
      "signature_hash": "<full 64-char sha256 hex>",
      "kind": "crash_panic",
      "channel": "output",
      "title": "Unclassified anomaly: panic in command output",
      "count": 2,
      "first_seq": 41,
      "last_seq": 88,
      "evidence": [
        { "seq": 41, "source_path": "a.jsonl", "line": 12, "snippet": "panic: …" }
      ]
    }
  ]
}
```

### Signature hash

`signature_hash = sha256(channel \0 tool \0 kind \0 normalizedToken)`, rendered as the
**full 64-char hex digest**. `normalizedToken` lowercases the full matched signal and
collapses digit runs and path/hex runs to placeholders, so the same shape groups across
files and runs regardless of incidental numbers or paths. The full digest — never a
truncated prefix — is both the group key and the ignore-registry key, so a collision
cannot suppress an unrelated anomaly. The hash uses the **full** matched line, not the
bounded evidence snippet.

## The self-improvement loop (promote → refine → ignore)

The point of Tier-3 is that its volume trends **down** as coverage improves; a spike is
an early warning of a new failure mode or an upstream change. A maintainer triages each
recurring anomaly group by one of:

1. **Promote** — add a Tier-1 fingerprint for it (it becomes a confirmed failure).
2. **Refine** — widen/adjust an existing rule so it now covers the shape.
3. **Ignore** — paste the group's `signature_hash` into `ignoredAnomalySignatures`
   (`anomaly.go`) with a one-line benign reason; it is suppressed on the next run.

As distinctions fold back in, the anomaly disappears. v1 keeps the loop deliberately
simple (a checked-in Go registry + a code edit); richer triage tooling (dashboards,
promotion automation) is out of scope.

## Validation

- Golden matrix (`anomaly_test.go`, `anomaly_wiring_test.go`) encodes every trigger and
  suppression case (`contracts/anomaly-matrix.md`).
- Frozen-corpus test (`anomaly_corpus_test.go` + `testdata/anomaly/`) asserts, on a
  fixture whose 7 lines map deterministically to seqs 1–7: **additivity** — findings are
  exactly `{json_error_event@seq4, generic_error@seq5}` and `Summary.FailureEvents == 2`
  / `FailureModes == 2`, so the anomaly events (seq 1–3) contribute zero findings and
  zero to the failure summary; the **genuine anomaly set** — exactly one
  `structured_exit_status@seq1`, `crash_panic@seq2`, `crash_segfault@seq3`; **no
  double-count** — anomaly seqs {1,2,3} are disjoint from finding seqs {4,5}; and
  **determinism** across repeated runs.
- The fuller manual validation — a back-to-back base-vs-candidate diff over frozen
  `~/.claude` + `~/.codex` with separate caches — is documented in the mission's
  `quickstart.md`.
