package reports

import (
	"os"
	"path/filepath"
	"strings"
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

func TestRenderersIncludeAnomaliesSection(t *testing.T) {
	report := analyzer.Report{
		Anomalies: []analyzer.Anomaly{{
			SignatureHash: "9f2a1c4e7b03d8a6f1c05e2b7a4419de0c3f88b21a6e7c9d4f0b1a2c3d4e5f607",
			Kind:          "crash_panic",
			Channel:       "output",
			Title:         "Unclassified anomaly: panic in command output",
			Count:         2,
			FirstSeq:      41,
			LastSeq:       88,
			Evidence:      []analyzer.AnomalyEvidence{{Seq: 41, SourcePath: "a.jsonl", Line: 12, Snippet: "panic: nil pointer dereference"}},
		}},
	}
	dir := t.TempDir()

	mdPath := filepath.Join(dir, "r.md")
	if err := WriteMarkdown(report, mdPath); err != nil {
		t.Fatal(err)
	}
	md, _ := os.ReadFile(mdPath)
	for _, want := range []string{"## Anomalies", "crash_panic", "9f2a1c4e7b03d8a6f1c05e2b7a4419de0c3f88b21a6e7c9d4f0b1a2c3d4e5f607", "panic: nil pointer dereference"} {
		if !strings.Contains(string(md), want) {
			t.Errorf("markdown report missing %q", want)
		}
	}

	htmlPath := filepath.Join(dir, "r.html")
	if err := WriteHTML(report, htmlPath); err != nil {
		t.Fatal(err)
	}
	h, _ := os.ReadFile(htmlPath)
	if !strings.Contains(string(h), "<h2>Anomalies</h2>") || !strings.Contains(string(h), "crash_panic") {
		t.Error("html report missing Anomalies section")
	}

	// PDF path must render without error/panic (exercises the slice truncation).
	if err := WritePDF(report, filepath.Join(dir, "r.pdf")); err != nil {
		t.Fatalf("WritePDF with anomalies: %v", err)
	}

	// Empty case across all three renderers: friendly placeholder, never omitted/panicked.
	emptyMD := filepath.Join(dir, "e.md")
	emptyHTML := filepath.Join(dir, "e.html")
	if err := WriteMarkdown(analyzer.Report{}, emptyMD); err != nil {
		t.Fatal(err)
	}
	if err := WriteHTML(analyzer.Report{}, emptyHTML); err != nil {
		t.Fatal(err)
	}
	if err := WritePDF(analyzer.Report{}, filepath.Join(dir, "e.pdf")); err != nil {
		t.Fatalf("WritePDF empty: %v", err)
	}
	em, _ := os.ReadFile(emptyMD)
	if !strings.Contains(string(em), "No unclassified anomalies detected.") {
		t.Error("empty markdown report should show the no-anomalies placeholder")
	}
	eh, _ := os.ReadFile(emptyHTML)
	if !strings.Contains(string(eh), "No unclassified anomalies detected.") {
		t.Error("empty html report should show the no-anomalies placeholder")
	}
}
