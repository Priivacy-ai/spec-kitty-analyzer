package analyzer

import (
	"strings"
	"testing"
)

// --- Detector (T001) ---------------------------------------------------------

func TestAnomalyDetectStructuredExitStatus(t *testing.T) {
	got := detectAnomalies(map[string]any{"exit_status": float64(2)}, "")
	if len(got) != 1 || got[0].kind != kindStructuredExitStatus || got[0].channel != channelStructured {
		t.Fatalf("expected one structured_exit_status candidate, got %+v", got)
	}
}

func TestAnomalyExitStatusZeroIsNotAnomaly(t *testing.T) {
	if got := detectAnomalies(map[string]any{"exit_status": float64(0)}, ""); len(got) != 0 {
		t.Fatalf("exit_status=0 must not be an anomaly, got %+v", got)
	}
}

func TestAnomalyDetectCrashSignatures(t *testing.T) {
	cases := []struct {
		name string
		out  string
		kind string
	}{
		{"panic", "goroutine 1 [running]:\npanic: runtime error: index out of range [7]", kindCrashPanic},
		{"segfault", "signal: segmentation fault", kindCrashSegfault},
		{"coredumped", "Aborted (core dumped)", kindCrashCoreDumped},
	}
	for _, c := range cases {
		got := detectAnomalies(nil, c.out)
		if len(got) != 1 || got[0].kind != c.kind || got[0].channel != channelOutput {
			t.Fatalf("%s: expected one %s output candidate, got %+v", c.name, c.kind, got)
		}
	}
}

func TestAnomalyMultiSignalEvent(t *testing.T) {
	got := detectAnomalies(map[string]any{"exit_status": float64(3)}, "panic: boom")
	if len(got) != 2 {
		t.Fatalf("expected two candidates (exit_status + panic), got %+v", got)
	}
}

func TestAnomalyExitStatusNonNumericIsNotAnomaly(t *testing.T) {
	// A bool/string exit_status is not a numeric non-zero indicator → no anomaly.
	for _, v := range []any{true, "2", "failed"} {
		if got := detectAnomalies(map[string]any{"exit_status": v}, ""); len(got) != 0 {
			t.Fatalf("non-numeric exit_status %v must not fire, got %+v", v, got)
		}
	}
}

func TestAnomalyExitStatusIsTopLevelOnly(t *testing.T) {
	// A nested exit_status must NOT be found (top-level-only read — H3: deterministic
	// + respects the post-#13 channel exclusion; nested content may be excluded).
	obj := map[string]any{"result": map[string]any{"exit_status": float64(2)}}
	if got := detectAnomalies(obj, ""); len(got) != 0 {
		t.Fatalf("nested exit_status must not fire (top-level only), got %+v", got)
	}
}

func TestSignatureHashLongLinesDoNotCollide(t *testing.T) {
	// Two long panic lines identical for 200+ chars but differing afterward must get
	// DIFFERENT signatures — the hash uses the full token, not the bounded snippet.
	prefix := "panic: " + strings.Repeat("x", 250)
	evA := []TimelineEvent{{Seq: 1, anomalyCandidates: detectAnomalies(nil, prefix+" ALPHA")}}
	evB := []TimelineEvent{{Seq: 1, anomalyCandidates: detectAnomalies(nil, prefix+" BETA")}}
	a := buildAnomalies(evA)
	b := buildAnomalies(evB)
	if len(a) != 1 || len(b) != 1 {
		t.Fatalf("expected one anomaly each, got %d/%d", len(a), len(b))
	}
	if a[0].SignatureHash == b[0].SignatureHash {
		t.Fatal("long lines differing after the snippet cap must not share a signature hash")
	}
}

func TestBuildAnomaliesEvidenceCapKeepsLowestSeqs(t *testing.T) {
	// Out-of-seq events: the retained (capped) evidence must be the lowest seqs.
	var events []TimelineEvent
	for _, seq := range []int{30, 10, 50, 20, 40, 5, 60} {
		events = append(events, TimelineEvent{Seq: seq, SourcePath: "a", anomalyCandidates: []anomalyCandidate{{kind: kindCrashPanic, channel: channelOutput, token: "panic: same", snippet: "panic: same"}}})
	}
	got := buildAnomalies(events)
	if len(got) != 1 || got[0].Count != 7 {
		t.Fatalf("expected one group with count 7, got %+v", got)
	}
	if len(got[0].Evidence) != maxAnomalyEvidence {
		t.Fatalf("expected evidence capped to %d, got %d", maxAnomalyEvidence, len(got[0].Evidence))
	}
	wantLowest := []int{5, 10, 20, 30, 40}
	for i, seq := range wantLowest {
		if got[0].Evidence[i].Seq != seq {
			t.Fatalf("evidence must be the lowest seqs sorted; got %d want %d at %d", got[0].Evidence[i].Seq, seq, i)
		}
	}
	if got[0].FirstSeq != 5 || got[0].LastSeq != 60 {
		t.Fatalf("first/last seq wrong: %d/%d", got[0].FirstSeq, got[0].LastSeq)
	}
}

func TestAnomalyResidualNegatives(t *testing.T) {
	// Bare generic words are NOT anomalies (no benign chatter).
	if got := detectAnomalies(nil, "an unexpected failure occurred; aborted"); len(got) != 0 {
		t.Fatalf("bare generic words must not fire, got %+v", got)
	}
	// Traceback is Tier-1/Tier-2, deliberately excluded from the residual set.
	if got := detectAnomalies(nil, "Traceback (most recent call last):"); len(got) != 0 {
		t.Fatalf("Traceback must not be a Tier-3 anomaly, got %+v", got)
	}
}

// --- Signature hash (T002) ---------------------------------------------------

func TestSignatureHashGroupsSameShape(t *testing.T) {
	a := signatureHash(channelOutput, "", kindCrashPanic, "panic: runtime error: index out of range [5]")
	b := signatureHash(channelOutput, "", kindCrashPanic, "panic: runtime error: index out of range [9]")
	if a != b {
		t.Fatalf("digit-varying panics must share a hash: %s vs %s", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("signature hash must be a full 64-char sha256 digest, got len %d", len(a))
	}
}

func TestSignatureHashDistinguishesKindChannelTool(t *testing.T) {
	base := signatureHash(channelOutput, "bash", kindCrashPanic, "panic: x")
	if base == signatureHash(channelOutput, "bash", kindCrashSegfault, "panic: x") {
		t.Fatal("different kind must change the hash")
	}
	if base == signatureHash(channelStructured, "bash", kindCrashPanic, "panic: x") {
		t.Fatal("different channel must change the hash")
	}
	if base == signatureHash(channelOutput, "other", kindCrashPanic, "panic: x") {
		t.Fatal("different tool must change the hash (FR-005)")
	}
}

func TestSignatureHashDeterministic(t *testing.T) {
	if signatureHash(channelOutput, "t", kindCrashPanic, "panic: A [1]") != signatureHash(channelOutput, "t", kindCrashPanic, "panic: A [2]") {
		t.Fatal("hash must be deterministic + digit-normalized")
	}
}

// --- Aggregation + ignore (T005/T003) ----------------------------------------

func TestBuildAnomaliesGroupsAndOrders(t *testing.T) {
	events := []TimelineEvent{
		{Seq: 5, SourcePath: "a.jsonl", anomalyCandidates: []anomalyCandidate{{kind: kindCrashPanic, channel: channelOutput, token: "panic: x [5]", snippet: "panic: x [5]"}}},
		{Seq: 9, SourcePath: "b.jsonl", anomalyCandidates: []anomalyCandidate{{kind: kindCrashPanic, channel: channelOutput, token: "panic: x [9]", snippet: "panic: x [9]"}}},
	}
	got := buildAnomalies(events)
	if len(got) != 1 {
		t.Fatalf("identical shapes across files must group into one anomaly, got %d", len(got))
	}
	if got[0].Count != 2 || got[0].FirstSeq != 5 || got[0].LastSeq != 9 {
		t.Fatalf("group aggregation wrong: %+v", got[0])
	}
	if len(got[0].Evidence) != 2 {
		t.Fatalf("expected two evidence rows, got %d", len(got[0].Evidence))
	}
}

func TestBuildAnomaliesIgnoreRegistry(t *testing.T) {
	events := []TimelineEvent{
		{Seq: 1, SourcePath: "a.jsonl", anomalyCandidates: []anomalyCandidate{{kind: kindCrashPanic, channel: channelOutput, token: "panic: ignore me", snippet: "panic: ignore me"}}},
	}
	hash := signatureHash(channelOutput, "", kindCrashPanic, "panic: ignore me")
	ignoredAnomalySignatures[hash] = "test-benign"
	defer delete(ignoredAnomalySignatures, hash)
	if got := buildAnomalies(events); len(got) != 0 {
		t.Fatalf("ignored signature must be suppressed, got %+v", got)
	}
}

func TestBuildAnomaliesDeterministicOrder(t *testing.T) {
	events := []TimelineEvent{
		{Seq: 2, SourcePath: "a", anomalyCandidates: []anomalyCandidate{{kind: kindCrashSegfault, channel: channelOutput, token: "segmentation fault", snippet: "segmentation fault"}}},
		{Seq: 1, SourcePath: "b", anomalyCandidates: []anomalyCandidate{{kind: kindCrashPanic, channel: channelOutput, token: "panic: y", snippet: "panic: y"}}},
	}
	first := buildAnomalies(events)
	second := buildAnomalies(events)
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("expected two groups")
	}
	for i := range first {
		if first[i].SignatureHash != second[i].SignatureHash {
			t.Fatal("buildAnomalies ordering must be deterministic across runs")
		}
	}
}

func TestNormalizeTokenCollapses(t *testing.T) {
	if normalizeToken("PANIC: err [42]") != normalizeToken("panic: err [7]") {
		t.Fatal("normalizeToken must lowercase + collapse digits")
	}
	if !strings.Contains(normalizeToken("exit_status=15"), "#") {
		t.Fatal("normalizeToken must collapse digit runs to a placeholder")
	}
}
