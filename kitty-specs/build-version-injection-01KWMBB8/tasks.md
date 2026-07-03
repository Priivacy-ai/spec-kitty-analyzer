# Tasks: Build Version & Metadata Injection

**Mission**: build-version-injection-01KWMBB8 | **Branch**: `feat/build-version-injection`
**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

3 work packages, 14 subtasks. WP01 is the foundation (all Go code + tests); WP02 (release workflow) and WP03 (release notes) depend on WP01 and are parallel to each other.

## Subtask Index

| ID | Description | WP | Parallel |
|----|-------------|----|----------|
| T001 | `const Version` → `var Version/Commit/BuildDate` + `Build` struct + `CurrentBuild()` in types.go; Report first field → `Build` | WP01 | |
| T002 | analyzer.go: construct Report with `Build: CurrentBuild()` | WP01 | |
| T003 | main.go missions struct: first field → `Build analyzer.Build json:"build"` | WP01 | |
| T004 | query.go QueryResult: first field → `Build`; source from report.Build | WP01 | |
| T005 | main.go `version` command: print version + commit + build date | WP01 | |
| T006 | Migrate existing tests reading `Version` (query_test.go, analyzer_test.go) to `Build` | WP01 | |
| T007 | Add tests: sentinel defaults; `build` present + no top-level `version` across Report/query/missions; new missions cmd test | WP01 | |
| T008 | release.yml: compute stamping vars gated on `GITHUB_REF_TYPE == 'tag'` (else empty → sentinels) | WP02 | |
| T009 | release.yml: add `-X` ldflags to the non-windows build line (lowercase module path, 3 symbols) | WP02 | |
| T010 | release.yml: add identical `-X` ldflags to the windows `.exe` build line | WP02 | |
| T011 | release.yml: post-build assertion that a tag build's `version` output is not `dev` | WP02 | |
| T012 | Draft `release-notes-0.3.0.md`: highlights + BREAKING `.version`→`.build.version` + migration | WP03 | |
| T013 | Include before/after JSON snippet from the contract in the notes | WP03 | |
| T014 | Cross-check draft against contract + FR-005/FR-006; note `--notes-file` release requirement | WP03 | |

---

## Phase 1 — Foundation

### WP01 — Build model, nested-`build` JSON surfacing, version command, tests

- **Goal**: Make `Version/Commit/BuildDate` injectable package vars with sentinel defaults, model them as a `Build` value, surface `build` as the nested first field across all three JSON emitters (removing top-level `version`), update the `version` command, and keep `go test ./...` green (migrating tests that read `Version`).
- **Priority**: P1 (MVP — everything else depends on it)
- **Requirements**: FR-001, FR-002, FR-004, FR-005, NFR-001, NFR-003
- **Independent test**: `go build ./... && go vet ./... && go test ./...` green; `go run ./cmd/spec-kitty-analyzer version` prints `spec-kitty-analyzer dev (commit none, built unknown)`; a marshaled report/missions/query has a `build` object and no top-level `version`.
- **Dependencies**: none
- **Included subtasks**:
  - [x] T001 const→vars + Build struct + CurrentBuild + Report.Build (WP01)
  - [x] T002 analyzer.go Report construction (WP01)
  - [x] T003 missions struct Build field (WP01)
  - [x] T004 query.go QueryResult Build field (WP01)
  - [x] T005 version command output (WP01)
  - [x] T006 migrate existing Version-reading tests (WP01)
  - [x] T007 add sentinel + JSON-shape + missions cmd tests (WP01)
- **Estimated prompt size**: ~330 lines
- **Risks**: missing one of the three emitters leaves an inconsistent schema; tests must assert the *absence* of top-level `version`.

## Phase 2 — Release integration & docs (parallel after WP01)

### WP02 — Tag-gated release-workflow ldflags injection

- **Goal**: Inject real version/commit/date into all 6 release binaries via `-ldflags -X`, gated on tag pushes so manual `workflow_dispatch` runs keep the sentinels.
- **Priority**: P1
- **Requirements**: FR-003, FR-004, NFR-002, NFR-003
- **Independent test**: locally simulate (`quickstart.md`) — injected build reports the version; a non-tag build reports `dev`. CI: tag build's `version` output is not `dev`.
- **Dependencies**: WP01 (the `var` symbols must exist for `-X` to work)
- **Included subtasks**:
  - [ ] T008 tag-gated stamping vars (WP02)
  - [ ] T009 non-windows build line ldflags (WP02)
  - [ ] T010 windows build line ldflags (WP02)
  - [ ] T011 post-build verification (WP02)
- **Estimated prompt size**: ~230 lines
- **Risks**: the lowercase module-path footgun (C-002, silent no-op); both build lines must match; the tag gate (C-006) must fall back to sentinels for non-tag runs.

### WP03 — 0.3.0 breaking-change release notes

- **Goal**: Produce the curated `release-notes-0.3.0.md` draft that documents the removed top-level `version` as a breaking change with consumer migration guidance.
- **Priority**: P2
- **Requirements**: FR-006
- **Independent test**: draft contains a BREAKING section naming `.version`→`.build.version` with a before/after JSON example consistent with the contract.
- **Dependencies**: WP01 (documents the realized behavior)
- **Included subtasks**:
  - [ ] T012 draft notes: highlights + BREAKING + migration (WP03)
  - [ ] T013 before/after JSON snippet (WP03)
  - [ ] T014 accuracy cross-check + `--notes-file` reminder (WP03)
- **Estimated prompt size**: ~150 lines
- **Risks**: notes must match the final JSON contract exactly; the 0.3.0 release runbook must use `--notes-file` or the warning is dropped.

---

## MVP scope

**WP01** is the MVP: the whole observable behavior (nested `build`, sentinels, version command) lands there. WP02 makes release builds report real values; WP03 documents the break. Ship all three for 0.3.0.
