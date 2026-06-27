package analyzer

import (
	"regexp"
	"strings"
)

// patternScope declares the channel a failure regex is allowed to match against
// (design issue-4 §3b). Scope is per PATTERN, not per rule, because several rules
// mix a high-precision output signature with low-precision prose. The zero value
// is scopeUnset on purpose: there is no implicit default, and a build-failing test
// (§7.1) rejects any pattern left unscoped so an unscoped pattern can never become
// silent false-negative debt.
type patternScope int

const (
	// scopeUnset is the zero value. It is never a valid scope for a registered
	// pattern; TestFingerprintRulesAllScoped fails the build if any pattern keeps
	// it.
	scopeUnset patternScope = iota
	// scopeOutput patterns match only against the output channel string (real
	// command/tool output + structured error text — the failure literally
	// occurred).
	scopeOutput
	// scopeDiagnostic patterns match against the diagnostic channel string
	// (output PLUS narrative). Only patterns distinctive enough that a prose
	// match is overwhelmingly a *report of an observed condition* earn this
	// scope (the §3b distinctiveness principle).
	scopeDiagnostic
)

// scopedPattern is one failure regex together with the channel it is allowed to
// scan. A rule may carry a mix of output- and diagnostic-scoped patterns.
type scopedPattern struct {
	re    *regexp.Regexp
	scope patternScope
}

type failureRule struct {
	id       string
	title    string
	severity string
	recovery string
	patterns []scopedPattern
}

func rx(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pattern)
}

// outRx builds an output-scoped pattern from a case-insensitive-ready source.
func outRx(pattern string) scopedPattern {
	return scopedPattern{re: rx(pattern), scope: scopeOutput}
}

// diagRx builds a diagnostic-scoped (narrative-eligible) pattern.
func diagRx(pattern string) scopedPattern {
	return scopedPattern{re: rx(pattern), scope: scopeDiagnostic}
}

// scoped wraps an already-compiled regex with a scope (used where a pattern is
// case-sensitive and cannot go through rx, e.g. the bare-word `CONFLICT`).
func scoped(re *regexp.Regexp, scope patternScope) scopedPattern {
	return scopedPattern{re: re, scope: scope}
}

var failureRules = []failureRule{
	{
		id:       "guard_failure",
		title:    "Runtime guard failure",
		severity: "high",
		recovery: "Read reason/guard_failures, repair the missing invariant, then rerun the same spec-kitty command.",
		patterns: []scopedPattern{
			outRx(`(?i)\bguard[_ -]?failures?\b`),
			outRx(`(?i)\bGuard failed\b`),
		},
	},
	{
		id:       "missing_artifact",
		title:    "Missing mission artifact",
		severity: "high",
		recovery: "Restore or create the required artifact via specify/plan/tasks before advancing.",
		patterns: []scopedPattern{
			outRx(`(?i)\bError:.*Required artifact missing`),
			outRx(`(?i)\bError:.*\b(spec\.md|plan\.md|tasks\.md|findings\.md|gap-analysis\.md)\b.*\b(missing|not found|must exist)\b`),
			outRx(`(?i)\bError:.*\bmissing (spec|plan|task|mission) artifact`),
			outRx(`(?i)analysis_report_required:.*Missing:`),
		},
	},
	{
		id:       "wrong_cli_surface",
		title:    "Wrong Spec Kitty CLI surface",
		severity: "medium",
		recovery: "Use the documented agent action surface; do not pass unsupported flags such as --json to text-only commands.",
		patterns: []scopedPattern{
			outRx(`(?i)agent action (implement|review).+--json`),
			outRx(`(?i)No such option: --json`),
			outRx(`(?i)unexpected extra argument`),
			outRx(`(?i)agent worktree repair`),
		},
	},
	{
		id:       "typer_usage_error",
		title:    "CLI usage error",
		severity: "medium",
		recovery: "Re-run the command with --help and correct flags/arguments before retrying the workflow.",
		patterns: []scopedPattern{
			outRx(`(?i)^Usage: spec-kitty`),
			outRx(`(?i)Error: No such (option|command)`),
			outRx(`(?i)Got unexpected extra argument`),
			outRx(`(?i)\bexit code 2\b`),
		},
	},
	{
		id:       "worktree_linkage_broken",
		title:    "Worktree linkage broken",
		severity: "high",
		recovery: "Run git worktree list/prune and use spec-kitty doctor workspaces --fix or re-enter through spec-kitty implement.",
		// §4 split: every worktree_linkage_broken pattern stays `output`. `\.git/worktrees`
		// is not a failure signal in prose; `worktree (broken|corrupt)` and `detached
		// worktree references` are candidates for `diagnostic` only after tightening,
		// which is out of scope here. None is promoted to narrative.
		patterns: []scopedPattern{
			outRx(`(?i)worktree (not found|missing|corrupt|broken)`),
			outRx(`(?i)detached worktree references?`),
			outRx(`(?i)\.git/worktrees`),
			outRx(`(?i)husk director`),
		},
	},
	{
		id:       "branch_worktree_confusion",
		title:    "Branch or worktree context confusion",
		severity: "high",
		recovery: "Run git status and git worktree list, verify the intended mission worktree/branch, cd into it, then retry the Spec Kitty action.",
		// §3b/§4 distinctiveness gate, applied per PATTERN (Codex review cycle 2).
		// This rule MIXES scopes:
		//   - The broad prose patterns (`wrong (branch|worktree)`, the bidirectional
		//     `(branch|worktree) … (confus|mismatch|ambiguous|unexpected|wrong)`
		//     windows) are NOT distinctive: ordinary review/guidance narrative ("you
		//     may be on the wrong branch", "the branch was unexpected") would match
		//     them. They fail the distinctiveness gate, so they are `output`-scoped —
		//     they still catch a real occurrence in command/tool OUTPUT, but they stop
		//     firing on narrative (the FP class this mission eliminates).
		//   - The DISTINCTIVE signatures (`(on|current) branch … mission targets`,
		//     `mission targets … branch`, `coord(ination) worktree … (main
		//     checkout|target|branch)`, `not (in|on) the (expected|target|mission)
		//     worktree`, `No auto-detection is performed … branch`) are overwhelmingly
		//     a *report of an observed condition*, not a discussion of a class of
		//     problem — so they keep `diagnostic` scope and remain narrative-eligible.
		//     These are the corpus-proven #1716/#2046 detection the blanket output-only
		//     prototype regressed to zero. branch_worktree_confusion is still the only
		//     rule that carries any `diagnostic` patterns (§4).
		patterns: []scopedPattern{
			outRx(`(?i)\bwrong (branch|worktree|work tree)\b`),
			outRx(`(?i)\b(branch|worktree|work tree).{0,100}\b(confus|mismatch|ambiguous|unexpected|wrong)\b`),
			outRx(`(?i)\b(confus|mismatch|ambiguous|unexpected|wrong).{0,100}\b(branch|worktree|work tree)\b`),
			diagRx(`(?i)\b(on|current) branch\b.{0,120}\bmission targets\b`),
			diagRx(`(?i)\bmission targets\b.{0,120}\bbranch\b`),
			diagRx(`(?i)\bcoord(?:ination)? worktree\b.{0,120}\b(main checkout|target|branch)\b`),
			diagRx(`(?i)\bnot (in|on) (the )?(expected|target|mission) (worktree|work tree|branch)\b`),
			diagRx(`(?i)\bNo auto-detection is performed.*branch`),
		},
	},
	{
		id:       "dirty_worktree_ref_advance",
		title:    "Dirty checked-out worktree blocked ref advance",
		severity: "high",
		recovery: "Commit, stash, or revert the dirty entries in the checked-out worktree, then retry merge/ref advance.",
		patterns: []scopedPattern{
			outRx(`(?i)REF_ADVANCE_DIRTY_WORKTREE`),
			outRx(`(?i)Refusing to advance branch`),
			outRx(`(?i)uncommitted local changes`),
		},
	},
	{
		id:       "ref_advance_non_fast_forward",
		title:    "Non-fast-forward ref advance refused",
		severity: "high",
		recovery: "Inspect branch ancestry; only advance to a descendant or run the documented merge/rebase path.",
		patterns: []scopedPattern{
			outRx(`(?i)REF_ADVANCE_NON_FAST_FORWARD`),
			outRx(`(?i)not a fast-forward descendant`),
			outRx(`(?i)move a branch backwards or sideways`),
		},
	},
	{
		id:       "merge_conflict",
		title:    "Merge or rebase conflict",
		severity: "high",
		recovery: "Resolve conflicts in the lane/merge worktree, commit the resolution, then retry the Spec Kitty merge or sync command.",
		// §4 split: distinctive git output signatures (`CONFLICT`, `Automatic merge
		// failed`) stay `output`; the generic prose `merge conflict` / `rebase …
		// conflict` are NOT distinctive enough for narrative, so they also stay
		// `output` for now (never promoted to `diagnostic`).
		patterns: []scopedPattern{
			scoped(regexp.MustCompile(`\bCONFLICT\b`), scopeOutput),
			outRx(`(?i)merge conflict`),
			outRx(`(?i)Automatic merge failed`),
			outRx(`(?i)rebase.*conflict`),
		},
	},
	{
		id:       "merge_operation_failed",
		title:    "Merge operation failed or was blocked",
		severity: "high",
		recovery: "Inspect the merge/ref-advance output, resolve branch/worktree preconditions, then rerun the merge command.",
		// §4: broad `merge … failed/blocked` windows match routine discussion, so all
		// stay `output` (never narrative).
		patterns: []scopedPattern{
			outRx(`(?i)\bmerge\b.{0,100}\b(failed|blocked|error|refused|aborted)\b`),
			outRx(`(?i)\b(failed|blocked|error|refused|aborted)\b.{0,100}\bmerge\b`),
			outRx(`(?i)\bmerge gate\b.{0,100}\b(failed|blocked|error)\b`),
			outRx(`(?i)\bmerge preflight\b.{0,100}\b(failed|blocked|error)\b`),
		},
	},
	{
		id:       "no_code_commits",
		title:    "Work package has no implementation commits",
		severity: "high",
		recovery: "Commit real code changes in the lane worktree before moving the WP to for_review.",
		patterns: []scopedPattern{
			outRx(`(?i)zero commits`),
			outRx(`(?i)no commits ahead`),
			outRx(`(?i)must commit (code|implementation)`),
		},
	},
	{
		id:       "review_rejected",
		title:    "Review rejected work package",
		severity: "medium",
		recovery: "Read the review feedback, re-dispatch implementation, and move the WP back to for_review after fixes.",
		patterns: []scopedPattern{
			outRx(`(?i)review_status:\s*["']?has_feedback`),
			outRx(`(?i)\breview (failed|rejected)`),
			outRx(`(?i)\bverdict:\s*rejected\b`),
		},
	},
	{
		id:       "reviewer_failed",
		title:    "Configured reviewer failed",
		severity: "high",
		recovery: "Retry the configured reviewer after fixing the cause or explicitly switch reviewer/self-review with failure metadata.",
		// §4 correction: reviewer_failed is a TEXT-pattern rule, not structural →
		// `output`.
		patterns: []scopedPattern{
			outRx(`(?i)Configured reviewer .* failed`),
			outRx(`(?i)self-review fallback`),
			outRx(`(?i)reviewer-failure-reason`),
		},
	},
	{
		id:       "stale_agent",
		title:    "Stale agent lease",
		severity: "medium",
		recovery: "Verify shell_pid/activity, move WP back to planned with feedback, preserve useful partial work, then redispatch.",
		// §4 correction: stale_agent is a TEXT-pattern rule, not structural → `output`.
		patterns: []scopedPattern{
			outRx(`(?i)stale (agent|implementation|lease|claim)`),
			outRx(`(?i)dead process`),
		},
	},
	{
		id:       "circular_dependencies",
		title:    "Circular WP dependencies",
		severity: "high",
		recovery: "Break the dependency cycle in WP frontmatter/tasks.md and rerun finalize-tasks validation.",
		patterns: []scopedPattern{
			outRx(`(?i)circular dependencies`),
			outRx(`(?i)dependency graph has a cycle`),
			outRx(`(?i)cycle detected`),
		},
	},
	{
		id:       "runtime_not_initialized",
		title:    "Spec Kitty runtime not initialized",
		severity: "high",
		recovery: "Run from the repo root and initialize or repair .kittify with spec-kitty init/doctor.",
		patterns: []scopedPattern{
			outRx(`(?i)runtime (not found|missing)`),
			outRx(`(?i)\.kittify/.*(missing|not found)`),
			outRx(`(?i)not initialized`),
		},
	},
	{
		id:       "skill_surface_missing",
		title:    "Skill or slash-command surface missing",
		severity: "medium",
		recovery: "Run spec-kitty doctor skills --json, then spec-kitty doctor skills --fix if managed files are missing.",
		patterns: []scopedPattern{
			outRx(`(?i)skill not found`),
			outRx(`(?i)missing skill files?`),
			outRx(`(?i)slash commands?.*(not found|missing|unavailable)`),
			outRx(`(?i)command-skills-manifest\.json.*missing`),
		},
	},
	{
		id:       "manifest_drift",
		title:    "Generated skill manifest drift",
		severity: "medium",
		recovery: "Regenerate managed command/skill files with spec-kitty doctor skills --fix.",
		patterns: []scopedPattern{
			outRx(`(?i)manifest drift`),
			outRx(`(?i)drifted skill files?`),
			outRx(`(?i)hash .*does not match.*manifest`),
		},
	},
	{
		id:       "config_yaml_invalid",
		title:    "Invalid .kittify config YAML",
		severity: "high",
		recovery: "Back up .kittify/config.yaml, repair YAML or regenerate config with spec-kitty init.",
		patterns: []scopedPattern{
			outRx(`(?i)YAML parse error.*\.kittify/config\.ya?ml`),
			outRx(`(?i)\.kittify/config\.ya?ml.*(invalid|corrupt|parse)`),
		},
	},
	{
		id:       "encoding_error",
		title:    "Encoding failure",
		severity: "medium",
		recovery: "Run spec-kitty validate-encoding and normalize ambiguous files before retrying.",
		patterns: []scopedPattern{
			outRx(`(?i)UnicodeDecodeError`),
			outRx(`(?i)invalid utf-?8`),
			outRx(`(?i)\bencoding (failed|failure|error)\b`),
			outRx(`(?i)\bfailed to decode\b`),
		},
	},
	{
		id:       "sync_auth_required",
		title:    "Hosted sync/auth failure",
		severity: "high",
		recovery: "Check spec-kitty auth/sync status; on this machine include SPEC_KITTY_ENABLE_SAAS_SYNC=1 for hosted sync tests.",
		patterns: []scopedPattern{
			outRx(`(?i)hosted auth.*(missing|required|failed)`),
			outRx(`(?i)\b(401|403)\b.*(sync|auth|teamspace|tracker)`),
			outRx(`(?i)foreground.*unauthenticated`),
		},
	},
	{
		id:       "sync_boundary_preflight",
		title:    "Sync boundary preflight failure",
		severity: "high",
		recovery: "Run sync doctor/status, resolve daemon-owner/offline-queue boundary failures, then retry sync.",
		patterns: []scopedPattern{
			outRx(`(?i)boundary preflight`),
			outRx(`(?i)daemon-owner.*mismatch`),
			outRx(`(?i)legacy queue rows`),
			outRx(`(?i)offline queue.*(malformed|failed|exceeded)`),
		},
	},
	{
		id:       "tracker_binding_missing",
		title:    "Tracker binding missing or invalid",
		severity: "medium",
		recovery: "Inspect tracker providers/discover/status and bind the project before tracker sync.",
		patterns: []scopedPattern{
			outRx(`(?i)tracker.*(not configured|binding missing|not bound|no binding)`),
			outRx(`(?i)no tracker provider`),
		},
	},
	{
		id:       "namespace_package_import",
		title:    "spec-kitty-events namespace package import failure",
		severity: "high",
		recovery: "Reinstall spec-kitty-events and remove stale namespace-package leftovers from site-packages.",
		patterns: []scopedPattern{
			outRx(`ImportError: cannot import name 'normalize_event_id' from 'spec_kitty_events' \(unknown location\)`),
			outRx(`(?i)spec_kitty_events.*_NamespacePath`),
		},
	},
	{
		id:       "test_failure",
		title:    "Verification/test failure",
		severity: "medium",
		recovery: "Inspect the first failing test/error, fix root cause, and rerun the same verification command.",
		patterns: []scopedPattern{
			outRx(`(?i)\bFAILED\b.*\b(pytest|test|assert)`),
			outRx(`(?i)\bAssertionError\b`),
			outRx(`(?i)\bgo test\b.*\bFAIL\b`),
			outRx(`(?i)\bexit status 1\b`),
		},
	},
	{
		id:       "timeout",
		title:    "Timeout",
		severity: "medium",
		recovery: "Determine whether the command is hung or under-provisioned; retry with narrower scope or fixed service readiness.",
		patterns: []scopedPattern{outRx(`(?i)\b(timed out|timeout|deadline exceeded)\b`)},
	},
	{
		id:       "permission_denied",
		title:    "Permission denied",
		severity: "medium",
		recovery: "Fix file permissions, executable bits, or credential access before retrying.",
		// A real denial appears as a *structured* error line — a tool/path token
		// immediately followed by `: permission denied` ("ls: ...: Permission
		// denied", "open /x: permission denied", "EACCES: permission denied",
		// "bash: x: permission denied") — or as a fixed runtime signature. Prose
		// that merely discusses the phrase ("Filesystem error — permission denied,
		// disk full, ..."), control flow (`except PermissionError`), doc vocabulary
		// ("EACCES or EAGAIN"), and the analyzer's own "findings :: Permission
		// denied" output all lack that `token: permission denied` shape, so they no
		// longer match. The `[^\s:]` before the colon excludes the `::` self-output.
		patterns: []scopedPattern{
			outRx(`(?i)[^\s:]:\s+permission denied\b`),
			outRx(`\[Errno 13\] Permission denied`),
			outRx(`\(os error 13\)`),
			outRx(`(?i)Permission denied \(publickey`),
			// Windows-native access-denied signatures (issue #3). Windows reports
			// ERROR_ACCESS_DENIED (code 5) as "Access is denied", which contains no
			// "permission denied" substring, so the rules above never match it.
			outRx(`\[WinError 5\]`), // Python: PermissionError: [WinError 5] Access is denied
			// Rust std::io::Error Display is "{message} (os error {code})" with the
			// raw platform errno. On Windows code 5 == ERROR_ACCESS_DENIED, but on
			// Unix errno 5 == EIO ("Input/output error"; EACCES is 13). Anchor to the
			// Windows message so a Unix EIO error never misclassifies as a denial.
			outRx(`(?i)access is denied\.?\s*\(os error 5\)`),
		},
	},
}

// findFailureRule returns the failureRules entry with the given id. It is used
// where a structural detection must emit the SAME finding as its text-rule
// counterpart (review_rejected) without duplicating the rule's title/severity/
// recovery strings — keeping a single source of truth for that finding's metadata.
func findFailureRule(id string) (failureRule, bool) {
	for _, r := range failureRules {
		if r.id == id {
			return r, true
		}
	}
	return failureRule{}, false
}

// classifyFailures is the backward-compatible entry point. It derives the two
// channel strings from the parsed event object (via the WP01 channel extraction)
// and delegates to classifyFailuresWithChannels.
//
//   - obj != nil: outputText/diagnosticText come from the §3a/§3c extraction, so a
//     code edit or file read (excluded by WP01) never reaches either string.
//   - obj == nil: this is a raw, non-JSON line. With no harness structure to route,
//     the raw text is treated as output-eligible AND narrative-eligible (both
//     strings = text). The §3d artifact-kind refinement (artifact prose →
//     diagnostic-only) is applied by the caller in WP03, not here.
func classifyFailures(text string, obj map[string]any, invocations []CLIInvocation) []FailureFingerprint {
	var outText, diagText string
	if obj != nil {
		outText = outputText(obj)
		diagText = diagnosticText(obj)
	} else {
		outText = text
		diagText = text
	}
	return classifyFailuresWithChannels(outText, diagText, obj, invocations)
}

// classifyFailuresWithChannels is the scoped core. Each text-pattern rule is
// matched only against the channel string for its scope: output-scoped patterns
// against outputText, diagnostic-scoped (narrative-eligible) patterns against
// diagnosticText. The structural obj-field rules and the invocation/sync rules are
// unchanged; the generic fallback runs through outputText. Callers that already
// hold the cached channel strings (WP03) call this directly.
func classifyFailuresWithChannels(outputText, diagnosticText string, obj map[string]any, invocations []CLIInvocation) []FailureFingerprint {
	seen := map[string]bool{}
	var out []FailureFingerprint
	add := func(rule failureRule, reason string) {
		if seen[rule.id] {
			return
		}
		seen[rule.id] = true
		out = append(out, FailureFingerprint{
			ID:            rule.id,
			Title:         rule.title,
			Severity:      rule.severity,
			Reason:        reason,
			Recovery:      rule.recovery,
			Deterministic: true,
		})
	}

	if obj != nil {
		kind := strings.ToLower(firstJSONStringByKey(obj, "kind", "type"))
		if kind == "blocked" {
			add(failureRule{id: "runtime_blocked", title: "Runtime returned blocked", severity: "high", recovery: "Use reason/guard_failures to repair blockers before retrying."}, "JSON decision kind is blocked")
		}
		if jsonHasAnyKey(obj, "guard_failures") {
			add(failureRule{id: "guard_failure", title: "Runtime guard failure", severity: "high", recovery: "Read reason/guard_failures, repair the missing invariant, then rerun the same spec-kitty command."}, "JSON contains guard_failures")
		}
		if reason := firstJSONStringByKey(obj, "reason", "message"); strings.Contains(strings.ToLower(reason), "required artifact missing") {
			add(failureRule{id: "missing_artifact", title: "Missing mission artifact", severity: "high", recovery: "Restore or create the required artifact via specify/plan/tasks before advancing."}, "JSON reason reports required artifact missing")
		}
		if kind == "decision_required" {
			add(failureRule{id: "decision_required", title: "Runtime requires decision", severity: "low", recovery: "Answer with --answer and --decision-id, or escalate to the user."}, "JSON decision kind is decision_required")
		}
		if kind == "step" {
			prompt := firstJSONStringByKey(obj, "prompt_file", "prompt_path")
			if strings.TrimSpace(prompt) == "" {
				add(failureRule{id: "null_prompt_step_runtime_bug", title: "Step decision has no prompt file", severity: "high", recovery: "Treat as runtime bug; rerun after upgrading/repairing Spec Kitty and capture decision JSON."}, "kind=step lacks prompt_file/prompt_path")
			}
		}
		if progress := firstJSONMapByKey(obj, "progress"); progress != nil && kind != "" && kind != "terminal" {
			done, doneOK := firstJSONNumberByKey(progress, "done_wps")
			total, totalOK := firstJSONNumberByKey(progress, "total_wps")
			if doneOK && totalOK && total > 0 && done == total {
				add(failureRule{id: "completed_not_terminal_runtime_bug", title: "All WPs complete but runtime not terminal", severity: "medium", recovery: "Treat as terminal; run accept, merge, mission review, and retrospective workflow."}, "progress.done_wps equals progress.total_wps while kind is not terminal")
			}
		}
		if jsonHasError(obj) {
			add(failureRule{id: "json_error_event", title: "JSON event reports error", severity: "medium", recovery: "Inspect structured error fields and rerun only after root cause is fixed."}, "JSON error/status/exit_code field indicates failure")
		}
		// review_rejected, detected STRUCTURALLY (additive to the output-scoped text
		// rule of the same id). A status event (e.g. a wp_lane_changed line in
		// status.events.jsonl, or a review-evidence event carrying
		// evidence.review.verdict) records the rejection as a bare JSON FIELD with no
		// output- or narrative-channel text, so the §3c channel extraction yields
		// nothing for the text scanner to match — a latent false negative under channel
		// scoping (the pre-scoping flattenJSON pipeline saw it).
		//
		// Match EXPLICIT, deterministic paths only (review_scope review #1): a
		// whole-object recursive search (firstJSONStringByKey) would (a) false-positive
		// on a stale/historical verdict stored elsewhere in the same event — e.g. an
		// approved transition whose history[] still carries an old "rejected" — and
		// (b) be order-nondeterministic (Go map iteration), violating FR-006. nestedString
		// descends only the named keys. Values mirror the text rule
		// (review_status==has_feedback, verdict==rejected); the seen[] dedup keeps an
		// event matched by both this and the text rule at ONE finding. findFailureRule
		// keeps the rule's title/severity/recovery single-sourced.
		if rule, ok := findFailureRule("review_rejected"); ok {
			if rs, ok := obj["review_status"].(string); ok && strings.EqualFold(strings.TrimSpace(rs), "has_feedback") {
				add(rule, "top-level review_status field is has_feedback")
			} else if v := nestedString(obj, "evidence", "review", "verdict"); strings.EqualFold(strings.TrimSpace(v), "rejected") {
				add(rule, "evidence.review.verdict field is rejected")
			}
		}
	}

	// The old source-read short-circuit is gone: the WP01 §3a exclusion
	// already keeps file-read and code-edit content out of outputText/diagnosticText,
	// so the scoped scan below can never see it.

	for _, inv := range invocations {
		if (inv.Verb == "sync" || inv.Verb == "tracker" || strings.HasPrefix(inv.Subcommand, "sync ") || strings.Contains(inv.Raw, " tracker ")) && !inv.SaaSSyncEnabled {
			add(failureRule{id: "saas_sync_flag_missing", title: "Hosted sync command missing local SaaS flag", severity: "medium", recovery: "On this computer rerun hosted sync/tracker/auth test commands with SPEC_KITTY_ENABLE_SAAS_SYNC=1."}, "sync/tracker command lacks SPEC_KITTY_ENABLE_SAAS_SYNC=1")
		}
	}

	for _, rule := range failureRules {
		for _, sp := range rule.patterns {
			haystack, ok := channelStringFor(sp.scope, outputText, diagnosticText)
			if !ok {
				// scopeUnset: guarded against by TestFingerprintRulesAllScoped; skip
				// defensively at runtime rather than scanning an arbitrary channel.
				continue
			}
			if sp.re.MatchString(haystack) {
				add(rule, "matched deterministic signature "+sp.re.String())
				break
			}
		}
	}
	if len(out) == 0 && genericFailureToolText(outputText) {
		add(failureRule{id: "generic_error", title: "Generic error signal", severity: "medium", recovery: "Inspect surrounding timeline context for the failed command and retry only after root cause is addressed."}, "generic execution failure signal")
	}
	return out
}

func fingerprintForRuleID(id, reason string) (FailureFingerprint, bool) {
	for _, rule := range failureRules {
		if rule.id != id {
			continue
		}
		return FailureFingerprint{
			ID:            rule.id,
			Title:         rule.title,
			Severity:      rule.severity,
			Reason:        reason,
			Recovery:      rule.recovery,
			Deterministic: true,
		}, true
	}
	return FailureFingerprint{}, false
}

// channelStringFor resolves a pattern scope to the channel string it must scan.
// The boolean is false for scopeUnset (no usable channel) so the caller skips it.
func channelStringFor(scope patternScope, outputText, diagnosticText string) (string, bool) {
	switch scope {
	case scopeOutput:
		return outputText, true
	case scopeDiagnostic:
		return diagnosticText, true
	default:
		return "", false
	}
}

var genericFailureSignals = []*regexp.Regexp{
	rx(`(?i)(^|\s)Error:\s`),
	rx(`(?i)Traceback \(most recent call last\):`),
	rx(`(?i)\b(exit code|exit status|returncode|return code)\s*[:=]?\s*[1-9][0-9]*\b`),
	rx(`(?i)\b(command|hook|tool|process|subprocess|pytest|ruff|mypy|spec-kitty)\b.{0,80}\b(failed|failure)\b`),
	rx(`(?i)\b(failed|failure)\b.{0,80}\b(command|hook|tool|process|subprocess|pytest|ruff|mypy|spec-kitty)\b`),
}

// genericFailureToolText decides whether the output channel string carries a
// generic execution-failure signal. When the whole string is a JSON object
// (re-decoded once), a structured op-context payload with no error field is NOT a
// failure; otherwise the generic-signal regexes apply. The channel routing (WP01)
// has already stripped narrative and file/edit content, so this only ever runs
// over real output text.
func genericFailureToolText(raw string) bool {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "{") {
		if nested, ok := decodeJSONObject([]byte(raw)); ok && !jsonHasError(nested) {
			return false
		}
	}
	return genericFailureText(raw)
}

func genericFailureText(text string) bool {
	for _, pattern := range genericFailureSignals {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}
