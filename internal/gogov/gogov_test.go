package gogov

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fixture models a realistic Claude Code transcript in which spec-kitty-go is
// wired as a PreToolUse governance hook and also invoked directly at the CLI.
const fixture = `{"type":"attachment","timestamp":"2026-07-12T09:51:39.772Z","attachment":{"type":"hook_success","hookName":"PreToolUse:Read","hookEvent":"PreToolUse","content":"ADMIT","stdout":"ADMIT\n","stderr":"","exitCode":0,"command":"/tmp/work/bin/spec-kitty hook run --adapter claude-code --event PreToolUse --governance-context-ref ctx/dogfood/x","durationMs":49}}
{"type":"attachment","timestamp":"2026-07-12T09:51:42.000Z","attachment":{"type":"hook_success","hookName":"PreToolUse:Bash","hookEvent":"PreToolUse","content":"DENY","stdout":"DENY\n","stderr":"","exitCode":0,"command":"/tmp/work/bin/spec-kitty hook run --adapter claude-code --event PreToolUse --governance-context-ref ctx/dogfood/x","durationMs":31}}
{"type":"attachment","timestamp":"2026-07-12T09:51:44.000Z","attachment":{"type":"hook_error","hookName":"PreToolUse:Bash","hookEvent":"PreToolUse","content":"","stdout":"","stderr":"panic: boom","exitCode":2,"command":"/tmp/work/bin/spec-kitty hook run --adapter claude-code --event PreToolUse","durationMs":12}}
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

	if s.GovernedActions != 3 {
		t.Fatalf("governed actions = %d, want 3 (the some-other-tool hook must be excluded)", s.GovernedActions)
	}
	if s.Verdicts[VerdictAdmit] != 1 || s.Verdicts[VerdictDeny] != 1 || s.Verdicts[VerdictError] != 1 {
		t.Fatalf("verdict breakdown wrong: %+v", s.Verdicts)
	}
	if s.Denials != 1 || s.Errors != 1 {
		t.Fatalf("denials=%d errors=%d, want 1 and 1", s.Denials, s.Errors)
	}
	if s.GovernedTools["Bash"] != 2 || s.GovernedTools["Read"] != 1 {
		t.Fatalf("governed tools wrong: %+v", s.GovernedTools)
	}
	if s.Adapters["claude-code"] != 3 {
		t.Fatalf("adapters wrong: %+v", s.Adapters)
	}
	if len(s.ContextRefs) != 1 || s.ContextRefs[0] != "ctx/dogfood/x" {
		t.Fatalf("context refs wrong: %+v", s.ContextRefs)
	}
	// Latency over the three hooks (49, 31, 12).
	if s.Latency.Count != 3 || s.Latency.MinMs != 12 || s.Latency.MaxMs != 49 {
		t.Fatalf("latency wrong: %+v", s.Latency)
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
		{"", "spec-kitty", "hook", true},                 // distinctive verb, bare ok
		{"", "spec-kitty", "ledger", true},               // distinctive verb, bare ok
		{"", "spec-kitty", "specify", false},             // python verb, reject
		{"", "spec-kitty", "dispatch", false},            // python verb, reject
		{"", "spec-kitty", "config", false},              // ambiguous, no path prefix -> reject
		{"/tmp/bin/", "spec-kitty", "config", true},      // ambiguous, path-prefixed -> accept
		{"./cmd/", "spec-kitty", "version", true},        // ambiguous, cmd-prefixed -> accept
		{"", "witness-sidecar", "verify-provenance", true}, // go-only binary, always
	}
	for _, c := range cases {
		if got := acceptGoInvocation(c.prefix, c.binary, c.verb); got != c.want {
			t.Errorf("acceptGoInvocation(%q,%q,%q) = %v, want %v", c.prefix, c.binary, c.verb, got, c.want)
		}
	}
}

func TestVerdictFrom(t *testing.T) {
	two := 2
	zero := 0
	cases := []struct {
		content, stdout, hookType string
		exit                      *int
		want                      string
	}{
		{"ADMIT", "ADMIT\n", "hook_success", &zero, VerdictAdmit},
		{"DENY", "DENY\n", "hook_success", &zero, VerdictDeny},
		{"DECISION_REQUIRED", "", "hook_success", &zero, VerdictDecisionRequired},
		{"", "", "hook_error", &two, VerdictError},
		{"", "", "hook_success", &two, VerdictError}, // non-zero exit still errors
		{"", "", "hook_success", &zero, VerdictUnknown},
	}
	for _, c := range cases {
		if got := verdictFrom(c.content, c.stdout, c.hookType, c.exit); got != c.want {
			t.Errorf("verdictFrom(%q,%q,%q) = %q, want %q", c.content, c.stdout, c.hookType, got, c.want)
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
