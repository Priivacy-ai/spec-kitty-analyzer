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

**Decision**: Accept `X.Y.Z` and prerelease `X.Y.Z(a|b|rc)N` plus the dotted `X.Y.Z-rc.N` spelling;
canonicalize the dotted form to the compact one for comparison. Order by tuple
`(major, minor, patch, stage_rank, stage_num)` with `stage_rank = {a:0, b:1, rc:2, stable:3}` so
`0.4.0rc1 < 0.4.0`. This is spec-kitty's `parse_release_version` ranking.
**Rationale**: Future-proofs prereleases at ~zero cost; matches the reference tool exactly.
**Alternatives**: Stable-only parsing (rejected per confirmed decision — diverges from spec-kitty,
needs rework for any future rc).

## R3 — Validator check set and modes (scoped)

**Decision**: `validate --mode branch|tag [--tag vX.Y.Z]`:
- **Common**: locate the top **released** heading in `CHANGELOG.md` (the first `## [X.Y.Z]` that is
  not `[Unreleased]`); assert it is well-formed SemVer and its section is **populated** (has ≥1
  non-blank line before the next heading).
- **Branch mode**: assert that version is **strictly greater** than the latest existing
  `v*.*.*` tag (monotonic). No tag exists for it yet.
- **Tag mode**: assert `--tag` (or `$GITHUB_REF_NAME`) equals `v` + that top released version
  (parity); assert monotonic **excluding the current tag** from the existing-tag set (the tag is
  already pushed when `release.yml` runs, so it must be excluded or the check falsely fails).

**Rationale**: This is spec-kitty's `run_validation` minus the file-sync checks that have no
analogue (pyproject/uv.lock/metadata). The `exclude` parameter on `discover_release_tags`
(verified in `validate_release.py:495`) is exactly how spec-kitty avoids the self-comparison in tag
mode — we mirror it.
**Alternatives**: `>=` in tag mode (rejected — masks a real "forgot to bump" error on branch PRs;
cleaner to exclude-self and keep strict `>`).

## R4 — Triple-consistency at release time (FR-009)

**Decision**: In `release.yml`, on **tag builds only**, after the cross-build:
1. `go run ./tools/release validate --mode tag --tag "${GITHUB_REF_NAME}"` — enforces
   **tag == CHANGELOG top section** (+ populated + monotonic-excluding-self). Fail build on error.
2. `bin_ver="$(dist/spec-kitty-analyzer_linux_amd64/spec-kitty-analyzer version | awk '{print $2}')"`
   then assert `bin_ver == "${GITHUB_REF_NAME#v}"` — enforces **binary build.version == tag**.
   (Extends the existing dev-sentinel footgun guard, which already parses this text output.)
3. `go run ./tools/release extract "${GITHUB_REF_NAME#v}" > dist/RELEASE_NOTES.md` and publish via
   `body_path: dist/RELEASE_NOTES.md`.

Because step 1 guarantees the section exists and matches the tag, and step 2 ties the binary to the
tag, the three loci (binary `build.version`, tag, CHANGELOG section) are all proven equal before any
Release is published.

**Rationale**: `version` prints text `spec-kitty-analyzer <ver> (commit <c>, built <d>)` (verified
`cmd/spec-kitty-analyzer/main.go:41`); field 2 is `build.version`. No `--json` on `version`, so
`awk '{print $2}'` is the simplest machine read and matches the existing guard's approach.
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
tuple descending, take `[0]`. In tag mode pass `exclude = $GITHUB_REF_NAME`.
**Rationale**: Mirrors `discover_release_tags`; robust to lexical-vs-numeric sort pitfalls (tuple
sort, not string sort).
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

## Open questions

None. All Technical Context unknowns resolved; no `[NEEDS CLARIFICATION]` markers remain.
