package gogov

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fixture models a realistic Claude Code transcript in which spec-kitty-go is
// wired as a governance hook and also invoked directly at the CLI. Verdict
// shapes follow the real `hook run` contract (cmd/spec-kitty/hook.go):
// "ADMIT" (exit 0), "DENY: <summary>" (exit 1), "DECISION_REQUIRED: <summary>"
// (exit 3). The tool_use blocks carry ids the hook attachments reference.
const fixture = `{"type":"assistant","timestamp":"2026-07-12T09:51:39.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_read1","name":"Read","input":{"file_path":"/tmp/work/README.md"}}]}}
{"type":"attachment","timestamp":"2026-07-12T09:51:39.772Z","attachment":{"type":"hook_success","hookName":"PreToolUse:Read","toolUseID":"toolu_read1","hookEvent":"PreToolUse","content":"ADMIT","stdout":"ADMIT\n","stderr":"","exitCode":0,"command":"/tmp/work/bin/spec-kitty hook run --adapter claude-code --event PreToolUse --governance-context-ref ctx/dogfood/x","durationMs":49}}
{"type":"assistant","timestamp":"2026-07-12T09:51:41.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_bash1","name":"Bash","input":{"command":"rm -rf /etc"}}]}}
{"type":"attachment","timestamp":"2026-07-12T09:51:42.000Z","attachment":{"type":"hook_error","hookName":"PreToolUse:Bash","toolUseID":"toolu_bash1","hookEvent":"PreToolUse","content":"DENY: destructive write outside workspace","stdout":"DENY: destructive write outside workspace\n","stderr":"","exitCode":1,"command":"/tmp/work/bin/spec-kitty hook run --adapter claude-code --event PreToolUse --governance-context-ref ctx/dogfood/x","durationMs":31}}
{"type":"attachment","timestamp":"2026-07-12T09:51:44.000Z","attachment":{"type":"hook_error","hookName":"PreToolUse:Bash","toolUseID":"toolu_bash2","hookEvent":"PreToolUse","content":"DECISION_REQUIRED: write to protected branch main","stdout":"DECISION_REQUIRED: write to protected branch main\n","stderr":"","exitCode":3,"command":"/tmp/work/bin/spec-kitty hook run --adapter claude-code --event PreToolUse --governance-context-ref ctx/dogfood/x","durationMs":12}}
{"type":"attachment","timestamp":"2026-07-12T09:51:45.000Z","attachment":{"type":"hook_success","hookName":"PostToolUse:Read","toolUseID":"toolu_read1","hookEvent":"PostToolUse","content":"ADMIT","stdout":"ADMIT\n","exitCode":0,"command":"/tmp/work/bin/spec-kitty hook run --adapter claude-code --event PostToolUse --governance-context-ref ctx/dogfood/x","durationMs":8}}
{"type":"assistant","timestamp":"2026-07-12T09:52:00.000Z","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"go run ./cmd/spec-kitty ledger verify && witness-sidecar verify-provenance att.json"}}]}}
{"type":"assistant","timestamp":"2026-07-12T09:52:10.000Z","message":{"role":"assistant","content":[{"type":"text","text":"I could run spec-kitty ledger verify but I will not."}]}}
{"type":"attachment","timestamp":"2026-07-12T09:52:20.000Z","attachment":{"type":"hook_success","hookName":"PreToolUse:Read","hookEvent":"PreToolUse","content":"lint ok","command":"/usr/bin/some-other-tool check"}}
`

func writeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestAnalyzeFiles_GovernanceHooks(t *testing.T) {
	path := writeFixture(t)
	rep := AnalyzeFiles([]string{path}, time.Unix(0, 0).UTC())
	s := rep.Summary

	// 4 spec-kitty-go hooks; the some-other-tool hook must be excluded.
	if s.GovernedActions != 4 {
		t.Fatalf("governed actions = %d, want 4 (the some-other-tool hook must be excluded)", s.GovernedActions)
	}
	if s.Verdicts[VerdictAdmit] != 2 || s.Verdicts[VerdictDeny] != 1 || s.Verdicts[VerdictDecisionRequired] != 1 {
		t.Fatalf("verdict breakdown wrong: %+v", s.Verdicts)
	}
	if s.Denials != 1 || s.DecisionsNeeded != 1 || s.Errors != 0 {
		t.Fatalf("denials=%d decisions=%d errors=%d, want 1,1,0", s.Denials, s.DecisionsNeeded, s.Errors)
	}
	if s.GovernedTools["Bash"] != 2 || s.GovernedTools["Read"] != 2 {
		t.Fatalf("governed tools wrong: %+v", s.GovernedTools)
	}
	if s.Adapters["claude-code"] != 4 {
		t.Fatalf("adapters wrong: %+v", s.Adapters)
	}
	if len(s.ContextRefs) != 1 || s.ContextRefs[0] != "ctx/dogfood/x" {
		t.Fatalf("context refs wrong: %+v", s.ContextRefs)
	}
	// Latency over the four hooks (sorted 8,12,31,49). Nearest-rank p50 = 12
	// (the 2nd of 4); a naive 0-indexed rank would wrongly report 31.
	if s.Latency.Count != 4 || s.Latency.MinMs != 8 || s.Latency.MaxMs != 49 ||
		s.Latency.P50Ms != 12 || s.Latency.P95Ms != 49 {
		t.Fatalf("latency wrong: %+v", s.Latency)
	}
}

func TestAnalyzeFiles_DenyAndDecisionReasons(t *testing.T) {
	path := writeFixture(t)
	rep := AnalyzeFiles([]string{path}, time.Unix(0, 0).UTC())

	if len(rep.Summary.Reasons) != 2 {
		t.Fatalf("reasons = %v, want 2 distinct", rep.Summary.Reasons)
	}
	var deny, decision *Event
	for i := range rep.Events {
		switch rep.Events[i].Verdict {
		case VerdictDeny:
			deny = &rep.Events[i]
		case VerdictDecisionRequired:
			decision = &rep.Events[i]
		}
	}
	if deny == nil || deny.Reason != "destructive write outside workspace" {
		t.Fatalf("deny reason not extracted: %+v", deny)
	}
	if decision == nil || decision.Reason != "write to protected branch main" {
		t.Fatalf("decision reason not extracted: %+v", decision)
	}
	// The DENY decision links to the exact governed Bash command.
	if deny.GovernedInput != "rm -rf /etc" {
		t.Fatalf("deny governed input not correlated: %q", deny.GovernedInput)
	}
}

func TestAnalyzeFiles_PrePostAndLedger(t *testing.T) {
	path := writeFixture(t)
	s := AnalyzeFiles([]string{path}, time.Unix(0, 0).UTC()).Summary

	if s.PreToolHooks != 3 || s.PostToolHooks != 1 {
		t.Fatalf("pre/post counts wrong: pre=%d post=%d", s.PreToolHooks, s.PostToolHooks)
	}
	// toolu_read1 has both a Pre and a Post hook -> one fully paired action.
	if s.PrePostPaired != 1 {
		t.Fatalf("pre/post paired = %d, want 1", s.PrePostPaired)
	}
	// Admission decisions = PreToolUse hooks only (3); the PostToolUse hook runs
	// after the side effect and is not an admission, so it must be excluded.
	if !s.Ledger.Derived || s.Ledger.AdmissionDecisionsRecorded != 3 {
		t.Fatalf("ledger derived accounting wrong (want 3 Pre-only admissions): %+v", s.Ledger)
	}
	if s.Ledger.LedgerCLIOps != 1 {
		t.Fatalf("ledger CLI ops = %d, want 1 (spec-kitty ledger verify)", s.Ledger.LedgerCLIOps)
	}
}

func TestAnalyzeFiles_CLIDetection(t *testing.T) {
	path := writeFixture(t)
	rep := AnalyzeFiles([]string{path}, time.Unix(0, 0).UTC())
	s := rep.Summary

	// The Bash tool_use holds two chained go invocations; the narrative text
	// block "I could run spec-kitty ledger verify" must NOT count.
	if s.CLIInvocations != 2 {
		t.Fatalf("cli invocations = %d, want 2 (narrative must be excluded): %+v", s.CLIInvocations, s.CLIVerbs)
	}
	if s.CLIVerbs["spec-kitty ledger verify"] != 1 {
		t.Fatalf("missing ledger verify: %+v", s.CLIVerbs)
	}
	if s.CLIVerbs["witness-sidecar verify-provenance"] != 1 {
		t.Fatalf("missing witness-sidecar: %+v", s.CLIVerbs)
	}
}

// TestWrapperHookDetection covers the DOG-GOV-03 shape: a wrapper hook
// (enrich-hook.py) whose transcript command is NOT the go binary, but whose
// stdout is spec-kitty-go's verdict passed through. It must still be attributed;
// a plain non-governance hook must not.
func TestWrapperHookDetection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wrapped.jsonl")
	lines := `{"type":"assistant","timestamp":"2026-07-12T13:40:00.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_w","name":"Write","input":{"file_path":"/home/specuser/dog-gov-03/MAIN_RELEASE.txt"}}]}}
{"type":"attachment","timestamp":"2026-07-12T13:40:01.000Z","attachment":{"type":"hook_error","hookName":"PreToolUse:Write","toolUseID":"toolu_w","hookEvent":"PreToolUse","content":"DECISION_REQUIRED: high-risk signed intent requires decision review","stdout":"DECISION_REQUIRED: high-risk signed intent requires decision review\n","exitCode":3,"command":"python3 /home/specuser/dog-gov-03/enrich-hook.py","durationMs":22}}
{"type":"attachment","timestamp":"2026-07-12T13:40:05.000Z","attachment":{"type":"hook_success","hookName":"PreToolUse:Bash","hookEvent":"PreToolUse","content":"formatted 3 files","stdout":"formatted 3 files\n","exitCode":0,"command":"python3 /some/prettier-hook.py"}}
`
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	rep := AnalyzeFiles([]string{path}, time.Unix(0, 0).UTC())
	if rep.Summary.GovernedActions != 1 {
		t.Fatalf("governed actions = %d, want 1 (wrapper verdict in, unrelated hook out)", rep.Summary.GovernedActions)
	}
	e := rep.Events[0]
	if e.Verdict != VerdictDecisionRequired || e.Reason != "high-risk signed intent requires decision review" {
		t.Fatalf("wrapper verdict/reason wrong: verdict=%q reason=%q", e.Verdict, e.Reason)
	}
	if e.GovernedTool != "Write" || e.GovernedInput != "/home/specuser/dog-gov-03/MAIN_RELEASE.txt" {
		t.Fatalf("wrapper correlation wrong: tool=%q input=%q", e.GovernedTool, e.GovernedInput)
	}
}

// TestVerdictNotEnforced models the DOG-GOV-03 finding: spec-kitty-go returns
// DENY/DECISION_REQUIRED (exit 1/3), the harness treats it as non-blocking, and
// the governed tool_use still produces a tool_result (executes). That pairing —
// non-ADMIT verdict + a tool_result for the same id — is the enforcement gap.
func TestVerdictNotEnforced(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notenforced.jsonl")
	lines := `{"type":"assistant","timestamp":"2026-07-12T11:57:44.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_write","name":"Write","input":{"file_path":"/home/x/MAIN_RELEASE.txt"}}]}}
{"type":"attachment","timestamp":"2026-07-12T11:57:45.000Z","attachment":{"type":"hook_error","hookName":"PreToolUse:Write","toolUseID":"toolu_write","hookEvent":"PreToolUse","content":"DECISION_REQUIRED: high-risk signed intent requires decision review","stdout":"DECISION_REQUIRED: high-risk signed intent requires decision review\n","exitCode":3,"command":"python3 /x/enrich-hook.py"}}
{"type":"user","timestamp":"2026-07-12T11:57:46.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_write","content":"File created successfully at: /home/x/MAIN_RELEASE.txt"}]}}
{"type":"assistant","timestamp":"2026-07-12T11:57:50.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_deny","name":"Bash","input":{"command":"git push origin main"}}]}}
{"type":"attachment","timestamp":"2026-07-12T11:57:51.000Z","attachment":{"type":"hook_error","hookName":"PreToolUse:Bash","toolUseID":"toolu_deny","hookEvent":"PreToolUse","content":"DENY: shell/exec forbidden under review","stdout":"DENY: shell/exec forbidden under review\n","exitCode":1,"command":"python3 /x/enrich-hook.py"}}
{"type":"assistant","timestamp":"2026-07-12T11:57:55.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_blocked","name":"Bash","input":{"command":"rm -rf /"}}]}}
{"type":"attachment","timestamp":"2026-07-12T11:57:56.000Z","attachment":{"type":"hook_error","hookName":"PreToolUse:Bash","toolUseID":"toolu_blocked","hookEvent":"PreToolUse","content":"DENY: catastrophic","stdout":"DENY: catastrophic\n","exitCode":1,"command":"python3 /x/enrich-hook.py"}}
`
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	rep := AnalyzeFiles([]string{path}, time.Unix(0, 0).UTC())

	// Only toolu_write (a DECISION_REQUIRED) has a tool_result, so it is the one
	// executed-despite-non-ADMIT case. The two DENYs have no tool_result, so they
	// must NOT be flagged not-enforced.
	if rep.Summary.UnenforcedVerdicts != 1 {
		t.Fatalf("unenforced verdicts = %d, want 1 (only the executed DECISION_REQUIRED)", rep.Summary.UnenforcedVerdicts)
	}
	var decision *Event
	for i := range rep.Events {
		if rep.Events[i].ToolUseID == "toolu_write" {
			decision = &rep.Events[i]
		}
		if rep.Events[i].ToolUseID == "toolu_blocked" && rep.Events[i].NotEnforced {
			t.Fatalf("toolu_blocked has no tool_result; must NOT be flagged not-enforced")
		}
	}
	if decision == nil || !decision.NotEnforced || decision.Executed == nil || !*decision.Executed {
		t.Fatalf("executed DECISION_REQUIRED not flagged not-enforced: %+v", decision)
	}
}

func TestIsGovernanceVerdictOutput(t *testing.T) {
	yes := []string{"ADMIT", "ADMIT\n", "DENY: nope", "DECISION_REQUIRED: ask a human", "deny: lower ok"}
	no := []string{"", "formatted 3 files", "I would ADMIT this normally", "all good, no deny here"}
	for _, s := range yes {
		if !isGovernanceVerdictOutput(s, "") {
			t.Errorf("expected %q to be a governance verdict", s)
		}
	}
	for _, s := range no {
		if isGovernanceVerdictOutput(s, "") {
			t.Errorf("expected %q NOT to be a governance verdict", s)
		}
	}
}

func TestAcceptGoInvocation_VerbGating(t *testing.T) {
	cases := []struct {
		prefix, binary, verb string
		want                 bool
	}{
		{"", "spec-kitty", "hook", true},                   // distinctive top-level verb, bare ok
		{"", "spec-kitty", "ledger", true},                 // distinctive top-level verb, bare ok
		{"", "spec-kitty", "review", true},                 // distinctive top-level verb, bare ok
		{"", "spec-kitty", "space", true},                  // distinctive top-level verb, bare ok
		{"", "spec-kitty", "specify", false},               // python verb, reject
		{"", "spec-kitty", "dispatch", false},              // python verb, reject
		{"", "spec-kitty", "seal", false},                  // NOT a top-level verb (it is `ledger seal`)
		{"", "spec-kitty", "governance", false},            // not in the pinned verb surface
		{"", "spec-kitty", "charter", false},               // python-only; not a go top-level verb
		{"", "spec-kitty", "init", false},                  // python-only; not a go top-level verb
		{"", "spec-kitty", "config", false},                // ambiguous, no path prefix -> reject
		{"/tmp/bin/", "spec-kitty", "config", true},        // ambiguous, path-prefixed -> accept
		{"./cmd/", "spec-kitty", "version", true},          // ambiguous, cmd-prefixed -> accept
		{"", "witness-sidecar", "verify-provenance", true}, // go-only binary, always
	}
	for _, c := range cases {
		if got := acceptGoInvocation(c.prefix, c.binary, c.verb); got != c.want {
			t.Errorf("acceptGoInvocation(%q,%q,%q) = %v, want %v", c.prefix, c.binary, c.verb, got, c.want)
		}
	}
}

func TestVerdictFrom(t *testing.T) {
	i := func(n int) *int { return &n }
	cases := []struct {
		name                      string
		content, stdout, hookType string
		exit                      *int
		wantVerdict, wantReason   string
	}{
		{"admit token", "ADMIT", "ADMIT\n", "hook_success", i(0), VerdictAdmit, ""},
		// Anchoring: trailing prose that merely mentions a verdict word must not
		// flip the classification (finding #2).
		{"admit with prose mentioning deny", "ADMIT\npolicy: would DENY outside workspace", "", "hook_success", i(0), VerdictAdmit, ""},
		{"deny reason mentioning decision word", "DENY: blocked (was DECISION_REQUIRED earlier)", "", "hook_error", i(1), VerdictDeny, "blocked (was DECISION_REQUIRED earlier)"},
		{"deny token+reason", "DENY: bad path", "DENY: bad path\n", "hook_error", i(1), VerdictDeny, "bad path"},
		{"decision token+reason", "DECISION_REQUIRED: protected branch", "", "hook_error", i(3), VerdictDecisionRequired, "protected branch"},
		// exit-code fallback when a harness recorded no textual content:
		{"deny by exit 1", "", "", "hook_error", i(1), VerdictDeny, ""},
		{"decision by exit 3", "", "", "hook_error", i(3), VerdictDecisionRequired, ""},
		{"usage error exit 2", "", "", "hook_error", i(2), VerdictError, ""},
		{"clean exit no token", "", "", "hook_success", i(0), VerdictUnknown, ""},
	}
	for _, c := range cases {
		gotVerdict, gotReason := verdictFrom(c.content, c.stdout, c.hookType, c.exit)
		if gotVerdict != c.wantVerdict || gotReason != c.wantReason {
			t.Errorf("%s: verdictFrom = (%q,%q), want (%q,%q)", c.name, gotVerdict, gotReason, c.wantVerdict, c.wantReason)
		}
	}
}

// TestTopLevelMessageNarrativeExcluded guards finding #1: a top-level string
// "message" is assistant/user narrative and must never register as a real CLI
// invocation, even when it quotes a distinctive go verb.
func TestTopLevelMessageNarrativeExcluded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "narrative.jsonl")
	lines := `{"type":"summary","timestamp":"2026-07-12T10:00:00.000Z","message":"I could run spec-kitty ledger verify and witness-sidecar verify-provenance but I will not."}
{"type":"assistant","timestamp":"2026-07-12T10:00:01.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"spec-kitty ledger verify"}}]}}
`
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	s := AnalyzeFiles([]string{path}, time.Unix(0, 0).UTC()).Summary
	// Only the real Bash tool_use invocation counts; the narrative string must not.
	if s.CLIInvocations != 1 || s.CLIVerbs["spec-kitty ledger verify"] != 1 {
		t.Fatalf("narrative message leaked into CLI invocations: count=%d verbs=%+v", s.CLIInvocations, s.CLIVerbs)
	}
	if s.Ledger.LedgerCLIOps != 1 {
		t.Fatalf("ledger CLI ops = %d, want 1 (narrative must not inflate)", s.Ledger.LedgerCLIOps)
	}
}

// TestExecutedIgnoresErrorResult guards finding #3: a tool_result carrying
// is_error:true is ambiguous (a blocked tool can still emit one) and must NOT
// count as "executed", so a correctly-enforced DENY is not mislabeled
// not-enforced.
func TestExecutedIgnoresErrorResult(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "errresult.jsonl")
	lines := `{"type":"assistant","timestamp":"2026-07-12T10:00:00.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_e","name":"Bash","input":{"command":"git push origin main"}}]}}
{"type":"attachment","timestamp":"2026-07-12T10:00:01.000Z","attachment":{"type":"hook_error","hookName":"PreToolUse:Bash","toolUseID":"toolu_e","hookEvent":"PreToolUse","content":"DENY: shell/exec forbidden under review","exitCode":1,"command":"/bin/spec-kitty hook run --adapter claude-code --event PreToolUse"}}
{"type":"user","timestamp":"2026-07-12T10:00:02.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_e","is_error":true,"content":"blocked by hook"}]}}
`
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	rep := AnalyzeFiles([]string{path}, time.Unix(0, 0).UTC())
	if rep.Summary.UnenforcedVerdicts != 0 {
		t.Fatalf("unenforced verdicts = %d, want 0 (is_error result is not proof of execution)", rep.Summary.UnenforcedVerdicts)
	}
	if rep.Events[0].Executed != nil {
		t.Fatalf("Executed should be nil for an is_error tool_result, got %v", *rep.Events[0].Executed)
	}
}

// TestPrePostPairingScopedByFile guards finding #6: the same tool-use id in two
// different transcripts represents two different governed actions and must not
// be paired across files.
func TestPrePostPairingScopedByFile(t *testing.T) {
	dir := t.TempDir()
	pre := `{"type":"attachment","timestamp":"2026-07-12T10:00:00.000Z","attachment":{"type":"hook_success","hookName":"PreToolUse:Read","toolUseID":"toolu_shared","hookEvent":"PreToolUse","content":"ADMIT","exitCode":0,"command":"/bin/spec-kitty hook run"}}` + "\n"
	post := `{"type":"attachment","timestamp":"2026-07-12T10:00:05.000Z","attachment":{"type":"hook_success","hookName":"PostToolUse:Read","toolUseID":"toolu_shared","hookEvent":"PostToolUse","content":"ADMIT","exitCode":0,"command":"/bin/spec-kitty hook run"}}` + "\n"
	p1 := filepath.Join(dir, "a.jsonl")
	p2 := filepath.Join(dir, "b.jsonl")
	if err := os.WriteFile(p1, []byte(pre), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p2, []byte(post), 0o600); err != nil {
		t.Fatal(err)
	}
	s := AnalyzeFiles([]string{p1, p2}, time.Unix(0, 0).UTC()).Summary
	if s.PrePostPaired != 0 {
		t.Fatalf("pre/post paired = %d, want 0 (same id in different files must not pair)", s.PrePostPaired)
	}
}

// TestHookEventInferredFromHookName guards the follow-up completeness fix: a
// wrapper hook that omits the hookEvent field and the --event flag but names its
// event in hookName ("PreToolUse:Bash") must still be counted as a PreToolUse
// admission (ledger + pairing), not silently dropped.
func TestHookEventInferredFromHookName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noevent.jsonl")
	lines := `{"type":"attachment","timestamp":"2026-07-12T10:00:00.000Z","attachment":{"type":"hook_error","hookName":"PreToolUse:Bash","toolUseID":"toolu_x","content":"DENY: nope","stdout":"DENY: nope\n","exitCode":1,"command":"python3 /x/enrich-hook.py"}}` + "\n"
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	rep := AnalyzeFiles([]string{path}, time.Unix(0, 0).UTC())
	s := rep.Summary
	if rep.Events[0].HookEvent != "PreToolUse" {
		t.Fatalf("hook event not inferred from hookName: %q", rep.Events[0].HookEvent)
	}
	if s.PreToolHooks != 1 || s.Ledger.AdmissionDecisionsRecorded != 1 {
		t.Fatalf("wrapper admission undercounted: pre=%d admissions=%d", s.PreToolHooks, s.Ledger.AdmissionDecisionsRecorded)
	}
}

// TestExecutedOKAmongMixedResults guards the follow-up refinement: a non-error
// tool_result proves execution even if an is_error result also exists for the
// same id (retry/duplicate blocks), so a DENY that ultimately ran is flagged.
func TestExecutedOKAmongMixedResults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mixed.jsonl")
	lines := `{"type":"assistant","timestamp":"2026-07-12T10:00:00.000Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_m","name":"Bash","input":{"command":"git push origin main"}}]}}
{"type":"attachment","timestamp":"2026-07-12T10:00:01.000Z","attachment":{"type":"hook_error","hookName":"PreToolUse:Bash","toolUseID":"toolu_m","hookEvent":"PreToolUse","content":"DENY: forbidden","exitCode":1,"command":"/bin/spec-kitty hook run"}}
{"type":"user","timestamp":"2026-07-12T10:00:02.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_m","is_error":true,"content":"transient error"}]}}
{"type":"user","timestamp":"2026-07-12T10:00:03.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_m","content":"pushed"}]}}
`
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	rep := AnalyzeFiles([]string{path}, time.Unix(0, 0).UTC())
	if rep.Summary.UnenforcedVerdicts != 1 {
		t.Fatalf("unenforced verdicts = %d, want 1 (a non-error result proves execution)", rep.Summary.UnenforcedVerdicts)
	}
}

// TestPercentileNearestRank guards finding #5 (off-by-one).
func TestPercentileNearestRank(t *testing.T) {
	twenty := make([]int, 20)
	for i := range twenty {
		twenty[i] = (i + 1) * 10 // 10..200
	}
	if got := percentile(twenty, 95); got != 190 {
		t.Errorf("p95 of (10..200) = %d, want 190 (19th value), not the max", got)
	}
	four := []int{8, 12, 31, 49}
	if got := percentile(four, 50); got != 12 {
		t.Errorf("p50 of [8,12,31,49] = %d, want 12", got)
	}
}

func TestAnalyzeFiles_NoActivity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"user","message":{"role":"user","content":"hello"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rep := AnalyzeFiles([]string{path}, time.Unix(0, 0).UTC())
	if rep.Summary.GovernedActions != 0 || rep.Summary.CLIInvocations != 0 {
		t.Fatalf("expected no activity, got %+v", rep.Summary)
	}
	if len(rep.Notes) == 0 {
		t.Fatalf("expected a note explaining no activity was found")
	}
}
