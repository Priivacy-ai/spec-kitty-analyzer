# Release checklist

How to cut a release of `spec-kitty-analyzer`. The version **source of truth is the git tag** —
there is no version file to bump. `release.yml` stamps the binary's `build.version` from the tag,
extracts the matching `CHANGELOG.md` section as the GitHub Release body, and refuses to publish
unless the binary, the tag, and the changelog section all agree.

## What the automation guarantees

On a `v*` tag push, `release.yml` proves **triple consistency** before publishing:

> built binary `build.version`  ==  the git tag  ==  the extracted `CHANGELOG.md` section

Any disagreement fails the build — you cannot ship a release whose binary, tag, and notes disagree.
`release-readiness.yml` runs the same validator on release-metadata PRs and nightly.

## Before you start

- You are on `main`, up to date, working tree clean.
- All changes for this release are merged into `main`.
- Decide the version `X.Y.Z` per SemVer (this project stays in `0.x`; breaking JSON/CLI changes bump
  the minor while `0.x`).

## Steps

1. **Promote the changelog** (on a short release-prep branch, e.g. `release/X.Y.Z`):
   - In `CHANGELOG.md`, rename `## [Unreleased]` to `## [X.Y.Z] - YYYY-MM-DD` (today's date).
   - Add a fresh empty `## [Unreleased]` above it.
   - Add the bottom link-reference line:
     `[X.Y.Z]: https://github.com/priivacy-ai/spec-kitty-analyzer/compare/vPREV...vX.Y.Z`
     and update the `[Unreleased]` compare link to `vX.Y.Z...HEAD`.

2. **Validate locally** (same check CI runs on the PR):
   ```bash
   go run ./tools/release validate --mode branch
   # → release readiness OK: X.Y.Z (mode=branch, state=release-prep, latest tag=vPREV)
   ```
   Fix any reported issue before continuing (empty section, non-advancing version, malformed heading).

3. **Preview the exact Release body** that will be published:
   ```bash
   go run ./tools/release extract X.Y.Z
   ```

4. **Open the release-prep PR** into `main`. `release-readiness.yml` runs `validate --mode branch`
   and blocks merge if the section is missing/empty or the version doesn't advance. Merge it.

5. **Tag and push** from updated `main`:
   ```bash
   git checkout main && git pull
   git tag vX.Y.Z -m "Release X.Y.Z"
   git push origin vX.Y.Z
   ```

6. **Confirm the release**: `release.yml` runs on the tag. Verify in the Actions log that the
   "Release notes + triple-consistency guard" step printed
   `Triple-consistency OK: binary=X.Y.Z == tag=X.Y.Z == CHANGELOG [X.Y.Z]`, then check the published
   GitHub Release: its body is the curated `## [X.Y.Z]` section (no auto-generated commit list), and
   the six binaries + installers + `checksums.txt` are attached.

## Dry-run a prospective tag (optional)

Before tagging, you can validate tag-mode parity without pushing:
```bash
go run ./tools/release validate --mode tag --tag vX.Y.Z
```
or trigger the **Release Readiness** workflow manually (Actions → Run workflow) with the `tag` input.

## Troubleshooting

- **`tag vX.Y.Z does not match top released changelog version …`** — the tag and the top `## [X.Y.Z]`
  heading disagree. Fix the heading or the tag so they match.
- **`changelog section [X.Y.Z] is empty (not populated)`** — add real notes under the heading.
- **`version X.Y.Z does not advance beyond latest tag vPREV`** — pick a higher version.
- **`binary build.version (…) != tag (…)`** — the release build didn't stamp correctly; check the
  `-X …analyzer.Version` ldflags path in `release.yml` (the footgun guard also catches a `dev` binary).
- **`malformed changelog heading …`** — a `## [...]` heading is neither `## [Unreleased]` nor a valid
  `## [X.Y.Z]`. Fix the typo (e.g. `## [0.3]` → `## [0.3.0]`).

## Notes

- This is a scoped mirror of spec-kitty's release-notes mechanism. It intentionally omits the
  package-registry pieces that don't apply here (no `pyproject.toml`/`uv.lock` version sync, no PyPI
  publish, no shared-package/SaaS-contract checks) — the analyzer ships GitHub Releases only, with the
  git tag as the single version source of truth.
- Prerelease publishing (`rc`/`alpha`/`beta`) is out of scope for now; the validator parses those
  version forms but the flow ships stable `0.x` versions.
