# Issue matrix — tier3-anomaly-trap-01KXF8KZ

Per FR-037 of the spec-kitty-mission-review skill Gate-4. One row per issue referenced in spec.md.

| Issue | Title | Verdict | Evidence ref |
|-------|-------|---------|--------------|
| #11 | Fast-follows for #4 (structural review_rejected, whitelist-by-kind, codex payload mapping, single-walk extraction) | verified-already-fixed | CLOSED; fast-follows shipped (PRs #12/#14). Tier-3 was item E, split out into #15 and delivered by THIS mission. Referenced here only as lineage. |
| #13 | Codex function_call_output file-read/grep content scanned as command output → false-positive failure detections | verified-already-fixed | CLOSED; shipped `d4aae87` (PR #28). Tier-3 consumes the post-#13 channel model (research.md "Interaction with #13"); no re-introduction of read-content scanning. |
| #15 | Tier-3 unclassified-anomaly trap: recall preservation for novel failure modes (self-improving) | in-mission | THE mission target. WP01 (module) approved; WP02 (wiring) + WP03 (corpus+docs) pending. Reaches terminal `fixed` at mission done. |
| #23 | feat!: build version & metadata injection via ldflags (nested `build` JSON) | verified-already-fixed | MERGED. Referenced re the report-version decision: #23 removed the top-level `version` field, so Tier-3 adds `Anomalies` as a purely additive field with no schema-version bump (research.md D3). |

Valid `Verdict` values: `fixed`, `verified-already-fixed`, `deferred-with-followup`, `in-mission` (being fixed by a later WP in this mission; must reach a terminal verdict before mission `done`).
