package analyzer

import (
	"path/filepath"
	"testing"
)

func TestAnalyzeFixture(t *testing.T) {
	report, err := Analyze([]string{filepath.Join("..", "..", "testdata", "fixture")})
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if report.Summary.Missions != 1 {
		t.Fatalf("missions=%d want 1", report.Summary.Missions)
	}
	if report.Summary.Ops != 1 {
		t.Fatalf("ops=%d want 1", report.Summary.Ops)
	}
	if report.Summary.OpenOps != 1 {
		t.Fatalf("open ops=%d want 1", report.Summary.OpenOps)
	}
	if report.Summary.UniqueCommands == 0 {
		t.Fatalf("expected slash command detection")
	}
	if report.Summary.UniqueSkills == 0 {
		t.Fatalf("expected skill detection")
	}
	if report.Summary.CLIInvocations < 3 {
		t.Fatalf("cli invocations=%d want >=3", report.Summary.CLIInvocations)
	}
	wantFindings := []string{
		"wrong_cli_surface",
		"runtime_blocked",
		"missing_artifact",
		"guard_failure",
		"completed_not_terminal_runtime_bug",
		"saas_sync_flag_missing",
		"review_rejected",
		"open_op_orphan",
	}
	for _, id := range wantFindings {
		if !hasFinding(report, id) {
			t.Fatalf("missing finding %s; got %#v", id, report.Findings)
		}
	}
	m := report.Missions[0]
	if m.Slug != "sample-mission" || m.MissionType != "software-dev" {
		t.Fatalf("bad mission summary: %#v", m)
	}
	if len(m.WorkPackages) != 1 || m.WorkPackages[0].ReviewStatus != "has_feedback" {
		t.Fatalf("bad WP summary: %#v", m.WorkPackages)
	}
}

func TestMissionHandleFiltering(t *testing.T) {
	text := "spec-kitty next --mission <slug>\nspec-kitty status --mission {mission_slug}\nspec-kitty next --mission real-feature-01KV"
	invocations := detectCLIInvocations(text)
	if len(invocations) != 3 {
		t.Fatalf("invocations=%d want 3", len(invocations))
	}
	if invocations[0].Mission != "" || invocations[1].Mission != "" {
		t.Fatalf("placeholder missions should be empty: %#v", invocations[:2])
	}
	if invocations[2].Mission != "real-feature-01KV" {
		t.Fatalf("actual mission=%q", invocations[2].Mission)
	}

	scope := scopeFromPathAndText("session.jsonl", "run spec-kitty next --mission <slug>", invocations[:1], nil)
	if scope.Type != "outside" || scope.MissionSlug != "" {
		t.Fatalf("placeholder scope=%#v want outside", scope)
	}
}

func TestSourceReadDoesNotBecomeFailure(t *testing.T) {
	sourceRead := map[string]any{
		"toolUseResult": map[string]any{
			"type": "text",
			"file": map[string]any{
				"filePath": "/tmp/source.py",
				"content":  "raise TaskCliError(\"Mission slug is required\")",
			},
		},
	}
	if failures := classifyFailures(flattenJSON(sourceRead), sourceRead, nil); len(failures) != 0 {
		t.Fatalf("source read failures=%#v want none", failures)
	}

	commandResult := map[string]any{
		"toolUseResult": "Error: Exit code 1\nFound 2 errors.",
	}
	failures := classifyFailures(flattenJSON(commandResult), commandResult, nil)
	if !failureListHas(failures, "generic_error") {
		t.Fatalf("command result failures=%#v want generic_error", failures)
	}
}

func TestSearchSnippetsDoNotBecomeFailures(t *testing.T) {
	text := `321: raise TaskCliError("Required artifact missing: plan.md")
322: message = "type conflict"
323: shell_pid=args.shell_pid or ""`
	if failures := classifyFailures(text, nil, nil); len(failures) != 0 {
		t.Fatalf("search snippet failures=%#v want none", failures)
	}
}

func TestStructuredOpContextDoesNotBecomeGenericFailure(t *testing.T) {
	stringResult := map[string]any{
		"toolUseResult": `{"invocation_id":"01KOP","profile_id":"python-pedro","action":"generate","governance_context_text":"Handle command failure carefully."}`,
	}
	if failures := classifyFailures(flattenJSON(stringResult), stringResult, nil); len(failures) != 0 {
		t.Fatalf("op context failures=%#v want none", failures)
	}
	stdoutResult := map[string]any{
		"toolUseResult": map[string]any{
			"stdout": `{"invocation_id":"01KOP","profile_id":"python-pedro","action":"generate","governance_context_text":"Handle command failure carefully.","status":"open"}`,
			"stderr": "",
		},
	}
	if failures := classifyFailures(flattenJSON(stdoutResult), stdoutResult, nil); len(failures) != 0 {
		t.Fatalf("op context failures=%#v want none", failures)
	}
}

func failureListHas(failures []FailureFingerprint, id string) bool {
	for _, failure := range failures {
		if failure.ID == id {
			return true
		}
	}
	return false
}

func hasFinding(report Report, id string) bool {
	for _, f := range report.Findings {
		if f.ID == id {
			return true
		}
	}
	return false
}
