# Quickstart: Curated CHANGELOG & Release Notes Pipeline

## For contributors — updating the changelog in a PR
Add an entry under `## [Unreleased]` in `CHANGELOG.md` describing your user-facing change, grouped
`### Added` / `### Changed` / `### Fixed`, with the issue/PR ref. That's it — the readiness gate only
enforces a populated section at release time, not on every PR.

## For the maintainer — cutting a release (scoped, tag-as-SSOT)

```bash
# 1. Promote Unreleased → the new version, on a release-prep branch.
#    Rename "## [Unreleased]" to "## [X.Y.Z] - YYYY-MM-DD", open a fresh empty "## [Unreleased]",
#    and add the bottom link-ref line for X.Y.Z.

# 2. Validate locally before opening the PR (same check CI runs).
go run ./tools/release validate --mode branch
#   → "release readiness OK: X.Y.Z (mode=branch, latest tag=vX.Y.(Z-1))"

# 3. Preview the exact Release body that will be published.
go run ./tools/release extract X.Y.Z

# 4. Open the release-prep PR. release-readiness.yml runs `validate --mode branch` and blocks
#    merge if the section is missing/empty or the version doesn't advance.

# 5. After merge, tag and push from main.
git tag vX.Y.Z -m "Release X.Y.Z"
git push origin vX.Y.Z
#   → release.yml builds, runs `validate --mode tag`, asserts binary build.version == tag,
#     extracts the [X.Y.Z] section, and publishes it as the GitHub Release body. No manual paste.
```

## Verifying the tooling
```bash
go test ./tools/release          # unit tests: parsing, extraction, validation, prerelease, edges
go build ./...                   # tools/release compiles with the rest of the module
go run ./tools/release validate --mode tag --tag v0.3.0   # dry-run tag-mode parity check
```

## Triple-consistency (what release.yml guarantees)
On a tag build, before any Release is published, the workflow proves all three agree:
- **binary `build.version`** (ldflags-injected from the tag) `==`
- **the git tag** (`$GITHUB_REF_NAME`) `==`
- **the extracted `CHANGELOG.md` section** (`validate --mode tag` parity).

Any mismatch fails the build — you cannot ship a release whose binary, tag, and notes disagree.

## Dispatch / nightly checks
```bash
# Manual dry-run of tag-mode readiness for a prospective tag (Actions → Release Readiness → Run):
#   input tag = v0.3.0
# Nightly cron re-runs branch-mode validation to catch drift between releases.
```
