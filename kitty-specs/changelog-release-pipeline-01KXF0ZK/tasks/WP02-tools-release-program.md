---
work_package_id: WP02
title: tools/release program (extract + validate) + tests
dependencies: []
requirement_refs:
- FR-003
- FR-004
- FR-005
- NFR-001
- NFR-003
- NFR-004
tracker_refs: []
planning_base_branch: feat/changelog-release-pipeline
merge_target_branch: feat/changelog-release-pipeline
branch_strategy: already-confirmed
subtasks:
- T005
- T006
- T007
- T008
- T009
- T010
- T011
agent: claude
history:
- 2026-07-14 created (tasks phase)
agent_profile: implementer-ivan
authoritative_surface: tools/release/
create_intent:
- tools/release/main.go
- tools/release/version.go
- tools/release/changelog.go
- tools/release/git.go
- tools/release/version_test.go
- tools/release/changelog_test.go
- tools/release/main_test.go
execution_mode: code_change
owned_files:
- tools/release/**
role: implementer
tags: []
---

## ⚡ Do This First: Load Agent Profile

```
/ad-hoc-profile-load implementer-ivan
```

Adopt its identity, governance scope, and boundaries before proceeding.

## Objective

Implement the standalone Go program `tools/release` (`package main`, module path
`github.com/priivacy-ai/spec-kitty-analyzer/tools/release`) providing `extract` and `validate`
subcommands over `CHANGELOG.md` and git tags, plus table-driven unit tests. **Standard library only**
— no third-party imports, and do **not** import `internal/analyzer` (keep it out of the shipped
binary). Go 1.25.

## Context — READ THESE FIRST

- **Contract**: `contracts/tools-release-cli.md` — the exact CLI surface, flags, output, and exit
  codes. This is authoritative for behavior.
- **Data model**: `data-model.md` — Version / ChangelogSection / ReleaseTag value types, the
  validation-rules table, and orderings.
- **Research**: `research.md` R1 (heading grammar), R2 (compact-only prerelease + ordering), R3
  (validate modes incl. state-aware branch monotonicity + tag self-exclusion), R5 (extract
  missing-section default), R6 (tag discovery), R10 (malformed heading = error).
- Mirrors `~/repos/spec-kitty/scripts/release/{extract_changelog.py,validate_release.py}` — consult
  for parity, but this is Go and scoped (no pyproject/uv.lock/PyPI).

### Subtasks

- **T005 `version.go`** — `Version{Major,Minor,Patch,Stage,StageNum}`; `ParseVersion(string)` accepting
  `X.Y.Z` and compact `X.Y.Z(a|b|rc)N` ONLY (reject dotted `-rc.N`); `Compare`/ordering by
  `(major,minor,patch,stageRank,stageNum)` with `stageRank={a:0,b:1,rc:2,stable:3}`; `Canonical()`
  string; `ParseTag("vX.Y.Z")→Version`; parity helper `tag == "v"+version.Canonical()`.
- **T006 `changelog.go`** — parse `CHANGELOG.md` into ordered sections. A `## [...]` heading whose
  bracket content is `Unreleased` → sentinel; a valid version → released section; **anything else →
  return a hard error** naming the heading (R10). `ExtractSection(version)` returns the body (heading
  excluded, leading/trailing blank lines trimmed) or the default `Release <v>\n\nNo changelog entry
  found for this version.`; `IsPopulated`; `TopReleasedVersion()`. Bottom `[x.y.z]: http...` link-ref
  lines are NOT headings. **Reads only the file — never calls git.**
- **T007 `git.go`** — `DiscoverReleaseTags(exclude string) ([]ReleaseTag, error)` via
  `git tag --list 'v*.*.*'`, filter to `v`+valid version, tuple-sort desc; `LatestTag(exclude)`.
  Clear error if not in a git work tree.
- **T008 `main.go`** — dispatch `extract`/`validate`; `flag` parsing; wire to the core. `extract
  <version>`: print section to stdout, exit 0 (exit 1 if CHANGELOG missing, 2 on usage). `validate
  --mode branch|tag [--tag]`: implement the R3 checks — common (top released valid + populated +
  malformed-heading error), branch state-aware (`V>T` OK / `V==T` OK / `V<T` error), tag (parity +
  strict `>` excluding `--tag`). Success → one-line summary to stderr, exit 0; failure → `- <issue>`
  lines to stderr, exit 1; usage error exit 2. Diagnostics to stderr, notes to stdout.
- **T009 `version_test.go`** — table tests: parse stable + each prerelease form; reject dotted/garbage;
  ordering incl. `0.4.0rc1 < 0.4.0`; parity `v0.3.0`↔`0.3.0`.
- **T010 `changelog_test.go`** — fixtures (inline strings): extract populated section; empty section
  → not populated; missing → default text; `## [Unreleased]` and `[x.y.z]:` link refs not parsed as
  released; malformed `## [0.3]` / `## [v0.3.0]` → error; TopReleasedVersion picks the right one.
- **T011 `main_test.go`** — subcommand dispatch + exit codes; branch **inter-release** (`V==T` → exit
  0) vs **release-prep** (`V>T` → exit 0) vs behind (`V<T` → exit 1); tag parity pass/fail. Use a temp
  dir with a fixture CHANGELOG.md and, where git is needed, either a temp git repo with tags or inject
  the tag set via a seam (prefer a small internal function taking tags as a parameter so tests don't
  need a real repo).

## Design guidance

- Keep a **pure core** (parse/extract/validate as functions taking strings/slices) and a thin git +
  CLI shell, so tests hit the core without a real repo. E.g. `validateBranch(sections, tags)` and
  `validateTag(sections, tags, tag)` returning `[]issue`; `main.go` gathers `tags` from `git.go` and
  calls these.
- No global state. No network. `os/exec` only for `git tag --list`.

## Branch Strategy

Base and merge target `feat/changelog-release-pipeline`; work in the lane from `lanes.json`.

## Definition of Done

- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` (clean) all pass.
- [ ] `go test ./tools/release` passes with the tables above.
- [ ] No third-party imports; `go.mod` unchanged (0 new `require`); no `internal/analyzer` import.
- [ ] `extract`/`validate` behavior matches `contracts/tools-release-cli.md` exactly (exit codes,
      output stream, messages).
- [ ] Manual dry runs succeed against the WP01 CHANGELOG once both exist:
      `go run ./tools/release validate --mode branch`, `... extract 0.3.0`,
      `... validate --mode tag --tag v0.3.0`.

## Risks / Reviewer guidance

- Reviewer: verify the malformed-heading error path (not a silent skip); the `V==T` inter-release
  case returns success (regression trap from Codex R4); tag-mode excludes the tag under release;
  `extract` has zero git calls; exit codes exactly per contract; prerelease is compact-only.
