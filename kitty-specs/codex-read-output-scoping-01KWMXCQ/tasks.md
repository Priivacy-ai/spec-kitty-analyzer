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
| WP01 | Read classifier, allowlist & output-envelope parser | `patterns.go` + `patterns_test.go` | 5 | — | ~340 |
| WP02 | codexCall registry, channelContext & payload gating | `channels.go` + `channels_test.go` | 8 | WP01 | ~520 |
| WP03 | Per-file prepass & context threading | `analyzer.go` + `analyzer_test.go` | 5 | WP01, WP02 | ~340 |
| WP04 | Frozen-corpus validation + fixture test + gate | `corpus_codex_test.go`, testdata | 2 | WP02, WP03 | ~220 |

> **Test co-location (A1 remediation, analyze gate 2026-07-03):** each code WP owns its own test file
> and authors tests **test-first** (DIRECTIVE_034/039) — golden matrix in WP02's `channels_test.go`,
> the analyzer integration case in WP03's `analyzer_test.go`, classifier/envelope units in WP01's
> `patterns_test.go`. WP04 is re-scoped to the frozen-corpus evidence diff + a committed black-box
> fixture test + the suite/schema gate. Distinct filenames keep `owned_files` non-overlapping.

**Sequencing**: WP01 (pure foundation) → WP02 (channel-layer gating) → WP03 (prepass wiring) → WP04 (validation).
WP01 is the only fully independent package. WP02 depends on WP01's classifier + envelope parser. WP03
depends on WP01+WP02 (threads the `channelContext` WP02 defines, calling the classifier WP01 provides).
WP04 depends on WP02+WP03 (validates the end-to-end routing and proves NFR-001/002).

**MVP scope**: There is no partial-value MVP — the FP fix is only observable once WP01→WP03 are all
merged (the prepass must feed the gating). WP04 is the acceptance proof. Treat WP01–WP04 as one
sequential delivery; each WP is independently *reviewable* but the behavior change lands with WP03.

## Ownership map (no overlap)

- WP01 → `internal/analyzer/patterns.go`, `internal/analyzer/patterns_test.go`
- WP02 → `internal/analyzer/channels.go`, `internal/analyzer/channels_test.go`
- WP03 → `internal/analyzer/analyzer.go`, `internal/analyzer/analyzer_test.go`
- WP04 → `internal/analyzer/corpus_codex_test.go`, `internal/analyzer/testdata/codex/**`

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
| T020 | Unit tests (TEST-FIRST) for classifier + envelope in `patterns_test.go` | WP01 | |
| T005 | Define `codexCall` + `channelContext` types; `codexCallID` (call_id/callId) helper | WP02 | |
| T006 | Context-aware entrypoint `channelTextPairCtx(obj, ctx)`; preserve stateless `channelTextPair(obj)` | WP02 | |
| T007 | `function_call` case: build `codexCall`, register into ctx (excluded from channels) | WP02 | |
| T008 | `function_call_output` gating: read+exit0 → exclude both; read+non-zero → header only; else scan | WP02 | |
| T009 | New payload types: `task_started`, `user_message`, empty; leave already-mapped types unchanged | WP02 | |
| T010 | §3a mirror + determinism guard (read content reaches neither channel) | WP02 | |
| T015 | Golden matrix cases (TEST-FIRST) mirroring channel-matrix rows 1–7 | WP02 | |
| T016 | Payload-type mapping golden cases (TEST-FIRST; new + unchanged types) | WP02 | [P] |
| T011 | Per-file prepass `buildCodexContext(data)` → registry (out-of-order tolerant) | WP03 | |
| T012 | Thread `channelContext` through parseFile → eventFromJSONObject → eventFromText → channelStringsForEvent | WP03 | |
| T013 | Call context-aware pair when ctx present; keep obj==nil / stateless paths unchanged | WP03 | |
| T014 | Build prepass once per file in parseFile; empty ctx for non-codex files | WP03 | |
| T017 | Analyzer integration test (TEST-FIRST): ABSENCE of read-content FPs + recall | WP03 | |
| T018 | Frozen-corpus diff + committed black-box fixture test | WP04 | |
| T019 | `go test ./...` green; report schema unchanged (NFR-004); behavior doc (C1) | WP04 | |

---

## WP01 — Read classifier, allowlist & output-envelope parser

**Goal**: Provide the pure, dependency-free primitives the gating layer needs — a conservative
read-command classifier over an allowlist, and a codex output-envelope parser — with recall-safe
defaults (any uncertainty → not-read / not-parseable). No behavior change on its own.
**Priority**: P1 (foundation). **Independent test**: pure functions, unit-tested test-first in
`patterns_test.go` (T020).
**Prompt**: [tasks/WP01-read-classifier-envelope.md](./tasks/WP01-read-classifier-envelope.md)

- [x] T001 Define `readCommandSet` allowlist + mutating-git denylist (WP01)
- [x] T002 `classifyCodexReadCommand(name, cmd)` conservative compound classifier (WP01)
- [x] T003 `parseCodexOutputEnvelope(output)` → header/bulk/exitCode/ok (WP01)
- [x] T004 Package doc + recall-safe invariants (WP01)
- [x] T020 Unit tests (TEST-FIRST) for classifier + envelope in `patterns_test.go` (WP01)

**Dependencies**: none. **Risks**: over-broad allowlist suppresses real failures — keep minimal;
`sed`/`awk` excluded (can mutate).

## WP02 — codexCall registry, channelContext & payload gating

**Goal**: Define the `codexCall` + `channelContext` types, thread a context through the codex
extraction path, and gate `function_call_output` using WP01's classifier + envelope parser — read
content excluded from BOTH channels (§3a mirror), envelope-aware. Map the remaining payload types.
**Priority**: P1. **Independent test**: golden channel-matrix cases (T015/T016), authored test-first,
exercise `channelTextPairCtx` with a hand-built `channelContext` in `channels_test.go`.
**Prompt**: [tasks/WP02-channel-context-gating.md](./tasks/WP02-channel-context-gating.md)

- [x] T015 Golden channel-matrix cases, rows 1–7 (TEST-FIRST) (WP02)
- [x] T016 Payload-type mapping golden cases (TEST-FIRST) (WP02)
- [x] T005 Define `codexCall` + `channelContext` types; `codexCallID` helper (WP02)
- [x] T006 Context-aware entrypoint; preserve stateless `channelTextPair(obj)` (WP02)
- [x] T007 `function_call` case: build + register `codexCall` (excluded) (WP02)
- [x] T008 `function_call_output` gating (exit0 exclude / non-zero header / else scan) (WP02)
- [x] T009 New payload types `task_started`/`user_message`/empty; unchanged types untouched (WP02)
- [x] T010 §3a mirror + determinism guard (WP02)

**Dependencies**: WP01. **Risks**: must preserve the stateless `channelTextPair(obj)` (empty ctx) —
existing tests depend on it; envelope parser must be tolerant (unknown → scan); do not regress
`reasoning`/`message`/`agent_message`/`task_complete`/`token_count`.

## WP03 — Per-file prepass & context threading

**Goal**: Build the per-file `function_call` → `codexCall` registry in a prepass before channel
extraction, and thread the resulting `channelContext` through the file walk so
`channelStringsForEvent` consults it. Out-of-order tolerant; empty ctx reproduces today's behavior.
**Priority**: P1 (lands the behavior change). **Independent test**: an analyzer-level test (T017,
test-first) — a paired `function_call`(read) + `function_call_output`(exit 0) yields no read-content finding.
**Prompt**: [tasks/WP03-prepass-threading.md](./tasks/WP03-prepass-threading.md)

- [x] T017 Analyzer integration test (TEST-FIRST): ABSENCE of read-content FPs + recall (WP03)
- [x] T011 Per-file prepass `buildCodexContext(data)` → registry (WP03)
- [x] T012 Thread `channelContext` through parseFile → eventFromText → channelStringsForEvent (WP03)
- [x] T013 Call context-aware pair when ctx present; keep stateless paths unchanged (WP03)
- [x] T014 Build prepass once per file; empty ctx for non-codex files (WP03)

**Dependencies**: WP01, WP02. **Risks**: the prepass must key off the SAME per-file scope as
extraction; the prepass (not inline threaded state) is what gives out-of-order tolerance (Codex rec 1);
do not change the obj==nil / source-kind path.

## WP04 — Frozen-corpus validation + fixture test + gate

**Goal**: Validate FP-down / TP-preserved on a frozen representative codex corpus (base vs candidate,
separate caches), commit a small black-box fixture test over redacted `testdata/codex/`, confirm
`go test ./...` green + report schema unchanged, and document the behavior change (C1). Per-routing
correctness is already pinned test-first by WP02 (golden matrix) and WP03 (analyzer integration).
**Priority**: P1 (empirical acceptance proof). **Independent test**: the committed fixture test + the
recorded corpus diff.
**Prompt**: [tasks/WP04-golden-matrix-corpus.md](./tasks/WP04-golden-matrix-corpus.md)

- [x] T018 Frozen-corpus diff + committed black-box fixture test over `testdata/codex/` (WP04)
- [x] T019 `go test ./...` green; report schema unchanged; behavior documented (C1) (WP04)

**Dependencies**: WP02, WP03. **Risks**: corpus must be representative but frozen (live `~/.codex` ≈
298 MB, impractical); the fixture test must assert the ABSENCE of read-content findings, not only the
presence of real ones; run base+candidate back-to-back in one job (live-session-in-corpus confound);
never commit private raw corpus content (redact fixtures).

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
| NFR-001 (no recall loss) | WP03 (integration test), WP04 (corpus) |
| NFR-002 (FP substantially reduced) | WP02 (golden), WP04 (corpus) |
| NFR-003 (deterministic) | WP01, WP02 (golden) |
| NFR-004 (suite green, schema unchanged) | WP04 |

## Next Command

`/spec-kitty.analyze` (optional consistency gate) or `/spec-kitty.implement` to begin WP01.
