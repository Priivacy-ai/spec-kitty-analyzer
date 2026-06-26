# Phase 1 Data Model: Scope failure detection to real channels

This is an in-memory analysis change; there is no persistence and no schema/storage
migration. The "model" below is the set of conceptual types the design introduces or
refines.

## Channel (classification of an event's text)

| Channel | Meaning | Scanned by |
|---------|---------|------------|
| `output` | real command/tool output + structured error fields (stdout, stderr, bare/JSON-decoded `toolUseResult`, `tool_result` blocks, codex `payload.output`, top-level `error`/`exception`/`traceback`) | `output` and `diagnostic` patterns |
| `narrative` | assistant/user message text, codex `payload.content`/reasoning | `diagnostic` patterns only |
| `excluded` | code-edit (`newString`/`oldString`/`structuredPatch`) and file-read (`file.content`) | no failure text rule |

Invariant: `diagnostic` text = `output` text ∪ `narrative` text. `excluded` content is
never part of either.

## Pattern (refined)

- `regexp` — the compiled signature (unchanged).
- `scope` — `output` | `diagnostic`. **Required** (no zero value); build-time test fails
  if unset.

## FailureRule (refined)

- `id`, `title`, `severity`, `recovery` — unchanged.
- `patterns` — now `[]scopedPattern` (each carries its own `scope`), so one rule may
  hold both `output` and `diagnostic` patterns (enables splitting mixed rules).

Validation rules:
- Every pattern has an explicit scope (FR-006 determinism + Decision 3).
- Only patterns meeting the distinctiveness gate may be `diagnostic` (today:
  `branch_worktree_confusion`).

## TimelineEvent (refined use)

- Gains two cached, per-event derived strings: `outputText`, `diagnosticText`
  (built once via the extraction matrix; `diagnosticText` ⊇ `outputText`).
- `source kind` (existing) governs the `obj == nil` plain-text routing (Decision 4).

## State transitions

None. Classification is a pure function of an event's channel texts and the rule set.
