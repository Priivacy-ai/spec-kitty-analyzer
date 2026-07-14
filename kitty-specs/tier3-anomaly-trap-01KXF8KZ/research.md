# Research — Tier-3 Unclassified-Anomaly Trap

Phase 0 output. Records the material design decisions (DIRECTIVE_003) and the re-derived residual-coverage matrix (FR-008), grounded in the current code.

## D1 — Residual trigger set (FR-008), re-derived against current code

**Decision**: Tier-3's net-new trigger set is exactly:
- **Structured**: a `exit_status` key with a non-zero numeric value.
- **Output crash signatures**: `panic:`, `segmentation fault`, `core dumped`.

**Rationale** — grounded in the current classifiers:

Tier-1 `jsonHasError` (`json_helpers.go`) already consumes, as `json_error_event`:
- non-empty `error` / `exception` / `traceback`;
- non-zero `exit_code` / `returncode` / `return_code`;
- `status`/`outcome`/`kind`/`verdict` ∈ {failed, failure, blocked, rejected, error}.

Tier-1 fingerprints (`fingerprints.go`) already include `Traceback (most recent call last):` (the Python crash form).

Tier-2 `genericFailureSignals` (`fingerprints.go`, output-channel-only, fires only when no Tier-1 rule matched) already catches the **text** forms:
- `Error:\s`;
- `Traceback (most recent call last):`;
- `\b(exit code|exit status|returncode|return code)\s*[:=]?\s*[1-9][0-9]*\b`;
- `command|hook|tool|process|subprocess|pytest|ruff|mypy|spec-kitty … failed|failure`.

Therefore the genuinely-uncovered residue is small:
- **`exit_status` (structured key)** — NOT in `jsonHasError`'s numeric-key list; and Tier-2's text regex matches the *space* form "exit status", not the JSON *underscore* key `exit_status`, which does not appear space-formed in the channel string. → real gap.
- **`panic:` / `segmentation fault` / `core dumped`** — not fingerprinted and not in `genericFailureSignals`. (A Go panic under `go test` often also emits "exit status 2", which Tier-2 already catches; in that case Tier-3 stays silent by residual-only design — no double-count. The residual value is a panic/segfault captured *without* an accompanying exit-status line.) → real gap.

**Alternatives considered / rejected**: adding the parenthetical/localized denial forms or bare generic words (`unexpected`, `aborted`, `unhandled`, `failure`) — rejected: those are the benign-chatter tokens that reopen the #4 FP class, and the issue explicitly excludes them. FR-008 is enforced as a **test obligation**: golden negatives assert Tier-1/Tier-2 events yield no anomaly.

## D2 — Structured-signal retention (C-004)

**Decision**: Compute the anomaly candidate **during event construction**, while the decoded JSON `obj` and the channel strings are still in scope (`analyzer.go`, right after `classifyFailuresWithChannels`), and stash it as an **unexported** `anomalyCandidate` field on `TimelineEvent`. `buildAnomalies` later reads the stashed candidates after `Seq` assignment.

**Rationale**: `TimelineEvent` retains `outputCh`/`diagnosticCh` but **not** the raw decoded object, so a post-hoc `buildAnomalies(events)` pass cannot re-read structured fields like `exit_status`. Computing inline (no second walk) is the deep-module-design-consistent choice and matches how channels were cached. Raw JSON is **never** serialized into the report (NFR / schema hygiene).

**Alternatives considered / rejected**: (a) serialize raw JSON onto the event for a later pass — rejected (schema bloat, and it would leak into the report); (b) a second full extraction walk — rejected (wasteful, and the whole point of the cached channels was to avoid it).

## D3 — Report-version signaling for the additive schema (C-005)

**Decision**: Ship `Report.Anomalies` as a **purely additive** top-level array with **no report schema-version field**. Document the addition in the report-contract docs.

**Rationale**: #23 deliberately **removed** the top-level report `version` field (a C1 breaking change; `build_test.go` guards its absence) and the report now leads with a nested `build` object. The report has **no** schema-version mechanism today, and adding a new optional array is backward-compatible for every existing consumer (absent/empty when there are no anomalies). Reintroducing a version field would partially undo #23 and add contract surface with no consumer benefit right now (DIRECTIVE_024, locality).

**Alternatives considered / rejected**: (a) reintroduce an explicit report schema version — rejected (reverses #23's intent; premature); (b) carry a schema version in build metadata — rejected (conflates build identity with report-schema identity). If a future change to the report is ever **non-additive**, introducing a schema version is revisited then, as its own decision.

> This is the one decision with a whiff of product-contract flavor; the issue explicitly delegated it to the plan, and it is surfaced here for the post-plan Codex review (and Kent) to challenge.

## D4 — Ignore-registry form (FR-006)

**Decision**: A checked-in **Go allowlist** — `ignoredAnomalySignatures map[string]string` (signature hash → human reason) in `anomaly.go`. `buildAnomalies` skips any candidate whose hash is a key.

**Rationale**: Consistent with how the analyzer already encodes detection knowledge in code (`genericFailureSignals`, the read-command allowlist, the fingerprint rules). No file IO, trivially testable, and it ships in the binary deterministically. For v1 the promote/refine/ignore loop is a code edit + rebuild — acceptable, and richer triage tooling is explicitly out of scope (C-007).

**Alternatives considered / rejected**: an embedded JSON/YAML registry via `go:embed` — deferred; it buys editability-without-recompile that a compiled single-maintainer CLI does not need yet, at the cost of a parse/validation surface.

## D5 — Residual-only gate mechanics

**Decision**: Emit an anomaly candidate only when `len(failures)==0` after `classifyFailuresWithChannels` **and** `skipArtifactMessage` did not drop the event.

**Rationale**: `generic_error` (Tier-2) is appended into the same `failures` slice as the Tier-1 rules, so an empty slice is a precise, single-check proxy for "neither Tier-1 nor Tier-2 fired". Routing through the existing `skipArtifactMessage` gate (rather than a new gate) satisfies C-002 and the pre-placed comment in `analyzer.go`.

## Post-plan Codex design review — findings folded (2026-07-14)

Codex (spec-kitty-review profile) reviewed spec+plan+research+data-model+contracts against current code. Verdict **READY-WITH-CHANGES**; 3 HIGH, 4 MEDIUM, 1 LOW. All verified against code and folded:

- **H1 — artifact gate is not blanket (analyzer.go:389).** `skipArtifactMessage` returns *false* (keeps the event) for an artifact-kind event with `Kind != "message"` and no failures, so routing anomalies "through the gate" would not stop an artifact/spec event from minting one. **Resolution:** anomaly emission is additionally gated on `!isArtifactKind(kind)` — anomalies only come from non-artifact (real session output) events. Updates FR-004 / C-002 / IC-02 / Scenario 7 / matrix N10.
- **H2 — candidate timing (analyzer.go:290,332).** The gate runs *after* classification in `parseFile` and can mutate `event.Failures`. **Resolution:** compute/stash the candidate at the **post-gate append site** in `parseFile` (both the single-object branch ~L290 and the scanner-loop branch ~L335), using the finalized `event.Failures` for the residual-only check and the in-scope top-level `obj` for structured reads. D2/D5 amended below.
- **H3 — structured read must be bounded + deterministic (json_helpers.go firstJSONNumberByKey).** A recursive unsorted-map walk is nondeterministic and would dig `exit_status` out of content the post-#13 channel model excluded (reintroducing the #13 FP class). **Resolution:** read `exit_status` from the **top-level decoded object only** (`obj["exit_status"]`, numeric, non-zero) — one deterministic key access, channel-respecting. D1 amended.
- **M1 — embedded-JSON boundary.** `exit_status` inside a re-decoded tool-result string or codex envelope is *not* structurally scanned (top-level-only). **Resolution:** documented as an explicit boundary; recall-safe (a rare nested exit_status is at worst a missed anomaly, never a FP) — see D1.
- **M2 — multi-signal events.** An event may carry both `exit_status` and a crash sig. **Resolution:** an event stashes a **slice** `anomalyCandidates []anomalyCandidate`; each candidate becomes its own anomaly (its own hash). No priority rule. data-model updated.
- **M3 — hash tuple + registry key.** **Resolution:** signature hash is over `(channel, tool, kind, normalizedToken)` (adds `tool` per FR-005); the ignore-registry key is the **full-length** hex digest (not a 12-char prefix) so collisions cannot suppress unrelated anomalies. A short prefix may be shown for display only. data-model updated.
- **M4 — two recompute paths (analyzer.go:69,117).** `buildFindings` runs in both `Analyze` and `filterReportByMission`. **Resolution:** call `buildAnomalies(report.Timeline)` at **both** sites (mission-filtered timeline rebuilds anomalies correctly). IC-05 updated.
- **L1 — null vs [] (normalize.go).** **Resolution:** `normalizeReport` normalizes `Anomalies` to `[]` (non-`omitempty`), matching the other report slices. IC-05 / data-model updated.

### D1 amendment (H3, M1)
Structured detection reads `exit_status` **only at the top level** of the decoded event object — a single deterministic key access, non-zero numeric — never a recursive walk. This keeps it deterministic and inside the post-#13 channel discipline (it cannot reach into excluded read/edit content). `exit_status` embedded inside a re-decoded tool-result string or a codex envelope is an explicit non-goal for structured detection (recall-safe).

### D2/D5 amendment (H1, H2)
The candidate is computed and stashed at the **post-gate append site** in `parseFile`, for kept events only, when **both** `!isArtifactKind(kind)` **and** the finalized `len(event.Failures)==0`. Structured reads use the in-scope top-level `obj` (JSON branches); crash-sig reads use `event.outputCh`. This removes the pre-gate/post-gate staleness and the artifact-event leak.

## References (current code)

- `internal/analyzer/json_helpers.go` — `jsonHasError`, `firstJSONNumberByKey`, `firstJSONStringByKey`.
- `internal/analyzer/fingerprints.go` — `genericFailureSignals`, `genericFailureToolText`, the `Traceback` fingerprint, the `len(out)==0` Tier-2 fallback.
- `internal/analyzer/types.go` — `TimelineEvent.outputCh`/`diagnosticCh` cache (+ the "Tier-3 anomaly trap, separate PR" comment); `Report` struct (nested `Build` first, no top-level `version`).
- `internal/analyzer/analyzer.go` — event construction, `classifyFailuresWithChannels`, the single `skipArtifactMessage` gate (+ "will later also gate Tier-3 anomalies" comment), the `buildFindings` call site.
- `internal/analyzer/channels.go` — post-#13 `channelContext` (codex read-content excluded from both channels).
