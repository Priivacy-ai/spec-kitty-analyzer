# Design: per-pattern channel scoping for failure detection (issue #4)

> **Status:** READY TO IMPLEMENT. Codex design-review v1 (*needs-rework*, 1C+4M+1m) →
> all findings folded in + the scope debate (distinctiveness principle) → Codex v2
> (*ready-with-minor-changes*) → those minor residuals folded in too (§3d kind set,
> §3c provisional-matrix caveat, §5 `skipArtifactMessage` mechanism, §7 item 2b).
> Pre-spec / pre-mission. Untracked scratch — folds into the spec-kitty mission
> spec/plan once kittify PR #7 lands; not committed standalone. Author: Claude (with
> adversarial Codex review). Date: 2026-06-25.

## 1. Problem

Text-pattern failure rules match against the **entire flattened event**
(`flattenJSON(obj)`), so text that merely *discusses* or *implements* error
handling — assistant/user narrative, code edits, codex reasoning, file reads — is
classified as a failure that actually occurred. (`internal/analyzer/fingerprints.go`
`classifyFailures`; `analyzer.go:265` builds the flattened `text`.)

The former guard, `jsonLooksLikeSourceRead`, suppressed just the Claude Read shape.
The generic fallback already narrows to
`toolUseResult`/stdout/stderr (`genericFailureSignal`, `:427`) — the intent exists,
but the main `failureRules` regex loop (`:405`) never inherited it.

See `Priivacy-ai/spec-kitty-analyzer#4` for the channel table and the four-way
deterministic reproduction.

## 2. Evidence (this session, 2026-06-25)

Built `main` vs prototype (`proto/scan-output-channels-only`) binaries and diffed
failure-fingerprint counts across a real-corpus sample:

| mission | events | main FP | proto FP | dropped (by id) |
|---|--:|--:|--:|---|
| finalize-inbox-file-01KVKG4S | 88 | 10 | **0** | `branch_worktree_confusion` ×10 |
| 028-agent-workspace-reconciliation | 901 | 34 | 5 | `generic_error` ×22, `timeout` ×7 |
| escalation / whatsapp / habit-today | small | 0 | 0 | — |

**Two opposite signals:**

- `generic_error` ×22 dropping is the **intended win** — narrative discussing errors.
- `branch_worktree_confusion` **10 → 0** is a **false-negative regression**. This is
  the fingerprint that, in the 2026-06-20 first probe, correctly surfaced the upstream
  spec-kitty **#1716/#2046** split-topology the mission's own `status.events.jsonl`
  missed — a *validated true positive*. The prototype suppresses it entirely because
  its signature lives in **narrative/message** channels, not stdout/stderr.

**Conclusion:** the prototype validates the *goal* (cut narrative FPs) but proves a
blanket "scan output channels only" switch is wrong — it cannot tell the two rule
families apart. (Codex review confirmed the `branch_worktree_confusion` claim against
the regex patterns and the existing narrative-only unit test at `analyzer_test.go:115`.)

## 3. Core mechanisms

### 3a. Universal channel exclusion (applies to ALL text-pattern rules)

Some channels can **never** contain a real runtime event *or* a meaningful
diagnosis — they are just file content the agent is reading or writing:

- **Code edits**: `toolUseResult.{newString, oldString}`, `structuredPatch`
- **File reads**: `toolUseResult.file.content` (the former source-read guard,
  generalized into this exclusion)

These are excluded from **every** failure text-pattern rule. This alone kills the
issue's reproduction line 3 (an Edit writing `raise AssertionError('boom')`).

### 3b. Per-PATTERN channel scope + the distinctiveness principle

Scope is declared **per regex pattern, not per rule** — Codex finding #2 showed
several rules mix high-precision output signatures with low-precision prose, so a
rule-level tag is too coarse. Two channel classes:

- **`output`** — real command/tool OUTPUT + structured error text (see the §3c schema
  matrix for the exact extraction): `toolUseResult.{stdout,stderr}`, bare
  `toolUseResult` string (incl. JSON re-decode), codex `payload.output`, Claude
  `tool_result` content blocks, top-level `error`/`exception`/`traceback`.
- **`diagnostic`** — `output` **plus narrative** (assistant/user message text, codex
  `payload.content`/reasoning).

**The distinctiveness principle (settled via the Codex scope debate):**

> A pattern earns `diagnostic` (narrative-eligible) scope **only if it is distinctive
> enough that a prose match is overwhelmingly a *report of an observed condition*, not
> *discussion of a class of problem*.** Distinctiveness of the pattern — not the topic
> of the rule — is the gate.

Corollary: to gain narrative recall for a rule whose patterns are generic, **tighten
the patterns into distinctive phrases first**, then promote them to `diagnostic`.
Never broaden the channel of a generic pattern.

**No implicit default.** Every text pattern must declare its scope explicitly; a
unit test fails the build if any pattern's scope is unset (Codex finding #3 — an
implicit `output` default is silent false-negative debt, not fail-safe).

### 3c. Channel extraction: the per-harness schema matrix

Codex finding #4: today there is *no* harness-aware extraction — the code flattens
everything, and `genericFailureSignal` only knows `toolUseResult` as a string or
`{stdout,stderr}`. Before migrating any rule, define and golden-test an explicit
extraction matrix mapping each harness shape to its `output` / `narrative` /
`excluded` channels:

| harness shape | output | narrative | excluded |
|---|---|---|---|
| Claude message (`message.content[].text`) | — | text blocks | — |
| Claude tool result (`toolUseResult{stdout,stderr}`) | stdout, stderr | — | — |
| Claude tool result (bare string) | string (JSON re-decode if `{`-prefixed) | — | — |
| Claude `tool_result` content block | block content/output | — | — |
| Claude Edit/Write (`toolUseResult{newString,oldString,structuredPatch}`) | — | — | all |
| Claude Read (`toolUseResult.file.content`) | — | — | all |
| Codex `payload.type=function_call_output` (`payload.output`) | output | — | — |
| Codex `payload.type=reasoning/message` (`payload.content`) | — | content | — |
| structured error (`error`/`exception`/`traceback`, nested objects) | string values | — | — |

Notes: handle `toolUseResult` strings that are themselves JSON (recursive decode,
extending the existing narrow check in `genericFailureToolText`); extract structured
`error` objects, not just strings; codex coverage is a **typed extraction path** keyed
on `payload.type`, not a bare key allowlist. Unmapped shapes are logged so the matrix
can grow (ties into §3e). The allowlist `{stdout,stderr,output,error,exception,
traceback}` is the *starting* output-key set, to be confirmed against the corpus.

> **This matrix is provisional, not a completeness claim.** The codex `payload.type`
> rows (`function_call_output` | `reasoning` | `message`) and the output-key set are
> what's known today; corpus golden-testing (§7) may surface more `payload.type`
> values or shapes. Unmapped shapes default to **excluded from output scanning** and
> are logged for matrix growth — never silently treated as output.

### 3d. The `obj == nil` / plain-text model (Codex finding #1, was an open question)

`eventFromText(..., obj=nil)` (`analyzer.go:256/310`) classifies raw non-JSON lines —
including plain-text mission artifacts and specs scanned line by line. Left alone,
Class A rules keep firing on artifact prose; naively calling all nil-text "not output"
would suppress genuine raw stdout/stderr logs. **Resolution (decided, not deferred):**

- Plain text from any **non-transcript artifact source kind** (existing `kind`
  detection — today `work_package`, `mission_artifact`, `mission_meta`,
  `mission_status_snapshot`; treat the set as "any artifact/spec kind", not a
  hard-coded pair) → **`diagnostic`-only** (so output-scoped patterns skip it; the
  prose is not command output). If a new prose-bearing kind is added later, it joins
  this set explicitly rather than by implication.
- Plain text from JSONL transcripts that lack JSON structure on a line → **output-
  eligible** (best-effort; rare).
- Standalone `.log` files → **`command_log`**, output-eligible. The README advertises
  direct transcript/log analysis, so `.log` cannot be accepted-but-unclassified.
- Generic standalone `.txt`/`.md`/`.yaml` files outside known artifact paths remain
  explicitly unsupported for now; no source kind proves they are command output.

This also subsumes part of old open-question #2 and is covered by `skipArtifactMessage`
ordering in §5.

### 3e. Recall preservation: the "unexpected result" anomaly trap (separate PR)

**The paradox (named by Kent).** The more specific the classification, the more
*useful* the output — but the more specifically we look for known conditions, the
greater the chance we **miss something new**. §§3a–3d push toward precision; left alone
they would make the tool structurally blind to novel failure modes. Tier 3 is the
deliberate recall-preserving counterpart.

**Tiered model:** Tier 1 = specific fingerprints (confirmed failures); Tier 2 =
`generic_error` (known generic distress in output, no specific rule); **Tier 3 =
unclassified anomaly** (output/structured signals that suggest a problem but match no
rule) — reported for triage, **never counted as a confirmed failure**.

**Tightened trigger boundary (Codex finding #5).** Tier 3 fires only on:

- a **structured failure indicator**: `exit_code`/`returncode` ≠ 0, `status` ∈
  {failed, blocked, error}, or an explicit error field; **or**
- a **strong crash signature**: `panic:`, a stack-trace shape, `segmentation fault`,
  `core dumped`.

Generic words (`unexpected`, `aborted`, `unhandled`, …) are **not** triggers on their
own — they are exactly the benign-chatter tokens that would reopen the FP problem.
Output/structured channels only; never narrative.

**Self-improvement loop:** each anomaly is emitted with provenance (source path,
seq/line, channel, matched signal, snippet) and a **signature hash normalized by
channel/tool/token**, grouped across runs, with an explicit **ignore registry** for
confirmed-benign signatures. Triage resolves each recurring anomaly by **promoting** it
to a Tier-1 fingerprint, **refining** a rule, or **ignoring** it. As distinctions fold
back in, the anomaly disappears; Tier-3 volume becomes a health/early-warning metric.

**Scoping:** Tier 3 reuses the §3c extraction but is a distinct feature. **Ship it in a
separate, fast-follow PR** (Codex concurred); #4 lands the channel infrastructure only.

## 4. Per-pattern taxonomy

Scope is per pattern. Today **only `branch_worktree_confusion` has `diagnostic`
patterns**; every other text pattern is `output`. Mixed rules are split so their
distinctive output signatures stay while their generic prose does not gain narrative
scope. Basis: **O** = corpus-proven · **o** = reasoned from patterns.

### `diagnostic` patterns (narrative-eligible)

| rule | basis | note |
|---|:--:|---|
| `branch_worktree_confusion` | O | the #1716/#2046 detection; distinctive prose (`mission targets … branch`, `No auto-detection is performed.*branch`, `not (in\|on) the (expected\|target\|mission) worktree`). The only rule that passes the distinctiveness gate today. |

### Split rules (distinctive part `output`; generic prose NOT promoted)

| rule | resolution |
|---|---|
| `merge_conflict` | `CONFLICT`, `Automatic merge failed` → `output`; `merge conflict`, `rebase.*conflict` → `output` for now (not distinctive enough for narrative). |
| `worktree_linkage_broken` | `\.git/worktrees` is not a failure signal alone → keep `output` (or tighten/remove). `worktree (broken\|corrupt)`, `detached worktree references` → `output` for now; candidates for `diagnostic` only after tightening. |

### `output` patterns (everything else — failure literally occurred)

`test_failure` (O), `timeout` (O), `generic_error` fallback (O), `merge_operation_failed`
(o — Codex conceded: broad `merge … failed/blocked` windows match routine discussion),
`encoding_error`, `namespace_package_import`, `permission_denied` (already anchored,
#2/#5), `typer_usage_error`, `wrong_cli_surface`, `config_yaml_invalid`,
`skill_surface_missing`, `manifest_drift`, `sync_auth_required`,
`sync_boundary_preflight`, `tracker_binding_missing`, `dirty_worktree_ref_advance`,
`ref_advance_non_fast_forward`, `circular_dependencies`, `no_code_commits`,
`runtime_not_initialized`, `missing_artifact` (text variant, `Error:`-anchored),
`guard_failure` (text variant), **`reviewer_failed`**, **`stale_agent`**.

> Correction from review: `reviewer_failed` and `stale_agent` are **text-pattern rules**
> (regexes at `fingerprints.go:163/174`), not structural — v1 mis-filed them. They are
> `output`.

### Genuinely structural (unchanged; not text rules)

The `obj != nil` structural block: `runtime_blocked`, `guard_failure` (the
`guard_failures` field), `missing_artifact` (the `reason` field), `decision_required`,
`null_prompt_step_runtime_bug`, `completed_not_terminal_runtime_bug`,
`json_error_event`, plus work-package frontmatter `review_status: has_feedback` for
`review_rejected`. These key off specific structured fields/locations and are out of
scope for broad artifact prose.

## 5. Implementation sketch

- Give each **pattern** an explicit `scope` (`output` | `diagnostic`); a test asserts
  no pattern is left unscoped (no implicit default).
- **Compute and cache two strings once per event** — `outputText(obj)` and
  `diagnosticText(obj)` (= output + narrative), both built from the §3c matrix with the
  §3a exclusion applied — then match each pattern against the cached string for its
  scope. This keeps cost at one extraction pass per event, not `O(rules × object size)`
  (Codex finding #6).
- Generalize the former source-read guard into the §3a code-edit/file-read exclusion.
- Route `genericFailureSignal` through the `output` collector (the prototype already
  does this).
- **Ordering** (make explicit, with tests): (1) structural `obj` rules; (2) §3a
  exclusion / source-read short-circuit; (3) per-pattern text rules over the cached
  strings; (4) `generic_error` fallback (output); (5) [later] Tier-3 anomaly capture.
  `skipArtifactMessage` (which drops artifact-derived failures *after* classification,
  `analyzer.go:285`) is applied as a single suppression gate **before any aggregation**
  — it gates emission for both the findings roll-up and (later) the Tier-3 anomaly
  roll-up at the same point, so an artifact event never contributes to either. Concretely:
  classify → if the event's source kind is suppressed by `skipArtifactMessage`
  (subject to the work-package-frontmatter `review_rejected` exception), drop its
  failures *and* anomalies before they reach `findings`/anomaly aggregation.

## 6. Resolved decisions (post-review + debate)

1. **`merge_conflict`** → split; all patterns `output` for now (§4).
2. **`obj == nil` plain text** → resolved in §3d (artifact/spec → diagnostic-only;
   transcript text and `.log` command logs → output-eligible; generic `.txt`/`.md`/`.yaml`
   unsupported, documented).
3. **Claude `tool_result` content blocks** → included as an `output` channel via the
   §3c matrix; exact prevalence confirmed during corpus golden-testing.
4. **Output-key allowlist + codex coverage** → replaced by the §3c typed schema matrix
   (JSON re-decode, `tool_result` blocks, codex `payload.type` paths); starting key set
   `{stdout,stderr,output,error,exception,traceback}`, corpus-confirmed.
5. **Default scope** → no default; explicit + test-enforced per pattern (§3b).
6. **Anomaly trap** → (a) separate PR; (b) triggers tightened to structured-indicator
   OR strong-crash-signature, generic words excluded; (c) signature hash normalized by
   channel/tool/token + explicit ignore registry (§3e).

Remaining to confirm *empirically during implementation* (non-blocking): exact corpus
prevalence of `tool_result` blocks and any codex `payload.type` beyond
`function_call_output`.

## 7. Validation plan (gates the PR — both directions)

1. **Explicit-scope test** — fail the build if any text pattern has no declared scope.
2. **Class-B regression guard** — `branch_worktree_confusion` in narrative MUST still
   classify; code-edit/file-read inputs with its signature MUST NOT.
2b. **Output-scoped narrative negative** — generic narrative prose that matches
   `merge_operation_failed` ("the merge failed because…") or `merge_conflict`
   ("resolve the merge conflict next") MUST NOT classify now that those patterns are
   `output`-scoped. This is the most likely regression from a future refactor that
   accidentally re-broadens scope.
3. **Golden tests per harness shape** (the §3c matrix): Claude message, `toolUseResult`
   {stdout,stderr}, bare-string `toolUseResult`, JSON-string-encoded `toolUseResult`,
   `tool_result` content block, Edit/Write, Read, codex `function_call_output`, codex
   `reasoning`. Each asserts correct output/narrative/excluded routing.
4. **Structural-vs-text ordering test** — a source-read object that *also* carries a
   structured `error`/`status` key, to pin down precedence.
5. **`obj == nil` tests** — artifact/spec line (diagnostic-only) vs transcript line
   (output-eligible).
6. **Corpus FN/FP sweep** across all five harness shapes, main vs candidate binary,
   per-mission FP + by-id delta. Must show `generic_error`/`timeout`/`test_failure` FP
   **down** and genuine `branch_worktree_confusion` narrative detections preserved
   (finalize-inbox 10→2: 2 genuine detections retained, 8 baseline FPs dropped). 233 missions cached
   locally; diff harness proven this session (zsh does not word-split `$var` in
   for-loops — list items literally or use bash).
7. **`timeout` ×7 spot-check** — confirm those were narrative, not real output-channel
   timeouts (validates the `output` call for `timeout`).
8. **(Tier 3, separate PR)** anomaly trap re-catches an output-channel structured/crash
   signal that no rule covers, while narrative distress does NOT surface.

## 8. Non-goals

- Re-tuning individual regex precision beyond the §4 splits (e.g. `permission_denied`
  already handled in #2/#5).
- Changing the structural JSON rules (§4).
- The residual intrinsic limit: command output that itself quotes failure strings
  (e.g. test output echoing "AssertionError") will still match — correctly, since it
  *is* output. Out of scope to disambiguate further.
