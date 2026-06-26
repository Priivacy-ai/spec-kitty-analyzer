---
work_package_id: WP01
title: Channel extraction module
dependencies: []
requirement_refs:
- FR-001
- FR-004
- FR-005
tracker_refs: []
planning_base_branch: fix/failure-scan-channel-scoping
merge_target_branch: fix/failure-scan-channel-scoping
branch_strategy: Planning artifacts for this mission were generated on fix/failure-scan-channel-scoping. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into fix/failure-scan-channel-scoping unless the human explicitly redirects the landing branch.
subtasks:
- T001
- T002
- T003
- T004
- T005
- T006
phase: Phase 1 - Foundation
assignee: ''
agent: claude
history:
- at: '2026-06-26T18:05:00Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/channels
create_intent:
- internal/analyzer/channels.go
- internal/analyzer/channels_test.go
execution_mode: code_change
model: ''
owned_files:
- internal/analyzer/channels.go
- internal/analyzer/channels_test.go
role: implementer
tags: []
task_type: implement
---

# Work Package Prompt: WP01 – Channel extraction module

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile specified in the frontmatter, and behave according to its guidance before parsing the rest of this prompt.

- **Profile**: `implementer-ivan`
- **Role**: `implementer`
- **Agent/tool**: `claude`

If no profile is specified, run `spec-kitty agent profile list` and select the best match for this work package's `task_type` and `authoritative_surface`.

---

## Markdown Formatting

Wrap HTML/XML tags in backticks. Use language identifiers in code blocks.

---

## Objectives & Success Criteria

Build the **channel-extraction foundation**: a self-contained module that, given one
parsed event object, returns the text belonging to each channel class — with
code-edit and file-read content excluded. This WP introduces **no classification
behavior change yet**; it provides the two strings WP02/WP03 will consume.

**Done when:**
- A new `internal/analyzer/channels.go` exposes two pure functions:
  - `outputText(obj map[string]any) string` — real command/tool output + structured error text only.
  - `diagnosticText(obj map[string]any) string` — `output` **plus** narrative (assistant/user message text, codex reasoning/message). Invariant: `diagnosticText ⊇ outputText`.
- Code-edit (`newString`/`oldString`/`structuredPatch`) and file-read (`file.content`) content is **excluded** from both (§3a).
- Every harness shape in the §3c matrix routes correctly, proven by golden tests in `channels_test.go`.
- Unmapped shapes default to **excluded from output** and are logged (never silently treated as output).
- `go build ./... && go vet ./... && go test ./internal/analyzer/` is green.

## Context & Constraints

- **Authoritative design**: `docs/design/issue-4-failure-scan-channel-scoping.md` §3a, §3c, §6. Read it before coding — the §3c matrix table is the contract.
- Mission docs: `kitty-specs/failure-scan-channel-scoping-01KW2HBG/{plan.md (IC-01),data-model.md,contracts/channel-classification.md,research.md (Decision 5)}`.
- Existing code to study/reuse (do NOT edit — they belong to other WPs):
  - `internal/analyzer/json_helpers.go` — `flattenJSON` (you may call it; do not modify).
  - `internal/analyzer/fingerprints.go` — `jsonLooksLikeSourceRead`, `genericFailureSignal`/`genericFailureToolText` show the existing narrow `toolUseResult` handling to generalize from (study only; WP02 owns this file).
- Constraints: Go stdlib only (`encoding/json`, `strings`, `regexp` if needed) — **no new deps** (NFR-001). Deterministic output for identical input (FR-006). One extraction pass; no per-rule re-walks (NFR-002).

## Branch Strategy

- **Strategy**: already-confirmed
- **Planning base branch**: fix/failure-scan-channel-scoping
- **Merge target branch**: fix/failure-scan-channel-scoping

> Execution worktrees are allocated per computed lane from `lanes.json`.

## Subtasks & Detailed Guidance

### Subtask T001 – Define channel model + extraction entry points
- **Purpose**: Establish the module's public surface within the package.
- **Steps**: Create `internal/analyzer/channels.go`. Define unexported `outputText(obj map[string]any) string` and `diagnosticText(obj map[string]any) string`. Internally accumulate per-channel fragments (e.g. an `output` builder and a `narrative` builder) and join deterministically. `diagnosticText` returns output ∪ narrative.
- **Files**: `internal/analyzer/channels.go`.
- **Notes**: Keep it a deep module with a narrow interface (DIRECTIVE_001). No exported surface change to the package.

### Subtask T002 – Per-harness extraction matrix (§3c)
- **Purpose**: Map each known harness shape to output/narrative/excluded.
- **Steps**: Implement the §3c table:
  - Claude message `message.content[].text` → narrative.
  - Claude tool result `toolUseResult.{stdout,stderr}` → output.
  - Claude `tool_result` content block → output.
  - Codex `payload.type=function_call_output` → `payload.output` → output; `payload.type ∈ {reasoning,message}` → `payload.content` → narrative (typed path keyed on `payload.type`, not a bare key scan).
  - Structured error: top-level `error`/`exception`/`traceback` (incl. nested objects) → output (string leaf values).
- **Files**: `internal/analyzer/channels.go`.
- **Notes**: Starting output-key set `{stdout,stderr,output,error,exception,traceback}` (corpus-confirmable). Matrix is provisional — see T004 for unmapped handling.

### Subtask T003 – Universal §3a exclusion (code-edit / file-read)
- **Purpose**: Kill issue-#4 repro line 3 (Edit writing `raise AssertionError`).
- **Steps**: Exclude `toolUseResult.{newString,oldString}`, `structuredPatch`, and `toolUseResult.file.content` from **both** builders. This generalizes the intent of today's `jsonLooksLikeSourceRead` (which only short-circuits the Read shape).
- **Files**: `internal/analyzer/channels.go`.
- **Notes**: Exclusion is absolute — excluded content is never part of output or narrative.

### Subtask T004 – Recursive JSON re-decode + unmapped logging
- **Purpose**: Handle `toolUseResult` strings that are themselves JSON; keep the matrix safe-by-default.
- **Steps**: When `toolUseResult` is a bare string that looks like JSON (`{`-prefixed), attempt one `json.Unmarshal` and re-route via the matrix; on failure treat the raw string as output. For any shape not matched by the matrix, **log** (matrix-growth signal) and default to excluded-from-output.
- **Files**: `internal/analyzer/channels.go`.
- **Notes**: Bounded recursion (single re-decode) to avoid pathological nesting.

### Subtask T005 – Golden tests per harness shape (§7.3)
- **Purpose**: Lock routing behavior (specification-by-example, DIRECTIVE_036).
- **Steps**: In `internal/analyzer/channels_test.go`, table-driven golden tests covering: Claude message, `toolUseResult.{stdout,stderr}`, bare-string `toolUseResult`, JSON-string-encoded `toolUseResult`, `tool_result` content block, Edit/Write, Read, codex `function_call_output`, codex `reasoning`. Each asserts the signature string lands in output / narrative / neither as the §3c matrix dictates.
- **Files**: `internal/analyzer/channels_test.go`.

### Subtask T006 – Structural-vs-text ordering fixture (§7.4)
- **Purpose**: Provide the fixture proving a source-read object that *also* carries a structured `error`/`status` key still surfaces its output text.
- **Steps**: Add a golden case: an object with `toolUseResult.file.content` (excluded) **and** a top-level `error` string (output) — assert the `error` value appears in `outputText` while the file content does not.
- **Files**: `internal/analyzer/channels_test.go`.
- **Notes**: This pins the precedence WP03 relies on for ordering.

## Test Strategy

- Authored test-first per DIRECTIVE_034. All tests live in `internal/analyzer/channels_test.go`.
- Run: `go test ./internal/analyzer/ -run TestChannel`.

## Risks & Mitigations

- **Harness-shape coverage gaps** → the unmapped-shape log (T004) makes gaps visible rather than silent FNs.
- **JSON re-decode pathologies** → single bounded re-decode only.
- **Determinism** → join fragments in a fixed order; no map-iteration-order leakage into output.

## Review Guidance

- Verify the §3c matrix is implemented exactly, including the typed codex `payload.type` path (not a key allowlist).
- Confirm §3a exclusion covers all three edit fields + file.content.
- Confirm `diagnosticText ⊇ outputText` holds for every test case.

## Activity Log

- 2026-06-26T18:05:00Z – system – Prompt created.
