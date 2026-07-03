# Research — Codex Read-Output Scoping

Phase 0. The implementation design was authored and **Codex-design-reviewed** (verdict DESIGN_NEEDS_REVISION → all 7 recommendations verified against the code and adopted, Kent-approved 2026-07-03). No `[NEEDS CLARIFICATION]` markers remain. Full design: `~/spec-kitty-analyzer-issue4-backup/...` / scratchpad `codex-payload-design.md`.

## R1 — Correlate via a per-file prepass, not inline threaded state (Codex rec 1, 5)
- **Decision**: A per-file prepass scans the file's events for `function_call` payloads and builds `map[callID] → codexCall{name, cmd, isRead}`; the existing event-construction pass then runs with a `channelContext` carrying that registry.
- **Rationale**: `eventFromText`/`channelStringsForEvent` cache `outputCh`/`diagnosticCh` once per event during the walk (analyzer.go), so post-hoc correlation fights the cache. A prepass is deterministic, tolerant of out-of-order streams, and testable with golden line pairs; correlation stays in the channel layer (rules remain channel-agnostic — they consume precomputed strings). Rejected: single-pass threaded mutable state (fails out-of-order); rule-layer correlation (leaks transcript structure into every rule).
- **Back-compat**: keep the stateless `channelTextPair(obj)` (empty context) so existing tests are unchanged.

## R2 — Exclude read content from BOTH channels (Codex rec 2)
- **Decision**: Read content reaches neither `output` nor `narrative` — mirror the Claude §3a exclusion exactly.
- **Rationale**: `diagnosticCh` IS scanned (`scopeDiagnostic` patterns, e.g. `branch_worktree_confusion`), and those tokens appear in diffs/docs. Diagnostic-only routing would reintroduce a narrower FP class.

## R3 — Envelope-aware gating preserves recall (Codex rec 3)
- **Decision**: Parse the codex output envelope (`Process exited with code N` … `Output:`). For a read/inspection call: exit 0 → exclude output fully; non-zero/interrupted/timed-out → keep only the **status/error header** on `output`, drop the bulk content after `Output:`; unknown/malformed envelope → conservative, scan.
- **Rationale**: preserves a genuine read error (e.g. `cat missing_file`) while dropping benign read content; the failure rules already treat exit status as a generic signal. Also leaves a future hook for reasoning/expectation-divergence signals.

## R4 — Conservative compound classifier, no shell-parser dependency (Codex rec 4, C-003)
- **Decision**: split `cmd` on `&&`/`||`/`;`/`|`; a call is pure-inspection only if EVERY segment's leading token is in the read allowlist, with no write redirection (`>`/`>>`) and no mutating `git` subcommand; any parse uncertainty → not read.
- **Rationale**: recall-safe (`git diff && go build` still scans) while not penalizing pure read pipelines (`rg foo | head`, `rg foo || true`). A full shell parser is rejected for the dependency cost.
- **Read allowlist (initial)**: `cat head tail nl wc rg grep egrep fgrep ls find stat file` + `git {show,diff,log,blame,status}`. `sed`/`awk` excluded (can mutate). Read-file **tool names** (codex `name` != `exec_command`) classify by name.

## R5 — Payload mapping: only add the genuinely-missing types (Codex rec 6 — corrected)
- **Decision**: ADD `function_call` (excluded + registers metadata), `task_started` (excluded marker), `user_message` (narrative only if the corpus confirms a user-prose field), empty `payload.type` (excluded only if truly known-empty). Leave `reasoning`/`message`/`agent_message`/`task_complete`/`token_count` UNCHANGED (already mapped).
- **Rationale**: the "unmapped reasoning" log seen in the field is the *known-type-with-absent-field* path (reasoning text lives under `summary`/`encrypted_content`, not `content`), not an unmapped type — re-mapping it would regress.

## R6 — call_id normalization (Codex rec 7)
- **Decision**: one helper reads `call_id` then `callId`; require a non-empty id AND (`name == "exec_command"` OR a known read-file tool name) before correlating. Unknown id → scan.
- **Rationale**: test fixtures already use camelCase `callId`; a spelling miss would silently disable the feature.

## R7 — Frozen validation corpus (C-005)
- **Decision**: assemble a small, **frozen** representative codex corpus (a curated set of codex session `.jsonl` files) for before/after diffing; the live full `~/.codex` (~298 MB) is impractical for iteration.
- **Method**: run base and candidate binaries back-to-back in one job with **separate `--cache`** per binary; compare fingerprint counts + evidence sources. The exact frozen set is selected during tasks/implementation (include sessions exhibiting read-content FPs: git diff/show, rg/grep, cat of source/docs).
