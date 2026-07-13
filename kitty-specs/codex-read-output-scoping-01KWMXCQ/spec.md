# Codex Read-Output Scoping

**Mission**: codex-read-output-scoping-01KWMXCQ
**Type**: software-dev
**Purpose (TL;DR)**: Stop the analyzer from flagging false failures on codex file-read/inspection output that merely contains error-like text.

## Overview

The analyzer classifies harness session logs into failure fingerprints by scanning the **output channel** for failure signatures. Codex surfaces file reads and inspections (`cat`, `head`, `rg`/`grep`, `git show|diff|log`) as ordinary `exec_command` calls whose **output is content** — source, diffs, PR/issue bodies — not command-failure output. Because that content routes wholesale to the scanned output channel, the text failure rules fire on failure-like tokens that merely *appear in* a file or document, producing false-positive detections (e.g. `typer_usage_error` on "exit code 2" inside source/docs, `merge_operation_failed` on PR bodies discussing merges).

This is the codex analog of the false-positive class that channel scoping (#4) closed for Claude. The existing code-edit/file-read exclusion only recognizes the Claude tool-result shape; codex reads reach the channel layer as an ordinary command result, indistinguishable at that layer from a real command's output. The command **intent** lives only in the *preceding* `function_call` event (which shares a `call_id` with the output and carries the command in its arguments).

This mission correlates each codex `function_call_output` to its originating `function_call`, and excludes the output of read/inspection commands from failure scanning — while still detecting genuine command failures. It also finishes mapping the codex payload types that remain unmapped.

Closes #13; addresses the remaining codex-payload-mapping item of #11.

## User Scenarios & Testing

**Primary actor**: the analyzer classifying codex session logs; and the maintainer/teammate consuming the resulting failure report.

### Scenario 1 — read content is not a failure (happy path)
1. A codex session runs `git diff` (exit 0) whose diff text contains the word "error" or "exit code 2".
2. The analyzer recognizes the call as a read/inspection and excludes its output from failure scanning.
3. No false-positive failure is reported for that content.

### Scenario 2 — a real command failure is still caught (recall)
1. A codex session runs `go build ./...` (a real command) that fails.
2. The analyzer scans its output as before and reports the failure.

### Scenario 3 — a real error on a read command (recall preserved)
1. A codex session runs `cat missing_file` (a read) that exits non-zero with "No such file".
2. The analyzer keeps the command's **status/error header** (so the genuine failure remains detectable) but does not scan the bulk read content.

### Scenario 4 — compound command with a real build (recall)
1. A codex session runs `git diff && go build` (a read followed by a real command).
2. Because not every segment is a read, the output is scanned normally — a build failure is still caught.

### Scenario 5 — id spelling variance
1. A codex payload spells the correlation id `callId` (camelCase) rather than `call_id`.
2. The analyzer still correlates the output to its command.

### Rule / invariant that must always hold
- **No recall loss.** Excluding read content must never suppress a genuine command failure. When intent is unknown (no correlated command, unrecognized command, or an unparseable output envelope), the analyzer defaults to scanning (recall-safe).

## Requirements

### Functional Requirements

| ID | Requirement | Status |
|----|-------------|--------|
| FR-001 | Output of a codex read/inspection command is excluded from failure scanning — from **both** the output and the diagnostic (narrative) channels — so its content is not matched by any failure rule. | Draft |
| FR-002 | The command behind a codex `function_call_output` is correlated from the preceding `function_call` via their shared correlation id, using a per-source-file registry populated before channel extraction. | Draft |
| FR-003 | A codex command is classified as read/inspection only when **every** operator-split (`&&`/`\|\|`/`;`/`\|`) segment's leading token is a known read command, with no write redirection and no mutating operation; otherwise it is treated as a real command (scanned). | Draft |
| FR-004 | Read-command output is handled envelope-aware: an exit-0 read has its output fully excluded; a non-zero/interrupted read retains only its status/error header on the scanned channel; an unparseable envelope defaults to scanning. | Draft |
| FR-005 | A non-read command, or an output whose correlation id has no matching command, is scanned exactly as before (recall-safe default). | Draft |
| FR-006 | The genuinely-unmapped codex payload types are mapped: `function_call` (excluded from channels; registers command metadata), `task_started` (excluded marker), `user_message` (routed to narrative when it carries user prose), and empty `payload.type` (excluded). Already-mapped types (`function_call_output`, `reasoning`, `message`, `agent_message`, `task_complete`, `token_count`) are unchanged. | Draft |
| FR-007 | Both `call_id` and `callId` spellings of the correlation id are recognized. | Draft |

### Non-Functional Requirements

| ID | Requirement | Threshold / Measure | Status |
|----|-------------|---------------------|--------|
| NFR-001 | No recall loss: genuine command failures remain detected. | Zero true-positive failure detections lost on the frozen validation corpus (before vs after). | Draft |
| NFR-002 | False positives from codex read content are substantially reduced. | Read-content false positives for the affected rules (`typer_usage_error`, `merge_operation_failed`, and others inflated by read content) are eliminated or near-eliminated on the frozen corpus. | Draft |
| NFR-003 | Classification is deterministic. | Same input log → identical findings across repeated runs. | Draft |
| NFR-004 | The existing test suite stays green and the report JSON schema is unchanged. | `go test ./...` exits 0; no new/renamed fields in the emitted report. | Draft |

### Constraints

| ID | Constraint | Status |
|----|-----------|--------|
| C-001 | Cross-event correlation is implemented as a per-file prepass building a correlation-id → command registry, carried into extraction via a context value; the existing stateless channel-extraction entrypoint is preserved (empty context) for back-compat and tests. | Draft |
| C-002 | Read exclusion mirrors the existing Claude file-read exclusion exactly — read content reaches neither the output nor the diagnostic channel (the diagnostic channel is also scanned). | Draft |
| C-003 | No new shell-parser dependency; command classification uses a small, conservative segment classifier over a read-command allowlist. | Draft |
| C-004 | Scoped to Codex. The multi-LLM / per-harness corpus strategy (#22) is explicitly out of scope and planned separately. | Draft |
| C-005 | Validation uses a **frozen, representative** codex corpus (not the live full `~/.codex`, which is impractically large) plus golden channel-matrix cases; corpus diffs run base and candidate binaries back-to-back with separate caches. | Draft |

## Success Criteria

| ID | Criterion |
|----|-----------|
| SC-001 | On the frozen validation corpus, false-positive failure detections attributable to codex read/inspection content are eliminated or near-eliminated. |
| SC-002 | Zero genuine (true-positive) failure detections are lost on the frozen corpus. |
| SC-003 | A maintainer reviewing a codex-session failure report sees no finding whose evidence is file/diff/document content from a read command. |

## Key Entities

- **Codex call record** — the correlated metadata for a codex command: its correlation id, the tool/function name, the parsed command string, and whether it is a read/inspection. Built from `function_call` events; consulted when a `function_call_output` is classified.
- **Channel context** — the per-source-file carrier that makes the codex call registry available to channel extraction without making the failure rules aware of codex transcript structure.

## Assumptions

- Within a codex session file, a `function_call` appears before its paired `function_call_output`; the prepass makes correlation robust to ordering regardless.
- The command string is available in the `function_call` arguments (`arguments.cmd`); read-file tool calls may instead be identified by the function `name`.
- The frozen validation corpus will be assembled from representative codex sessions (defined during planning), sized for practical before/after diffing.
