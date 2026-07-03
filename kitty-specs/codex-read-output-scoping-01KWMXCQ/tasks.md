# Tasks: Codex Read-Output Scoping

**Mission**: codex-read-output-scoping-01KWMXCQ
**Branch**: `fix/codex-read-output-scoping`
**Input**: [spec.md](./spec.md), [plan.md](./plan.md), [data-model.md](./data-model.md), [research.md](./research.md), [contracts/channel-matrix.md](./contracts/channel-matrix.md), [quickstart.md](./quickstart.md)

Exclude codex read/inspection `function_call_output` from failure scanning by correlating each
output to its originating `function_call` (shared correlation id; command in `arguments.cmd`) via a
per-file prepass registry carried into a context-aware channel extractor — envelope-aware so exit-0
reads drop fully while non-zero reads keep only the status header (recall). Also finish mapping the
remaining codex payload types. Recall-safe on every uncertainty (unknown command / unmatched id /
unparseable envelope → scan).

## Work Package Overview

| WP | Title | Owns | Subtasks | Deps | Est. lines |
|----|-------|------|----------|------|-----------|
| WP01 | Read classifier, allowlist & output-envelope parser | `internal/analyzer/patterns.go` | 4 | — | ~300 |
| WP02 | codexCall registry, channelContext & payload gating | `internal/analyzer/channels.go` | 6 | WP01 | ~450 |
| WP03 | Per-file prepass & context threading | `internal/analyzer/analyzer.go` | 4 | WP01, WP02 | ~300 |
| WP04 | Golden channel-matrix + frozen-corpus validation | `internal/analyzer/*_test.go`, testdata | 5 | WP02, WP03 | ~380 |

**Sequencing**: WP01 (pure foundation) → WP02 (channel-layer gating) → WP03 (prepass wiring) → WP04 (validation).
WP01 is the only fully independent package. WP02 depends on WP01's classifier + envelope parser. WP03
depends on WP01+WP02 (threads the `channelContext` WP02 defines, calling the classifier WP01 provides).
WP04 depends on WP02+WP03 (validates the end-to-end routing and proves NFR-001/002).

**MVP scope**: There is no partial-value MVP — the FP fix is only observable once WP01→WP03 are all
merged (the prepass must feed the gating). WP04 is the acceptance proof. Treat WP01–WP04 as one
sequential delivery; each WP is independently *reviewable* but the behavior change lands with WP03.

## Ownership map (no overlap)

- WP01 → `internal/analyzer/patterns.go`
- WP02 → `internal/analyzer/channels.go`
- WP03 → `internal/analyzer/analyzer.go`
- WP04 → `internal/analyzer/channels_test.go`, `internal/analyzer/analyzer_test.go`, `internal/analyzer/testdata/codex/**`

> **Structure note (DIRECTIVE_003)**: plan IC-01 sketched the read classifier "in channels.go". Tasks
> relocates the pure, dependency-free helpers (read-command classifier, allowlist, output-envelope
> parser) into `patterns.go` — the module that already houses text-parsing/detection helpers
> (`parseCLIInvocation`, `shellishFields`, `trimShell`). This gives a clean, non-overlapping WP01
> foundation and keeps `channels.go` focused on channel routing. The plan's IC map is explicitly
> "concerns, not work packages"; this is the intended decomposition call, not drift.

## Subtask Index

| ID | Description | WP | Parallel |
|----|-------------|----|----------|
| T001 | Define `readCommandSet` allowlist + mutating-git denylist | WP01 | |
| T002 | `classifyCodexReadCommand(name, cmd)` conservative compound classifier | WP01 | |
| T003 | `parseCodexOutputEnvelope(output)` → header/bulk/exitCode/ok | WP01 | [P] |
| T004 | Package doc + recall-safe invariants (uncertainty → not-read / not-ok) | WP01 | |
| T005 | Define `codexCall` + `channelContext` types; `codexCallID` (call_id/callId) helper | WP02 | |
| T006 | Context-aware entrypoint `channelTextPairCtx(obj, ctx)`; preserve stateless `channelTextPair(obj)` | WP02 | |
| T007 | `function_call` case: build `codexCall`, register into ctx (excluded from channels) | WP02 | |
| T008 | `function_call_output` gating: read+exit0 → exclude both; read+non-zero → header only; else scan | WP02 | |
| T009 | New payload types: `task_started`, `user_message`, empty; leave already-mapped types unchanged | WP02 | |
| T010 | §3a mirror + determinism guard (read content reaches neither channel) | WP02 | |
| T011 | Per-file prepass `buildCodexContext(data)` → registry (out-of-order tolerant) | WP03 | |
| T012 | Thread `channelContext` through parseFile → eventFromJSONObject → eventFromText → channelStringsForEvent | WP03 | |
| T013 | Call context-aware pair when ctx present; keep obj==nil / stateless paths unchanged | WP03 | |
| T014 | Build prepass once per file in parseFile; empty ctx for non-codex files | WP03 | |
| T015 | Golden matrix cases mirroring channel-matrix rows 1–7 | WP04 | |
| T016 | Payload-type mapping golden cases (new + unchanged types) | WP04 | [P] |
| T017 | Assert ABSENCE of read-content FPs + presence of real failure (recall) | WP04 | |
| T018 | Frozen-corpus assembly + base/candidate before-after diff runbook | WP04 | |
| T019 | `go test ./...` green; confirm report JSON schema unchanged (NFR-004) | WP04 | |

---

## WP01 — Read classifier, allowlist & output-envelope parser

**Goal**: Provide the pure, dependency-free primitives the gating layer needs — a conservative
read-command classifier over an allowlist, and a codex output-envelope parser — with recall-safe
defaults (any uncertainty → not-read / not-parseable). No behavior change on its own.
**Priority**: P1 (foundation). **Independent test**: pure functions, unit-testable in isolation (WP04
adds the golden cases; WP01 may include focused unit tests in `patterns.go`'s companion — but all
committed tests are owned by WP04 to keep ownership clean).
**Prompt**: [tasks/WP01-read-classifier-envelope.md](./tasks/WP01-read-classifier-envelope.md)

- [ ] T001 Define `readCommandSet` allowlist + mutating-git denylist (WP01)
- [ ] T002 `classifyCodexReadCommand(name, cmd)` conservative compound classifier (WP01)
- [ ] T003 `parseCodexOutputEnvelope(output)` → header/bulk/exitCode/ok (WP01)
- [ ] T004 Package doc + recall-safe invariants (WP01)

**Dependencies**: none. **Risks**: over-broad allowlist suppresses real failures — keep minimal;
`sed`/`awk` excluded (can mutate).

## WP02 — codexCall registry, channelContext & payload gating

**Goal**: Define the `codexCall` + `channelContext` types, thread a context through the codex
extraction path, and gate `function_call_output` using WP01's classifier + envelope parser — read
content excluded from BOTH channels (§3a mirror), envelope-aware. Map the remaining payload types.
**Priority**: P1. **Independent test**: golden channel-matrix cases exercise `extractCodexPayload`
with a hand-built `channelContext` (WP04).
**Prompt**: [tasks/WP02-channel-context-gating.md](./tasks/WP02-channel-context-gating.md)

- [ ] T005 Define `codexCall` + `channelContext` types; `codexCallID` helper (WP02)
- [ ] T006 Context-aware entrypoint; preserve stateless `channelTextPair(obj)` (WP02)
- [ ] T007 `function_call` case: build + register `codexCall` (excluded) (WP02)
- [ ] T008 `function_call_output` gating (exit0 exclude / non-zero header / else scan) (WP02)
- [ ] T009 New payload types `task_started`/`user_message`/empty; unchanged types untouched (WP02)
- [ ] T010 §3a mirror + determinism guard (WP02)

**Dependencies**: WP01. **Risks**: must preserve the stateless `channelTextPair(obj)` (empty ctx) —
existing tests depend on it; envelope parser must be tolerant (unknown → scan); do not regress
`reasoning`/`message`/`agent_message`/`task_complete`/`token_count`.

## WP03 — Per-file prepass & context threading

**Goal**: Build the per-file `function_call` → `codexCall` registry in a prepass before channel
extraction, and thread the resulting `channelContext` through the file walk so
`channelStringsForEvent` consults it. Out-of-order tolerant; empty ctx reproduces today's behavior.
**Priority**: P1 (lands the behavior change). **Independent test**: an analyzer-level test that a
paired `function_call`(read) + `function_call_output`(exit 0) in one file yields no output-channel text.
**Prompt**: [tasks/WP03-prepass-threading.md](./tasks/WP03-prepass-threading.md)

- [ ] T011 Per-file prepass `buildCodexContext(data)` → registry (WP03)
- [ ] T012 Thread `channelContext` through parseFile → eventFromText → channelStringsForEvent (WP03)
- [ ] T013 Call context-aware pair when ctx present; keep stateless paths unchanged (WP03)
- [ ] T014 Build prepass once per file; empty ctx for non-codex files (WP03)

**Dependencies**: WP01, WP02. **Risks**: the prepass must key off the SAME per-file scope as
extraction; the prepass (not inline threaded state) is what gives out-of-order tolerance (Codex rec 1);
do not change the obj==nil / source-kind path.

## WP04 — Golden channel-matrix + frozen-corpus validation

**Goal**: Prove every routing decision with golden cases mirroring `contracts/channel-matrix.md`, and
validate FP-down / TP-preserved on a frozen representative codex corpus (base vs candidate, separate
caches). Confirm `go test ./...` green and the report JSON schema unchanged.
**Priority**: P1 (acceptance proof). **Independent test**: this WP *is* the test surface.
**Prompt**: [tasks/WP04-golden-matrix-corpus.md](./tasks/WP04-golden-matrix-corpus.md)

- [ ] T015 Golden matrix cases (rows 1–7) (WP04)
- [ ] T016 Payload-type mapping golden cases (WP04)
- [ ] T017 Assert ABSENCE of read-content FPs + presence of real failure (WP04)
- [ ] T018 Frozen-corpus assembly + before/after diff runbook (WP04)
- [ ] T019 `go test ./...` green; report JSON schema unchanged (WP04)

**Dependencies**: WP02, WP03. **Risks**: corpus must be representative but frozen (live `~/.codex` ≈
298 MB, impractical); tests must assert the ABSENCE of read-content findings, not only the presence of
real ones; run base+candidate back-to-back in one job (live-session-in-corpus confound).

---

## Requirement Coverage

| Requirement | WP(s) |
|-------------|-------|
| FR-001 (exclude read output from both channels) | WP02 |
| FR-002 (correlate via preceding function_call, per-file registry) | WP02, WP03 |
| FR-003 (all-segment compound read classifier) | WP01 |
| FR-004 (envelope-aware exit0/non-zero handling) | WP01, WP02 |
| FR-005 (recall-safe default: non-read / unknown id → scan) | WP02 |
| FR-006 (remaining payload-type mapping) | WP02 |
| FR-007 (call_id + callId spellings) | WP02 |
| NFR-001 (no recall loss) | WP04 |
| NFR-002 (FP substantially reduced) | WP04 |
| NFR-003 (deterministic) | WP04 |
| NFR-004 (suite green, schema unchanged) | WP04 |

## Next Command

`/spec-kitty.analyze` (optional consistency gate) or `/spec-kitty.implement` to begin WP01.
