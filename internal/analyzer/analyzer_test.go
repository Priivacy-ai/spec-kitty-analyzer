package analyzer

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestMissionHandleNormalizationRejectsNoise(t *testing.T) {
	if got := normalizeMissionHandle("task-workflow-bug-fixes-"); got != "task-workflow-bug-fixes" {
		t.Fatalf("trailing dash normalized to %q", got)
	}
	if got := normalizeMissionHandle("01KV69BZEHXDSGGMR6J3QN0J2E"); got != "" {
		t.Fatalf("standalone mission id normalized to %q want empty", got)
	}
}

func TestClassifyPathKindUsesKittySegments(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"kitty-specs/sample/tasks/WP01.md", "work_package"},
		{"repo/kitty-specs/sample/tasks/WP01.md", "work_package"},
		{"/tmp/repo/kitty-specs/sample/status.events.jsonl", "mission_status_events"},
		{"kitty-ops/01KTEST.jsonl", "op_jsonl"},
		{"notkitty-specs/sample/tasks/WP01.md", "text"},
	}
	for _, c := range cases {
		if got := classifyPathKind(c.path); got != c.want {
			t.Fatalf("classifyPathKind(%q)=%q want %q", c.path, got, c.want)
		}
	}

	path := "kitty-specs/sample/tasks/WP01.md"
	events, _ := parseFile(path, classifyPathKind(path), []byte("---\nreview_status: has_feedback\n---\n"), 0, newBuildState())
	if len(events) == 0 || !failureListHas(events[0].Failures, "review_rejected") {
		t.Fatalf("root-relative work package frontmatter must surface review_rejected; events=%#v", events)
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

func TestBranchWorktreeAndMergeFingerprints(t *testing.T) {
	branchText := "Exit code 1 Branch: on 'fix/task-workflow-bug-fixes', mission targets 'main'. Agent appears to be on the wrong worktree."
	branchFailures := classifyFailures(branchText, nil, nil)
	if !failureListHas(branchFailures, "branch_worktree_confusion") {
		t.Fatalf("branch failures=%#v want branch_worktree_confusion", branchFailures)
	}

	mergeText := "Error: merge preflight blocked because branch ref advance failed before merge."
	mergeFailures := classifyFailures(mergeText, nil, nil)
	if !failureListHas(mergeFailures, "merge_operation_failed") {
		t.Fatalf("merge failures=%#v want merge_operation_failed", mergeFailures)
	}
}

func TestPermissionDeniedFingerprintPrecision(t *testing.T) {
	positives := []string{
		// Structured tool/runtime error lines (the dominant real form).
		"ls: cannot open directory '/root': Permission denied",
		"open /etc/hosts: permission denied",
		"Error: EACCES: permission denied, open '/foo'",
		"bash: ./deploy.sh: Permission denied",
		// Fixed runtime signatures.
		"OSError: [Errno 13] Permission denied: '/etc/hosts'",
		"PermissionError: [Errno 13] Permission denied",
		"failed to write file (os error 13)",
		"git@github.com: Permission denied (publickey).",
	}
	for _, text := range positives {
		failures := classifyFailures(text, nil, nil)
		if !failureListHas(failures, "permission_denied") {
			t.Errorf("text %q: failures=%#v want permission_denied", text, failures)
		}
	}

	// Prose, control flow, and doc vocabulary that merely *discuss* the phrase must
	// NOT classify as a real denial. Every case below is a real false positive
	// observed in harness logs against the prior rules (em-dash prose, parenthetical
	// lists, `except` control flow, EACCES-as-vocabulary, string literals, and the
	// analyzer's own "findings :: Permission denied" output).
	negatives := []string{
		"Filesystem error — permission denied, disk full, cross-filesystem rename",
		"Exit code 2 contract: handle filesystem errors such as permission denied gracefully.",
		"if the write fails (disk full, permission denied), log the event",
		"    except PermissionError as e:",
		"documents flock(LOCK_NB) contention as EACCES or EAGAIN",
		`_completed(1, stderr="permission denied"),`,
		"/timeline/18/failures :: Permission denied",
	}
	for _, text := range negatives {
		failures := classifyFailures(text, nil, nil)
		if failureListHas(failures, "permission_denied") {
			t.Errorf("prose %q: failures=%#v want no permission_denied", text, failures)
		}
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

func TestFlattenJSONIsDeterministic(t *testing.T) {
	obj := map[string]any{
		"z": "last",
		"a": map[string]any{"b": "nested"},
		"m": []any{"middle"},
	}
	want := "nested middle last"
	for i := 0; i < 10; i++ {
		if got := flattenJSON(obj); got != want {
			t.Fatalf("flattenJSON=%q want %q", got, want)
		}
	}
}

func TestMissionFilterExcludesOtherMissionScope(t *testing.T) {
	report := Report{
		Version:     Version,
		GeneratedAt: time.Now(),
		Redactions:  map[string]int{},
		Surface:     defaultSurface(),
		Inputs:      []InputFile{{Path: "session.jsonl", Kind: "jsonl_transcript"}},
		Timeline: []TimelineEvent{
			{
				Seq:         1,
				SourcePath:  "session.jsonl",
				Scope:       Scope{Type: "mission", MissionSlug: "target-01KV"},
				TextPreview: "target-01KV",
			},
			{
				Seq:         2,
				SourcePath:  "session.jsonl",
				Scope:       Scope{Type: "mission", MissionSlug: "other-01KV"},
				TextPreview: "mentions target-01KV",
			},
		},
	}
	filtered := filterReportByMission(report, "target-01KV")
	if len(filtered.Missions) != 1 || filtered.Missions[0].Slug != "target-01KV" {
		t.Fatalf("missions=%#v", filtered.Missions)
	}
	if len(filtered.Timeline) != 1 {
		t.Fatalf("timeline len=%d want 1", len(filtered.Timeline))
	}
}

func TestWindowsPermissionDeniedSignatures(t *testing.T) {
	// Positive: Windows-native access-denied forms that contain no "permission
	// denied" substring and so were missed before issue #3. The first two are
	// VERBATIM strings captured from a real Windows 11 host (2026-06-23):
	//   python -c "import os; os.listdir(r'C:\System Volume Information')"
	//   rg test "C:\System Volume Information"
	positive := []struct {
		name string
		text string
	}{
		{"python_winerror_captured", `PermissionError: [WinError 5] Access is denied: 'C:\\System Volume Information'`},
		{"rust_os_error_5_captured", `C:\System Volume Information: Access is denied. (os error 5)`},
		{"rust_os_error_5_no_period", `caused by: Access is denied (os error 5)`}, // defensive: a Rust tool that omits the trailing period
	}
	for _, c := range positive {
		failures := classifyFailures(c.text, nil, nil)
		if !failureListHas(failures, "permission_denied") {
			t.Fatalf("%s: failures=%#v want permission_denied", c.name, failures)
		}
	}

	// Regression: the existing Unix "permission denied" recall must survive.
	unix := classifyFailures(`ls: cannot open directory '/root': Permission denied`, nil, nil)
	if !failureListHas(unix, "permission_denied") {
		t.Fatalf("unix denial dropped: failures=%#v want permission_denied", unix)
	}

	// Documented finding (captured 2026-06-23): the SAME Windows denial via the C
	// runtime (open()) rather than the Win32 API surfaces as the errno form
	// "[Errno 13] Permission denied", which the pre-existing bare rule already
	// catches. Both Windows Python denial forms are therefore covered.
	crt := classifyFailures(`PermissionError: [Errno 13] Permission denied: 'C:\\Windows'`, nil, nil)
	if !failureListHas(crt, "permission_denied") {
		t.Fatalf("windows CRT errno form dropped: failures=%#v want permission_denied", crt)
	}

	// Precision: a bare "(os error 5)" must NOT match, because on Unix errno 5 is
	// EIO ("Input/output error"), not an access denial. Guards against the naive
	// candidate regex that would misclassify Rust EIO on Linux/macOS.
	eio := classifyFailures(`Error: Input/output error (os error 5)`, nil, nil)
	if failureListHas(eio, "permission_denied") {
		t.Fatalf("Unix EIO misclassified as permission_denied: failures=%#v", eio)
	}
}

// TestIssue4FourWayReproThroughEvent is the headline acceptance test (Contract B).
// The SAME signature `AssertionError` appears in four channels of an event; only the
// stderr (output) line is a real failure once the event is wired through the channel
// extraction (WP01) + scoped classification (WP02). This exercises the WP03 wiring
// end to end via eventFromJSONObject (T014/T017a).
func TestIssue4FourWayReproThroughEvent(t *testing.T) {
	cases := []struct {
		name     string
		obj      map[string]any
		wantFail bool
		wantID   string
	}{
		{
			name: "assistant_message_text_narrative",
			obj: map[string]any{
				"type": "assistant",
				"message": map[string]any{
					"role": "assistant",
					"content": []any{
						map[string]any{"type": "text", "text": "Catch the AssertionError and log it before re-raising."},
					},
				},
			},
			wantFail: false,
		},
		{
			name: "edit_new_string_excluded",
			obj: map[string]any{
				"toolUseResult": map[string]any{
					"filePath":        "/repo/tests/test_x.py",
					"oldString":       "pass",
					"newString":       "raise AssertionError('boom')",
					"structuredPatch": []any{},
				},
			},
			wantFail: false,
		},
		{
			name: "codex_reasoning_narrative",
			obj: map[string]any{
				"payload": map[string]any{
					"type": "reasoning",
					"content": []any{
						map[string]any{"type": "text", "text": "Handle AssertionError defensively in the harness."},
					},
				},
			},
			wantFail: false,
		},
		{
			name: "tool_use_result_stderr_output",
			obj: map[string]any{
				"toolUseResult": map[string]any{
					"stdout": "",
					"stderr": "E       AssertionError: boom",
				},
			},
			wantFail: true,
			wantID:   "test_failure",
		},
	}
	for _, c := range cases {
		ev := eventFromJSONObject("session.jsonl", 1, 1, c.obj)
		got := len(ev.Failures) > 0
		if got != c.wantFail {
			t.Errorf("%s: failures=%#v want failure=%v", c.name, ev.Failures, c.wantFail)
			continue
		}
		if c.wantFail {
			if len(ev.Failures) != 1 || ev.Failures[0].ID != c.wantID {
				t.Errorf("%s: failures=%#v want exactly [%s]", c.name, ev.Failures, c.wantID)
			}
		}
	}
}

// TestObjNilChannelRouting covers the §3d plain-text model (Contract D) through
// the event path (T015/T017b).
func TestObjNilChannelRouting(t *testing.T) {
	// Contract D row 1: an artifact/spec line that merely discusses an error is
	// diagnostic-only, so output-scoped failure patterns never see it.
	artifactPath := "repo/kitty-specs/sample/research.md"
	proseLine := "The pytest run reported AssertionError: boom and exited with exit code 1."
	artifactOut, artifactDiag := channelStringsForEvent(artifactPath, proseLine, nil)
	if artifactOut != "" || !strings.Contains(artifactDiag, "AssertionError") {
		t.Fatalf("artifact prose must be diagnostic-only; out=%q diag=%q", artifactOut, artifactDiag)
	}
	preGate := eventFromText(artifactPath, 1, 1, proseLine, nil)
	if len(preGate.Failures) != 0 {
		t.Fatalf("artifact prose must not classify output-scoped failures pre-gate; got %#v", preGate.Failures)
	}
	events, _ := parseFile(artifactPath, "mission_artifact", []byte(proseLine+"\n"), 0, newBuildState())
	for _, e := range events {
		if len(e.Failures) > 0 {
			t.Fatalf("artifact prose surfaced a failure end-to-end: %#v", e.Failures)
		}
	}

	// The one whitelisted artifact signal — review_rejected on WP frontmatter — must
	// still surface via the frontmatter detector, not artifact-wide output routing.
	wpData := []byte("---\nreview_status: has_feedback\n---\n")
	wpEvents, _ := parseFile("repo/kitty-specs/sample/tasks/WP01.md", "work_package", wpData, 0, newBuildState())
	foundReview := false
	for _, e := range wpEvents {
		if failureListHas(e.Failures, "review_rejected") {
			foundReview = true
		}
	}
	if !foundReview {
		t.Fatalf("whitelisted review_rejected must survive on WP frontmatter; events=%#v", wpEvents)
	}

	researchEvents, _ := parseFile("repo/kitty-specs/sample/research.md", "mission_artifact", []byte("review_status: has_feedback\n"), 0, newBuildState())
	for _, e := range researchEvents {
		if failureListHas(e.Failures, "review_rejected") {
			t.Fatalf("research prose mentioning review_status must not surface review_rejected; events=%#v", researchEvents)
		}
	}
	researchJSON := []byte(`{"toolUseResult":{"stdout":"review_status: has_feedback"}}` + "\n")
	researchJSONEvents, _ := parseFile("repo/kitty-specs/sample/research.md", "mission_artifact", researchJSON, 0, newBuildState())
	for _, e := range researchJSONEvents {
		if failureListHas(e.Failures, "review_rejected") {
			t.Fatalf("research JSON output mentioning review_status must not surface review_rejected; events=%#v", researchJSONEvents)
		}
	}
	wpBodyJSONEvents, _ := parseFile("repo/kitty-specs/sample/tasks/WP01.md", "work_package", researchJSON, 0, newBuildState())
	for _, e := range wpBodyJSONEvents {
		if failureListHas(e.Failures, "review_rejected") {
			t.Fatalf("WP non-frontmatter JSON output mentioning review_status must not surface review_rejected; events=%#v", wpBodyJSONEvents)
		}
	}

	// Contract D row 2: a transcript-derived stray non-JSON line carrying real output
	// failure text is output-eligible and classifies.
	out, diag := channelStringsForEvent("session.jsonl", "AssertionError: boom", nil)
	if out == "" || diag == "" {
		t.Fatalf("transcript stray line must be output-eligible; out=%q diag=%q", out, diag)
	}
	strayEv := eventFromText("session.jsonl", 1, 1, "AssertionError: boom", nil)
	if !failureListHas(strayEv.Failures, "test_failure") {
		t.Fatalf("transcript stray failure=%#v want test_failure", strayEv.Failures)
	}

	// Direct .log files are command-output logs and remain output-eligible.
	logOut, logDiag := channelStringsForEvent("build.log", "AssertionError: boom", nil)
	if logOut == "" || logDiag == "" {
		t.Fatalf(".log command output must be output-eligible; out=%q diag=%q", logOut, logDiag)
	}
	logEv := eventFromText("build.log", 1, 1, "AssertionError: boom", nil)
	if !failureListHas(logEv.Failures, "test_failure") {
		t.Fatalf(".log command output must classify output failures; got %#v", logEv.Failures)
	}

	// Generic standalone .txt remains unsupported: no source kind proves it is command
	// output rather than prose.
	txtOut, txtDiag := channelStringsForEvent("notes.txt", "AssertionError: boom", nil)
	if txtOut != "" || txtDiag != "" {
		t.Fatalf("generic .txt must remain unsupported; out=%q diag=%q", txtOut, txtDiag)
	}
	txtEv := eventFromText("notes.txt", 1, 1, "AssertionError: boom", nil)
	if len(txtEv.Failures) != 0 {
		t.Fatalf("generic .txt must not classify; got %#v", txtEv.Failures)
	}
}

// TestArtifactSuppressionAllFourKinds covers Codex cycle-2 Fix 1: artifact-failure
// suppression must apply UNIFORMLY across all four artifact kinds, not just
// work_package/mission_artifact. A mission_meta (meta.json) and a
// mission_status_snapshot (status.json) each carrying a failure signature must reach
// findings with ZERO failures. Pre-gate the same object classifies, proving the gate
// (not a missing match) is what suppresses it.
func TestArtifactSuppressionAllFourKinds(t *testing.T) {
	cases := []struct {
		name string
		kind string
		path string
		data string
	}{
		{
			name: "mission_meta",
			kind: "mission_meta",
			path: "repo/kitty-specs/sample/meta.json",
			data: `{"error":"AssertionError: boom"}`,
		},
		{
			name: "mission_status_snapshot",
			kind: "mission_status_snapshot",
			path: "repo/kitty-specs/sample/status.json",
			data: `{"error":"AssertionError: boom"}`,
		},
	}
	for _, c := range cases {
		obj, ok := decodeJSONObject([]byte(c.data))
		if !ok {
			t.Fatalf("%s: test JSON did not decode", c.name)
		}
		preGate := eventFromJSONObject(c.path, 1, 1, obj)
		if len(preGate.Failures) == 0 {
			t.Fatalf("%s: object should classify a failure pre-gate (else the test proves nothing); got none", c.name)
		}
		events, _ := parseFile(c.path, c.kind, []byte(c.data+"\n"), 0, newBuildState())
		for _, e := range events {
			if len(e.Failures) > 0 {
				t.Fatalf("%s: artifact kind leaked a failure end-to-end: %#v", c.name, e.Failures)
			}
		}
	}
}

// TestReviewRejectedStructuralEndToEndStatusEvents is the integration guard for
// fast-follow item A. It drives the FULL pipeline (parseFile -> path-kind
// classification -> eventFromJSONObject -> classify -> skipArtifactMessage gate ->
// findings) from a status.events.jsonl source — kind mission_status_events, which is
// NOT an artifact kind, so the gate must let review_rejected through. The unit test
// (TestFingerprintReviewRejectedStructural) exercises the classifier directly; this
// covers the integration layer it skips (the layer where the Codex whitelist/gate
// finding lived). A rejection recorded as a bare structural field with no channel text
// is invisible to the pre-A channel scanner (baseline false negative); the structural
// detector must recover exactly the two rejection events and ignore the approval.
func TestReviewRejectedStructuralEndToEndStatusEvents(t *testing.T) {
	path := "repo/kitty-specs/demo/status.events.jsonl"
	lines := []string{
		// Rejection #1: top-level review_status on a wp_lane_changed status event.
		`{"event_type":"wp_lane_changed","wp_id":"WP01","to_lane":"planned","review_status":"has_feedback"}`,
		// Rejection #2: nested evidence.review.verdict on a review-evidence event.
		`{"event_id":"x","evidence":{"review":{"reviewer":"K","verdict":"rejected"}},"wp_id":"WP02"}`,
		// Not a rejection: an approved verdict must NOT classify.
		`{"event_id":"y","evidence":{"review":{"reviewer":"K","verdict":"approved"}},"wp_id":"WP03"}`,
	}
	data := strings.Join(lines, "\n") + "\n"

	events, _ := parseFile(path, "mission_status_events", []byte(data), 0, newBuildState())
	n := 0
	for _, e := range events {
		if failureListHas(e.Failures, "review_rejected") {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("review_rejected must surface end-to-end on exactly the 2 rejection events (has_feedback + verdict:rejected), not the approval; got %d. events=%#v", n, events)
	}
}

// TestArtifactReviewRejectedWhitelist proves WP frontmatter review_rejected survives
// while neighboring artifact diagnostic failures are still suppressed.
func TestArtifactReviewRejectedWhitelist(t *testing.T) {
	data := []byte("---\nreview_status: has_feedback\nnote: mission targets main branch\n---\n")
	wpPath := "repo/kitty-specs/sample/tasks/WP02.md"

	events, _ := parseFile(wpPath, "work_package", data, 0, newBuildState())
	var got []FailureFingerprint
	for _, e := range events {
		got = append(got, e.Failures...)
	}
	if len(got) != 1 || got[0].ID != "review_rejected" {
		t.Fatalf("exactly review_rejected must survive artifact suppression; got %#v", got)
	}
}

func TestArtifactReviewRejectedTitle(t *testing.T) {
	wpPath := "repo/kitty-specs/sample/tasks/WP03.md"
	events, _ := parseFile(wpPath, "work_package", []byte("---\nreview_status: has_feedback\n---\n"), 0, newBuildState())
	var surviving []TimelineEvent
	for _, e := range events {
		if len(e.Failures) > 0 {
			surviving = append(surviving, e)
		}
	}
	if len(surviving) != 1 {
		t.Fatalf("exactly one surviving failure event expected; got %#v", surviving)
	}
	e := surviving[0]
	if len(e.Failures) != 1 || e.Failures[0].ID != "review_rejected" {
		t.Fatalf("exactly review_rejected must survive; got %#v", e.Failures)
	}
	if e.Title != "Review rejected work package" {
		t.Fatalf("title must match review_rejected; got %q", e.Title)
	}
}

// TestContractCThroughEvent confirms the no-regression invariant (Contract C / SC-002)
// through the event path: the distinctive branch_worktree_confusion narrative still
// classifies (diagnostic-scoped), while generic output-scoped prose in narrative does
// not, and the same signature in command output does (T017c).
func TestContractCThroughEvent(t *testing.T) {
	narrative := func(text string) map[string]any {
		return map[string]any{
			"message": map[string]any{
				"content": []any{map[string]any{"type": "text", "text": text}},
			},
		}
	}

	// Distinctive branch_worktree_confusion narrative → still classified (diagnostic).
	bwc := eventFromJSONObject("session.jsonl", 1, 1, narrative(
		"You are on branch 'fix/foo' but the mission targets the 'main' branch. No auto-detection is performed for the branch."))
	if !failureListHas(bwc.Failures, "branch_worktree_confusion") {
		t.Fatalf("narrative branch_worktree_confusion regressed: %#v", bwc.Failures)
	}

	// Generic output-scoped prose in narrative → NOT classified (merge_operation_failed
	// and merge_conflict are output-scoped; narrative is not output).
	mergeProse := eventFromJSONObject("session.jsonl", 1, 1, narrative(
		"The merge failed earlier so let us resolve the merge conflict next."))
	if len(mergeProse.Failures) != 0 {
		t.Fatalf("generic merge prose in narrative must not classify: %#v", mergeProse.Failures)
	}

	// CONFLICT in command output → classified (output-scoped).
	conflictOut := eventFromJSONObject("session.jsonl", 1, 1, map[string]any{
		"toolUseResult": map[string]any{"stdout": "Auto-merging x\nCONFLICT (content): Merge conflict in x\n", "stderr": ""},
	})
	if !failureListHas(conflictOut.Failures, "merge_conflict") {
		t.Fatalf("CONFLICT in output must classify merge_conflict: %#v", conflictOut.Failures)
	}
}

// TestChannelCacheNotSerialized pins NFR-003: the cached channel strings are
// in-memory only and never leak into the serialized report schema.
func TestChannelCacheNotSerialized(t *testing.T) {
	ev := eventFromJSONObject("session.jsonl", 1, 1, map[string]any{
		"toolUseResult": map[string]any{"stderr": "AssertionError: boom"},
	})
	if ev.outputCh == "" || ev.diagnosticCh == "" {
		t.Fatalf("expected cached channel strings populated; outputCh=%q diagnosticCh=%q", ev.outputCh, ev.diagnosticCh)
	}
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	s := string(b)
	for _, leak := range []string{"outputCh", "diagnosticCh", "output_text", "diagnostic_text", "outputText", "diagnosticText"} {
		if strings.Contains(s, leak) {
			t.Fatalf("serialized event leaked channel cache key %q: %s", leak, s)
		}
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
