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

## Usage

```bash
go run ./cmd/spec-kitty-analyzer analyze \
  /path/to/repo/kitty-specs \
  /path/to/repo/kitty-ops \
  ~/.codex/sessions \
  --out spec-kitty-analyzer-report.json
```

By default the command writes JSON, Markdown, HTML, and PDF reports next to the
JSON path. Use `--json-only` for structured output only.

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
artifacts, wrong command surface, dirty worktree/ref-advance failures, stale
agents, review rejections, sync/auth boundary failures, tracker binding gaps,
namespace-package import failures, and unclosed Ops.
