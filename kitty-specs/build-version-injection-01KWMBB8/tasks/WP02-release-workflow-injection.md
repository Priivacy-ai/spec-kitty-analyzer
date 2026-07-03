---
work_package_id: WP02
title: Tag-gated release-workflow ldflags injection
dependencies:
- WP01
requirement_refs:
- FR-003
- FR-004
- NFR-002
- NFR-003
tracker_refs:
- '#19'
- '#21'
planning_base_branch: feat/build-version-injection
merge_target_branch: feat/build-version-injection
branch_strategy: Planning artifacts for this mission were generated on feat/build-version-injection. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/build-version-injection unless the human explicitly redirects the landing branch.
subtasks:
- T008
- T009
- T010
- T011
phase: Phase 2 - Release integration
agent: claude
history:
- at: '2026-07-03T16:30:00Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: .github/workflows/
create_intent: []
execution_mode: code_change
model: ''
owned_files:
- .github/workflows/release.yml
role: implementer
tags: []
task_type: implement
---

# Work Package Prompt: WP02 – Tag-gated release-workflow ldflags injection

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile in the frontmatter, and behave per its guidance before parsing the rest of this prompt.

- **Profile**: `implementer-ivan`
- **Role**: `implementer`
- **Agent/tool**: `claude`

If no profile is specified, run `spec-kitty agent profile list` and select the best match.

---

## Markdown Formatting

Wrap HTML/XML tags in backticks. Use language identifiers in code blocks.

## Objective

Make tagged release builds stamp the real version, commit, and build date into all six binaries via `-ldflags -X` — while **manual `workflow_dispatch` runs keep the sentinels** (C-006). This depends on WP01 having turned the `const` into `var` symbols.

## Branch Strategy

- Planning/base + merge target: `feat/build-version-injection`. Execution worktrees are per-lane from `lanes.json`.

## Context

- File: `.github/workflows/release.yml`. It triggers on **both** `on: push: tags: v*` AND `workflow_dispatch` — this is why stamping must be tag-gated.
- The two existing build invocations (windows `.exe` and non-windows) both use `-ldflags="-s -w"` with `CGO_ENABLED=0 -trimpath`. Codex confirmed `-X` composes with these.
- **The footgun (C-002)**: the `-X` symbol path MUST be the lowercase module path `github.com/priivacy-ai/spec-kitty-analyzer/internal/analyzer`. A wrong path fails **silently** — no error, binary keeps `dev`.
- Reference: `research.md` R2/R3, `quickstart.md`.

## Subtasks

### T008 — Tag-gated stamping variables

**Purpose**: Compute injection flags only for tag builds; empty otherwise.

**Steps**:
1. In the "Build packages" step, before the `targets` loop, add:
   ```bash
   PKG=github.com/priivacy-ai/spec-kitty-analyzer/internal/analyzer
   LDFLAGS_META=""
   if [[ "${GITHUB_REF_TYPE}" == "tag" ]]; then
     VERSION="${GITHUB_REF_NAME#v}"
     COMMIT="$(git rev-parse --short HEAD)"
     BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
     LDFLAGS_META="-X ${PKG}.Version=${VERSION} -X ${PKG}.Commit=${COMMIT} -X ${PKG}.BuildDate=${BUILD_DATE}"
   fi
   ```
2. Non-tag runs leave `LDFLAGS_META` empty → binaries keep `dev`/`none`/`unknown`.

**Validation**: shell is valid; on a non-tag ref `LDFLAGS_META` is empty.

### T009 — Inject into the non-windows build line

**Steps**:
1. Change the non-windows build to weave `${LDFLAGS_META}` into the existing ldflags:
   ```bash
   CGO_ENABLED=0 GOOS="${os}" GOARCH="${arch}" go build -trimpath \
     -ldflags="-s -w ${LDFLAGS_META}" -o "${stage}/spec-kitty-analyzer" ./cmd/spec-kitty-analyzer
   ```

**Validation**: quotes correct; a tag build embeds the version.

### T010 — Inject into the windows build line

**Steps**:
1. Apply the **identical** change to the windows `.exe` build line:
   ```bash
   CGO_ENABLED=0 GOOS="${os}" GOARCH="${arch}" go build -trimpath \
     -ldflags="-s -w ${LDFLAGS_META}" -o "${stage}/spec-kitty-analyzer.exe" ./cmd/spec-kitty-analyzer
   ```

**Validation**: both build lines carry `${LDFLAGS_META}` identically.

### T011 — Post-build verification (footgun guard)

**Purpose**: Fail the release if injection silently no-ops.

**Steps**:
1. After the build loop, on tag builds only, run the just-built host binary and assert the version is not `dev`:
   ```bash
   if [[ "${GITHUB_REF_TYPE}" == "tag" ]]; then
     out="$(dist/spec-kitty-analyzer_linux_amd64/spec-kitty-analyzer version 2>/dev/null || true)"
     echo "version output: ${out}"
     case "${out}" in
       *" dev "*|*"dev (commit none"*) echo "::error::ldflags injection failed — version still 'dev' (check the -X module path)"; exit 1;;
     esac
   fi
   ```
   (Adjust the path/target to whichever built artifact is convenient to execute on the linux runner.)

**Validation**: a correct injection passes; a broken `-X` path fails the job.

## Definition of Done

- [ ] Stamping vars computed only when `GITHUB_REF_TYPE == 'tag'`; empty otherwise.
- [ ] Both build lines (windows + non-windows) inject the three `-X` symbols with the **lowercase** module path.
- [ ] A tag build reports the tag-derived version/commit/date; a non-tag/manual run reports `dev`/`none`/`unknown`.
- [ ] Post-build assertion fails the job if a tag build still reports `dev`.
- [ ] YAML is valid (`actionlint`/`yamllint` if available, else careful review).

## Reviewer guidance

- Verify the `-X` path is the **lowercase** `priivacy-ai` module path — the single highest-risk detail (silent no-op).
- Verify the tag gate: a `workflow_dispatch` run must NOT stamp a branch name (INV-2 / C-006).
- Confirm both build lines were updated identically — a common miss.
