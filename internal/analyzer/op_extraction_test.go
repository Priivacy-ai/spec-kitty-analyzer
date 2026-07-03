package analyzer

import "testing"

// TestFirstInvocationIDTextValidatesToken guards #24: a bare substring scan for
// `invocation_id` must not capture struct tags / prose (e.g. the literal
// "omitempty" from `json:"invocation_id,omitempty"`); only real invocation-id
// shapes (32-hex / 26-ULID) are accepted.
func TestFirstInvocationIDTextValidatesToken(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"struct tag omitempty", "InvocationID string `json:\"invocation_id,omitempty\"`", ""},
		{"prose mention", "the invocation_id field is optional here", ""},
		{"valid hex32 json", `"invocation_id": "c03904a5846f4232a42bf568546961d8"`, "c03904a5846f4232a42bf568546961d8"},
		{"valid ulid eq", "invocation_id=01KWMBB878K806KR7N6YAV67EK", "01KWMBB878K806KR7N6YAV67EK"},
		{"absent", "nothing to see here", ""},
	}
	for _, c := range cases {
		if got := firstInvocationIDText(c.text); got != c.want {
			t.Errorf("%s: firstInvocationIDText(%q)=%q want %q", c.name, c.text, got, c.want)
		}
	}
}

// TestOpenOpOrphanRequiresRealOpEvent guards #25: an op synthesized from an
// invocation_id merely mentioned in transcript/prose (no kitty-ops op log) must
// not be flagged as an orphaned Op; a real kitty-ops-backed open op must be.
func TestOpenOpOrphanRequiresRealOpEvent(t *testing.T) {
	synthetic := []OpSummary{{InvocationID: "c03904a5846f4232a42bf568546961d8", Status: "open", sawOpEvent: false}}
	for _, f := range buildFindings(nil, synthetic) {
		if f.ID == "open_op_orphan" {
			t.Fatalf("open_op_orphan fired for a text-synthesized op (should be gated by sawOpEvent)")
		}
	}

	real := []OpSummary{{InvocationID: "c03904a5846f4232a42bf568546961d8", Status: "open", sawOpEvent: true}}
	found := false
	for _, f := range buildFindings(nil, real) {
		if f.ID == "open_op_orphan" {
			found = true
		}
	}
	if !found {
		t.Fatal("open_op_orphan did not fire for a real kitty-ops-backed open op")
	}
}

// TestIsOpLogPath guards the Codex review fix: the op-log signal must match a
// real kitty-ops/<id>.jsonl source whether the path is absolute or a relative
// top-level path (no leading slash) — else genuine orphan findings get suppressed.
func TestIsOpLogPath(t *testing.T) {
	id := "c03904a5846f4232a42bf568546961d8"
	yes := []string{
		"kitty-ops/" + id + ".jsonl",                   // relative top-level (the MEDIUM finding)
		"./kitty-ops/" + id + ".jsonl",                 // relative with dot
		"/Users/x/.kittify/kitty-ops/" + id + ".jsonl", // absolute
	}
	no := []string{
		"/Users/x/.claude/projects/session.jsonl", // harness transcript
		"kitty-specs/mission/status.events.jsonl", // mission event log, not an op log
		"kitty-ops/" + id + ".txt",                // wrong extension
		"",
	}
	for _, p := range yes {
		if !isOpLogPath(p) {
			t.Errorf("isOpLogPath(%q)=false want true", p)
		}
	}
	for _, p := range no {
		if isOpLogPath(p) {
			t.Errorf("isOpLogPath(%q)=true want false", p)
		}
	}
}
