# Contract — Codex channel-routing matrix (specification-by-example)

Each row: a codex event (given its correlated command) → where its text lands. `output` = scanned by output rules; `narrative` = scanned by diagnostic rules; `—` = excluded (neither).

| # | function_call command | function_call_output | → output | → narrative | Rule |
|---|----------------------|---------------------|----------|-------------|------|
| 1 | `git diff` (read) | exit 0, diff content | — | — | read exit-0 fully excluded (FR-001/FR-004) |
| 2 | `cat missing` (read) | exit 1, "No such file" | status header only | — | non-zero read keeps header, drops bulk (FR-004) |
| 3 | `go build ./...` (real) | exit 1, build errors | full output | — | non-read scanned as today (FR-005) |
| 4 | `git diff && go build` (compound) | any | full output | — | not all-read → scanned (FR-003) |
| 5 | `rg foo \| head` (read pipeline) | exit 0/1, matches | — | — | all-read pipeline excluded (FR-003) |
| 6 | (no matching call_id) | any | full output | — | unknown intent → scan (FR-005) |
| 7 | tool `name` spells `callId` | read, exit 0 | — | — | callId recognized (FR-007) |

## Payload-type mapping (FR-006)

| payload.type | routing | note |
|--------------|---------|------|
| `function_call` | — (excluded) | registers codexCall metadata; no channel text |
| `task_started` | — (excluded) | marker, no human text |
| `user_message` | narrative | only if the payload carries user prose (verify field) |
| empty | — (excluded) | when schema is truly empty |
| `function_call_output` | per matrix above | changed by this mission |
| `reasoning` / `message` / `agent_message` / `task_complete` / `token_count` | UNCHANGED | already mapped — do not touch |

## Invariants
- Any uncertainty (unknown command, unmatched id, unparseable envelope) → **scan** (recall-safe).
- Read content never reaches `output` OR `narrative` (§3a mirror).
- No change to the emitted report JSON schema.
