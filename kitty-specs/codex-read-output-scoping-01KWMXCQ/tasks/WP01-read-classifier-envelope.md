---
work_package_id: WP01
title: Read classifier, allowlist & output-envelope parser
dependencies: []
requirement_refs:
- FR-003
- FR-004
- NFR-003
tracker_refs: []
planning_base_branch: fix/codex-read-output-scoping
merge_target_branch: fix/codex-read-output-scoping
branch_strategy: Planning artifacts for this mission were generated on fix/codex-read-output-scoping. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into fix/codex-read-output-scoping unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-codex-read-output-scoping-01KWMXCQ
base_commit: 5e6ea6fe9f89cf9f67a3a9f7dd20d84d29c17f88
created_at: '2026-07-03T21:38:22Z'
subtasks:
- T001
- T002
- T003
- T004
- T020
phase: Phase 1 - Foundation
assignee: ''
agent: "codex"
shell_pid: "1346"
history:
- at: '2026-07-03T21:38:22Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/patterns
create_intent:
- internal/analyzer/patterns_test.go
execution_mode: code_change
model: ''
owned_files:
- internal/analyzer/patterns.go
- internal/analyzer/patterns_test.go
role: implementer
tags: []
task_type: implement
---

# Work Package Prompt: WP01 – Read classifier, allowlist & output-envelope parser

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

Provide the **pure, dependency-free primitives** the codex read-output gating (WP02) will consume:

1. A **read-command allowlist** (`readCommandSet`) plus the mutating-git denylist.
2. A **conservative compound classifier** `classifyCodexReadCommand(name, cmd string) bool` that
   returns true only when a command is a *pure* read/inspection — every operator-split segment leads
   with a read command, no write redirection, no mutating git. Any uncertainty → false (scan).
3. An **output-envelope parser** `parseCodexOutputEnvelope(output string) (header, bulk string, exitCode int, ok bool)`
   that splits a codex `function_call_output` string into its status header, its bulk content (after
   `Output:`), and the exit code. Unparseable → `ok == false` (caller scans).

These functions introduce **no behavior change on their own** — they are consumed by WP02/WP03.

**Done when:**
- `internal/analyzer/patterns.go` exposes `readCommandSet`, `classifyCodexReadCommand`, and
  `parseCodexOutputEnvelope`, each documented with its recall-safe default.
- `classifyCodexReadCommand` is conservative per FR-003 (see matrix rows 1–7 in the contract).
- `parseCodexOutputEnvelope` is tolerant per FR-004 (unknown/malformed → `ok=false`).
- `go build ./... && go vet ./... && gofmt -l internal/analyzer/patterns.go` clean (no output from gofmt).
- No new module dependency (Go stdlib only: `strings`, `regexp`).

## Context & Constraints

- **Authoritative design**: mission `research.md` (R4 classifier, R3 envelope), `data-model.md`
  ("Envelope (parsed, not stored)"), `contracts/channel-matrix.md` (rows 1–7 are the classifier
  contract). Read these before coding.
- **Durable design doc**: `~/spec-kitty-analyzer-issue4-backup/catfood-findings/codex-payload-design.md`
  ("FINAL DESIGN") — the Codex-reviewed source of truth for the classifier + envelope shapes.
- **Study, do NOT edit** (owned by other WPs):
  - `internal/analyzer/channels.go` (WP02) — `extractCodexPayload` is where your functions get called.
  - `internal/analyzer/analyzer.go` (WP03) — the prepass that builds the registry.
- **patterns.go is the right home**: it already holds text-parsing/detection helpers
  (`parseCLIInvocation`, `shellishFields`, `trimShell`). Keep these new helpers in the same idiom —
  small, pure, deterministic. (This is the documented WP-decomposition relocation from plan IC-01;
  see the structure note in `tasks.md`.)
- **Constraints**: C-003 (no shell-parser dependency — a conservative segment classifier only);
  NFR-003 (deterministic). Recall-safe on every uncertainty.

## Branch Strategy

- **Strategy**: already-confirmed
- **Planning base branch**: fix/codex-read-output-scoping
- **Merge target branch**: fix/codex-read-output-scoping

> Execution worktrees are allocated per computed lane from `lanes.json`.

## Subtasks & Detailed Guidance

### Subtask T001 – Read-command allowlist + mutating-git denylist
- **Purpose**: Enumerate the commands that are pure reads, and the git subcommands that are NOT.
- **Steps**:
  - Add `var readCommandSet = map[string]bool{...}` with the **initial allowlist** (R4):
    `cat head tail nl wc rg grep egrep fgrep ls find stat file`.
  - Add a `gitReadSubcommands` set: `show diff log blame status`. A `git` command counts as a read
    **only** when its first non-flag argument is in this set; any other git subcommand (`add`,
    `commit`, `checkout`, `restore`, `reset`, `push`, `merge`, …) is mutating → not a read.
  - Deliberately **exclude** `sed` and `awk` (can mutate with `-i` / redirects) — document why in a comment.
- **Files**: `internal/analyzer/patterns.go`.
- **Notes**: Keep the sets minimal (recall over reach). Adding a command later is cheap; a wrong
  inclusion silently suppresses real failures.

### Subtask T002 – `classifyCodexReadCommand(name, cmd string) bool`
- **Purpose**: Decide whether a codex call is a pure read/inspection (FR-003).
- **Steps**:
  1. **Tool-name path**: if `name != "exec_command"` (a read-file tool name), classify by `name`
     against a small known-read-file-tool set; unknown tool name → false. (Confirm actual read-file
     tool names from the corpus during WP04; if none exist in the corpus, keep the hook but default
     unknown → false.)
  2. **exec_command path** (`name == "exec_command"`, `cmd` non-empty):
     - Split `cmd` on the shell operators `&&`, `||`, `;`, and `|` (pipe). Use a simple scanner that
       splits on these tokens **outside** of quotes — do NOT implement a full shell parser (C-003).
       A pragmatic split on the literal operator strings is acceptable; if quote-aware splitting is
       hard, prefer **false** (scan) on anything containing unbalanced quotes.
     - For **every** segment: trim, take the leading token (first whitespace-delimited word). The
       command is a read only if **every** segment's leading token is in `readCommandSet`, OR is
       `git` with a first non-flag arg in `gitReadSubcommands`.
     - If **any** segment contains a write redirection (`>` or `>>`) → false.
     - If **any** segment fails the above → false.
  3. Empty `cmd` with `name == "exec_command"` → false (unknown intent → scan).
- **Files**: `internal/analyzer/patterns.go`.
- **Notes**: Recall-safe examples that MUST classify as **not-read** (scan): `git diff && go build`,
  `cat x > y`, `rg foo && rm bar`. Examples that MUST classify as **read**: `git diff`, `rg foo | head`,
  `rg foo || true`, `cat a b`, `ls -la`. These map to contract rows 1,2,4,5.

### Subtask T003 – `parseCodexOutputEnvelope(output string) (header, bulk string, exitCode int, ok bool)`  [P]
- **Purpose**: Split a codex `function_call_output` string so WP02 can keep the status header of a
  failed read while dropping benign bulk content (FR-004).
- **Steps**:
  - The envelope shape (`data-model.md`):
    ```
    Chunk ID: …
    Wall time: … seconds
    Process exited with code <N>
    Original token count: …
    Output:
    <bulk content>
    ```
  - Parse the integer `<N>` after `Process exited with code` (a tolerant regex, e.g.
    `Process exited with code (\d+)`). If absent → `ok = false` (caller scans everything).
  - `header` = the substring up to and **including** the status line (the `Process exited with code N`
    line). `bulk` = everything after the `Output:` marker line (empty string if no `Output:` marker).
  - Return `ok = true` only when the exit-code line was found. Malformed / missing → `ok = false`,
    and leave `header`/`bulk` empty (caller falls back to scanning the raw output).
- **Files**: `internal/analyzer/patterns.go`.
- **Notes**: Be tolerant of leading `Chunk ID`/`Wall time` lines being absent. Do NOT assume fixed
  line positions — locate by marker strings. This function is the recall hook for `cat missing_file`
  (row 2): a non-zero read keeps `header`, drops `bulk`.

### Subtask T004 – Package doc + recall-safe invariants
- **Purpose**: Make the recall-safe contract explicit at the call site of future maintainers.
- **Steps**: Add a short doc comment block above the three helpers stating the governing invariant:
  *any uncertainty (unknown command, unbalanced quotes, missing exit-code line) resolves to the
  scanning default (`false` / `ok=false`), never to exclusion.* Reference FR-003/FR-004 and C-003.
- **Files**: `internal/analyzer/patterns.go`.

### Subtask T020 – Unit tests for the classifier + envelope (TEST-FIRST)
- **Purpose**: Drive T002/T003 test-first per DIRECTIVE_034 + DIRECTIVE_039 (specification-by-example).
- **Steps**: Create `internal/analyzer/patterns_test.go`. **Author these BEFORE the production code**
  (red → green): table-driven cases for `classifyCodexReadCommand` and `parseCodexOutputEnvelope`.
  - Classifier reads (expect true): `git diff`, `rg foo | head`, `rg foo || true`, `cat a b`, `ls -la`,
    `git show HEAD`, `stat x`. Non-reads (expect false): `git diff && go build`, `cat x > y`,
    `rg foo && rm bar`, `sed -i s/a/b/ f`, `go build ./...`, empty cmd, unknown tool name.
  - Envelope: a well-formed exit-0 body → `ok=true`, exit 0, `bulk` = content after `Output:`; a
    non-zero body → `ok=true`, exitCode!=0, `header` contains the status line; a malformed body (no
    exit-code line) → `ok=false`; empty/short input → `ok=false`, no panic.
- **Files**: `internal/analyzer/patterns_test.go`.
- **Notes**: Run `go test ./internal/analyzer/ -run 'ReadCommand|Envelope'` red first, then implement
  T001–T003 to green. These are unit tests of pure functions (governed by unit-test rules, not the
  black-box integration directive).

## Test Strategy

- **Test-first** (DIRECTIVE_034/039): write `patterns_test.go` (T020) red, then implement to green.
- Run: `go test ./internal/analyzer/ -run 'ReadCommand|Envelope' -v` then `go test ./internal/analyzer/`.

## Risks & Mitigations

- **Over-broad allowlist → recall loss** → keep `readCommandSet` minimal; `sed`/`awk` excluded.
- **Naive operator split mis-parses quoted operators** → on unbalanced quotes, return false (scan).
- **Rigid envelope parsing** → locate by marker strings, tolerate absent optional lines, default `ok=false`.

## Review Guidance

- Confirm `classifyCodexReadCommand` requires **every** segment to be a read (row 4 `git diff && go build`
  must be false).
- Confirm write redirection anywhere → false.
- Confirm `parseCodexOutputEnvelope` returns `ok=false` on a missing exit-code line and never panics
  on short/empty input.
- Confirm no new dependency and gofmt-clean.

## Activity Log

- 2026-07-03T21:38:22Z – system – Prompt created.
- 2026-07-03T22:07:40Z – claude – shell_pid=85733 – Assigned agent via action command
- 2026-07-03T22:46:12Z – claude – shell_pid=85733 – Implemented test-first; Codex adversarial review 9 rounds -> APPROVE
- 2026-07-03T22:47:00Z – codex – shell_pid=1346 – Started review via action command
- 2026-07-03T22:49:10Z – user – shell_pid=1346 – Codex review 9 rounds -> APPROVE; go build/vet/test green
