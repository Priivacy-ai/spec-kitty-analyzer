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
{"type":"attachment","timestamp":"2026-07-12T09:52:20.000Z","attachment":{"type":"hook_success","hookName":"PreToolUse:Read","hookEvent":"PreToolUse","content":"ADMIT","command":"/usr/bin/some-other-tool check"}}
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
	// Latency over the four hooks (49, 31, 12, 8).
	if s.Latency.Count != 4 || s.Latency.MinMs != 8 || s.Latency.MaxMs != 49 {
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
	if !s.Ledger.Derived || s.Ledger.AdmissionDecisionsRecorded != 4 {
		t.Fatalf("ledger derived accounting wrong: %+v", s.Ledger)
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
