package analyzer

import (
	"path/filepath"
	"testing"
)

// TestAnomalyCorpusAdditivityAndGenuineness runs the frozen anomaly fixture and
// verifies the two NFR guarantees at once (issue #15, NFR-001 / NFR-004):
//
//   - ADDITIVITY: the Tier-3 events (exit_status / panic / segfault) produce NO
//     findings, and the only findings are the genuine Tier-1/Tier-2 ones — so
//     enabling Tier-3 changed nothing in the failure roll-up.
//   - GENUINENESS + NO DOUBLE-COUNT: the anomaly set is exactly the expected
//     residual signals, and no anomaly shares a timeline seq with any finding.
func TestAnomalyCorpusAdditivityAndGenuineness(t *testing.T) {
	rep, err := Analyze([]string{filepath.Join("testdata", "anomaly")})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// The fixture's 7 timestamped lines map deterministically to seqs 1..7:
	//   seq 1 exit_status=2      -> Tier-3 structured_exit_status anomaly (no finding)
	//   seq 2 clean panic:       -> Tier-3 crash_panic anomaly           (no finding)
	//   seq 3 segmentation fault -> Tier-3 crash_segfault anomaly        (no finding)
	//   seq 4 error+exit_code    -> Tier-1 json_error_event finding      (no anomaly)
	//   seq 5 "...failed: exit status 2" -> Tier-2 generic_error finding (no anomaly)
	//   seq 6 benign / seq 7 narrative panic -> neither
	//
	// Additivity (NFR-001): the failure path is exactly what it would be WITHOUT
	// Tier-3 — the anomaly events (seq 1-3) contribute zero findings and zero to the
	// failure summary, so nothing about findings/Summary changed.

	// --- Findings: exact ID -> evidence-seq mapping (order-independent) ---
	findingSeqByID := map[string][]int{}
	for _, f := range rep.Findings {
		for _, e := range f.Evidence {
			findingSeqByID[f.ID] = append(findingSeqByID[f.ID], e.Seq)
		}
	}
	if got := findingSeqByID["json_error_event"]; len(got) != 1 || got[0] != 4 {
		t.Errorf("json_error_event must be exactly seq 4, got %v (findings=%+v)", got, rep.Findings)
	}
	if got := findingSeqByID["generic_error"]; len(got) != 1 || got[0] != 5 {
		t.Errorf("generic_error must be exactly seq 5, got %v", got)
	}
	if len(findingSeqByID) != 2 {
		t.Errorf("Tier-3 must be additive — expected exactly 2 finding kinds {json_error_event, generic_error}, got %v", findingSeqByID)
	}

	// --- Summary failure counts unchanged by Tier-3 (2 failure events, 2 modes) ---
	if rep.Summary.FailureEvents != 2 {
		t.Errorf("expected 2 failure events, got %d", rep.Summary.FailureEvents)
	}
	if rep.Summary.FailureModes != 2 {
		t.Errorf("expected 2 failure modes, got %d", rep.Summary.FailureModes)
	}

	// --- Anomalies: exact kind -> seq mapping, one group each ---
	anomalySeqByKind := map[string][]int{}
	for _, a := range rep.Anomalies {
		for _, e := range a.Evidence {
			anomalySeqByKind[a.Kind] = append(anomalySeqByKind[a.Kind], e.Seq)
		}
	}
	wantAnomaly := map[string]int{kindStructuredExitStatus: 1, kindCrashPanic: 2, kindCrashSegfault: 3}
	for kind, seq := range wantAnomaly {
		if got := anomalySeqByKind[kind]; len(got) != 1 || got[0] != seq {
			t.Errorf("anomaly %s must be exactly seq %d, got %v (anomalies=%+v)", kind, seq, got, rep.Anomalies)
		}
	}
	if len(rep.Anomalies) != 3 {
		t.Errorf("expected exactly 3 anomaly groups, got %d: %+v", len(rep.Anomalies), rep.Anomalies)
	}

	// --- No double-count: anomaly seqs {1,2,3} and finding seqs {4,5} are disjoint ---
	findingSeqs := map[int]bool{4: false, 5: false}
	for _, seqs := range findingSeqByID {
		for _, s := range seqs {
			findingSeqs[s] = true
		}
	}
	for _, a := range rep.Anomalies {
		for _, e := range a.Evidence {
			if findingSeqs[e.Seq] {
				t.Errorf("anomaly %s shares seq %d with a finding (double-count)", a.Kind, e.Seq)
			}
		}
	}
}

// TestAnomalyCorpusDeterministic verifies repeated analysis of the frozen fixture
// yields identical anomaly output (NFR-002).
func TestAnomalyCorpusDeterministic(t *testing.T) {
	r1, err := Analyze([]string{filepath.Join("testdata", "anomaly")})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := Analyze([]string{filepath.Join("testdata", "anomaly")})
	if err != nil {
		t.Fatal(err)
	}
	if len(r1.Anomalies) != len(r2.Anomalies) {
		t.Fatalf("anomaly count not deterministic: %d vs %d", len(r1.Anomalies), len(r2.Anomalies))
	}
	for i := range r1.Anomalies {
		if r1.Anomalies[i].SignatureHash != r2.Anomalies[i].SignatureHash {
			t.Fatalf("anomaly ordering/hash not deterministic at %d: %s vs %s", i, r1.Anomalies[i].SignatureHash, r2.Anomalies[i].SignatureHash)
		}
	}
}
