# Issue matrix — build-version-injection-01KWMBB8

Per FR-037 of the spec-kitty-mission-review skill Gate-4. One row per issue referenced in spec.md.

| Issue | Title | Verdict | Evidence ref |
|-------|-------|---------|--------------|
| #19 | Inject version from git tag via ldflags, not a hardcoded const | in-mission | WP01 vars+Build+version cmd (da2c1f0, 2cac40b); tag injection lands in WP02 |
| #21 | Emit commit SHA + build date alongside version in all binaries | in-mission | WP01 Build{commit,build_date} surfaced in CLI+JSON (da2c1f0); real values injected in WP02 |
| #20 | Adopt spec-kitty CHANGELOG + release-notes automation | deferred-with-followup | Out of scope by design (research R7); WP03 ships an interim curated 0.3.0 note, #20 remains the follow-up for automation |

Valid `Verdict` values: `fixed`, `verified-already-fixed`, `deferred-with-followup`, `in-mission` (being fixed by a later WP in this mission; must reach a terminal verdict before mission `done`).
