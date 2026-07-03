---
work_package_id: WP03
title: Per-file prepass & context threading
dependencies:
- WP01
- WP02
requirement_refs:
- FR-002
- NFR-001
tracker_refs: []
planning_base_branch: fix/codex-read-output-scoping
merge_target_branch: fix/codex-read-output-scoping
branch_strategy: Planning artifacts for this mission were generated on fix/codex-read-output-scoping. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into fix/codex-read-output-scoping unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-codex-read-output-scoping-01KWMXCQ
base_commit: 5e6ea6fe9f89cf9f67a3a9f7dd20d84d29c17f88
created_at: '2026-07-03T21:38:22Z'
subtasks:
- T011
- T012
- T013
- T014
- T017
phase: Phase 3 - Prepass wiring
assignee: ''
agent: claude
history:
- at: '2026-07-03T21:38:22Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/analyzer
create_intent: []
execution_mode: code_change
model: ''
owned_files:
- internal/analyzer/analyzer.go
- internal/analyzer/analyzer_test.go
role: implementer
tags: []
task_type: implement
---

# Work Package Prompt: WP03 – Per-file prepass & context threading

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile specified in the frontmatter, and
behave according to its guidance before parsing the rest of this prompt.

- **Profile**: `implementer-ivan`
- **Role**: `implementer`
- **Agent/tool**: `claude`

If no profile is specified, run `spec-kitty agent profile list` and select the best match for this
work package's `task_type` and `authoritative_surface`.

---

## Markdown Formatting

Wrap HTML/XML tags in backticks. Use language identifiers in code blocks.

---

## Objectives & Success Criteria

Wire WP02's context-aware extraction into the file walk. Build a **per-file prepass** over the raw
file bytes that collects every `function_call` payload into a `channelContext` registry (out-of-order
tolerant), then thread that context through the per-line event construction so
`channelStringsForEvent` consults it — landing the actual FP fix (FR-002).

**Done when:**
- `internal/analyzer/analyzer.go` builds a `channelContext` **once per file** (in `parseFile`, before
  the line-by-line event loop) by scanning the file's `function_call` payloads via WP02's
  `newCodexCall`.
- The context is threaded: `parseFile` → `eventFromJSONObject` → `eventFromText` →
  `channelStringsForEvent`, which calls WP02's `channelTextPairCtx(obj, ctx)` when `obj != nil`.
- Files with no codex `function_call` payloads produce an **empty** context → byte-identical behavior
  to today (the `obj == nil` / source-kind path is unchanged).
- `go build ./... && go vet ./... && go test ./internal/analyzer/ && gofmt -l internal/analyzer/analyzer.go` clean.

## Context & Constraints

- **Authoritative design**: `research.md` R1 (prepass, not inline threaded state — the caching of
  `outCh`/`diagCh` per event is why a prepass is required), `plan.md` IC-02, `data-model.md`.
- **The current code you are extending** (in `analyzer.go`, study it):
  - `parseFile(path, kind, data, startTurn, state)` — scans `data` line by line with a
    `bufio.Scanner`; for each line decodes JSON and calls `eventFromJSONObject`, else `eventFromText`.
    The JSON single-object branch (top of the func) also calls `eventFromJSONObject`.
  - `eventFromJSONObject(path, line, turn, obj)` → `eventFromText(path, line, turn, text, obj)`.
  - `eventFromText(...)` → `channelStringsForEvent(path, text, obj)` → returns `outCh, diagCh`.
  - `channelStringsForEvent(path, text, obj)`: `if obj != nil { return channelTextPair(obj) }` else
    the source-kind path. **You will change the `obj != nil` branch to use the context-aware pair.**
- **From WP02** (call, do not reimplement): `channelContext`, `channelTextPairCtx(obj, ctx)`,
  `newCodexCall(payload) codexCall`, `codexCallID(payload)`.
- **Constraints**: C-001 (prepass builds the registry; stateless entrypoint preserved). Threading must
  be per-file scoped — the registry for file A must never leak into file B. Determinism (NFR-003).

## Branch Strategy

- **Strategy**: already-confirmed
- **Planning base branch**: fix/codex-read-output-scoping
- **Merge target branch**: fix/codex-read-output-scoping

> Execution worktrees are allocated per computed lane from `lanes.json`.

## Subtasks & Detailed Guidance

### Subtask T011 – Per-file prepass `buildCodexContext(data []byte) channelContext`
- **Purpose**: Build the callID → codexCall registry for one file, tolerant of ordering (FR-002, R1).
- **Steps**:
  - Add `func buildCodexContext(data []byte) channelContext`. Scan `data` line by line (same
    `bufio.Scanner` buffer sizing as `parseFile`: `Buffer(make([]byte,0,64*1024), 4*1024*1024)`), plus
    handle the single-JSON-object file shape the way `parseFile` does (a whole-file JSON object).
  - For each decoded object, look for `obj["payload"]` as a map with `type == "function_call"`; for
    each, `call := newCodexCall(payload)` and, when `call.callID != ""`, store
    `ctx.codexCalls[call.callID] = call`.
  - Return a `channelContext` with an initialized map (empty map if no function_calls — NOT nil is
    fine, but the zero value must also be safe per WP02's nil-safe `lookup`).
- **Files**: `internal/analyzer/analyzer.go`.
- **Notes**: This is a *separate* lightweight pass — it only decodes to find `function_call` payloads;
  it does NOT construct `TimelineEvent`s. Because it runs before the main loop, a `function_call_output`
  appearing before its `function_call` in the byte stream still correlates (out-of-order tolerance).

### Subtask T012 – Thread `channelContext` through the walk
- **Purpose**: Carry the per-file context down to channel extraction.
- **Steps**:
  - Thread a `channelContext` parameter through: `eventFromJSONObject(path, line, turn, obj, ctx)`,
    `eventFromText(path, line, turn, text, obj, ctx)`, and `channelStringsForEvent(path, text, obj, ctx)`.
  - Update ALL call sites in `analyzer.go` (the `parseFile` JSON-object branch, the `parseFile` line
    loop, and any test-facing callers within analyzer.go). For the existing plain-text
    `eventFromText(path, lineNo, turn, string(raw), nil)` call, pass the same `ctx` (the `obj == nil`
    path ignores it).
- **Files**: `internal/analyzer/analyzer.go`.
- **Notes**: If `eventFromText`/`eventFromJSONObject`/`channelStringsForEvent` are called from test
  files owned by WP04, coordinate the signature: prefer adding the `ctx` parameter and letting WP04
  pass `channelContext{}`. Note the signature change in your Activity Log so WP04 updates its callers.

### Subtask T013 – Use the context-aware pair when `obj != nil`
- **Purpose**: Land the behavior change at the single extraction site (C-001).
- **Steps**:
  - In `channelStringsForEvent`, change the `if obj != nil { return channelTextPair(obj) }` branch to
    `return channelTextPairCtx(obj, ctx)`.
  - Leave the entire `obj == nil` source-kind switch (`text`/`command_log`/artifact/default) exactly
    as-is — a raw non-JSON line has no payload to correlate.
- **Files**: `internal/analyzer/analyzer.go`.
- **Notes**: An empty `ctx` makes `channelTextPairCtx` behave identically to `channelTextPair`, so
  non-codex files and files with no function_calls are unaffected.

### Subtask T014 – Build the prepass once per file; empty ctx otherwise
- **Purpose**: Invoke the prepass at the right scope with no per-line cost (R1).
- **Steps**:
  - In `parseFile`, compute `ctx := buildCodexContext(data)` once, before both the single-JSON-object
    branch and the line loop, and pass it into every `eventFromJSONObject` / `eventFromText` call.
  - Confirm the prepass is cheap (one extra decode pass, O(lines)); no change to `state` handling,
    `skipArtifactMessage` gating, or the frontmatter logic.
- **Files**: `internal/analyzer/analyzer.go`.
- **Notes**: The registry is a local `ctx` — it lives and dies with the `parseFile` call, so file A's
  calls never leak into file B (per-file scope, C-001).

### Subtask T017 – Analyzer integration test: no read-content FP, recall preserved (TEST-FIRST)
- **Purpose**: Prove the end-to-end behavior through the analyzer's own interface (DIRECTIVE_036
  black-box: drive the analyzer over an in-memory `.jsonl`, assert on emitted findings).
- **Steps**: In `internal/analyzer/analyzer_test.go`, build a small two-line codex `.jsonl` — a
  `function_call` `git diff` + a `function_call_output` exit-0 whose bulk contains "exit code 2" and
  "merge" — run the analyzer over it and assert **no** `typer_usage_error` / `merge_operation_failed`
  finding. Add a companion input with a real failing `go build` and assert its failure IS reported
  (recall). Also add an out-of-order case (the `function_call_output` line before its `function_call`)
  to prove the prepass tolerates ordering.
- **Files**: `internal/analyzer/analyzer_test.go`.
- **Notes**: Author **before** T011–T014 (red → green): on unfixed code the read-content case yields a
  `typer_usage_error` finding (the bug), so the assertion goes red for the right reason (DIRECTIVE_034).

## Test Strategy

- **Test-first** (DIRECTIVE_034/039): author the T017 analyzer integration test red (it fails on
  unfixed code because the read content is scanned), then implement T011–T014 to green.
- Run: `go test ./internal/analyzer/ -run 'Codex|Prepass|ReadOutput' -v` then `go test ./internal/analyzer/`.

## Risks & Mitigations

- **Cross-file registry leak** → keep `ctx` local to `parseFile`; never store it on `state`.
- **Signature churn breaking WP04 callers** → note the new `ctx` parameter in the Activity Log; WP04
  passes `channelContext{}` where it does not need correlation.
- **Prepass missing the single-JSON-object file shape** → mirror `parseFile`'s two intake shapes
  (whole-file JSON object + line-delimited).

## Review Guidance

- Confirm the prepass runs once per file, before event construction.
- Confirm `channelStringsForEvent`'s `obj == nil` path is unchanged.
- Confirm empty-context byte-identical behavior on a non-codex fixture (existing tests stay green).
- Confirm out-of-order correlation works (a `function_call_output` line before its `function_call`).

## Activity Log

- 2026-07-03T21:38:22Z – system – Prompt created.
