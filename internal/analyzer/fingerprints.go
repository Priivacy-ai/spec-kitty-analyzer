package analyzer

import (
	"regexp"
	"strings"
)

type failureRule struct {
	id       string
	title    string
	severity string
	recovery string
	patterns []*regexp.Regexp
}

func rx(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pattern)
}

var failureRules = []failureRule{
	{
		id:       "guard_failure",
		title:    "Runtime guard failure",
		severity: "high",
		recovery: "Read reason/guard_failures, repair the missing invariant, then rerun the same spec-kitty command.",
		patterns: []*regexp.Regexp{
			rx(`(?i)\bguard[_ -]?failures?\b`),
			rx(`(?i)\bGuard failed\b`),
		},
	},
	{
		id:       "missing_artifact",
		title:    "Missing mission artifact",
		severity: "high",
		recovery: "Restore or create the required artifact via specify/plan/tasks before advancing.",
		patterns: []*regexp.Regexp{
			rx(`(?i)\bError:.*Required artifact missing`),
			rx(`(?i)\bError:.*\b(spec\.md|plan\.md|tasks\.md|findings\.md|gap-analysis\.md)\b.*\b(missing|not found|must exist)\b`),
			rx(`(?i)\bError:.*\bmissing (spec|plan|task|mission) artifact`),
			rx(`(?i)analysis_report_required:.*Missing:`),
		},
	},
	{
		id:       "wrong_cli_surface",
		title:    "Wrong Spec Kitty CLI surface",
		severity: "medium",
		recovery: "Use the documented agent action surface; do not pass unsupported flags such as --json to text-only commands.",
		patterns: []*regexp.Regexp{
			rx(`(?i)agent action (implement|review).+--json`),
			rx(`(?i)No such option: --json`),
			rx(`(?i)unexpected extra argument`),
			rx(`(?i)agent worktree repair`),
		},
	},
	{
		id:       "typer_usage_error",
		title:    "CLI usage error",
		severity: "medium",
		recovery: "Re-run the command with --help and correct flags/arguments before retrying the workflow.",
		patterns: []*regexp.Regexp{
			rx(`(?i)^Usage: spec-kitty`),
			rx(`(?i)Error: No such (option|command)`),
			rx(`(?i)Got unexpected extra argument`),
			rx(`(?i)\bexit code 2\b`),
		},
	},
	{
		id:       "worktree_linkage_broken",
		title:    "Worktree linkage broken",
		severity: "high",
		recovery: "Run git worktree list/prune and use spec-kitty doctor workspaces --fix or re-enter through spec-kitty implement.",
		patterns: []*regexp.Regexp{
			rx(`(?i)worktree (not found|missing|corrupt|broken)`),
			rx(`(?i)detached worktree references?`),
			rx(`(?i)\.git/worktrees`),
			rx(`(?i)husk director`),
		},
	},
	{
		id:       "branch_worktree_confusion",
		title:    "Branch or worktree context confusion",
		severity: "high",
		recovery: "Run git status and git worktree list, verify the intended mission worktree/branch, cd into it, then retry the Spec Kitty action.",
		patterns: []*regexp.Regexp{
			rx(`(?i)\bwrong (branch|worktree|work tree)\b`),
			rx(`(?i)\b(branch|worktree|work tree).{0,100}\b(confus|mismatch|ambiguous|unexpected|wrong)\b`),
			rx(`(?i)\b(confus|mismatch|ambiguous|unexpected|wrong).{0,100}\b(branch|worktree|work tree)\b`),
			rx(`(?i)\b(on|current) branch\b.{0,120}\bmission targets\b`),
			rx(`(?i)\bmission targets\b.{0,120}\bbranch\b`),
			rx(`(?i)\bcoord(?:ination)? worktree\b.{0,120}\b(main checkout|target|branch)\b`),
			rx(`(?i)\bnot (in|on) (the )?(expected|target|mission) (worktree|work tree|branch)\b`),
			rx(`(?i)\bNo auto-detection is performed.*branch`),
		},
	},
	{
		id:       "dirty_worktree_ref_advance",
		title:    "Dirty checked-out worktree blocked ref advance",
		severity: "high",
		recovery: "Commit, stash, or revert the dirty entries in the checked-out worktree, then retry merge/ref advance.",
		patterns: []*regexp.Regexp{
			rx(`(?i)REF_ADVANCE_DIRTY_WORKTREE`),
			rx(`(?i)Refusing to advance branch`),
			rx(`(?i)uncommitted local changes`),
		},
	},
	{
		id:       "ref_advance_non_fast_forward",
		title:    "Non-fast-forward ref advance refused",
		severity: "high",
		recovery: "Inspect branch ancestry; only advance to a descendant or run the documented merge/rebase path.",
		patterns: []*regexp.Regexp{
			rx(`(?i)REF_ADVANCE_NON_FAST_FORWARD`),
			rx(`(?i)not a fast-forward descendant`),
			rx(`(?i)move a branch backwards or sideways`),
		},
	},
	{
		id:       "merge_conflict",
		title:    "Merge or rebase conflict",
		severity: "high",
		recovery: "Resolve conflicts in the lane/merge worktree, commit the resolution, then retry the Spec Kitty merge or sync command.",
		patterns: []*regexp.Regexp{
			regexp.MustCompile(`\bCONFLICT\b`),
			rx(`(?i)merge conflict`),
			rx(`(?i)Automatic merge failed`),
			rx(`(?i)rebase.*conflict`),
		},
	},
	{
		id:       "merge_operation_failed",
		title:    "Merge operation failed or was blocked",
		severity: "high",
		recovery: "Inspect the merge/ref-advance output, resolve branch/worktree preconditions, then rerun the merge command.",
		patterns: []*regexp.Regexp{
			rx(`(?i)\bmerge\b.{0,100}\b(failed|blocked|error|refused|aborted)\b`),
			rx(`(?i)\b(failed|blocked|error|refused|aborted)\b.{0,100}\bmerge\b`),
			rx(`(?i)\bmerge gate\b.{0,100}\b(failed|blocked|error)\b`),
			rx(`(?i)\bmerge preflight\b.{0,100}\b(failed|blocked|error)\b`),
		},
	},
	{
		id:       "no_code_commits",
		title:    "Work package has no implementation commits",
		severity: "high",
		recovery: "Commit real code changes in the lane worktree before moving the WP to for_review.",
		patterns: []*regexp.Regexp{
			rx(`(?i)zero commits`),
			rx(`(?i)no commits ahead`),
			rx(`(?i)must commit (code|implementation)`),
		},
	},
	{
		id:       "review_rejected",
		title:    "Review rejected work package",
		severity: "medium",
		recovery: "Read the review feedback, re-dispatch implementation, and move the WP back to for_review after fixes.",
		patterns: []*regexp.Regexp{
			rx(`(?i)review_status:\s*["']?has_feedback`),
			rx(`(?i)\breview (failed|rejected)`),
			rx(`(?i)\bverdict:\s*rejected\b`),
		},
	},
	{
		id:       "reviewer_failed",
		title:    "Configured reviewer failed",
		severity: "high",
		recovery: "Retry the configured reviewer after fixing the cause or explicitly switch reviewer/self-review with failure metadata.",
		patterns: []*regexp.Regexp{
			rx(`(?i)Configured reviewer .* failed`),
			rx(`(?i)self-review fallback`),
			rx(`(?i)reviewer-failure-reason`),
		},
	},
	{
		id:       "stale_agent",
		title:    "Stale agent lease",
		severity: "medium",
		recovery: "Verify shell_pid/activity, move WP back to planned with feedback, preserve useful partial work, then redispatch.",
		patterns: []*regexp.Regexp{
			rx(`(?i)stale (agent|implementation|lease|claim)`),
			rx(`(?i)dead process`),
		},
	},
	{
		id:       "circular_dependencies",
		title:    "Circular WP dependencies",
		severity: "high",
		recovery: "Break the dependency cycle in WP frontmatter/tasks.md and rerun finalize-tasks validation.",
		patterns: []*regexp.Regexp{
			rx(`(?i)circular dependencies`),
			rx(`(?i)dependency graph has a cycle`),
			rx(`(?i)cycle detected`),
		},
	},
	{
		id:       "runtime_not_initialized",
		title:    "Spec Kitty runtime not initialized",
		severity: "high",
		recovery: "Run from the repo root and initialize or repair .kittify with spec-kitty init/doctor.",
		patterns: []*regexp.Regexp{
			rx(`(?i)runtime (not found|missing)`),
			rx(`(?i)\.kittify/.*(missing|not found)`),
			rx(`(?i)not initialized`),
		},
	},
	{
		id:       "skill_surface_missing",
		title:    "Skill or slash-command surface missing",
		severity: "medium",
		recovery: "Run spec-kitty doctor skills --json, then spec-kitty doctor skills --fix if managed files are missing.",
		patterns: []*regexp.Regexp{
			rx(`(?i)skill not found`),
			rx(`(?i)missing skill files?`),
			rx(`(?i)slash commands?.*(not found|missing|unavailable)`),
			rx(`(?i)command-skills-manifest\.json.*missing`),
		},
	},
	{
		id:       "manifest_drift",
		title:    "Generated skill manifest drift",
		severity: "medium",
		recovery: "Regenerate managed command/skill files with spec-kitty doctor skills --fix.",
		patterns: []*regexp.Regexp{
			rx(`(?i)manifest drift`),
			rx(`(?i)drifted skill files?`),
			rx(`(?i)hash .*does not match.*manifest`),
		},
	},
	{
		id:       "config_yaml_invalid",
		title:    "Invalid .kittify config YAML",
		severity: "high",
		recovery: "Back up .kittify/config.yaml, repair YAML or regenerate config with spec-kitty init.",
		patterns: []*regexp.Regexp{
			rx(`(?i)YAML parse error.*\.kittify/config\.ya?ml`),
			rx(`(?i)\.kittify/config\.ya?ml.*(invalid|corrupt|parse)`),
		},
	},
	{
		id:       "encoding_error",
		title:    "Encoding failure",
		severity: "medium",
		recovery: "Run spec-kitty validate-encoding and normalize ambiguous files before retrying.",
		patterns: []*regexp.Regexp{
			rx(`(?i)UnicodeDecodeError`),
			rx(`(?i)invalid utf-?8`),
			rx(`(?i)\bencoding (failed|failure|error)\b`),
			rx(`(?i)\bfailed to decode\b`),
		},
	},
	{
		id:       "sync_auth_required",
		title:    "Hosted sync/auth failure",
		severity: "high",
		recovery: "Check spec-kitty auth/sync status; on this machine include SPEC_KITTY_ENABLE_SAAS_SYNC=1 for hosted sync tests.",
		patterns: []*regexp.Regexp{
			rx(`(?i)hosted auth.*(missing|required|failed)`),
			rx(`(?i)\b(401|403)\b.*(sync|auth|teamspace|tracker)`),
			rx(`(?i)foreground.*unauthenticated`),
		},
	},
	{
		id:       "sync_boundary_preflight",
		title:    "Sync boundary preflight failure",
		severity: "high",
		recovery: "Run sync doctor/status, resolve daemon-owner/offline-queue boundary failures, then retry sync.",
		patterns: []*regexp.Regexp{
			rx(`(?i)boundary preflight`),
			rx(`(?i)daemon-owner.*mismatch`),
			rx(`(?i)legacy queue rows`),
			rx(`(?i)offline queue.*(malformed|failed|exceeded)`),
		},
	},
	{
		id:       "tracker_binding_missing",
		title:    "Tracker binding missing or invalid",
		severity: "medium",
		recovery: "Inspect tracker providers/discover/status and bind the project before tracker sync.",
		patterns: []*regexp.Regexp{
			rx(`(?i)tracker.*(not configured|binding missing|not bound|no binding)`),
			rx(`(?i)no tracker provider`),
		},
	},
	{
		id:       "namespace_package_import",
		title:    "spec-kitty-events namespace package import failure",
		severity: "high",
		recovery: "Reinstall spec-kitty-events and remove stale namespace-package leftovers from site-packages.",
		patterns: []*regexp.Regexp{
			rx(`ImportError: cannot import name 'normalize_event_id' from 'spec_kitty_events' \(unknown location\)`),
			rx(`(?i)spec_kitty_events.*_NamespacePath`),
		},
	},
	{
		id:       "test_failure",
		title:    "Verification/test failure",
		severity: "medium",
		recovery: "Inspect the first failing test/error, fix root cause, and rerun the same verification command.",
		patterns: []*regexp.Regexp{
			rx(`(?i)\bFAILED\b.*\b(pytest|test|assert)`),
			rx(`(?i)\bAssertionError\b`),
			rx(`(?i)\bgo test\b.*\bFAIL\b`),
			rx(`(?i)\bexit status 1\b`),
		},
	},
	{
		id:       "timeout",
		title:    "Timeout",
		severity: "medium",
		recovery: "Determine whether the command is hung or under-provisioned; retry with narrower scope or fixed service readiness.",
		patterns: []*regexp.Regexp{rx(`(?i)\b(timed out|timeout|deadline exceeded)\b`)},
	},
	{
		id:       "permission_denied",
		title:    "Permission denied",
		severity: "medium",
		recovery: "Fix file permissions, executable bits, or credential access before retrying.",
		// Anchored error signatures only. A bare `permission denied` match also
		// fires on documentation/spec prose that merely discusses the phrase
		// (e.g. "Filesystem error — permission denied, disk full, ..."), so we
		// require a concrete denial signature from a real OS/runtime/shell error.
		patterns: []*regexp.Regexp{
			rx(`\[Errno 13\] Permission denied`),
			rx(`\bPermissionError\b`),
			rx(`\bEACCES\b`),
			rx(`\(os error 13\)`),
			rx(`(?i)\b(?:bash|sh|zsh):.*permission denied`),
			rx(`(?i)Permission denied \(publickey`),
		},
	},
}

func classifyFailures(text string, obj map[string]any, invocations []CLIInvocation) []FailureFingerprint {
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
	}

	if obj != nil && jsonLooksLikeSourceRead(obj) {
		return out
	}

	for _, inv := range invocations {
		if (inv.Verb == "sync" || inv.Verb == "tracker" || strings.HasPrefix(inv.Subcommand, "sync ") || strings.Contains(inv.Raw, " tracker ")) && !inv.SaaSSyncEnabled {
			add(failureRule{id: "saas_sync_flag_missing", title: "Hosted sync command missing local SaaS flag", severity: "medium", recovery: "On this computer rerun hosted sync/tracker/auth test commands with SPEC_KITTY_ENABLE_SAAS_SYNC=1."}, "sync/tracker command lacks SPEC_KITTY_ENABLE_SAAS_SYNC=1")
		}
	}

	for _, rule := range failureRules {
		for _, pattern := range rule.patterns {
			if pattern.MatchString(text) {
				add(rule, "matched deterministic signature "+pattern.String())
				break
			}
		}
	}
	if len(out) == 0 && genericFailureSignal(text, obj) {
		add(failureRule{id: "generic_error", title: "Generic error signal", severity: "medium", recovery: "Inspect surrounding timeline context for the failed command and retry only after root cause is addressed."}, "generic execution failure signal")
	}
	return out
}

var genericFailureSignals = []*regexp.Regexp{
	rx(`(?i)(^|\s)Error:\s`),
	rx(`(?i)Traceback \(most recent call last\):`),
	rx(`(?i)\b(exit code|exit status|returncode|return code)\s*[:=]?\s*[1-9][0-9]*\b`),
	rx(`(?i)\b(command|hook|tool|process|subprocess|pytest|ruff|mypy|spec-kitty)\b.{0,80}\b(failed|failure)\b`),
	rx(`(?i)\b(failed|failure)\b.{0,80}\b(command|hook|tool|process|subprocess|pytest|ruff|mypy|spec-kitty)\b`),
}

func genericFailureSignal(text string, obj map[string]any) bool {
	if obj != nil {
		if raw, ok := obj["toolUseResult"].(string); ok {
			return genericFailureToolText(raw)
		}
		if result, ok := obj["toolUseResult"].(map[string]any); ok {
			if stdout, ok := result["stdout"].(string); ok {
				return genericFailureToolText(stdout)
			}
			if stderr, ok := result["stderr"].(string); ok {
				return genericFailureToolText(stderr)
			}
		}
	}
	return genericFailureText(text)
}

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
