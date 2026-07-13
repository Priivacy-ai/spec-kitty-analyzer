package analyzer

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// evidenceContains reports whether any finding's evidence text contains sub. Used to
// prove (SC-003) that no finding is built from read-command content.
func evidenceContains(report Report, sub string) bool {
	for _, f := range report.Findings {
		for _, e := range f.Evidence {
			if strings.Contains(e.Text, sub) {
				return true
			}
		}
	}
	return false
}

// TestCodexCorpusFixture is the WP04 empirical acceptance proof over a small, redacted,
// committed codex session (internal/analyzer/testdata/codex). It drives the analyzer
// through its public Analyze API (black-box, DIRECTIVE_036) and asserts the mission
// outcome: read/inspection content that merely CONTAINS failure-like text produces no
// finding (SC-001/SC-003), while a genuine command failure is still detected (NFR-001).
//
// The fixture deliberately places the SAME failure-like tokens ("Usage: spec-kitty",
// "exit code 2", "merge failed/conflict") both inside a read command's output (git show
// of a doc, git diff) and inside a real command's failure output (spec-kitty analyze
// --bogus). Before this mission the read content produced false-positive
// typer_usage_error / merge_operation_failed findings (#13); after it, only the real
// failure remains.
func TestCodexCorpusFixture(t *testing.T) {
	report, err := Analyze([]string{filepath.Join("testdata", "codex")})
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Recall (NFR-001): the genuine usage error from a real (non-read) command is detected.
	if !hasFinding(report, "typer_usage_error") {
		t.Errorf("genuine typer_usage_error from a real command must be detected (recall/NFR-001)")
	}
	// The real failure's evidence is present.
	if !evidenceContains(report, "REAL_FAIL_SIG") {
		t.Errorf("real command failure evidence (REAL_FAIL_SIG) missing")
	}

	// SC-003: no finding's evidence is read/inspection content. The read markers appear
	// ONLY inside exit-0 read output (git show of a doc, git diff), which must be excluded.
	for _, marker := range []string{"DOC_CONTENT_SIG", "READDIFF_SIG"} {
		if evidenceContains(report, marker) {
			t.Errorf("a finding's evidence is read-command content (%s) — the #13 false positive", marker)
		}
	}

	// Determinism (NFR-003): the same input yields byte-identical findings across runs —
	// not merely the same count, but identical ids, order, scopes, and evidence.
	report2, err := Analyze([]string{filepath.Join("testdata", "codex")})
	if err != nil {
		t.Fatalf("Analyze (second run) failed: %v", err)
	}
	b1, _ := json.Marshal(report.Findings)
	b2, _ := json.Marshal(report2.Findings)
	if string(b1) != string(b2) {
		t.Errorf("non-deterministic findings across runs:\n run1=%s\n run2=%s", b1, b2)
	}
}
