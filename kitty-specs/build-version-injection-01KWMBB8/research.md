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

## R3 — Value sources in CI, gated on tag pushes (Codex HIGH-1)

- **Decision**: Stamp only when the triggering ref is a tag. Guard the build step on `GITHUB_REF_TYPE == 'tag'`; when true, `Version = ${GITHUB_REF_NAME#v}` (strip leading `v`), `Commit = $(git rev-parse --short HEAD)`, `BuildDate = $(date -u +%Y-%m-%dT%H:%M:%SZ)`. When false (manual `workflow_dispatch`, branch run), inject nothing — the binary keeps the `dev`/`none`/`unknown` sentinels.
- **Rationale**: `release.yml` allows BOTH `on: push: tags: v*` AND `workflow_dispatch`. On a manual dispatch, `GITHUB_REF_NAME` is the *branch* name, so stamping unconditionally would brand a binary `main`/`<branch>` — a value that is neither a real release version nor the sentinel, violating INV-2 (C-006). Gating on `GITHUB_REF_TYPE` closes that hole while keeping manual dispatch available for re-runs. UTC ISO-8601 (C-004) is unambiguous and sortable.
- **Alternatives**: (a) remove `workflow_dispatch` entirely — simpler but loses a useful manual re-run path; rejected in favor of gating. (b) `git describe --tags` — redundant when the ref is already the tag; can append `-g<sha>` noise. Rejected.
- **Verification**: quickstart/CI check asserts a tagged build shows the injected version and a non-tag build shows `dev`.

## R4 — Local/dev default sentinels

- **Decision**: Defaults `Version="dev"`, `Commit="none"`, `BuildDate="unknown"`.
- **Rationale**: FR-004 requires local builds to be unmistakably non-release. `dev` is the widely recognized convention. No injection → these values surface as-is.

## R5 — JSON shape: nested `build`, top-level `version` removed (C1, breaking)

- **Decision**: Emit `"build": {"version","commit","build_date"}`; remove top-level `"version"`. Implement with a `Build` struct referenced (named field with `json:"build"`) — NOT embedded — so the object nests rather than promoting to top level.
- **Rationale**: Provenance is one cohesive thing → one object (deep-module-design). Named field (not Go embedding) produces the nested shape. C1 clean break chosen over additive C3 because 0.x + tiny known consumer set makes now the cheapest time to break; ships in 0.3.0 (breaking → minor bump per SemVer).
- **Alternatives**: flat sibling fields (A/B) — scatters provenance, triplicates fields; additive C3 (keep top-level `version` + `build:{commit,build_date}`) — non-breaking but splits version from its siblings. Both rejected per maintainer decision.

## R6 — Test strategy (specification-by-example), incl. existing-test updates (Codex MEDIUM-3)

- **Decision**: Unit tests assert (a) un-injected `CurrentBuild()` returns the sentinels; (b) marshaled JSON for `Report`, `missions`, AND `query` each contains a `build` object with the three fields AND has **no** top-level `version` key (guards the breaking contract in both directions); keep `go test ./...` green.
- **Existing tests that must be updated (not just added to)**: `internal/query/query_test.go` (~line 10) and `internal/analyzer/analyzer_test.go` (~line 245) currently construct/read `Version` directly — they will fail to compile once the field becomes `Build`, so NFR-001 requires migrating them to read `Build`. The `missions` output has no `cmd`-level test today; add one so all three JSON surfaces are covered.
- **Rationale**: Asserting the *absence* of top-level `version` is what actually protects FR-005; presence-of-`build` alone would pass even if the old field leaked. Covering all three emitters (including a new `missions` cmd test) prevents an inconsistent schema slipping through one un-tested surface.

## R7 — FR-006 breaking-change documentation scope + delivery (Codex HIGH-2)

- **Decision**: This mission produces a curated `docs/releases/release-notes-0.3.0.md` draft that explicitly carries the `.version` → `.build.version` migration note, plus the PR body callout. (Repo `docs/releases/`, not the mission dir — WP `owned_files` cannot live under `kitty-specs/`, and a release artifact belongs in the tree anyway.) At release time the note reaches the **published GitHub Release body** via the proven curated-notes swap (`gh release edit --notes-file …`, exactly as done for 0.2.0), NOT via the workflow's `generate_release_notes` auto-list. Do NOT introduce `CHANGELOG.md` or wire `body_path` into `release.yml` here.
- **Rationale**: Codex correctly flagged that `generate_release_notes: true` alone would let the breaking change ship without a warning in the published release. The curated-notes swap already guarantees a human-authored body at release time (proven on 0.2.0), so FR-006 is satisfied without new automation. Fully automating notes delivery (`body_path`, changelog extraction) is issue #20's design and would overlap it — explicitly deferred.
- **Release-time reminder**: the 0.3.0 release runbook MUST use `--notes-file release-notes-0.3.0.md`; relying on auto-notes would drop the breaking-change warning.

## R8 — JSON field ordering (Codex LOW-4)

- **Decision**: Make `Build` the **first field** in each emitter struct (`analyzer.Report`, the `missions` result struct, `query.QueryResult`), so the `build` object appears first in the marshaled JSON as the contract's examples show.
- **Rationale**: Go's `encoding/json` emits fields in struct-declaration order. `Build` cleanly replaces `Version`, which is already the first field in these structs, so this is the natural drop-in and keeps the contract examples accurate rather than aspirational.
