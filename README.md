# Spec Kitty Analyzer

Go CLI for turning Spec Kitty mission artifacts, `kitty-ops` records, and
agent transcript/log files into one deterministic story.

The parser borrows the useful local-first pieces from
`Priivacy-ai/agent-log-analyzer`: redaction, JSONL/text fallback parsing,
recursive JSON flattening, signature-style detection, and multi-format report
generation.

It detects:

- `/spec-kitty.*` command surfaces
- `spec-kitty ...` CLI invocations, including sync/tracker SaaS flag issues
- `spk-*` and legacy `spec-kitty-*` skills read by agents
- agent/profile strings from `--agent` and `--profile`
- mission vs Op vs outside scope
- timeline order from JSON timestamps, file order, and line numbers
- documented Spec Kitty failure modes with recovery guidance

## Install

Release installers use prebuilt native binaries. Go is not required on the
machine where the tool is installed or run.

macOS/Linux:

```bash
curl -fsSL https://github.com/Priivacy-ai/spec-kitty-analyzer/releases/latest/download/install.sh | sh
```

Windows PowerShell:

```powershell
iwr https://github.com/Priivacy-ai/spec-kitty-analyzer/releases/latest/download/install.ps1 -OutFile install.ps1
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

The installers put `spec-kitty-analyzer` on PATH and copy the bundled
`spec-kitty-analyzer` skill into existing `~/.agents/skills` and
`~/.claude/skills` directories. Missing skill roots are left untouched.

Release assets include:

- `spec-kitty-analyzer_linux_amd64.tar.gz`
- `spec-kitty-analyzer_linux_arm64.tar.gz`
- `spec-kitty-analyzer_darwin_amd64.tar.gz`
- `spec-kitty-analyzer_darwin_arm64.tar.gz`
- `spec-kitty-analyzer_windows_amd64.zip`
- `spec-kitty-analyzer_windows_arm64.zip`
- `install.sh`, `install.ps1`, and `checksums.txt`

## Reports

Mission-first mode scans harness logs, caches mission-to-log mappings in
`~/.spec-kitty-analyzer/cache.json`, then reports only the selected mission:

```bash
spec-kitty-analyzer analyze task-workflow-bug-fixes-01KV69BZ \
  --out spec-kitty-analyzer-report.json
```

If no mission slug or explicit path is given, the CLI refreshes the cache and
shows the 10 most recent harness logs as an interactive selection. Each row
shows detected mission slugs plus a derived short title, for example
`task-workflow-bug-fixes-01KV69BZ (Task Workflow Bug Fixes)`.

Use `--cache-bust` to force a full rescan of harness logs and rebuild the cache.
Without it, unchanged log files are reused and only new or modified logs are
rescanned.

Default harness roots include Claude, Codex, `.agents`, and OpenCode-style log
locations under the current user's home directory. Add custom roots with
repeatable `--log-root` flags.

Explicit path mode still works for direct analysis:

```bash
spec-kitty-analyzer analyze \
  /path/to/repo/kitty-specs \
  /path/to/repo/kitty-ops \
  ~/.codex/sessions \
  --out spec-kitty-analyzer-report.json
```

By default the command writes JSON, Markdown, HTML, and PDF reports next to the
JSON path. Use `--json-only` for structured output only.

(Running from source instead of an install? Replace `spec-kitty-analyzer` with
`go run ./cmd/spec-kitty-analyzer` in any command above.)

## spec-kitty-go activity (`go-activity`)

> **Experimental branch feature.** This reports what the **spec-kitty-go**
> binary (the governed-operation platform rewrite) actually *did* during agent
> sessions — a different surface from the Python Spec Kitty the rest of this
> tool detects.

spec-kitty-go wires into a harness as a governance hook: each governed tool call
runs `spec-kitty hook run --adapter <harness> --event PreToolUse …` and returns
one of `ADMIT | DENY | DECISION_REQUIRED` before any side effect. In a Claude
Code transcript that decision is recorded as a `hook` attachment — a shape the
main analyzer pipeline deliberately ignores. `go-activity` reads those
attachments (plus direct invocations of the go binary's own verb surface —
`hook/review/space/ledger/seal/governance/config` and `witness-sidecar`) and
rolls them up:

```bash
# Scan the most recent harness logs
spec-kitty-analyzer go-activity

# Analyze one session transcript (human summary)
spec-kitty-analyzer go-activity /path/to/session.jsonl

# Structured JSON for scripts/agents
spec-kitty-analyzer go-activity /path/to/session.jsonl --json

# Resolve logs by mission slug from the harness cache
spec-kitty-analyzer go-activity --mission my-mission-slug
```

The report leads with **governance decisions**:

- **Verdict breakdown** — ADMIT / DENY / DECISION_REQUIRED / ERROR. Verdicts
  follow spec-kitty-go's exact `hook run` contract: stdout `ADMIT` |
  `DENY: <reason>` | `DECISION_REQUIRED: <reason>` with exit codes 0 / 1 / 3
  (2 = usage/error). The stdout token is authoritative; the exit code is the
  fallback when a harness recorded no text. **DENY/DECISION reasons** are
  extracted and shown.
- **Governed-action correlation** — each verdict is linked back through its
  `toolUseID` to the exact tool call it gated, so you see *what* was governed
  (`govern Bash -> DENY :: rm -rf /etc — destructive write outside workspace`).
- **Pre/Post pairing** — counts of `PreToolUse` vs `PostToolUse` hooks and how
  many governed calls were seen with both.
- **Hook-hot-path latency** — min/p50/p95/max (spec-kitty-go's core claim).
- **Ledger accounting** — per the hook contract each admission durably appends
  `OperationRequested → OperationClassified → GovernanceContextResolved →
  AdmissionDecision` to `ledger.db`. That store is out-of-band (not in the
  transcript), so the count is reported as **derived**, alongside any *observed*
  `spec-kitty ledger`/`seal` CLI operations.

It then lists direct CLI verb usage and a timeline. Attribution is pinned to
spec-kitty-go's actual top-level verb surface — `hook | review | space | ledger
| config | version` (guarded by its own `verb_surface_doc_test.go`). Direct-CLI
attribution is conservative: distinctive go verbs (`hook/review/space/ledger`,
with `ledger` subcommands `list/show/tail/verify/seal`) and the go-only
`witness-sidecar` are always attributed, while verbs the Python CLI could share
(`config/version`) count only when invoked via an unambiguous go-build path
(`bin/spec-kitty`, `cmd/spec-kitty`, `go run ./cmd/spec-kitty`).

The same view is woven into the agent-facing `query` command as a
`spec_kitty_go` section (`--include go`, or `all`), and `analyze` prints a
one-line spec-kitty-go governance summary next to its mission stats:

```bash
spec-kitty-analyzer query my-mission-slug --include timeline,signals,go
```

## Agent JSON API

Agents and scripts should prefer `query`, which emits filtered JSON. The
timeline in this output only contains positive Spec Kitty signals: mission/Op
scope, slash commands, CLI invocations, skill reads, agent profiles, Spec Kitty
tool names, and deterministic Spec Kitty failure fingerprints.

```bash
spec-kitty-analyzer missions --limit 20
```

```bash
spec-kitty-analyzer query task-workflow-bug-fixes-01KV69BZ \
  --include timeline,signals,findings
```

Useful focused queries:

```bash
spec-kitty-analyzer query task-workflow-bug-fixes-01KV69BZ \
  --include timeline,signals,findings \
  --failure-id branch_worktree_confusion
```

```bash
spec-kitty-analyzer query task-workflow-bug-fixes-01KV69BZ \
  --include timeline,signals,findings \
  --command merge
```

Selectors can be repeated or comma-separated:

- `--include all|inputs|missions|ops|findings|timeline|signals|surface`
- `--failure-id <id-or-title>`
- `--command <slash-name|cli-verb|mission|work-package|agent|profile>`
- `--skill <skill-name-or-path>`
- `--profile <profile-or-agent>`
- `--scope <mission:<slug>|op:<id>|outside|mission|op>`
- `--contains <text>` for ad-hoc search inside already filtered Spec Kitty
  timeline events

## Scope Model

- `mission:<slug>`: event belongs to `kitty-specs/<slug>` or names
  `--mission <slug>` / `mission_slug`.
- `op:<invocation_id>`: event belongs to `kitty-ops/<id>.jsonl` or carries
  `invocation_id`.
- `outside`: event is part of the surrounding agent session but not clearly in
  a mission or Op.

## Failure Fingerprints

Rules are deterministic regex/JSON-field recognizers, grounded in Spec Kitty
skills and CLI behavior: blocked runtime decisions, guard failures, missing
artifacts, wrong command surface, branch/worktree confusion, merge failures,
dirty worktree/ref-advance failures, permission and EPERM denials, stale agents,
review rejections (including structural status events), sync/auth boundary
failures, tracker binding gaps, namespace-package import failures, and unclosed
Ops.

Matching is **channel-scoped**: each rule runs against real command/tool output
and structured error fields, not against narrative discussion of a problem. An
agent *talking about* an error — or a file or diff that merely contains an error
phrase — does not register as a failure. This sharply reduces false positives
while preserving the distinctive signatures that indicate a real, observed
condition.

The same principle covers **codex file-read and inspection commands**. When a
codex agent runs `cat`, `git diff`/`git show`, or `rg`/`grep`, the returned text
is file/diff/search *content*, not command-failure output — so error-like phrases
inside it (a diff that adds `raise AssertionError`, a doc that mentions `exit code
2`) are excluded from failure scanning. A genuine command failure is still caught:
a non-read command is scanned normally, and a read that itself fails keeps its
status/error header. When the command's intent is unknown, the analyzer scans (it
never suppresses a real failure to gain precision).

## Limitations

The analyzer is deterministic and pattern-based: it recognizes *documented* Spec
Kitty failure modes from real output channels. That keeps false positives low,
but there are known boundaries worth setting expectations around:

- **Validated primarily against a macOS + Claude/Codex corpus.** Error and denial
  phrasings specific to other platforms or shells (for example Windows
  `Access is denied`, fish/PowerShell, some Java/.NET forms) are not all covered
  yet, so detection on those environments is partial ([#6]).
- **Codex file-inspection output** — a `cat`/`grep`/`git show` surfaced as a tool
  result — is currently scanned as command output, so file or document *content*
  that merely contains an error phrase can occasionally produce a false positive
  ([#13]).
- **Known failure modes only.** Novel failures that match no rule are not
  surfaced yet; a segregated "unclassified anomaly" trap to preserve recall on new
  failure modes is planned ([#15]).
- **Local-first and read-only.** It reads harness logs under your home directory
  (Claude, Codex, `.agents`, OpenCode); it does not fetch or modify remote state.

Precision and recall are tracked in the issue tracker — reports of false
positives or missed failures against your own logs are welcome.

[#6]: https://github.com/Priivacy-ai/spec-kitty-analyzer/issues/6
[#13]: https://github.com/Priivacy-ai/spec-kitty-analyzer/issues/13
[#15]: https://github.com/Priivacy-ai/spec-kitty-analyzer/issues/15
