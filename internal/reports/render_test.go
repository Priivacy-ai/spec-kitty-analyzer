package reports

import (
	"testing"

	"github.com/priivacy-ai/spec-kitty-analyzer/internal/analyzer"
)

func TestReportTimelineEventsFiltersHarnessNoiseButKeepsSpecKittySignals(t *testing.T) {
	report := analyzer.Report{
		Timeline: []analyzer.TimelineEvent{
			{
				Seq:         1,
				Kind:        "message",
				TextPreview: "/Users/robert/spec-kitty-dev/spec-kitty ordinary harness chatter",
			},
			{
				Seq:            2,
				Kind:           "cli_invocation",
				CLIInvocations: []analyzer.CLIInvocation{{Raw: "spec-kitty next --mission sample-01KS", Verb: "next", Mission: "sample-01KS"}},
			},
			{
				Seq:         3,
				Kind:        "message",
				TextPreview: "Branch: on 'fix/sample', mission targets 'main'; wrong worktree suspected",
				Failures: []analyzer.FailureFingerprint{{
					ID:     "branch_worktree_confusion",
					Title:  "Branch or worktree context confusion",
					Reason: "matched deterministic branch/worktree signal",
				}},
			},
			{
				Seq:         4,
				Kind:        "message",
				TextPreview: "merge preflight blocked before ref advance",
				Failures: []analyzer.FailureFingerprint{{
					ID:     "merge_operation_failed",
					Title:  "Merge operation failed or was blocked",
					Reason: "matched deterministic merge signal",
				}},
			},
			{
				Seq:  5,
				Kind: "failure",
				Failures: []analyzer.FailureFingerprint{{
					ID:     "generic_error",
					Title:  "Generic error signal",
					Reason: "plain command failed",
				}},
			},
		},
	}

	got := FilteredTimeline(report)
	if len(got) != 3 {
		t.Fatalf("timeline len=%d want 3: %#v", len(got), got)
	}
	for i, seq := range []int{2, 3, 4} {
		if got[i].Seq != seq {
			t.Fatalf("got seq %d at index %d, want %d", got[i].Seq, i, seq)
		}
	}
}
