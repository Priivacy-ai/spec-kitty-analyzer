# Research — Build Version & Metadata Injection

Phase 0 decisions. No `[NEEDS CLARIFICATION]` markers remain (all resolved with the maintainer during specify).

## R1 — Inject via `-ldflags -X` into package variables

- **Decision**: Declare `Version`, `Commit`, `BuildDate` as package-level `var` (not `const`) in `internal/analyzer`, and overwrite them at link time with `go build -ldflags="-X <pkg>.Version=..."`.
- **Rationale**: `-X` is the standard Go mechanism for stamping build metadata; it only works on string *variables*, so the existing `const` must become a `var`. Keeps zero runtime cost and no new dependency.
- **Alternatives considered**:
  - `runtime/debug.ReadBuildInfo()` (VCS stamping) — Go can auto-embed VCS revision/time with `-buildvcs`, but it does NOT carry the release *tag/version*, `CGO_ENABLED=0` + `-trimpath` interactions are fiddly, and it gives less control over exact fields/format. Rejected: doesn't cover version-from-tag, our primary requirement.
  - Generated `version.go` written by CI — more moving parts, commits noise, still needs a source edit. Rejected.

## R2 — Symbol path is the lowercase module path (the footgun)

- **Decision**: Use `-X github.com/priivacy-ai/spec-kitty-analyzer/internal/analyzer.Version=...` (lowercase `priivacy-ai`, matching `go.mod`).
- **Rationale**: `-X` silently no-ops if the symbol path is wrong — no error, no warning; the binary just keeps the default. The GitHub org is `Priivacy-ai` (capital P) but the Go module path is lowercase, an easy and invisible mismatch. Verified against `go.mod` line 1.
- **Verification**: A build check greps the `version` command output for the injected value; a mismatch (still `dev`) fails the check.

## R3 — Value sources in CI

- **Decision**: `Version = ${GITHUB_REF_NAME#v}` (strip leading `v`), `Commit = $(git rev-parse --short HEAD)`, `BuildDate = $(date -u +%Y-%m-%dT%H:%M:%SZ)`.
- **Rationale**: `GITHUB_REF_NAME` is the tag that triggered `release.yml` (`on: push: tags: v*`); stripping `v` gives a clean SemVer string matching `analyzer.Version`'s historical form. UTC ISO-8601 (C-004) is unambiguous and sortable.
- **Alternatives**: `git describe --tags` — redundant when the ref is already the tag; can append `-g<sha>` noise. Rejected.

## R4 — Local/dev default sentinels

- **Decision**: Defaults `Version="dev"`, `Commit="none"`, `BuildDate="unknown"`.
- **Rationale**: FR-004 requires local builds to be unmistakably non-release. `dev` is the widely recognized convention. No injection → these values surface as-is.

## R5 — JSON shape: nested `build`, top-level `version` removed (C1, breaking)

- **Decision**: Emit `"build": {"version","commit","build_date"}`; remove top-level `"version"`. Implement with a `Build` struct referenced (named field with `json:"build"`) — NOT embedded — so the object nests rather than promoting to top level.
- **Rationale**: Provenance is one cohesive thing → one object (deep-module-design). Named field (not Go embedding) produces the nested shape. C1 clean break chosen over additive C3 because 0.x + tiny known consumer set makes now the cheapest time to break; ships in 0.3.0 (breaking → minor bump per SemVer).
- **Alternatives**: flat sibling fields (A/B) — scatters provenance, triplicates fields; additive C3 (keep top-level `version` + `build:{commit,build_date}`) — non-breaking but splits version from its siblings. Both rejected per maintainer decision.

## R6 — Test strategy (specification-by-example)

- **Decision**: Unit tests assert (a) un-injected `CurrentBuild()` returns the sentinels; (b) marshaled JSON for `Report`/`missions`/`query` contains a `build` object with the three fields AND has **no** top-level `version` key (guards the breaking contract in both directions); keep `go test ./...` green.
- **Rationale**: Asserting the *absence* of top-level `version` is what actually protects FR-005; presence-of-`build` alone would pass even if the old field leaked.

## R7 — FR-006 breaking-change documentation scope

- **Decision**: Satisfy FR-006 via the PR body's breaking-change callout plus a short `release-notes-0.3.0` draft kept in the mission dir. Do NOT introduce `CHANGELOG.md` here.
- **Rationale**: Formal Keep-a-Changelog adoption is its own mission (issue #20); creating `CHANGELOG.md` now would overlap and pre-empt that design. This mission documents the break where it lands (PR + release notes) without claiming the changelog-automation work.
