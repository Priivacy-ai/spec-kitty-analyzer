package analyzer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// analyzeLines writes the given JSONL lines to a temp session file and analyzes it.
func analyzeLines(t *testing.T, lines ...string) Report {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Analyze([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func anomalyKinds(rep Report) map[string]int {
	m := map[string]int{}
	for _, a := range rep.Anomalies {
		m[a.Kind] += a.Count
	}
	return m
}

// P1 — a top-level non-zero exit_status with no Tier-1/Tier-2 finding → anomaly.
func TestWiringStructuredExitStatusAnomaly(t *testing.T) {
	rep := analyzeLines(t, `{"type":"tool_result","exit_status":2,"toolUseResult":{"stdout":"job done"}}`)
	if anomalyKinds(rep)[kindStructuredExitStatus] != 1 {
		t.Fatalf("expected one structured_exit_status anomaly, got anomalies=%+v findings=%d", rep.Anomalies, len(rep.Findings))
	}
	if len(rep.Findings) != 0 {
		t.Fatalf("expected no findings, got %+v", rep.Findings)
	}
}

// P2 — a panic in tool output with no Tier-1/Tier-2 finding → anomaly. (Note: a
// panic message containing "error:" would trip the Tier-2 generic_error signal and,
// by residual-only design, correctly produce a finding instead — see
// TestWiringPanicWithErrorTextDefersToTier2.)
func TestWiringPanicAnomaly(t *testing.T) {
	rep := analyzeLines(t, `{"type":"tool_result","toolUseResult":{"stdout":"panic: nil pointer dereference [recovered]"}}`)
	if anomalyKinds(rep)[kindCrashPanic] != 1 {
		t.Fatalf("expected one crash_panic anomaly, got %+v", rep.Anomalies)
	}
}

// Residual-only in practice: a Go runtime panic whose message contains "runtime
// error:" trips Tier-2 generic_error, so it is a FINDING, not a Tier-3 anomaly.
func TestWiringPanicWithErrorTextDefersToTier2(t *testing.T) {
	rep := analyzeLines(t, `{"type":"tool_result","toolUseResult":{"stdout":"panic: runtime error: index out of range [3]"}}`)
	if len(rep.Anomalies) != 0 {
		t.Fatalf("panic containing 'error:' must defer to Tier-2 (no anomaly), got %+v", rep.Anomalies)
	}
	if len(rep.Findings) == 0 {
		t.Fatal("expected a Tier-2 generic_error finding")
	}
}

// N1 — a Tier-1 json_error_event (error+exit_code) → finding, NO anomaly (residual-only).
func TestWiringTier1NoAnomaly(t *testing.T) {
	rep := analyzeLines(t, `{"type":"tool_result","error":"Error: boom","exit_code":1}`)
	if len(rep.Anomalies) != 0 {
		t.Fatalf("Tier-1 event must not produce an anomaly, got %+v", rep.Anomalies)
	}
	if len(rep.Findings) == 0 {
		t.Fatal("expected a Tier-1 finding")
	}
}

// N4 — a Tier-2 generic_error (exit status text) → finding, NO anomaly.
func TestWiringTier2NoAnomaly(t *testing.T) {
	rep := analyzeLines(t, `{"type":"tool_result","toolUseResult":{"stdout":"the command failed: exit status 2"}}`)
	if len(rep.Anomalies) != 0 {
		t.Fatalf("Tier-2 event must not produce an anomaly, got %+v", rep.Anomalies)
	}
}

// N7 — panic only in the NARRATIVE channel → no anomaly (outputCh excludes narrative).
func TestWiringNarrativeNoAnomaly(t *testing.T) {
	rep := analyzeLines(t, `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"panic: this is just me discussing a panic"}]}}`)
	if len(rep.Anomalies) != 0 {
		t.Fatalf("narrative-channel panic must not produce an anomaly, got %+v", rep.Anomalies)
	}
}

// N10 + residual-only — stashAnomalyCandidates gate: artifact kind or an existing
// failure suppresses candidates even when the signal is present.
func TestWiringStashGate(t *testing.T) {
	obj := map[string]any{"exit_status": float64(2)}

	// Artifact kind → no candidates (H1: skipArtifactMessage alone would keep it).
	artifact := TimelineEvent{}
	stashAnomalyCandidates("work_package", &artifact, obj)
	if len(artifact.anomalyCandidates) != 0 {
		t.Fatalf("artifact-kind event must not mint anomalies, got %+v", artifact.anomalyCandidates)
	}

	// Non-artifact + no failures → candidate.
	kept := TimelineEvent{}
	stashAnomalyCandidates("jsonl_transcript", &kept, obj)
	if len(kept.anomalyCandidates) != 1 {
		t.Fatalf("non-artifact residual event must mint an anomaly, got %+v", kept.anomalyCandidates)
	}

	// Non-artifact but already has a Tier-1/Tier-2 failure → no candidate (residual-only).
	withFailure := TimelineEvent{Failures: []FailureFingerprint{{ID: "json_error_event"}}}
	stashAnomalyCandidates("jsonl_transcript", &withFailure, obj)
	if len(withFailure.anomalyCandidates) != 0 {
		t.Fatalf("event with an existing finding must not mint an anomaly, got %+v", withFailure.anomalyCandidates)
	}
}

// G4 — segregation: an anomaly never enters Findings and never inflates failure counts.
func TestWiringSegregation(t *testing.T) {
	rep := analyzeLines(t, `{"type":"tool_result","toolUseResult":{"stdout":"panic: boom"}}`)
	if len(rep.Anomalies) != 1 {
		t.Fatalf("expected one anomaly, got %+v", rep.Anomalies)
	}
	if len(rep.Findings) != 0 {
		t.Fatalf("anomaly must not appear as a finding, got %+v", rep.Findings)
	}
	if rep.Summary.FailureEvents != 0 || rep.Summary.FailureModes != 0 {
		t.Fatalf("anomaly must not inflate failure summary: events=%d modes=%d", rep.Summary.FailureEvents, rep.Summary.FailureModes)
	}
}

// L1 — an anomaly-free report marshals "anomalies": [] (never null).
func TestWiringNormalizeEmptyAnomalies(t *testing.T) {
	rep := analyzeLines(t, `{"type":"tool_result","toolUseResult":{"stdout":"all good"}}`)
	if rep.Anomalies == nil {
		t.Fatal("Anomalies must be normalized to a non-nil empty slice")
	}
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"anomalies":[]`) {
		t.Fatalf("report must serialize anomalies:[] not null; got %s", string(b))
	}
}

// Covers the whole-object .json early-return path in parseFile (distinct from the
// JSONL scanner loop) — both append sites must stash candidates.
func TestWiringSingleObjectJSONPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	if err := os.WriteFile(path, []byte(`{"type":"tool_result","exit_status":5,"toolUseResult":{"stdout":"done"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Analyze([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if anomalyKinds(rep)[kindStructuredExitStatus] != 1 {
		t.Fatalf("single-object .json early-return path must stash anomalies, got %+v", rep.Anomalies)
	}
}

// M4 — mission-filtered reports rebuild anomalies from the filtered timeline.
func TestWiringMissionFilterKeepsAnomalies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	line := `{"type":"tool_result","mission_slug":"demo-mission","toolUseResult":{"stdout":"panic: boom"}}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := AnalyzeMission([]string{dir}, "demo-mission")
	if err != nil {
		t.Fatalf("AnalyzeMission: %v", err)
	}
	if anomalyKinds(rep)[kindCrashPanic] != 1 {
		t.Fatalf("mission-filtered report must carry the anomaly, got %+v", rep.Anomalies)
	}
}
