# Changelog

All notable changes to `spec-kitty-analyzer` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

The section for the version being released is published verbatim as the GitHub Release body
by the release workflow (see `RELEASE_CHECKLIST.md`).

## [Unreleased]

### Fixed

- **Codex `sed -n` reads no longer leak file content into failure scanning (#37).** A read-only
  `sed` (e.g. `sed -n 'M,Np' file`) is now recognized as an inspection read, so failure/crash tokens
  *inside* a file that codex dumps via `sed` no longer produce false detections — across any tier.
  Any mutating/writing/executing `sed` form (`-i`, `w`/`r`/`e`, `s///w`, …) is still scanned.

## [0.3.0] - 2026-07-14

Build-provenance, plus two new analysis surfaces. Binaries now self-report their **version, commit,
and build date**, stamped automatically from the git tag at release (no more hand-edited version
constant) — with one **breaking change** to the JSON schema (see below). This release also adds a
**Tier-3 unclassified-anomaly** channel and an opt-in **spec-kitty-go governance-activity** view.

### Added

- **Structured `build` object (#19, #21).** The `version` command and every JSON report (`analyze`,
  `query`, `missions`) now expose a nested `build` object with `build.version`, `build.commit`, and
  `build.build_date`, so any binary is traceable to the exact commit that produced it.
- **`version` command shows all three:** `spec-kitty-analyzer 0.3.0 (commit a1b2c3d, built 2026-07-03T18:00:00Z)`.
- **Automatic release stamping (#19).** Tagged release builds inject the version (from the tag),
  short commit, and UTC build date via linker flags — the version constant no longer has to be
  bumped by hand. Local/dev builds report `dev` / `none` / `unknown`, so a development build is
  never mistaken for a release.
- **Tier-3 unclassified-anomaly trap (#15).** Reports now include a segregated `anomalies`
  collection that surfaces output/structured distress signals matching no failure fingerprint — a
  non-zero structured `exit_status` and the crash signatures `panic:` / `segmentation fault` /
  `core dumped` — for triage, without ever counting them as confirmed failures or inflating failure
  totals. Anomalies group by a stable signature hash and can be suppressed via a checked-in ignore
  registry (promote → refine → ignore). Shown in the JSON, Markdown, HTML, and PDF reports.
- **spec-kitty-go governance activity (#29).** A new opt-in view — the `go-activity` command, or
  `--include go` on `analyze` — reconstructs what the spec-kitty-go binary did from harness
  transcripts: governance-hook verdicts (ADMIT / DENY / DECISION_REQUIRED), direct CLI verb usage,
  derived ledger activity, and hook latency. A host-blocked action whose typed verdict Claude
  discarded (hook exit 2) is surfaced as `UNRESOLVED` rather than silently counted as ADMIT, so the
  report never fabricates an all-admit summary. Contributed by Robert Douglass.

### Changed

- **⚠️ BREAKING — top-level `version` removed from JSON.** The top-level `version` field is removed
  from the `analyze`, `query`, and `missions` JSON output; the analyzer version now lives at
  **`build.version`**.

  Before (≤ 0.2.x):
  ```json
  { "version": "0.2.0", "generated_at": "...", "...": "..." }
  ```
  After (0.3.0+):
  ```json
  { "build": { "version": "0.3.0", "commit": "a1b2c3d", "build_date": "..." }, "generated_at": "...", "...": "..." }
  ```

  **Migration:** if your tooling reads the top-level `.version`, change it to **`.build.version`**.
  The new `.build.commit` and `.build.build_date` are additive. This is a deliberate, one-time
  schema change made while the consumer set is still small; per SemVer it lands in a minor release
  (0.3.0), not a patch.

### Internal

- **Curated CHANGELOG + release-notes pipeline (#20).** This `CHANGELOG.md` is now the source of
  record for release notes: the `release` workflow extracts the tagged section as the GitHub
  Release body and asserts binary/tag/changelog consistency before publishing, and a
  release-readiness check validates changelog metadata on PRs. See `RELEASE_CHECKLIST.md`.

## [0.2.0] - 2026-07-03

Detection-quality release: sharply fewer false positives, plus new real-failure coverage.

### Improved

- **Channel-scoped failure detection (#4).** Rules now match real command/tool output and structured
  error fields, not narrative *discussion* of a problem or file/diff content that merely mentions an
  error phrase. ~46% fewer false-positive detections across the validation corpus, with no loss of
  the distinctive real-failure signatures.
- **`typer_usage_error` tightened (#11 C).** The bare `exit code 2` proxy now requires a usage-error
  companion token, removing false positives from source/docs that merely contain the phrase.
- **`permission_denied` precision + Windows coverage (#2, #5).** Anchored to real denial signatures
  (`token: permission denied`, `[Errno 13]`, `os error 13`) instead of a broad phrase match — plus
  Windows-native forms (`[WinError 5]`, `Access is denied (os error 5)`).

### Added

- **Structural `review_rejected` from status events (#11 A).** Rejections recorded as bare
  `review_status` / `evidence.review.verdict` fields in `status.events.jsonl` are now detected
  (source-kind-gated to the mission event log).
- **Codex `payload.type` mapping (#11 F).** `agent_message`, `task_complete`, and `token_count`
  codex payloads are now routed correctly instead of logged as unmapped.
- **EPERM / errno-1 "Operation not permitted" denials (#6).** macOS sandbox/TCC, git lock-file, and
  Node/Python EPERM denials are now recognized under `permission_denied`.

### Internal

- Go quality-gate CI (build · vet · gofmt · `go test -race`).

## [0.1.1] - 2026-06-20

### Fixed

- Correct checksum generation in the release packaging step so published archive checksums match
  the uploaded assets.

## [0.1.0] - 2026-06-20

### Added

- Initial release of `spec-kitty-analyzer`: a Go CLI that turns spec-kitty mission logs into a
  deterministic, human-readable story of what happened, with failure-mode detection.
- Mission-log discovery with a local cache.
- Agent-facing query API for structured report access.
- Cross-platform release installers (`install.sh` / `install.ps1`) and prebuilt binaries
  (macOS/Linux/Windows, amd64 + arm64).

[Unreleased]: https://github.com/priivacy-ai/spec-kitty-analyzer/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/priivacy-ai/spec-kitty-analyzer/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/priivacy-ai/spec-kitty-analyzer/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/priivacy-ai/spec-kitty-analyzer/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/priivacy-ai/spec-kitty-analyzer/releases/tag/v0.1.0
