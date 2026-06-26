# Feature Specification: Scope failure detection to real channels

**Mission:** failure-scan-channel-scoping-01KW2HBG
**Type:** software-dev
**Related issue:** Priivacy-ai/spec-kitty-analyzer#4

## Purpose

When the analyzer reads a mission's logs, it should report a failure only when a
failure *actually occurred*. Today it reports a failure whenever a failure-looking
phrase appears anywhere in an event — including text that merely *discusses* error
handling (assistant/user narrative, model reasoning) or *implements* it (code edits,
file reads). This inflates failure counts with false positives and erodes trust in
the analyzer's reports, which is especially damaging now that the tool is being used
to triage real product issues.

This mission scopes failure detection to the channels where a genuine failure would
appear — real command/tool output and a small set of distinctive diagnostic signals —
so reported failures reflect what truly happened, **without losing** detections that
legitimately read off narrative (notably branch/worktree confusion).

## User Scenarios & Testing

**Primary actor:** a maintainer (or an automated catfooding loop) running `analyze`
on a completed mission's agent logs to find real failures and recovery guidance.

**Primary scenario:** the maintainer runs `analyze`; events that only talk about or
write error-handling code are reported as ordinary activity, while events where a
command or tool actually failed are reported as failures with recovery guidance.

### Acceptance scenarios

1. **Narrative mention is not a failure.** Given a log event that is assistant/user
   message text saying e.g. "remember to catch the AssertionError here", When the
   maintainer runs `analyze`, Then no failure is reported for that event.
2. **Code edit is not a failure.** Given an event that writes error-handling code
   (e.g. an edit inserting `raise AssertionError('boom')`), When `analyze` runs, Then
   no failure is reported for that event.
3. **Model reasoning is not a failure.** Given a model-reasoning event discussing how
   to handle an error, When `analyze` runs, Then no failure is reported for that event.
4. **Real output still fails.** Given an event whose command/tool output contains a
   genuine failure (e.g. a test runner emitting `AssertionError` on its error stream),
   When `analyze` runs, Then the corresponding failure IS reported.
5. **Diagnostic detections are preserved (no regression).** Given a log in which the
   branch/worktree-confusion condition is described in narrative (the signature that
   surfaced upstream split-topology issues #1716/#2046), When `analyze` runs, Then
   that detection IS still reported.

### Edge cases

- **Output quoting a failure string** (e.g. test output that itself prints
  "AssertionError"): still reported — it is genuine output, so this is correct.
- **Plain-text artifact/spec files** that merely discuss errors in prose: not
  reported as failures.
- **Logs from different harnesses** (the supported agent-log formats): the same
  channel discipline applies regardless of which harness produced the log.

## Domain Language

- **Output channel** — text captured from a command or tool's actual execution
  (e.g. standard output/error, structured tool results, structured error fields).
- **Narrative** — human/model prose: assistant or user messages, model reasoning.
- **Code-edit / file-read content** — file text being written or read, not execution.
- **Diagnostic signal** — a *distinctive* phrasing that reliably reports an observed
  condition rather than discussing a class of problem (e.g. branch/worktree confusion).
- **Failure fingerprint** — a named detection the analyzer reports as a failure.
- **False positive (FP)** — a reported failure that did not occur. **False negative
  (FN)** — a real failure that goes unreported.

## Requirements

### Functional Requirements

| ID | Requirement | Status |
|----|-------------|--------|
| FR-001 | The analyzer MUST report a failure only when its signature appears in a genuine output channel or a distinctive diagnostic signal — not when it appears solely in narrative, code-edit, or file-read content. | Planned |
| FR-002 | The analyzer MUST continue to detect conditions that legitimately appear in narrative, specifically branch/worktree confusion, with no loss relative to current behavior. | Planned |
| FR-003 | The analyzer MUST continue to report genuine failures present in command/tool output (no loss of true positives across the validation corpus). | Planned |
| FR-004 | Plain-text artifact/spec content that only discusses errors MUST NOT be classified as a failure. | Planned |
| FR-005 | Output that genuinely contains a failure signature MUST still be reported even if that output also echoes or quotes failure text. | Planned |
| FR-006 | Failure reporting MUST remain deterministic: identical input logs yield identical failure classifications. | Planned |

### Non-Functional Requirements

| ID | Requirement | Status |
|----|-------------|--------|
| NFR-001 | The change MUST add no new third-party runtime dependencies. | Planned |
| NFR-002 | `analyze` wall-clock time on the largest locally-cached mission MUST NOT increase by more than 10% versus the pre-change baseline. | Planned |
| NFR-003 | The report output format/schema MUST remain backward compatible (no schema-version change); only which events qualify as failures changes. | Planned |

### Constraints

| ID | Constraint | Status |
|----|------------|--------|
| C-001 | Scope is limited to channel-scoping of failure detection. The novel/"unexpected result" anomaly trap and the versioning/CHANGELOG work are explicitly out of scope (separate efforts). | Active |
| C-002 | Structural (field-keyed) detection rules are not changed by this mission. | Active |
| C-003 | Per-detection regex precision tuning beyond what channel-scoping requires is out of scope (handled in prior/other work). | Active |

## Success Criteria

| ID | Criterion |
|----|-----------|
| SC-001 | On the corpus's worst-case transcript, narrative/edit-sourced false failures are substantially reduced from the measured baseline (baseline example: a transcript where the majority of flagged events came from message/edit channels). |
| SC-002 | Zero regression in branch/worktree-confusion detection across the corpus sample (baseline example: a mission that reports it 10× must still report it 10×). |
| SC-003 | Across a representative multi-mission corpus sweep, no event that was a *true* failure before the change becomes unreported after it. |
| SC-004 | A maintainer reviewing a report can trust that listed failures correspond to events that actually occurred. |

## Key Entities

- **Timeline event** — one parsed log entry; carries text across one or more channels.
- **Channel** — the origin of an event's text (output / diagnostic / narrative /
  code-edit / file-read).
- **Failure fingerprint** — a named, reported failure classification with severity
  and recovery guidance.

## Assumptions

- The locally-cached corpus of missions is representative enough to validate FP
  reduction and the absence of FN regressions in both directions.
- The supported agent-log formats are those the analyzer parses today; new formats
  are out of scope.

## Out of Scope

- The "unexpected result" anomaly trap (recall-preservation tier) — separate mission.
- SemVer/release-management and CHANGELOG automation — separate effort.
- Changes to structural (field-keyed) detection rules.
