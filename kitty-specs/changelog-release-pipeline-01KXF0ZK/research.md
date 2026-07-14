# Phase 0 Research: Curated CHANGELOG & Release Notes Pipeline

All unknowns from the plan's Technical Context resolved below. Each item follows
Decision / Rationale / Alternatives.

## R1 — CHANGELOG heading grammar (the parser ↔ author contract)

**Decision**: Adopt Keep a Changelog headings of the exact form:
- `## [Unreleased]`
- `## [X.Y.Z] - YYYY-MM-DD` (released sections; prerelease `X.Y.ZrcN` etc. allowed but unused now)
- Bottom link-reference lines `[X.Y.Z]: https://github.com/priivacy-ai/spec-kitty-analyzer/compare/v...`

The parser recognizes a **version heading** only when a line matches
`^##\s+\[(?P<ver>...)\]\s*(?:-\s*\S.*)?$` where `<ver>` is `Unreleased` (case-insensitive) or a
release version. A link-reference line (`[X.Y.Z]: http...`) starts with a single `[` **without**
the leading `## ` and is therefore never matched. This mirrors spec-kitty's `CHANGELOG_HEADING_RE`
(`extract_changelog.py`) scoped to the bracketed Keep-a-Changelog spelling this repo will use.

**Rationale**: Mirrors the proven spec-kitty regex; the `## ` + `[` prefix unambiguously separates
headings from link refs and from prose that merely mentions a version.
**Alternatives**: A looser `^## ` match (rejected — would catch `## Added` subsection headings if we
used them; we keep group headings as `###`/bold to avoid ambiguity, and require the bracket form).

## R2 — Version grammar (stable + prerelease) and ordering

**Decision**: Accept `X.Y.Z` and **compact** prerelease `X.Y.Z(a|b|rc)N` only. Do NOT accept the
dotted `X.Y.Z-rc.N` spelling. Order by tuple `(major, minor, patch, stage_rank, stage_num)` with
`stage_rank = {a:0, b:1, rc:2, stable:3}` so `0.4.0rc1 < 0.4.0`. This is spec-kitty's
`parse_release_version` grammar and ranking, verbatim.
**Rationale**: (post-plan Codex R9) The reference tool's regex accepts compact only; supporting a
second dotted spelling would force a canonical-equivalence comparison in tag parity for zero benefit
while prerelease publishing is out of scope. Mirroring compact-only keeps parity a simple string
compare of canonical forms and stays faithful to the reference.
**Alternatives**: Accept dotted too and canonicalize (rejected — added ambiguity, no use case);
stable-only parsing (rejected per confirmed decision — needs rework for any future rc).

## R3 — Validator check set and modes (scoped)

**Decision**: `validate --mode branch|tag [--tag vX.Y.Z]`:
- **Common**: locate the top **released** heading in `CHANGELOG.md` (the first `## [X.Y.Z]` that is
  not `[Unreleased]`); assert it is well-formed SemVer and its section is **populated** (has ≥1
  non-blank line before the next heading). A bracketed `## [...]` heading whose content is neither
  `Unreleased` nor a valid version is a **hard error** (see R10), never silently skipped.
- **Branch mode (state-aware monotonicity)**: compare the top released version `V` to the latest
  existing `v*.*.*` tag `T`:
  - `V > T` → **release-prep** state: OK (a new version is being prepared).
  - `V == T` → **inter-release** state: OK, no monotonic failure (routine PRs add to `[Unreleased]`
    without promoting a version; the top released section legitimately equals the last tag).
  - `V < T` → **error**: the changelog's top released version is behind the published tags.
- **Tag mode**: assert `--tag` (or `$GITHUB_REF_NAME`) equals `v` + that top released version
  (parity); assert `V` is **strictly greater** than the latest tag **excluding the tag under
  release** from the set (the tag is already pushed when `release.yml` runs, so it must be excluded
  or the check falsely fails).

**Rationale**: This is spec-kitty's `run_validation` minus the file-sync checks that have no
analogue. **(post-plan Codex R4 — HIGH)** The earlier "branch mode always requires `V > T`" was wrong:
after `v0.3.0` ships, the top released section is `[0.3.0]` and `T = v0.3.0`, so every routine PR
adding `[Unreleased]` entries would fail. spec-kitty avoids exactly this — its readiness workflow
only runs the monotonic check when the version source is actually bumped (`version_bump` scope),
and downgrades other release-path PRs to a consistency-only check that explicitly does *not* verify
progression (`release-readiness.yml:106`). Our tag-as-SSOT analogue is the `V==T` inter-release
carve-out. The `exclude` parameter on `discover_release_tags` (`validate_release.py:495`) is how
spec-kitty avoids self-comparison in tag mode — mirrored.
**Alternatives**: `>=` everywhere (rejected — masks a genuine "forgot to bump" on a release-prep PR);
always-strict `>` (rejected — the finding above; breaks routine PRs).

## R4 — Triple-consistency at release time (FR-009)

**Decision**: In `release.yml`, on **tag builds only**, after the cross-build:
1. `go run ./tools/release validate --mode tag --tag "${GITHUB_REF_NAME}"` — enforces
   **tag == CHANGELOG top section** (+ populated + monotonic-excluding-self). Fail build on error.
2. `bin_ver="$(dist/spec-kitty-analyzer_linux_amd64/spec-kitty-analyzer version | awk '{print $2}')"`
   then **fail closed if `bin_ver` is empty or not version-shaped** (guards against the `version`
   output format drifting — Codex R7), and assert `bin_ver == "${GITHUB_REF_NAME#v}"` — enforces
   **binary build.version == tag**. (Extends the existing dev-sentinel footgun guard.)
3. `go run ./tools/release extract "${GITHUB_REF_NAME#v}" > dist/RELEASE_NOTES.md` and publish via
   `body_path: dist/RELEASE_NOTES.md`.

**Release-body file always exists (Codex R3 — HIGH).** The `action-gh-release` upload step runs
unconditionally in the current workflow. So `dist/RELEASE_NOTES.md` MUST be created in *all* build
paths: on a tag build it is the extracted section; on a non-tag `workflow_dispatch` build it is
created empty (and steps 1–3 above are skipped). Otherwise `body_path` would point at a missing file
and fail the dispatch run.

Because step 1 guarantees the section exists and matches the tag, and step 2 ties the binary to the
tag, the three loci (binary `build.version`, tag, CHANGELOG section) are all proven equal before any
Release is published. A missing/empty matching section makes step 1 fail and the build stops — there
is no "publish a default body anyway" path (resolves the spec self-contradiction, Codex HIGH #1).

**Rationale**: `version` prints text `spec-kitty-analyzer <ver> (commit <c>, built <d>)` (verified
`cmd/spec-kitty-analyzer/main.go:41`); field 2 is `build.version`. No `--json` on `version`, so
`awk '{print $2}'` is the simplest machine read and matches the existing guard's approach; the
fail-closed shape check bounds its brittleness.
**Alternatives**: Parse `analyze --json` `.build.version` (rejected — needs a mission argument and
`jq`; heavier for identical information). Add a `version --json` flag (rejected — out of scope; a
separate product change, not release infra).

## R5 — Extractor contract (missing-section behavior)

**Decision**: `extract <version>` prints the section body (heading excluded, leading/trailing blank
lines trimmed) to stdout and exits 0. If no populated section exists it prints a one-line default
(`Release <version>\n\nNo changelog entry found for this version.`) and exits 0 — matching
spec-kitty. `extract` reads only `CHANGELOG.md`; it does **not** call git (so it cross-compiles and
runs anywhere). Release-time correctness is guaranteed by the preceding `validate --mode tag`, not by
`extract` failing.
**Rationale**: Keeps `extract` a pure text function (trivially testable, no git dependency); the
validator owns the gating.
**Alternatives**: `extract` exits non-zero on missing (rejected — would make the Release body step
brittle; the validator already gates, and a non-fatal default body is a safer fallback).

## R6 — Latest-tag discovery

**Decision**: `git tag --list 'v*.*.*'`, filter to `v` + valid release version, sort by the R2
tuple descending, take `[0]`. In tag mode pass `exclude = $GITHUB_REF_NAME`. **Both workflows that
run `validate` MUST check out with `fetch-depth: 0`** (Codex R2 — HIGH) so the local tag set is
complete; the reference `release-readiness.yml` does this. A shallow checkout can omit tags and make
the monotonic check meaningless. (The existing `release.yml` uses default checkout depth today — the
mission adds `fetch-depth: 0` to it; captured as FR-010.)
**Rationale**: Mirrors `discover_release_tags`; robust to lexical-vs-numeric sort pitfalls (tuple
sort, not string sort); complete tag set is a correctness precondition.
**Alternatives**: `git tag --sort=-version:refname | head -1` (rejected for the validator — git's
version sort handles prerelease ordering differently than PEP440/our rank; do the tuple sort in Go
for determinism. The RELEASE_CHECKLIST may still show the git one-liner as a human convenience.)

## R7 — Seed data for the CHANGELOG

**Decision**: Seed released sections with dates from the tags (verified via `git log -1 --format=%cs`):
`0.1.0` → 2026-06-20, `0.1.1` → 2026-06-20, `0.2.0` → 2026-07-03. `0.3.0` date is the unreleased
placeholder (the section carries the curated notes; the date is set when v0.3.0 is cut). Content:
- `0.3.0` ← `docs/releases/release-notes-0.3.0.md` (build-provenance + the breaking top-level
  `version` → `build.version` notice), re-rendered into Added/Changed groups.
- `0.2.0` ← the curated `v0.2.0` GitHub Release body (Improved/Added/Internal/Known limitations).
- `0.1.1` / `0.1.0` ← reconstructed concisely from git history (initial analyzer + early fixes);
  these predate curated notes, so keep them short and factual.
**Rationale**: Uses the real curated sources of record; no invented content.
**Alternatives**: Reconstruct 0.2.0 from commits (rejected — the curated Release body is the better
source and already exists).

## R8 — PR #30 overlap (release.yml)

**Decision**: PR #30 edits the `actions/*` version pins (Node-24 bumps) near the top of
`release.yml`; this mission edits only the final "Upload release" step (remove
`generate_release_notes`, add `body_path`) and inserts the extractor/guard steps in the build job —
disjoint lines. Land order per Kent: rebase this branch on `main` after #30 merges. If a conflict
still arises it will be a trivial pin-vs-step hunk.
**Rationale**: Verified the two edit regions do not overlap.
**Alternatives**: Block on #30 first (unnecessary — disjoint; rebase is cheap).

## R10 — Malformed heading is an error, not a skip (Codex R5 — MEDIUM)

**Decision**: The parser recognizes a **candidate** heading with a broad match `^##\s+\[(.+?)\]`,
then classifies the bracket content: `Unreleased` (sentinel), a valid version (released section),
or **neither → hard error** naming the offending heading. A malformed heading like `## [0.3]` or
`## [v0.3.0]` therefore fails validation loudly instead of being silently skipped (which could let a
typo'd top heading hide the real top section and make the validator reason about the wrong version).
Link-reference lines still never match (they lack the `## ` prefix).
**Rationale**: Silent skip is a correctness trap; a loud error is cheap and safe.
**Alternatives**: Strict-match-and-skip (rejected — the finding: wrong-section reasoning).

## R11 — Cross-build coverage for tools/release (Codex R6 — MEDIUM)

**Decision**: `tools/release` is pure portable Go (stdlib, no build tags, no cgo, no GOOS-specific
code), so building it on the runner platform is strong evidence it builds everywhere. To make
NFR-002 literally verified rather than assumed, extend the existing `ci.yml` "Release cross-build
smoke" loop to also `go build ./tools/release` for each of the six GOOS/GOARCH targets (it currently
builds only `./cmd/spec-kitty-analyzer`). Cheap (a second `go build` per target).
**Rationale**: The plan's original claim ("the six-target smoke covers tools/release") was false as
written — `go build ./...` only covers the runner platform. Extending the smoke makes it true.
**Alternatives**: Relax NFR-002 to runner-only (rejected — the extension is trivial and the
guarantee is worth having); leave as-is (rejected — overclaim).

## Open questions

None. All Technical Context unknowns resolved; no `[NEEDS CLARIFICATION]` markers remain.
Post-plan Codex review folded in (R2/R3 grammar & mode semantics, R4 body_path/fetch-depth/awk
guard, R10 malformed-heading error, R11 cross-build) — see the review log in the mission scratchpad.
