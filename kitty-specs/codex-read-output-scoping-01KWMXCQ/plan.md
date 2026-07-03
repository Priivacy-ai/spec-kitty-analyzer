# Implementation Plan: Codex Read-Output Scoping

**Branch**: `fix/codex-read-output-scoping` | **Date**: 2026-07-03 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `kitty-specs/codex-read-output-scoping-01KWMXCQ/spec.md`

## Summary

Correlate each codex `function_call_output` to its originating `function_call` (shared correlation id; command in `arguments.cmd`) via a per-file prepass registry carried into a context-aware channel extractor. When the correlated command is a read/inspection, exclude its output from BOTH the output and diagnostic channels (mirroring the Claude §3a exclusion), envelope-aware so exit-0 reads drop fully while non-zero reads keep only the status header (recall). Map the remaining unmapped codex payload types. Design is Codex-design-reviewed (7 recommendations adopted).

## Technical Context

**Language/Version**: Go 1.25.0 (module `github.com/priivacy-ai/spec-kitty-analyzer`)
**Primary Dependencies**: Standard library only (`encoding/json`, `strings`, `regexp`). No new dependency — explicitly **no shell parser** (C-003).
**Storage**: N/A (in-memory per-file processing)
**Testing**: `go test ./...`; golden **channel-matrix** cases (channels_test.go) for each routing decision; frozen-corpus before/after diff for FP/TP (NFR-001/002).
**Target Platform**: single Go CLI + internal packages
**Project Type**: single
**Performance Goals**: N/A (per-file prepass is O(lines); no runtime hot path)
**Constraints**: **no report-JSON-schema change** (NFR-004); read exclusion mirrors Claude §3a — content reaches NEITHER channel (C-002); deterministic (NFR-003); recall-safe defaults on any uncertainty (FR-005); scoped to Codex, `#22` out of scope (C-004).
**Scale/Scope**: ~3 source files (`internal/analyzer/channels.go`, `internal/analyzer/analyzer.go`, `internal/analyzer/patterns.go`) + tests + a frozen validation corpus.

## Charter Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Charter present; paradigms **deep-module-design** + **specification-by-example**.
- **DIRECTIVE_001 / deep-module-design** — ✅ the `channelContext` + per-file registry keeps codex-transcript correlation encapsulated in the channel layer; the failure rules stay channel-agnostic (they consume precomputed strings). Codex review rec 5.
- **DIRECTIVE_024 (Locality of Change)** — ✅ change is confined to channel extraction + the file walk that already computes channels; no failure-rule changes.
- **DIRECTIVE_003 (Decision Documentation)** — ✅ the design + the Codex review's 7 recommendations are recorded in `research.md`.
- **DIRECTIVE_025 (Boy Scout Rule)** — touched-file fixes folded in-scope; none identified beyond the change.
- **specification-by-example** — ✅ `contracts/channel-matrix.md` expresses every routing decision as a concrete input→channel example; golden tests mirror it.

No violations → Complexity Tracking omitted.

## Project Structure

### Documentation (this mission)

```
kitty-specs/codex-read-output-scoping-01KWMXCQ/
├── plan.md              # This file
├── research.md          # Phase 0 — design decisions + Codex-review outcomes
├── data-model.md        # Phase 1 — codexCall record + channelContext
├── quickstart.md        # Phase 1 — validation (frozen corpus + golden matrix)
├── contracts/
│   └── channel-matrix.md  # Phase 1 — input shape → channel routing (before/after)
└── tasks.md             # Phase 2 (/spec-kitty.tasks — NOT created here)
```

### Source Code (repository root)

```
internal/analyzer/
├── patterns.go     # read-command allowlist (readCommandSet)
├── channels.go     # extractCodexPayload: function_call (register) + function_call_output gating; channelContext; read-classifier; envelope parse; payload-type mapping
├── analyzer.go     # per-file prepass building the call registry, wired into the file walk that computes channelStringsForEvent
└── *_test.go       # golden channel-matrix cases
```

**Structure Decision**: No new packages/files required; the change lives in the existing `internal/analyzer` channel-extraction surface. A `channelContext` value (holding the per-file registry) is the one new type; the stateless `channelTextPair(obj)` is preserved (empty context) for back-compat and existing tests.

## Complexity Tracking

*No Charter Check violations — section intentionally empty.*

## Implementation Concern Map

> Concerns, not work packages. `/spec-kitty.tasks` decomposes these into WPs.

### IC-01 — Read-command classifier + allowlist

- **Purpose**: Decide whether a codex command string is a pure read/inspection. Conservative segment classifier (no shell parser): split on `&&`/`||`/`;`/`|`; every segment's leading token must be in the read allowlist, with no write redirection (`>`,`>>`) and no mutating `git` subcommand; any uncertainty → not-read.
- **Relevant requirements**: FR-003; C-003
- **Affected surfaces**: `internal/analyzer/patterns.go` (allowlist), a pure classifier func in `channels.go`
- **Sequencing/depends-on**: none (pure function, foundational)
- **Risks**: over-broad allowlist suppresses real failures (recall) — keep it minimal; `sed`/`awk` excluded (can mutate).

### IC-02 — Codex call registry + per-file prepass

- **Purpose**: Build a `map[callID] → codexCall{name, cmd, isRead}` from `function_call` payloads before channel extraction; normalize `call_id`/`callId`.
- **Relevant requirements**: FR-002, FR-007
- **Affected surfaces**: `internal/analyzer/analyzer.go` (prepass in the per-file walk), a `codexCall`/registry type
- **Sequencing/depends-on**: IC-01 (isRead computed via the classifier at registration time)
- **Risks**: the prepass must key off the SAME per-file scope as extraction; the prepass (not inline) is what gives out-of-order tolerance (Codex rec 1).

### IC-03 — Context-aware gating of function_call_output

- **Purpose**: Thread the registry via a `channelContext` into a context-aware `extractCodexPayload`; add the `function_call` case (excluded + already registered); gate `function_call_output` — read command → exclude output from BOTH channels; envelope-aware (parse `Process exited with code N` / `Output:`): exit-0 read fully excluded, non-zero read keeps only the status header on output; non-read / unknown id / unparseable envelope → scan as today.
- **Relevant requirements**: FR-001, FR-004, FR-005; C-001, C-002
- **Affected surfaces**: `internal/analyzer/channels.go`, `internal/analyzer/analyzer.go` (pass context)
- **Sequencing/depends-on**: IC-01, IC-02
- **Risks**: must preserve stateless `channelTextPair(obj)` (empty context) — existing tests depend on it; the envelope parser must be tolerant (unknown → scan).

### IC-04 — Remaining payload-type mapping

- **Purpose**: Map the genuinely-unmapped types: `function_call` (handled by IC-03), `task_started` (excluded marker), `user_message` (narrative if it carries user prose — verify field in corpus), empty `payload.type` (excluded). Leave already-mapped `reasoning`/`message`/`agent_message`/`task_complete`/`token_count` untouched.
- **Relevant requirements**: FR-006
- **Affected surfaces**: `internal/analyzer/channels.go` (extractCodexPayload cases)
- **Sequencing/depends-on**: none (independent switch cases)
- **Risks**: don't regress existing mappings (Codex rec 6); `user_message` routing needs corpus confirmation of the prose field.

### IC-05 — Golden matrix + frozen-corpus validation

- **Purpose**: Golden channel-matrix cases for every routing decision (read exit-0 → excluded; read non-zero → header only; real command → scanned; compound → scanned; the new payload types; call_id + callId). Define a **frozen representative codex corpus**; measure before/after FP (target rules) + TP (zero loss).
- **Relevant requirements**: NFR-001, NFR-002, NFR-003; C-005
- **Affected surfaces**: `internal/analyzer/channels_test.go` (+ corpus fixtures), a frozen corpus location
- **Sequencing/depends-on**: IC-03, IC-04
- **Risks**: corpus must be representative but frozen (live full `~/.codex` = 298 MB, impractical); tests assert the ABSENCE of read-content findings, not just presence of real ones.
