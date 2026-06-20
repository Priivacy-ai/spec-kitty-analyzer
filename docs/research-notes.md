# Research Notes

## Source Analyzer Borrow Points

`agent-log-analyzer` is a Go tool. Reused/adapted concepts:

- local-only redaction before report generation
- JSONL-first parsing with text fallback
- recursive JSON string flattening
- bounded event/timeline model
- deterministic regex/signature registries
- JSON/HTML/Markdown/PDF report pack

## Spec Kitty Surface

Observed `spec-kitty --help` top-level commands include:

`init`, `accept`, `config`, `dashboard`, `implement`, `intake`, `specify`,
`plan`, `tasks`, `lint`, `materialize`, `merge`, `next`, `research`, `review`,
`safe-commit`, `session-start`, `session-stop`, `upgrade`,
`validate-encoding`, `validate-tasks`, `verify-setup`, `dispatch`, `agent`,
`auth`, `charter`, `context`, `doctor`, `doctrine`, `glossary`, `migrate`,
`mission`, `mission-type`, `ops`, `plugin`, `orchestrator-api`, `sync`,
`workflow`, `profiles`, `profile-invocation`, `invocations`, `retrospect`.

Generated user commands are `/spec-kitty.*`; public operating skills are
`spk-*`; legacy detailed skills are `spec-kitty-*`.

## Scope Model

- Mission: `kitty-specs/<slug>`, `--mission <slug>`, or `mission_slug`.
- Op: `kitty-ops/<invocation_id>.jsonl` or `invocation_id`.
- Outside: surrounding agent transcript with no mission/Op anchor.

## Failure Fingerprints

Deterministic rules cover:

- runtime `blocked`, `decision_required`, null-prompt step, and completion-not-terminal bug
- guard failures and missing artifacts
- wrong CLI surfaces such as `agent action implement/review --json`
- Typer usage errors
- worktree linkage, dirty ref advance, non-fast-forward ref advance, merge conflicts
- missing implementation commits
- review rejection, reviewer failure, stale agent leases, dependency cycles
- runtime/init/config/manifest/skill surface failures
- encoding errors
- sync/auth boundary failures, missing local SaaS flag, tracker binding failures
- `spec-kitty-events` namespace-package import failure
- verification failures, timeouts, permissions
- open Op orphan records
