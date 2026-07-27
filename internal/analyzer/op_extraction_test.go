package analyzer

import (
	"strings"
	"testing"
)

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
		"kitty-ops/lifecycle.jsonl",               // #43: non-op file under kitty-ops/ (was a phantom op + false open_op_orphan)
		"kitty-ops/01KTEST.jsonl",                 // #43: too-short id is not a valid invocation id
		"kitty-ops/" + id + ".jsonl.tmp",          // #43: trailing suffix must not match as a substring
		"kitty-ops/archive/" + id + ".jsonl",      // #43: valid id in a SUBDIR is not a live op log (direct child only)
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

// findingListHas reports whether a []Finding contains a finding with the given id.
func findingListHas(findings []Finding, id string) bool {
	for _, f := range findings {
		if f.ID == id {
			return true
		}
	}
	return false
}

// findingEvidenceContains reports whether the finding with id has evidence text
// containing substr.
func findingEvidenceContains(findings []Finding, id, substr string) bool {
	for _, f := range findings {
		if f.ID != id {
			continue
		}
		for _, e := range f.Evidence {
			if strings.Contains(e.Text, substr) {
				return true
			}
		}
	}
	return false
}

// opLogPath is a real ULID op-log path used by the #43 op-fault tests.
const opLogPath = "kitty-ops/01KTTCEAF0WTVAHYGND1D16R68.jsonl"

// TestOpLogGateAgreement guards the #43 consistency fix: classifyPathKind=="op_jsonl"
// and isOpLogPath must agree for every path (they share isOpLogBasename), including
// case variants and the trailing-child edge case Codex flagged.
func TestOpLogGateAgreement(t *testing.T) {
	id := "01KTTCEAF0WTVAHYGND1D16R68"
	paths := []string{
		"kitty-ops/" + id + ".jsonl",                       // canonical uppercase ULID
		"kitty-ops/" + strings.ToLower(id) + ".jsonl",      // lowercase ULID — must agree, not split
		"kitty-ops/c03904a5846f4232a42bf568546961d8.jsonl", // 32-hex
		"repo/.kittify/kitty-ops/" + id + ".jsonl",         // nested absolute-ish
		"kitty-ops/lifecycle.jsonl",                        // non-op file
		"kitty-ops/notes.md",                               // non-jsonl
		"kitty-ops/" + id + ".jsonl/child.jsonl",           // trailing child dir must NOT be an op log
		"kitty-ops/archive/" + id + ".jsonl",               // valid id in a subdir — direct child only
		"kitty-specs/m/status.events.jsonl",                // not under kitty-ops
	}
	for _, p := range paths {
		classified := classifyPathKind(p) == "op_jsonl"
		if classified != isOpLogPath(p) {
			t.Fatalf("gate disagreement for %q: classifyPathKind op_jsonl=%v isOpLogPath=%v", p, classified, isOpLogPath(p))
		}
	}
}

// TestOpLogPathIDIsAuthoritative guards #43 Codex finding 1: for a real op log, the
// path basename — not a content invocation_id — determines the op identity, so
// structural op fields never attach to the wrong op.
func TestOpLogPathIDIsAuthoritative(t *testing.T) {
	// An op event whose JSON carries a DIFFERENT invocation_id than the path basename.
	obj := map[string]any{"event": "completed", "outcome": "abandoned", "closed_by": "doctor_sweep", "invocation_id": "01KOTHER0000000000000000AB"}
	ev := eventFromJSONObject(opLogPath, 1, 0, obj)
	if ev.Scope.InvocationID != "01KTTCEAF0WTVAHYGND1D16R68" {
		t.Fatalf("op-log path id must win over content invocation_id; got %q", ev.Scope.InvocationID)
	}
}

// absorbOne feeds a single op-log event (with structural op fields) through the real
// buildState pipeline and returns the resulting OpSummary.
func absorbOne(opEvent, outcome, closedBy string) OpSummary {
	s := newBuildState()
	s.absorbTimeline([]TimelineEvent{{
		SourcePath: opLogPath,
		Scope:      Scope{Type: "op", InvocationID: "01KTTCEAF0WTVAHYGND1D16R68"},
		opEvent:    opEvent,
		opOutcome:  outcome,
		opClosedBy: closedBy,
	}})
	ops := s.opSummaries()
	if len(ops) != 1 {
		panic("expected exactly one op")
	}
	return ops[0]
}

// TestAbsorbOpEventStructural guards the #43 parser fix: op Status/Outcome/ClosedBy are
// read from the structural op-log fields, NOT the flattened TextPreview. The old text
// scan set Outcome="done" for ANY line containing "completed", so an event=completed
// carrying outcome=abandoned was misclassified as done.
func TestAbsorbOpEventStructural(t *testing.T) {
	// completed + outcome=abandoned + closed_by=doctor_sweep must classify as abandoned
	// (the misparse the old text scan produced was "done").
	if op := absorbOne("completed", "abandoned", "doctor_sweep"); op.Status != "completed" || op.Outcome != "abandoned" || op.closedBy != "doctor_sweep" {
		t.Fatalf("completed/abandoned/doctor_sweep => %+v", op)
	}
	// completed + outcome=failed.
	if op := absorbOne("completed", "failed", "agent"); op.Outcome != "failed" {
		t.Fatalf("completed/failed => outcome %q want failed", op.Outcome)
	}
	// completed + null/absent outcome must still close the op (Status=completed) with an
	// empty Outcome — a recorded completion with no verdict must not read as an open orphan.
	if op := absorbOne("completed", "", ""); op.Status != "completed" || op.Outcome != "" {
		t.Fatalf("completed/null-outcome => %+v want completed with empty outcome", op)
	}
	// started-only stays open.
	if op := absorbOne("started", "", ""); op.Status != "open" || op.Outcome != "" {
		t.Fatalf("started-only => %+v want open", op)
	}
	// An unrecognized outcome value is ignored (still completed, empty Outcome) — no
	// arbitrary string becomes a fault outcome.
	if op := absorbOne("completed", "weird", ""); op.Outcome != "" {
		t.Fatalf("completed/unknown-outcome => outcome %q want empty", op.Outcome)
	}
}

// TestOpTrailFaultFindings guards the #43 detectors in buildFindings: op_failed and
// op_abandoned fire for real op-log-backed ops with a fault outcome (distinct from
// open_op_orphan), carry the closed_by detail, and never fire for a text-synthesized op.
func TestOpTrailFaultFindings(t *testing.T) {
	id := "01KTTCEAF0WTVAHYGND1D16R68"
	// op_failed.
	failed := []OpSummary{{InvocationID: id, Status: "completed", Outcome: "failed", sawOpEvent: true}}
	if !findingListHas(buildFindings(nil, failed), "op_failed") {
		t.Fatalf("outcome=failed must classify op_failed")
	}
	// op_abandoned + doctor_sweep detail in the evidence.
	swept := []OpSummary{{InvocationID: id, Status: "completed", Outcome: "abandoned", closedBy: "doctor_sweep", sawOpEvent: true}}
	fs := buildFindings(nil, swept)
	if !findingListHas(fs, "op_abandoned") {
		t.Fatalf("outcome=abandoned must classify op_abandoned")
	}
	if !findingEvidenceContains(fs, "op_abandoned", "doctor_sweep") {
		t.Fatalf("op_abandoned evidence must name closed_by=doctor_sweep")
	}
	// A completed op that succeeded is not a fault.
	done := []OpSummary{{InvocationID: id, Status: "completed", Outcome: "done", sawOpEvent: true}}
	if fs := buildFindings(nil, done); findingListHas(fs, "op_failed") || findingListHas(fs, "op_abandoned") {
		t.Fatalf("outcome=done must not classify an op fault: %#v", fs)
	}
	// A text-synthesized op (no real op log) must never mint an op fault, mirroring the
	// open_op_orphan sawOpEvent gate.
	synthetic := []OpSummary{{InvocationID: id, Status: "completed", Outcome: "abandoned", closedBy: "doctor_sweep", sawOpEvent: false}}
	if fs := buildFindings(nil, synthetic); findingListHas(fs, "op_abandoned") {
		t.Fatalf("op_abandoned fired for a text-synthesized op (should be gated by sawOpEvent)")
	}
}

// TestOpGatingExcludesNonOpKittyOpsFile is the #43 negative guard: a non-op file under
// kitty-ops/ (a lifecycle-shaped stream with no op event) must NOT synthesize an op or
// mint any op finding — the false open_op_orphan the pre-#43 opPathRE produced.
func TestOpGatingExcludesNonOpKittyOpsFile(t *testing.T) {
	path := "kitty-ops/lifecycle.jsonl"
	kind := classifyPathKind(path)
	if kind == "op_jsonl" {
		t.Fatalf("lifecycle.jsonl must not classify as op_jsonl; got %q", kind)
	}
	state := newBuildState()
	line := `{"event":"started","canonical_action_id":"discovery::research","mission_id":"01KWVD6Y5CC8ARAKD3JZAJEXBX","phase":"started"}`
	events, _ := parseFile(path, kind, []byte(line+"\n"), 0, state)
	state.absorbTimeline(events)
	if ops := state.opSummaries(); len(ops) != 0 {
		t.Fatalf("lifecycle.jsonl must not synthesize an op; got %#v", ops)
	}
	for _, f := range buildFindings(events, state.opSummaries()) {
		if f.ID == "open_op_orphan" || f.ID == "op_failed" || f.ID == "op_abandoned" {
			t.Fatalf("lifecycle.jsonl must not mint op finding %q", f.ID)
		}
	}
}
