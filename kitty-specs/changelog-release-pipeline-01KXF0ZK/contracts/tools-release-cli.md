# Contract: `tools/release` CLI

Standalone Go program (`package main`, module-internal path
`github.com/priivacy-ai/spec-kitty-analyzer/tools/release`). Invoked in CI and locally via
`go run ./tools/release <subcommand> ...`. Stdlib only. Not linked into the shipped binary.

## Global behavior
- Unknown subcommand or bad flags → usage to stderr, exit **2**.
- All normal diagnostics to stderr; primary output (extracted notes) to stdout.
- Reads `CHANGELOG.md` from the current working directory (repo root in CI).

## Subcommand: `extract`

```
tools/release extract <version>
```
- **Input**: `<version>` — canonical release version, e.g. `0.3.0` (no leading `v`).
- **Behavior**: prints the body of `## [<version>] - ...` (heading excluded; leading/trailing blank
  lines trimmed) to stdout.
- **Missing/empty section**: prints `Release <version>\n\nNo changelog entry found for this version.`
  to stdout.
- **Exit codes**: `0` always on a readable `CHANGELOG.md`; `1` if `CHANGELOG.md` is absent/unreadable;
  `2` on usage error (missing `<version>`).
- **Git**: not used. `extract` never shells out.

**Examples**
```
$ tools/release extract 0.3.0
### Added — build provenance (#19, #21)
- Structured `build` object. ...
⚠️ Breaking change — top-level `version` removed from JSON
...

$ tools/release extract 9.9.9
Release 9.9.9

No changelog entry found for this version.
```

## Subcommand: `validate`

```
tools/release validate --mode branch
tools/release validate --mode tag --tag v0.3.0
```
- **Flags**:
  - `--mode branch|tag` (required).
  - `--tag vX.Y.Z` (tag mode: optional; defaults to `$GITHUB_REF_NAME` when unset).
- **Checks** (see data-model validation-rules table):
  - Top released section parses as a valid Version and `IsPopulated`.
  - branch: version strictly `>` latest `v*.*.*` tag.
  - tag: `--tag` parity with top released version; version strictly `>` latest tag **excluding
    `--tag`**.
- **Output**: on success, a one-line summary to stderr (`release readiness OK: <version> (mode=...)`);
  on failure, one `- <issue>` line per problem to stderr, each naming the offending value.
- **Exit codes**: `0` on OK; `1` on any validation failure or missing `CHANGELOG.md`; `2` on usage
  error (bad/missing `--mode`, unparseable `--tag`).
- **Git**: `validate` runs `git tag --list 'v*.*.*'`. If not in a git work tree, it exits `1` with a
  clear message (validation cannot assert monotonicity without the tag set).

**Examples**
```
$ tools/release validate --mode branch
release readiness OK: 0.3.0 (mode=branch, latest tag=v0.2.0)          # exit 0

$ tools/release validate --mode tag --tag v0.3.0
release readiness OK: 0.3.0 (mode=tag)                                # exit 0

$ tools/release validate --mode tag --tag v0.2.0
- tag v0.2.0 does not match top released changelog version 0.3.0      # exit 1
```

## Consumed by

- `.github/workflows/release-readiness.yml` — `validate --mode branch` (PR/nightly),
  `validate --mode tag --tag <input>` (dispatch).
- `.github/workflows/release.yml` — `validate --mode tag --tag $GITHUB_REF_NAME` then
  `extract ${GITHUB_REF_NAME#v}` (tag builds only), plus the binary `build.version == tag` assertion.
