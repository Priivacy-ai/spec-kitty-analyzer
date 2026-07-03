---
work_package_id: WP01
title: Build model, nested-build JSON surfacing, version command, tests
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-004
- FR-005
- NFR-001
- NFR-003
tracker_refs:
- '#19'
- '#21'
planning_base_branch: feat/build-version-injection
merge_target_branch: feat/build-version-injection
branch_strategy: Planning artifacts for this mission were generated on feat/build-version-injection. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/build-version-injection unless the human explicitly redirects the landing branch.
subtasks:
- T001
- T002
- T003
- T004
- T005
- T006
- T007
phase: Phase 1 - Foundation
agent: "claude"
shell_pid: "5812"
history:
- at: '2026-07-03T16:30:00Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/
create_intent:
- internal/analyzer/build_test.go
- cmd/spec-kitty-analyzer/main_test.go
execution_mode: code_change
model: ''
owned_files:
- internal/analyzer/types.go
- internal/analyzer/analyzer.go
- internal/analyzer/analyzer_test.go
- internal/analyzer/build_test.go
- internal/query/query.go
- internal/query/query_test.go
- cmd/spec-kitty-analyzer/main.go
- cmd/spec-kitty-analyzer/main_test.go
role: implementer
tags: []
task_type: implement
---

# Work Package Prompt: WP01 – Build model, nested-`build` JSON surfacing, version command, tests

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile in the frontmatter, and behave per its guidance before parsing the rest of this prompt.

- **Profile**: `implementer-ivan`
- **Role**: `implementer`
- **Agent/tool**: `claude`

If no profile is specified, run `spec-kitty agent profile list` and select the best match for this WP's `task_type` and `authoritative_surface`.

---

## Markdown Formatting

Wrap HTML/XML tags in backticks. Use language identifiers in code blocks.

## Objective

Replace the hardcoded `const Version` with **injectable package variables** and a cohesive `Build` value, and surface build provenance as a **nested `build` object** — the first field — in every JSON emitter and in the `version` command. This is the breaking change (C1): the top-level `version` JSON field is **removed**. Local builds fall back to sentinel defaults. `go test ./...` must stay green, which requires migrating the existing tests that read `Version`.

This WP is the foundation. WP02 (release-workflow injection) and WP03 (release notes) depend on it.

## Branch Strategy

- Planning/base branch: `feat/build-version-injection`
- Final merge target: `feat/build-version-injection` (the mission later PRs this branch → `main`)
- Execution worktrees are allocated per computed lane from `lanes.json`; do not hand-create branches.

## Context

- Module path (for reference / consistency): `github.com/priivacy-ai/spec-kitty-analyzer` (lowercase).
- Design references in this mission dir: `data-model.md` (the `Build` entity), `contracts/build-output.md` (exact before/after JSON + CLI), `research.md` R5/R6/R8.
- Go 1.25.0, standard library only. No new dependencies.

## Subtasks

### T001 — `Build` model + injectable vars in `internal/analyzer/types.go`

**Purpose**: Make version/commit/date injectable and model them as one value; make `Report` carry a nested `build`.

**Steps**:
1. Replace `const Version = "0.2.0"` (line 5) with injectable vars and the `Build` type:
   ```go
   // Build metadata. Defaults mark a non-release (local) build; release.yml
   // overrides all three via -ldflags -X at build time. These MUST be vars,
   // not consts — ldflags -X can only overwrite string variables.
   var (
       Version   = "dev"
       Commit    = "none"
       BuildDate = "unknown"
   )

   // Build is the provenance of a compiled binary: one cohesive value.
   type Build struct {
       Version   string `json:"version"`
       Commit    string `json:"commit"`
       BuildDate string `json:"build_date"`
   }

   // CurrentBuild is the sole constructor for build provenance.
   func CurrentBuild() Build { return Build{Version, Commit, BuildDate} }
   ```
2. In the `Report` struct (line 7), replace the current first field `Version string \`json:"version"\`` with `Build Build \`json:"build"\`` — keep it **first** so `build` leads the marshaled JSON (R8).

**Files**: `internal/analyzer/types.go`
**Validation**: `go build ./internal/analyzer/` compiles.

### T002 — Report construction in `internal/analyzer/analyzer.go`

**Steps**:
1. At the `Report{` literal (~line 32-33) replace `Version: Version,` with `Build: CurrentBuild(),`.

**Files**: `internal/analyzer/analyzer.go`
**Validation**: package compiles; no remaining reference to a `Version` field on `Report`.

### T003 — `missions` result struct in `cmd/spec-kitty-analyzer/main.go`

**Steps**:
1. In `runMissions` the anonymous result struct (~line 138-144) currently declares `Version string \`json:"version"\`` first and sets `Version: analyzer.Version`. Replace the field with `Build analyzer.Build \`json:"build"\`` (first field) and set `Build: analyzer.CurrentBuild()`.

**Files**: `cmd/spec-kitty-analyzer/main.go`
**Validation**: compiles; missions JSON now leads with `build`.

### T004 — `query.QueryResult` in `internal/query/query.go`

**Steps**:
1. In `QueryResult` (~line 23-24) replace the first field `Version string \`json:"version"\`` with `Build analyzer.Build \`json:"build"\``.
2. At the build site (~line 139) replace `Version: report.Version,` with `Build: report.Build,`.
3. Ensure `internal/query` imports the analyzer package (it already references `report`); use `analyzer.Build`.

**Files**: `internal/query/query.go`
**Validation**: compiles; query JSON leads with `build`.

### T005 — `version` command output in `cmd/spec-kitty-analyzer/main.go`

**Steps**:
1. Replace line 41 `fmt.Println("spec-kitty-analyzer " + analyzer.Version)` with:
   ```go
   fmt.Printf("spec-kitty-analyzer %s (commit %s, built %s)\n",
       analyzer.Version, analyzer.Commit, analyzer.BuildDate)
   ```

**Files**: `cmd/spec-kitty-analyzer/main.go`
**Validation**: `go run ./cmd/spec-kitty-analyzer version` → `spec-kitty-analyzer dev (commit none, built unknown)`.

### T006 — Migrate existing tests that read `Version`

**Purpose**: NFR-001 — the suite must stay green; two tests construct/read the old `Version` field and will not compile after T001-T004.

**Steps**:
1. `internal/query/query_test.go` (~line 10): update any construction/assertion using `Version` to use `Build` (e.g. `analyzer.Build{Version: "...", ...}` or read `.Build.Version`).
2. `internal/analyzer/analyzer_test.go` (~line 245): same migration.
3. Search the whole tree for any other `.Version` reads on these structs: `grep -rn "\.Version" internal/ cmd/ --include=*.go` and fix any that refer to the report/query/missions structs (leave unrelated `.Version` such as cache version alone).

**Files**: `internal/query/query_test.go`, `internal/analyzer/analyzer_test.go`
**Validation**: `go test ./...` compiles and passes.

### T007 — New tests: sentinels, nested `build`, no top-level `version`

**Purpose**: Lock the new contract, including the *absence* of top-level `version` (guards FR-005 in both directions).

**Steps**:
1. `internal/analyzer/build_test.go` (new): assert `CurrentBuild()` returns `{Version:"dev", Commit:"none", BuildDate:"unknown"}` in a default build.
2. Add a marshal test (in `analyzer_test.go` or `build_test.go`): marshal a `Report`, unmarshal into `map[string]any`, assert `m["build"]` exists with the three keys and `m["version"]` is absent.
3. `cmd/spec-kitty-analyzer/main_test.go` (new): exercise the `missions` output path (or its result struct) and assert the same — `build` present, top-level `version` absent.
4. Add the equivalent assertion for `query.QueryResult` in `internal/query/query_test.go`.

**Files**: `internal/analyzer/build_test.go` (new), `internal/analyzer/analyzer_test.go`, `cmd/spec-kitty-analyzer/main_test.go` (new), `internal/query/query_test.go`
**Validation**: `go test ./...` green; the absence-of-`version` assertions pass.

## Definition of Done

- [ ] `const Version` replaced by `var Version/Commit/BuildDate` (defaults `dev`/`none`/`unknown`) + `Build` struct + `CurrentBuild()`.
- [ ] All three JSON emitters (`Report`, missions, `query`) carry `build` as the **first** field; **no** top-level `version` anywhere in JSON output.
- [ ] `version` command prints version + commit + build date.
- [ ] Existing `Version`-reading tests migrated; new sentinel + shape tests added (incl. a `missions` cmd test).
- [ ] `go build ./... && go vet ./... && go test ./...` all green.
- [ ] No stray `.Version` reference to the report/query/missions structs remains.

## Reviewer guidance

- Confirm each emitter’s marshaled JSON has `build` first and **no** top-level `version` — grep the tests for an explicit absence assertion, not just presence of `build`.
- Confirm `Build` is a **named** field (`json:"build"`), not an embedded/anonymous field (embedding would flatten the object — the wrong shape).
- Confirm no functional behavior beyond version surfacing changed.

## Activity Log

- 2026-07-03T16:58:44Z – claude – shell_pid=96597 – Assigned agent via action command
- 2026-07-03T17:07:15Z – claude – shell_pid=96597 – WP01 complete: build model + nested-build JSON + version cmd + tests; go build/vet/test green; behavioral acceptance (dev sentinels, injected 0.3.0, JSON build object + no top-level version) verified end-to-end. Includes analysis finding C1 (version-output test).
- 2026-07-03T17:21:36Z – codex – shell_pid=4210 – Started review via action command
- 2026-07-03T17:25:09Z – user – shell_pid=4210 – Moved to planned
- 2026-07-03T17:25:36Z – claude – shell_pid=5812 – Started implementation via action command
- 2026-07-03T17:27:38Z – claude – shell_pid=5812 – Cycle-2: addressed all 3 codex findings (query Result contract test, missions build sub-field asserts, stale comment). go build/vet/test green.
