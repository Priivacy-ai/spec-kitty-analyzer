# Tasks: Curated CHANGELOG & Release Notes Pipeline

**Mission**: changelog-release-pipeline-01KXF0ZK | **Branch**: `feat/changelog-release-pipeline`
**Planning base / merge target**: `feat/changelog-release-pipeline` → (PR) → `main`

Four work packages mapping to the plan's Implementation Concern Map (IC-02 core+CLI merged into one
well-sized package). Tests are included because the spec requires them (NFR-003).

## Subtask Index

| ID | Description | WP | Parallel |
|----|-------------|----|----------|
| T001 | Author CHANGELOG.md skeleton (Keep a Changelog header, Unreleased, bottom link refs) | WP01 | | [D] |
| T002 | Seed `[0.3.0]` from docs/releases/release-notes-0.3.0.md (with breaking-change notice) | WP01 | | [D] |
| T003 | Seed `[0.2.0]` from the curated v0.2.0 GitHub Release body | WP01 | | [D] |
| T004 | Seed `[0.1.1]` and `[0.1.0]` from git history (concise) | WP01 | | [D] |
| T005 | `version.go` — SemVer parse (stable + compact prerelease), compare, canonical, tag↔version parity | WP02 | [D] |
| T006 | `changelog.go` — heading parse (malformed = error), section extract, populated, top released version | WP02 | [D] |
| T007 | `git.go` — discover `v*.*.*` tags, exclude-under-release, latest by tuple sort | WP02 | [D] |
| T008 | `main.go` — dispatch, flags, `extract`, `validate` (branch state-aware + tag), exit codes | WP02 | | [D] |
| T009 | `version_test.go` — parse/compare/parity/prerelease/monotonic table tests | WP02 | [D] |
| T010 | `changelog_test.go` — extract/populated/malformed/missing-default/Unreleased/link-ref tests | WP02 | [D] |
| T011 | `main_test.go` — dispatch + exit codes + branch inter-release vs release-prep + tag parity | WP02 | | [D] |
| T012 | Edit `release.yml` — `fetch-depth: 0` on checkout (FR-010) | WP03 | | [D] |
| T013 | Edit `release.yml` — always create `dist/RELEASE_NOTES.md`; on tag builds run validate+extract | WP03 | | [D] |
| T014 | Edit `release.yml` — triple-consistency guard (validate tag + binary version==tag, fail closed); set `body_path`; drop `generate_release_notes` | WP03 | | [D] |
| T015 | New `release-readiness.yml` — validator on release-control-path PRs + nightly + dispatch; `fetch-depth: 0` | WP03 | | [D] |
| T016 | Edit `ci.yml` — extend cross-build smoke to also build `./tools/release` for the six targets | WP03 | | [D] |
| T017 | Local end-to-end verification of the workflows' commands (validate/extract dry runs) | WP03 | | [D] |
| T018 | Author `RELEASE_CHECKLIST.md` — scoped tag-as-SSOT procedure | WP04 | | [D] |
| T019 | Cross-check the checklist's commands/flags against the shipped `tools/release` CLI | WP04 | | [D] |
| T020 | Link the checklist from README (Limitations/Releases pointer) if a natural spot exists | WP04 | | [D] |

## WP01 — Curated CHANGELOG.md

- **Goal**: Add the source-of-record `CHANGELOG.md` seeded with real curated content.
- **Priority**: MVP (with WP02). **Independent test**: `tools/release extract 0.3.0` (once WP02 lands)
  prints the 0.3.0 section; humans can read the file.
- **Dependencies**: none.
- **Subtasks**: T001, T002, T003, T004.
- **Requirements**: FR-001, FR-002.
- **Risks**: heading grammar must match the WP02 parser (`## [X.Y.Z] - YYYY-MM-DD`, `## [Unreleased]`,
  bottom `[x.y.z]:` link refs — fixed in research R1); faithful transcription of the 0.3.0 breaking
  notice; correct tag dates (0.1.0/0.1.1 = 2026-06-20, 0.2.0 = 2026-07-03).
- **Prompt**: `tasks/WP01-curated-changelog.md` (~150 lines).

## WP02 — tools/release program (extract + validate) + tests

- **Goal**: Implement the standalone Go program and its unit tests.
- **Priority**: MVP (with WP01). **Independent test**: `go test ./tools/release` passes; `go build ./...`
  clean.
- **Dependencies**: none (parser is content-agnostic; tests use fixtures, not the real CHANGELOG).
- **Subtasks**: T005–T011.
- **Requirements**: FR-003, FR-004, FR-005, NFR-001, NFR-003, NFR-004.
- **Risks**: heading-regex false matches AND malformed-heading-must-error; state-aware branch
  monotonicity (`V==T` inter-release passes); tag-mode self-exclusion; compact-only prerelease;
  `extract` never calls git.
- **Prompt**: `tasks/WP02-tools-release-program.md` (~320 lines).

## WP03 — Release + readiness workflow wiring

- **Goal**: Wire the tooling into `release.yml`, add `release-readiness.yml`, extend `ci.yml`.
- **Priority**: High. **Independent test**: `act`/manual dry-run of the validate/extract steps; YAML
  lints; the six-target smoke includes tools/release.
- **Dependencies**: WP02 (needs the working commands).
- **Subtasks**: T012–T017.
- **Requirements**: FR-006, FR-007, FR-009, FR-010, NFR-002.
- **Risks**: `body_path` file must always exist (non-tag runs); tag-only extraction/guard;
  `fetch-depth: 0` in both workflows; awk version read fails closed; readiness path filter includes
  `release.yml`; PR #30 overlap (rebase after it merges).
- **Prompt**: `tasks/WP03-workflow-wiring.md` (~300 lines).

## WP04 — RELEASE_CHECKLIST.md

- **Goal**: Document the scoped, tag-as-SSOT release procedure end to end.
- **Priority**: Polish. **Independent test**: every command in the checklist matches the shipped CLI.
- **Dependencies**: WP03 (documents the final workflow/command shapes).
- **Subtasks**: T018–T020.
- **Requirements**: FR-008.
- **Risks**: drift from real command/flag shapes — cross-check against `tools/release` and quickstart.
- **Prompt**: `tasks/WP04-release-checklist.md` (~120 lines).

## Sequencing & parallelism

- WP01 and WP02 are independent (parallelizable). WP03 depends on WP02. WP04 depends on WP03.
- MVP = WP01 + WP02 (a working CHANGELOG + tooling). WP03 wires it into CI; WP04 documents it.
