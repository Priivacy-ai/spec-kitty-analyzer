# Issue matrix — changelog-release-pipeline-01KXF0ZK

Per FR-037 of the spec-kitty-mission-review skill Gate-4. One row per issue referenced in spec.md.

| Issue | Title | Verdict | Evidence ref |
|-------|-------|---------|--------------|
| #20 | release: adopt spec-kitty's curated CHANGELOG + release-notes automation | in-mission | This mission (changelog-release-pipeline-01KXF0ZK) implements #20; terminal `fixed` at mission `done`. |
| #23 | build version & metadata injection via ldflags (nested `build` JSON) | verified-already-fixed | Already merged (commit 9a6f962). Referenced only as the tag-as-SSOT / `build.version` foundation the triple-consistency check (FR-009) leverages; not re-fixed here. |
| #30 | ci: bump CI/release actions to Node 24 majors | deferred-with-followup | Independent in-flight PR #30, not addressed by this mission. Follow-up: rebase this branch's `release.yml` edits onto main after #30 merges (constraint C-005). |

Valid `Verdict` values: `fixed`, `verified-already-fixed`, `deferred-with-followup`, `in-mission` (being fixed by a later WP in this mission; must reach a terminal verdict before mission `done`).
