package analyzer

import "testing"

// TestFingerprintRulesAllScoped enforces the §7.1 explicit-scope gate: every text
// pattern in failureRules must declare an output or diagnostic scope. An unscoped
// pattern (the zero value scopeUnset) is silent false-negative debt — it would
// never be matched against any channel — so the build fails here rather than
// shipping a pattern that can never fire.
func TestFingerprintRulesAllScoped(t *testing.T) {
	for _, rule := range failureRules {
		if len(rule.patterns) == 0 {
			t.Errorf("rule %q has no patterns", rule.id)
			continue
		}
		for i, sp := range rule.patterns {
			switch sp.scope {
			case scopeOutput, scopeDiagnostic:
				// declared — ok
			default:
				t.Errorf("rule %q pattern[%d] %q is unscoped (scope=%d); every pattern must declare scopeOutput or scopeDiagnostic",
					rule.id, i, sp.re.String(), sp.scope)
			}
		}
	}
}

// TestFingerprintTaxonomyDiagnosticIsBranchOnly pins the §4 contract: the only
// rule allowed to carry diagnostic (narrative-eligible) patterns today is
// branch_worktree_confusion. If a future edit promotes another rule's generic
// pattern to diagnostic to chase recall, this fails.
func TestFingerprintTaxonomyDiagnosticIsBranchOnly(t *testing.T) {
	for _, rule := range failureRules {
		for i, sp := range rule.patterns {
			if sp.scope == scopeDiagnostic && rule.id != "branch_worktree_confusion" {
				t.Errorf("rule %q pattern[%d] %q is diagnostic-scoped; only branch_worktree_confusion may be diagnostic (§4)",
					rule.id, i, sp.re.String())
			}
		}
	}
	// branch_worktree_confusion is a MIXED rule after the §3b distinctiveness split
	// (Codex review cycle 2): its broad prose patterns are output-scoped, its
	// distinctive signatures are diagnostic-scoped. It is no longer *entirely*
	// diagnostic, but it must still carry AT LEAST ONE diagnostic pattern — that is
	// what keeps the corpus-proven #1716/#2046 narrative detection alive. If a future
	// edit demotes every branch pattern to output, narrative recall silently goes to
	// zero, so this guards the floor.
	for _, rule := range failureRules {
		if rule.id != "branch_worktree_confusion" {
			continue
		}
		diagnosticCount := 0
		for _, sp := range rule.patterns {
			if sp.scope == scopeDiagnostic {
				diagnosticCount++
			}
		}
		if diagnosticCount == 0 {
			t.Errorf("branch_worktree_confusion must keep at least one diagnostic-scoped pattern (the #1716/#2046 narrative detection); found none")
		}
	}
}

// TestFingerprintOutputScopedNarrativeNegative is the §7.2b guard. Generic merge
// prose that matches merge_operation_failed ("the merge failed because…") and
// merge_conflict ("resolve the merge conflict next") arrives ONLY in the
// diagnostic channel (narrative). Because those patterns are output-scoped, they
// must not classify. This is the most likely regression if a refactor accidentally
// re-broadens scope.
func TestFingerprintOutputScopedNarrativeNegative(t *testing.T) {
	narrative := "I think the merge failed because of stale refs; you should resolve the merge conflict next before retrying."
	failures := classifyFailuresWithChannels("", narrative, nil, nil)
	if len(failures) != 0 {
		t.Fatalf("output-scoped merge prose classified from narrative-only channel: %#v", failures)
	}
	if failureListHas(failures, "merge_operation_failed") {
		t.Errorf("merge_operation_failed must not classify from narrative (it is output-scoped)")
	}
	if failureListHas(failures, "merge_conflict") {
		t.Errorf("merge_conflict must not classify from narrative (it is output-scoped)")
	}
}

// TestFingerprintBranchWorktreeDiagnosticScope is the §7.2 Class-B regression
// guard, updated for the §3b distinctiveness split (Codex review cycle 2).
// branch_worktree_confusion is now a MIXED rule:
//   - (a) its DISTINCTIVE signature (`mission targets … branch`) is diagnostic-scoped
//     and MUST still classify when it appears only as narrative (diagnostic channel) —
//     this is the corpus-proven #1716/#2046 detection.
//   - (b) a BROAD-only narrative phrase ("you may be on the wrong branch") matches only
//     the now-output-scoped `wrong (branch|worktree)` pattern, so it MUST NOT classify
//     from a narrative-only channel — that broad prose is exactly the FP class this
//     mission eliminates.
//
// Conversely, when a signature reaches neither cached channel (simulating §3a-excluded
// code-edit/file-read content), nothing classifies.
func TestFingerprintBranchWorktreeDiagnosticScope(t *testing.T) {
	// (a) Distinctive narrative signature (the corpus #1716/#2046 shape: a report of an
	// observed condition, "mission targets … branch"). Present only in the diagnostic
	// channel; the output channel is empty.
	distinctive := "Branch context check: mission targets 'main' but the agent is on branch 'fix/task-workflow-bug-fixes'."
	got := classifyFailuresWithChannels("", distinctive, nil, nil)
	if !failureListHas(got, "branch_worktree_confusion") {
		t.Fatalf("distinctive branch signature must classify from diagnostic narrative: %#v", got)
	}

	// (b) Broad-only narrative prose. After the split this matches only the
	// `wrong (branch|worktree)` output-scoped pattern, so a narrative-only channel
	// (output empty) must NOT classify — otherwise the FP class returns.
	broadProse := "When reviewing, you may be on the wrong branch, so double-check before continuing."
	broad := classifyFailuresWithChannels("", broadProse, nil, nil)
	if failureListHas(broad, "branch_worktree_confusion") {
		t.Fatalf("broad branch prose must NOT classify from narrative (its pattern is output-scoped): %#v", broad)
	}

	// The same broad prose IN the output channel still classifies (the broad pattern
	// remains an output detector — it was demoted from narrative, not deleted).
	broadOut := classifyFailuresWithChannels(broadProse, broadProse, nil, nil)
	if !failureListHas(broadOut, "branch_worktree_confusion") {
		t.Fatalf("broad branch prose in OUTPUT channel must still classify: %#v", broadOut)
	}

	// Same signature excluded from BOTH channels (e.g. it only ever lived in a code
	// edit / file read that WP01 §3a strips) → no classification at all.
	none := classifyFailuresWithChannels("", "", nil, nil)
	if failureListHas(none, "branch_worktree_confusion") {
		t.Fatalf("branch_worktree_confusion must not classify when absent from both channels: %#v", none)
	}
	if len(none) != 0 {
		t.Fatalf("empty channels must yield no failures: %#v", none)
	}
}

// TestFingerprintOutputScopedClassifiesFromOutput is the positive counterpart:
// an output-scoped pattern fed via the output channel still classifies (proving
// the scope routing is wired, not merely suppressing everything). merge_conflict's
// distinctive git signature in output must classify.
func TestFingerprintOutputScopedClassifiesFromOutput(t *testing.T) {
	output := "Automatic merge failed; fix conflicts and then commit the result."
	got := classifyFailuresWithChannels(output, output, nil, nil)
	if !failureListHas(got, "merge_conflict") {
		t.Fatalf("merge_conflict must classify from output channel: %#v", got)
	}
	// The same git output present only in the narrative channel must NOT classify
	// (output-scoped patterns ignore diagnosticText-only content).
	narrativeOnly := classifyFailuresWithChannels("", output, nil, nil)
	if failureListHas(narrativeOnly, "merge_conflict") {
		t.Fatalf("merge_conflict must not classify from narrative-only channel: %#v", narrativeOnly)
	}
}

// TestFingerprintFileReadContentExcludedThroughWrapper is the real §7.2 / §3a
// exclusion guard (Codex review cycle 2 replaced the tautological empty-string
// version). It drives the PUBLIC wrapper end-to-end with a Claude file-read object
// (`toolUseResult.file.content`) whose content CONTAINS a distinctive
// branch_worktree_confusion signature. Even though flattenJSON(obj) surfaces that
// signature as text, the wrapper routes obj through the WP01 channel extraction,
// which excludes file-read content from BOTH channels (§3a), so nothing classifies.
// This cannot pass by accident: if file-read content ever leaked into the scan path,
// the embedded distinctive signature would classify and fail the test.
func TestFingerprintFileReadContentExcludedThroughWrapper(t *testing.T) {
	obj := map[string]any{
		"toolUseResult": map[string]any{
			"file": map[string]any{
				"filePath": "/tmp/notes.md",
				// Carries a distinctive (diagnostic-scoped) branch signature on purpose.
				"content": "mission targets 'main' but the agent is on branch 'fix/x'.",
			},
		},
	}
	failures := classifyFailures(flattenJSON(obj), obj, nil)
	if failureListHas(failures, "branch_worktree_confusion") {
		t.Fatalf("file-read content carrying a branch signature must be excluded (§3a): %#v", failures)
	}
	if len(failures) != 0 {
		t.Fatalf("file-read object must yield no failures: %#v", failures)
	}
}

// TestFingerprintCodeEditContentExcludedThroughWrapper is the companion §3a guard
// for code edits. A Claude Edit object (`toolUseResult.newString`) whose written text
// carries strong failure signatures (an Error line + an AssertionError) must NOT
// classify: writing code that mentions a failure is not an observed failure. The §3a
// code-edit exclusion keeps newString out of both channels.
func TestFingerprintCodeEditContentExcludedThroughWrapper(t *testing.T) {
	obj := map[string]any{
		"toolUseResult": map[string]any{
			"newString": "Error: Exit code 1\nraise AssertionError('boom')",
		},
	}
	failures := classifyFailures(flattenJSON(obj), obj, nil)
	if len(failures) != 0 {
		t.Fatalf("code-edit (newString) content carrying failure signatures must be excluded (§3a): %#v", failures)
	}
}
