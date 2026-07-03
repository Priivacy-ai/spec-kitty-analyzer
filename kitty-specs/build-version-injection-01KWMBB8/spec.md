# Build Version & Metadata Injection

**Mission**: build-version-injection-01KWMBB8
**Type**: software-dev
**Purpose (TL;DR)**: Binaries report their real version, commit, and build date from the release build instead of a hand-edited constant.

## Overview

Today the analyzer's version is a hardcoded Go constant (`const Version` in `internal/analyzer/types.go`) that a maintainer must manually edit to match each release tag. Shipped binaries carry no record of the commit or date they were built from, so a binary in the field cannot be traced back to the source that produced it. When the manual edit is forgotten or wrong, a published binary self-reports a version that disagrees with its own tag, and nothing catches it.

This mission makes build provenance authoritative and automatic: the release build injects the version (from the git tag), the commit SHA, and the build date into each binary via linker flags, and the binary surfaces all three — in the `version` command and as a structured `build` object in JSON output. Local (non-release) builds report explicit sentinel values so a dev build is never mistaken for a release.

Closes #19 (version from tag, not a drift-prone constant) and #21 (emit commit + build date alongside version).

## User Scenarios & Testing

**Primary actor**: A maintainer or teammate operating the analyzer CLI, and machine tooling that consumes the analyzer's JSON output.

### Scenario 1 — Cutting a release (happy path)
1. The maintainer pushes a release tag (e.g. `v0.2.1`).
2. The release build produces binaries for all six targets.
3. Running `spec-kitty-analyzer version` on any produced binary prints the release version, the short commit it was built from, and the build timestamp.
4. No Go source was edited to set the version.

### Scenario 2 — Tracing a binary to its source
1. A teammate has a binary and needs to know exactly which commit it came from.
2. They run the `version` command (or read the `build` object in any JSON report).
3. The output identifies the exact commit and build date — enough to check out that source.

### Scenario 3 — Local development build (exception path)
1. A developer runs `go build` / `go run` locally with no linker injection.
2. The binary reports version `dev`, commit `none`, build date `unknown`.
3. The dev build is unambiguously not a release.

### Scenario 4 — Machine consumer reads build provenance (breaking change)
1. A tooling consumer parses an analyzer JSON report.
2. Build provenance is available as a nested `build` object: `build.version`, `build.commit`, `build.build_date`.
3. The previously top-level `version` field is **no longer present at the top level** — consumers must read `build.version`. This is a breaking output-schema change and is documented as such in the release notes.

### Rule / invariant that must always hold
- A binary's reported version always reflects how it was built: a tagged release build reports the tag; any other build reports the `dev` sentinel. There is no path by which a non-release build reports a real release version, and no manual step is required for a release build to report the correct one.

## Requirements

### Functional Requirements

| ID | Requirement | Status |
|----|-------------|--------|
| FR-001 | The `version` command reports the running binary's version, commit identifier, and build date in a single human-readable line. | Draft |
| FR-002 | JSON output from the `analyze`, `query`, and `missions` commands exposes build provenance as a nested `build` object containing `version`, `commit`, and `build_date`. | Draft |
| FR-003 | A binary produced by the tagged release build reports the release version derived from the git tag, the commit it was built from, and the build timestamp. | Draft |
| FR-004 | A binary produced by any non-release (local) build reports sentinel values: version `dev`, commit `none`, build date `unknown`. | Draft |
| FR-005 | The top-level `version` field is removed from all JSON output; version is available only under the `build` object. | Draft |
| FR-006 | The release notes / changelog for the shipping release document the JSON schema change (removal of top-level `version`) as a breaking change. | Draft |

### Non-Functional Requirements

| ID | Requirement | Threshold / Measure | Status |
|----|-------------|---------------------|--------|
| NFR-001 | The existing automated test suite continues to pass with no regressions. | `go test ./...` exits 0 with zero failing tests. | Draft |
| NFR-002 | Producing a release requires no manual edit to Go source to set the version. | Number of Go source edits required per release to set version = 0 (down from 1 today). | Draft |
| NFR-003 | Reported build provenance is accurate. | The commit reported by a built binary equals the commit the binary was built from; the version equals the tag (minus leading `v`). | Draft |

### Constraints

| ID | Constraint | Status |
|----|-----------|--------|
| C-001 | Version, commit, and build date are injected at build time via linker flags (`-ldflags -X`); the source declares them as overridable package variables with the sentinel defaults, not as constants. | Draft |
| C-002 | The linker symbol path must use the actual lowercase module path `github.com/priivacy-ai/spec-kitty-analyzer/internal/analyzer`; an incorrect path silently fails to inject (no error) and must be avoided/verified. | Draft |
| C-003 | Injection is applied to every release target — both the Windows and the non-Windows build invocations in the release workflow (all six OS/arch packages). | Draft |
| C-004 | The build date is recorded in UTC ISO-8601 (e.g. `2026-07-03T18:00:00Z`). | Draft |
| C-005 | This is a breaking change to the JSON output schema. It ships under a version bump with a changelog breaking-change note. Target release: **0.3.0** — a breaking change to the public JSON output contract warrants a minor bump under SemVer (patch is reserved for backwards-compatible fixes). Decision recorded per DIRECTIVE_003; the small, known consumer set makes the break low-risk to execute now. | Draft |

## Success Criteria

| ID | Criterion |
|----|-----------|
| SC-001 | 100% of tagged release binaries report a version equal to the release tag (minus the leading `v`). |
| SC-002 | A teammate can identify the exact commit a binary was built from using only the binary's own output, in a single command. |
| SC-003 | Cutting a release requires zero manual edits to Go source to set the version (down from one edit per release). |
| SC-004 | Local build output unambiguously identifies itself as a non-release build. |

## Key Entities

- **Build** — the provenance of a compiled binary: its `version`, the `commit` it was built from, and its `build_date`. Modeled as a single cohesive object and surfaced as the `build` object in JSON.

## Assumptions

- Release builds run in CI where the git tag and commit are available to the build step.
- The analyzer's report JSON is currently consumed by at most a small number of internal tooling consumers (maintainer + a couple of teammates); the breaking removal of the top-level `version` field is acceptable given this small, known blast radius (maintainer's call, recorded in C-005).
- The `version` field currently emitted at the top level of reports carries the analyzer's version; moving it under `build` preserves that meaning while grouping it with the new provenance fields.
