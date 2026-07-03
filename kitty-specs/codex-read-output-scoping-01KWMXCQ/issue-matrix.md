# Issue matrix — codex-read-output-scoping-01KWMXCQ

Per FR-037 of the spec-kitty-mission-review skill Gate-4. One row per issue referenced in spec.md.

| Issue | Title | Verdict | Evidence ref |
|-------|-------|---------|--------------|
| #13 | codex function_call_output read-content scanned as command output (false positives) | in-mission | Fixed across WP01–WP03 (classifier/envelope → gating → prepass); terminal at mission done, proven by WP04 corpus. |
| #11 | failure-scan follow-ups — remaining codex payload-type mapping | in-mission | Payload-type mapping (FR-006) landed in WP02 (T009); terminal at mission done. |
| #22 | multi-LLM / per-harness corpus strategy | deferred-with-followup | Out of scope here (spec C-004). Follow-up: #22 remains the tracking issue; corpus strategy planned in the tracer evidence-engine RFC #10. |

Valid `Verdict` values: `fixed`, `verified-already-fixed`, `deferred-with-followup`, `in-mission` (being fixed by a later WP in this mission; must reach a terminal verdict before mission `done`).
