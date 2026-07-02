# Contract: Channel classification of failure detection

This is a library/CLI behavior contract (not a network API). It specifies the
observable classification output for representative input events — the black-box
contract that the example-based tests (DIRECTIVE_036) assert against.

## Contract A — text-rule channel scoping

Given an event and a failure signature `S`:

| Event shape (where `S` appears) | Channel | Output-scoped rule fires? | Diagnostic-scoped rule fires? |
|---|---|---|---|
| assistant/user message text | narrative | **no** | yes |
| codex `payload` reasoning/message | narrative | **no** | yes |
| `toolUseResult.stdout` / `.stderr` | output | yes | yes |
| bare-string `toolUseResult` (incl. JSON-decoded) | output | yes | yes |
| Claude `tool_result` content block | output | yes | yes |
| codex `payload.output` (function_call_output) | output | yes | yes |
| top-level `error`/`exception`/`traceback` | output | yes | yes |
| Edit/Write (`newString`/`oldString`/`structuredPatch`) | excluded | **no** | **no** |
| Read (`toolUseResult.file.content`) | excluded | **no** | **no** |

## Contract B — issue #4 four-way reproduction (acceptance)

For signature `AssertionError`:

| Input line | Expected (post-change) |
|---|---|
| assistant text "catch the AssertionError and log it" | not a failure |
| Edit writing `raise AssertionError('boom')` | not a failure |
| codex reasoning "handle AssertionError defensively" | not a failure |
| `toolUseResult.stderr` = `E  AssertionError: boom` | `test_failure` |

## Contract C — no-regression invariant (FR-002 / SC-002)

| Input | Expected |
|---|---|
| `branch_worktree_confusion` signature in narrative | still classified (`diagnostic`) |
| generic `merge … failed` prose in narrative (`merge_operation_failed`) | not classified (now `output`) |
| `CONFLICT` in command output (`merge_conflict`) | classified (`output`) |

## Contract D — `obj == nil` plain text

| Input | Expected |
|---|---|
| artifact/spec line discussing an error (kind ∈ artifact kinds) | not a failure |
| transcript stray non-JSON line containing real output failure text | output-eligible |
| standalone `.log` command-output file | output-eligible |
| generic standalone `.txt`/`.md`/`.yaml` prose file | unsupported / not classified |

## Contract E — determinism & schema

- Same input logs ⇒ identical failures (FR-006).
- Report schema/`report.version` unchanged (NFR-003); only failure membership changes.
