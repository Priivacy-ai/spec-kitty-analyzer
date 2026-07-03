## spec-kitty-analyzer 0.3.0

Build-provenance release: binaries now self-report their **version, commit, and build date**, and release builds stamp real values automatically from the git tag (no more hand-edited version constant). Ships one **breaking change** to the JSON output schema — see below. Cross-platform installers and prebuilt binaries as before (`install.sh` / `install.ps1`; macOS/Linux/Windows, amd64 + arm64).

### Added — build provenance (#19, #21)
- **Structured `build` object.** The `version` command and every JSON report (`analyze`, `query`, `missions`) now expose a nested `build` object with `build.version`, `build.commit`, and `build.build_date`, so any binary is traceable to the exact commit that produced it.
- **`version` command shows all three:**
  ```
  spec-kitty-analyzer 0.3.0 (commit a1b2c3d, built 2026-07-03T18:00:00Z)
  ```
- **Automatic release stamping (#19).** Tagged release builds inject the version (from the tag), short commit, and UTC build date via linker flags — the version constant no longer has to be bumped by hand. Local/dev builds report `dev` / `none` / `unknown`, so a development build is never mistaken for a release.

### ⚠️ Breaking change — top-level `version` removed from JSON
The top-level `version` field is **removed** from the `analyze`, `query`, and `missions` JSON output. The analyzer version now lives at **`build.version`**.

Before (≤ 0.2.x):

```json
{
  "version": "0.2.0",
  "generated_at": "2026-07-03T18:00:00Z",
  "...": "..."
}
```

After (0.3.0+):

```json
{
  "build": {
    "version": "0.3.0",
    "commit": "a1b2c3d",
    "build_date": "2026-07-03T18:00:00Z"
  },
  "generated_at": "2026-07-03T18:00:00Z",
  "...": "..."
}
```

**Migration:** if your tooling reads the top-level `.version`, change it to **`.build.version`**. The new `.build.commit` and `.build.build_date` are additive.

This is a deliberate, one-time schema change made while the consumer set is still small; per SemVer it lands in a minor release (0.3.0), not a patch.

### Known limitations
Unchanged from 0.2.0 — detection is validated primarily against a macOS + Claude/Codex corpus (#6), codex file-read content can occasionally false-positive (#13), and novel failures are not yet surfaced (#15). See the README "Limitations" section.

**Full diff:** https://github.com/Priivacy-ai/spec-kitty-analyzer/compare/v0.2.0...v0.3.0

<!-- Release runbook: publish this file as the GitHub Release body via
     `gh release edit v0.3.0 --notes-file docs/releases/release-notes-0.3.0.md`.
     Do NOT rely on the workflow's auto-generated notes — they omit the breaking-change warning above (FR-006). -->
