package query

import (
	"encoding/json"
	"testing"

	"github.com/priivacy-ai/spec-kitty-analyzer/internal/analyzer"
)

// TestResultJSONHasNestedBuildAndNoTopLevelVersion covers FR-002 + FR-005 for
// the query emitter: nested `build` object present with its three fields, and
// no top-level `version` key.
func TestResultJSONHasNestedBuildAndNoTopLevelVersion(t *testing.T) {
	data, err := json.Marshal(Result{Build: analyzer.Build{Version: "0.3.0", Commit: "abc1234", BuildDate: "2026-07-03T18:00:00Z"}})
	if err != nil {
		t.Fatalf("marshal Result: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal Result: %v", err)
	}
	if _, ok := m["version"]; ok {
		t.Errorf("top-level \"version\" must be absent, got: %s", data)
	}
	raw, ok := m["build"]
	if !ok {
		t.Fatalf("nested \"build\" object missing, got: %s", data)
	}
	var b analyzer.Build
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("unmarshal build: %v", err)
	}
	if b.Version != "0.3.0" || b.Commit != "abc1234" || b.BuildDate != "2026-07-03T18:00:00Z" {
		t.Errorf("build=%+v, want {0.3.0 abc1234 2026-07-03T18:00:00Z}", b)
	}
}

func TestBuildReturnsFilteredTimelineAndSignals(t *testing.T) {
	report := analyzer.Report{
		Build: analyzer.Build{Version: "test"},
		Timeline: []analyzer.TimelineEvent{
			{
				Seq:         1,
				Kind:        "message",
				TextPreview: "/tmp/spec-kitty checkout chatter without a signal",
			},
			{
				Seq:            2,
				Kind:           "cli_invocation",
				CLIInvocations: []analyzer.CLIInvocation{{Raw: "spec-kitty next --mission alpha-01KV", Verb: "next", Mission: "alpha-01KV"}},
			},
			{
				Seq:  3,
				Kind: "failure",
				Failures: []analyzer.FailureFingerprint{{
					ID:     "branch_worktree_confusion",
					Title:  "Branch or worktree context confusion",
					Reason: "wrong branch/worktree signal",
				}},
			},
		},
	}

	result := Build(report, "alpha-01KV", "Alpha", Options{Include: []string{"timeline", "signals"}})
	if result.Summary.FilteredTimelineEvents != 2 {
		t.Fatalf("filtered=%d want 2", result.Summary.FilteredTimelineEvents)
	}
	if result.Summary.MatchedTimelineEvents != 2 || len(result.Timeline) != 2 {
		t.Fatalf("matched=%d timeline=%d want 2", result.Summary.MatchedTimelineEvents, len(result.Timeline))
	}
	if result.Signals == nil || len(result.Signals.CLIInvocations) != 1 || len(result.Signals.Failures) != 1 {
		t.Fatalf("signals=%#v", result.Signals)
	}
}

func TestBuildFiltersByFailureID(t *testing.T) {
	report := analyzer.Report{
		Build: analyzer.Build{Version: "test"},
		Timeline: []analyzer.TimelineEvent{
			{
				Seq:            1,
				CLIInvocations: []analyzer.CLIInvocation{{Raw: "spec-kitty merge --mission alpha-01KV", Verb: "merge", Mission: "alpha-01KV"}},
			},
			{
				Seq:  2,
				Kind: "failure",
				Failures: []analyzer.FailureFingerprint{{
					ID:     "merge_operation_failed",
					Title:  "Merge operation failed or was blocked",
					Reason: "merge preflight blocked",
				}},
			},
		},
		Findings: []analyzer.Finding{{ID: "merge_operation_failed", Title: "Merge operation failed or was blocked"}},
	}

	result := Build(report, "alpha-01KV", "Alpha", Options{Include: []string{"timeline", "findings"}, FailureIDs: []string{"merge_operation_failed"}})
	if len(result.Timeline) != 1 || result.Timeline[0].Seq != 2 {
		t.Fatalf("timeline=%#v", result.Timeline)
	}
	if len(result.Findings) != 1 || result.Findings[0].ID != "merge_operation_failed" {
		t.Fatalf("findings=%#v", result.Findings)
	}
}

func TestBuildCommandMergeIncludesMergeFailureFingerprint(t *testing.T) {
	report := analyzer.Report{
		Build: analyzer.Build{Version: "test"},
		Timeline: []analyzer.TimelineEvent{
			{
				Seq:            1,
				CLIInvocations: []analyzer.CLIInvocation{{Raw: "spec-kitty merge --mission alpha-01KV", Verb: "merge", Mission: "alpha-01KV"}},
			},
			{
				Seq:  2,
				Kind: "failure",
				Failures: []analyzer.FailureFingerprint{{
					ID:     "merge_operation_failed",
					Title:  "Merge operation failed or was blocked",
					Reason: "merge preflight blocked",
				}},
			},
		},
	}

	result := Build(report, "alpha-01KV", "Alpha", Options{Include: []string{"timeline"}, Commands: []string{"merge"}})
	if len(result.Timeline) != 2 {
		t.Fatalf("timeline len=%d want 2: %#v", len(result.Timeline), result.Timeline)
	}
}
