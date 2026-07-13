# Data Model — Codex Read-Output Scoping

## Entity: `codexCall` (unexported; in-memory only)

The correlated metadata for one codex command, built from a `function_call` payload and consulted when its paired `function_call_output` is classified.

| Field | Type | Source | Meaning |
|-------|------|--------|---------|
| callID | string | `payload.call_id` or `payload.callId` | correlation key shared with the output |
| name | string | `payload.name` | tool/function name (`exec_command`, or a read-file tool) |
| cmd | string | `payload.arguments` → `.cmd` (JSON-in-string) | the shell command, when `name == exec_command` |
| isRead | bool | classifier over `name`/`cmd` | true = pure read/inspection (gate its output) |

- **Invariant**: `isRead` is computed once, at registration (prepass), via the conservative classifier (R4). Not recomputed per output.

## Entity: `channelContext` (unexported)

Per-source-file carrier that makes the codex call registry available to channel extraction without exposing transcript structure to the failure rules.

| Field | Type | Meaning |
|-------|------|---------|
| codexCalls | map[string]codexCall | registry: callID → command metadata |

- The **empty** `channelContext` (nil/empty map) reproduces today's stateless behavior — used by the preserved `channelTextPair(obj)` entrypoint and existing tests.

## Envelope (parsed, not stored)

The codex `function_call_output.output` string has the shape:
```
Chunk ID: …
Wall time: … seconds
Process exited with code <N>
Original token count: …
Output:
<bulk content>
```
- **exitCode** ← the integer after `Process exited with code`; absence/parse-failure → unknown (scan).
- **header** ← the lines up to and including the status line; **bulk** ← everything after `Output:`.

## Routing decision (no schema change)

The outcome is purely which text lands in `ct.output` / `ct.narrative` (the existing channel strings). No new report-JSON fields (NFR-004). `codexCall`/`channelContext` are unexported and never serialized.
