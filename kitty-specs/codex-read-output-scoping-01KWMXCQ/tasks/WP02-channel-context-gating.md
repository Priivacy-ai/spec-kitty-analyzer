---
work_package_id: WP02
title: codexCall registry, channelContext & payload gating
dependencies:
- WP01
requirement_refs:
- FR-001
- FR-002
- FR-004
- FR-005
- FR-006
- FR-007
- NFR-002
- NFR-003
tracker_refs: []
planning_base_branch: fix/codex-read-output-scoping
merge_target_branch: fix/codex-read-output-scoping
branch_strategy: Planning artifacts for this mission were generated on fix/codex-read-output-scoping. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into fix/codex-read-output-scoping unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-codex-read-output-scoping-01KWMXCQ
base_commit: 5e6ea6fe9f89cf9f67a3a9f7dd20d84d29c17f88
created_at: '2026-07-03T21:38:22Z'
subtasks:
- T005
- T006
- T007
- T008
- T009
- T010
- T015
- T016
phase: Phase 2 - Channel gating
assignee: ''
agent: claude
history:
- at: '2026-07-03T21:38:22Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/channels
create_intent: []
execution_mode: code_change
model: ''
owned_files:
- internal/analyzer/channels.go
- internal/analyzer/channels_test.go
role: implementer
tags: []
task_type: implement
---

# Work Package Prompt: WP02 – codexCall registry, channelContext & payload gating

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

Make the codex channel-extraction path **context-aware** so read/inspection output is excluded from
scanning, while genuine failures still surface. Concretely, in `internal/analyzer/channels.go`:

1. Define the two unexported types (`data-model.md`): `codexCall{callID, name, cmd string; isRead bool}`
   and `channelContext{codexCalls map[string]codexCall}`.
2. Add a **context-aware** extraction entrypoint that threads a `channelContext` into
   `extractCodexPayload`, while **preserving** the existing stateless `channelTextPair(obj)` (empty
   context) so every current test and the obj-only call site keep working.
3. Handle the `function_call` payload: build a `codexCall` (using WP01's
   `classifyCodexReadCommand`), register it into the context, contribute **no** channel text.
4. Gate `function_call_output`: when its correlated command `isRead`, exclude its content from BOTH
   the output and narrative channels (§3a mirror), **envelope-aware** — exit-0 read fully excluded;
   non-zero read keeps only the status header on `output`; unknown id / non-read / unparseable → scan
   exactly as today (recall-safe).
5. Map the remaining payload types (`task_started`, `user_message`, empty), leaving the already-mapped
   types (`reasoning`/`message`/`agent_message`/`task_complete`/`token_count`) **unchanged**.

**Done when:**
- `channels.go` defines `codexCall` + `channelContext` and a `codexCallID(payload)` helper reading
  `call_id` then `callId` (FR-007).
- The stateless `channelTextPair(obj)` still exists and behaves identically (empty context path).
- `extractCodexPayload` gates `function_call_output` per `contracts/channel-matrix.md` rows 1–7.
- New payload types routed per the FR-006 mapping table; unchanged types untouched (verify by diff).
- `go build ./... && go vet ./... && go test ./internal/analyzer/ && gofmt -l internal/analyzer/channels.go` clean.

## Context & Constraints

- **Authoritative design**: `contracts/channel-matrix.md` (the routing contract — every row),
  `data-model.md` (the two types + the envelope), `research.md` (R1 prepass/back-compat, R2 both-channel
  exclusion, R3 envelope-aware, R5 payload mapping, R6 call_id normalization).
- **Durable design doc**: `~/spec-kitty-analyzer-issue4-backup/catfood-findings/codex-payload-design.md`.
- **The current code you are extending** (already in `channels.go`, study it):
  - `extractChannels(obj)` calls `extractCodexPayload(payload, &ct)` at the `obj["payload"]` branch.
  - `extractCodexPayload` currently switches on `payload["type"]`: `function_call_output` →
    `collectStringLeaves(payload["output"])` → output; `reasoning`/`message` → `payload.content` →
    narrative; `agent_message` → `payload.message` → narrative; `task_complete` →
    `payload.last_agent_message` → narrative; `token_count` → excluded (not logged); `default` → log
    unmapped. **Do not disturb the non-`function_call_output` behavior except to ADD the new types.**
  - `channelTextPair(obj)` → `channelStrings(extractChannels(obj))`. This is the stateless entrypoint
    WP03 will complement with a context-aware sibling.
- **From WP01** (call, do not reimplement): `classifyCodexReadCommand(name, cmd)`,
  `parseCodexOutputEnvelope(output)`, `readCommandSet`.
- **Constraints**: C-001 (context value carries the registry; stateless entrypoint preserved for
  back-compat), C-002 (read content reaches NEITHER channel — diagnosticCh IS scanned), NFR-004 (no
  report-JSON-schema change — `codexCall`/`channelContext` unexported, never serialized), NFR-003
  (deterministic).

## Branch Strategy

- **Strategy**: already-confirmed
- **Planning base branch**: fix/codex-read-output-scoping
- **Merge target branch**: fix/codex-read-output-scoping

> Execution worktrees are allocated per computed lane from `lanes.json`.

## Subtasks & Detailed Guidance

### Subtask T005 – Types + `codexCallID` normalization helper
- **Purpose**: Establish the registry data shape and id normalization (FR-007).
- **Steps**:
  - Define unexported `codexCall struct { callID, name, cmd string; isRead bool }`.
  - Define unexported `channelContext struct { codexCalls map[string]codexCall }`. Add a helper
    `(ctx channelContext) lookup(callID string) (codexCall, bool)` that is nil-map safe (an empty/zero
    context returns `false`).
  - Add `func codexCallID(payload map[string]any) string` returning `payload["call_id"]` if a
    non-empty string, else `payload["callId"]`, else `""`.
- **Files**: `internal/analyzer/channels.go`.
- **Notes**: The zero `channelContext{}` (nil map) MUST behave exactly like "no registry" — this is
  what preserves the stateless path.

### Subtask T006 – Context-aware entrypoint; preserve stateless `channelTextPair`
- **Purpose**: Thread a `channelContext` into codex extraction without breaking the obj-only API (C-001).
- **Steps**:
  - Add `func channelTextPairCtx(obj map[string]any, ctx channelContext) (outCh, diagCh string)` that
    runs the same extraction as `channelTextPair` but passes `ctx` down to the codex path.
  - Refactor `extractChannels(obj)` → `extractChannelsCtx(obj, ctx)` (or thread `ctx` as a parameter),
    and have `extractCodexPayload` receive `ctx`. Keep `extractChannels(obj)` as a thin wrapper
    calling `extractChannelsCtx(obj, channelContext{})` so existing callers/tests are untouched.
  - Keep `channelTextPair(obj)` as `channelTextPairCtx(obj, channelContext{})` — byte-identical output
    for the empty context.
- **Files**: `internal/analyzer/channels.go`.
- **Notes**: Minimize surface churn — thread one parameter; do not restructure the Claude/message/error
  branches. `outputText`/`diagnosticText` helpers keep calling the stateless pair.

### Subtask T007 – `function_call` case: build + register `codexCall`
- **Purpose**: Turn a `function_call` payload into registry metadata; contribute no channel text (FR-002).
- **Steps**:
  - Add a `case "function_call":` to `extractCodexPayload`. Extract `name` (`payload["name"]`), the
    command string from `payload["arguments"]` → `.cmd` (arguments is a JSON-in-string; decode it with
    the existing `decodeJSONObject` then read `cmd`, tolerating absence), and `callID` via `codexCallID`.
  - Compute `isRead := classifyCodexReadCommand(name, cmd)` and register
    `ctx.codexCalls[callID] = codexCall{callID, name, cmd, isRead}` — **but only inside the prepass**
    (WP03 owns the write). In the EXTRACTION pass, `function_call` contributes no text and is simply
    excluded. To keep registration in one place, expose a helper `newCodexCall(payload) codexCall`
    (pure, in channels.go) that WP03's prepass calls; the extraction-pass `case "function_call":`
    just returns (excluded, no logging — it is a mapped type).
- **Files**: `internal/analyzer/channels.go`.
- **Notes**: Registration WRITE happens in the WP03 prepass; WP02 provides `newCodexCall` and the
  excluded extraction case. This keeps the single-write-site invariant (registry built once, in the
  prepass) that gives out-of-order tolerance (R1).

### Subtask T008 – `function_call_output` gating (the core change)
- **Purpose**: Exclude read content, envelope-aware, recall-safe (FR-001, FR-004, FR-005).
- **Steps** — replace the current `case "function_call_output":` body with:
  1. `callID := codexCallID(payload)`; `call, ok := ctx.lookup(callID)`.
  2. **Not found, or found but `!call.isRead`** → scan exactly as today: `collectStringLeaves(payload["output"])`
     → output (preserve the existing zero-fragment schema-drift logging). (Rows 3, 6.)
  3. **Found and `isRead`**:
     - Read the raw output string (`payload["output"]` as string; if it is not a plain string, fall
       back to scanning — unknown envelope).
     - `header, _, exitCode, parsed := parseCodexOutputEnvelope(raw)`.
     - `parsed && exitCode == 0` → **exclude entirely** (contribute nothing to output OR narrative). (Row 1, 5, 7.)
     - `parsed && exitCode != 0` → append only `header` to **output** (drop the bulk). (Row 2.)
     - `!parsed` (unparseable envelope) → scan the raw output as today (recall-safe). (Invariant.)
- **Files**: `internal/analyzer/channels.go`.
- **Notes**: The read-excluded branch must add to NEITHER `ct.output` NOR `ct.narrative` (§3a mirror,
  C-002). Do **not** emit the schema-drift log for a deliberately-excluded read (it is mapped/handled).

### Subtask T009 – New payload types; leave mapped types unchanged
- **Purpose**: Finish the payload-type mapping (FR-006) without regressing existing mappings (R5).
- **Steps** — add cases to `extractCodexPayload`:
  - `case "task_started":` → excluded marker, no text, no log (mapped).
  - `case "user_message":` → if the payload carries user prose, route it to **narrative**. Read the
    prose field defensively (likely `payload["message"]` as string, or `payload["content"]`); **verify
    the exact field against the corpus in WP04** and leave a `// TODO(WP04): confirm field` only if
    unresolved. If no prose field is present, exclude (no log — mapped).
  - empty `payload.type` (`ptype == ""`): excluded (no text). Only treat as mapped-empty when the type
    key is genuinely absent/empty; keep the `default:` log for *unknown non-empty* types.
  - **Do NOT touch** `reasoning`/`message`/`agent_message`/`task_complete`/`token_count` — verify with
    `git diff` that those case bodies are byte-identical.
- **Files**: `internal/analyzer/channels.go`.
- **Notes**: R5 warning — the field-seen "unmapped reasoning" log is a known-type-absent-field path, not
  an unmapped type. Re-mapping `reasoning` would regress; leave it.

### Subtask T010 – §3a mirror + determinism guard
- **Purpose**: Guarantee excluded read content reaches neither channel, deterministically (C-002, NFR-003).
- **Steps**:
  - Audit the new code paths: the read-excluded branch appends to neither builder; the non-zero branch
    appends only `header` to `output`.
  - Ensure no map-iteration-order leaks into output (the registry is keyed by callID but only *looked
    up* here — order-independent). Confirm identical input → identical strings.
- **Files**: `internal/analyzer/channels.go`.
- **Notes**: This is a review/audit subtask plus any small guard needed; it produces the invariant WP04
  asserts.

### Subtask T015 – Golden channel-matrix cases, rows 1–7 (TEST-FIRST)
- **Purpose**: Lock every routing decision (specification-by-example; DIRECTIVE_034/036/039).
- **Steps**: In `internal/analyzer/channels_test.go`, add table-driven cases exercising
  `channelTextPairCtx` with a hand-built `channelContext` (build a `codexCall` with the right `isRead`,
  as the prepass would). One case per `contracts/channel-matrix.md` row:
  1. `git diff` (read) + exit-0 diff containing "error"/"exit code 2" → `output` empty, `narrative` empty.
  2. `cat missing` (read) + exit-1 "No such file" → `output` contains the status header, NOT the bulk.
  3. `go build ./...` (real) + exit-1 build errors → `output` contains the full output.
  4. `git diff && go build` (compound) → `output` full (not all-read → scanned).
  5. `rg foo | head` (read pipeline) + any → `output` empty, `narrative` empty.
  6. output whose call_id is absent from the registry → `output` full (unknown → scan).
  7. `callId` camelCase read, exit 0 → excluded.
- **Files**: `internal/analyzer/channels_test.go`.
- **Notes**: Author these **before** T007/T008 (red → green). Each case asserts BOTH the presence side
  (real → scanned) and the ABSENCE side (read → empty). Also keep `diagnosticText ⊇ outputText`.

### Subtask T016 – Payload-type mapping cases (TEST-FIRST)
- **Purpose**: Pin FR-006 mapping and guard R5 (no regression of already-mapped types).
- **Steps**: Cases asserting: `function_call` contributes no channel text; `task_started` excluded;
  `user_message` with prose → `narrative`; empty `payload.type` excluded; and that
  `reasoning`/`message`/`agent_message`/`task_complete`/`token_count` still route exactly as before
  (copy representative existing assertions).
- **Files**: `internal/analyzer/channels_test.go`.
- **Notes (U1)**: the `user_message` prose field is **unconfirmed**. Before asserting, confirm the
  actual key from a real codex sample (coordinate with WP04's corpus curation). If no `user_message`
  event exists in the corpus, make T009 route it to **excluded** (recall-safe) and assert that instead,
  documenting the deferral — do not ship a guessed narrative field that no test exercises.

## Test Strategy

- **Test-first** (DIRECTIVE_034/039): author T015/T016 in `channels_test.go` red, then implement
  T005–T010 to green. Golden cases mirror `contracts/channel-matrix.md` exactly (specification-by-example).
- Run: `go test ./internal/analyzer/ -run 'Codex|Channel|ReadOutput' -v` then `go test ./internal/analyzer/`.

## Risks & Mitigations

- **Breaking the stateless path** → keep `channelTextPair(obj)` = `channelTextPairCtx(obj, channelContext{})`;
  run the existing `go test ./internal/analyzer/` before finishing.
- **Regressing mapped types** → diff the `reasoning`/`message`/`agent_message`/`task_complete`/`token_count`
  cases; they must be unchanged.
- **Non-string `output` / odd envelope** → fall back to scanning (recall-safe), never exclude on doubt.

## Review Guidance

- Verify rows 1–7 of `contracts/channel-matrix.md` are each realized by a code path.
- Verify read exclusion is **both-channel** (not diagnostic-only).
- Verify `codexCallID` reads `call_id` then `callId`.
- Verify no report-JSON field added/renamed (grep the report structs — nothing new serialized).

## Activity Log

- 2026-07-03T21:38:22Z – system – Prompt created.
