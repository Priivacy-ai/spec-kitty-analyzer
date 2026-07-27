package analyzer

import "testing"

// forcedEvent builds a synthetic status-event TimelineEvent carrying a genuine
// forced_transition failure for (mission, wp), with the given seq and event_id. It mirrors
// what parseFile produces for a status.events.jsonl line that the forced_transition detector
// has already classified.
func forcedEvent(seq int, mission, wp, eventID string) TimelineEvent {
	return TimelineEvent{
		Seq:        seq,
		SourcePath: "kitty-specs/" + mission + "/status.events.jsonl",
		Line:       seq,
		Scope:      Scope{Type: "mission", MissionSlug: mission, WorkPackage: wp},
		eventID:    eventID,
		Failures: []FailureFingerprint{{
			ID:       "forced_transition",
			Title:    "Forced state-machine override",
			Severity: "medium",
		}},
	}
}

// findingByID returns the finding with id, or false.
func findingByID(findings []Finding, id string) (Finding, bool) {
	for _, f := range findings {
		if f.ID == id {
			return f, true
		}
	}
	return Finding{}, false
}

// TestRepeatedlyForcedWorkPackage guards the #48 aggregate: a WP force-overridden
// forcedOverrideRepeatThreshold+ times is flagged, a single override is not, and the per-WP
// count is computed from raw per-event failures (which the aggregated forced_transition
// Finding cannot preserve).
func TestRepeatedlyForcedWorkPackage(t *testing.T) {
	m := "061-finished-goods-inventory-service"
	events := []TimelineEvent{
		// WP01 forced 3 times — must flag, count 3.
		forcedEvent(1, m, "WP01", "01KMKK29XX0DKV2D3T6CAZESS1"),
		forcedEvent(2, m, "WP01", "01KMKK29XX0DKV2D3T6CAZESS2"),
		forcedEvent(3, m, "WP01", "01KMKK29XX0DKV2D3T6CAZESS3"),
		// WP02 forced once — must NOT contribute a repeat.
		forcedEvent(4, m, "WP02", "01KMKK29XX0DKV2D3T6CAZESS4"),
		// WP03 forced twice (threshold boundary) — must flag.
		forcedEvent(5, m, "WP03", "01KMKK29XX0DKV2D3T6CAZESS5"),
		forcedEvent(6, m, "WP03", "01KMKK29XX0DKV2D3T6CAZESS6"),
	}
	findings := buildFindings(events, nil)
	f, ok := findingByID(findings, "repeatedly_forced_work_package")
	if !ok {
		t.Fatalf("expected repeatedly_forced_work_package finding; got %v", findingIDs(findings))
	}
	// Two WPs flagged (WP01 x3, WP03 x2); WP02 x1 excluded.
	if f.Count != 2 {
		t.Fatalf("Count = %d, want 2 (WP01+WP03)", f.Count)
	}
	if len(f.Scopes) != 2 {
		t.Fatalf("Scopes = %d, want 2", len(f.Scopes))
	}
	// Deterministic order: highest count first (WP01 x3 before WP03 x2).
	if f.Scopes[0].WorkPackage != "WP01" || f.Scopes[1].WorkPackage != "WP03" {
		t.Fatalf("scope order = %q,%q; want WP01,WP03", f.Scopes[0].WorkPackage, f.Scopes[1].WorkPackage)
	}
	// Emitted scope is normalized (no action/invocation leakage).
	if f.Scopes[0].Type != "mission" || f.Scopes[0].Action != "" || f.Scopes[0].InvocationID != "" {
		t.Fatalf("scope not normalized: %+v", f.Scopes[0])
	}
	// Evidence carries the per-WP count and provenance.
	if !findingEvidenceContains(findings, "repeatedly_forced_work_package", "WP01 in "+m+" forced 3 times") {
		t.Fatalf("evidence missing WP01 count; evidence=%+v", f.Evidence)
	}
	if f.Evidence[0].SourcePath == "" || f.Evidence[0].Seq == 0 {
		t.Fatalf("evidence missing provenance: %+v", f.Evidence[0])
	}
	// FirstSeq/LastSeq span the counted events.
	if f.FirstSeq != 1 || f.LastSeq != 6 {
		t.Fatalf("FirstSeq/LastSeq = %d/%d, want 1/6", f.FirstSeq, f.LastSeq)
	}
}

// TestRepeatedlyForcedSingleOverrideQuiet confirms a mission whose WPs were each forced at
// most once produces no repeatedly_forced_work_package finding (the clean-mission direction).
func TestRepeatedlyForcedSingleOverrideQuiet(t *testing.T) {
	m := "clean-mission"
	events := []TimelineEvent{
		forcedEvent(1, m, "WP01", "01KMKK29XX0DKV2D3T6CAZESA1"),
		forcedEvent(2, m, "WP02", "01KMKK29XX0DKV2D3T6CAZESA2"),
		forcedEvent(3, m, "WP03", "01KMKK29XX0DKV2D3T6CAZESA3"),
	}
	if findingListHas(buildFindings(events, nil), "repeatedly_forced_work_package") {
		t.Fatalf("no WP forced >1x must not flag repeatedly_forced_work_package")
	}
}

// TestRepeatedlyForcedDedupByEventID guards the #48 FP fix: a duplicated status-event line
// (e.g. a .worktrees/ mirror, which collectFiles does not exclude) carries the same event_id,
// so it must NOT inflate a genuine single override into a false repeat.
func TestRepeatedlyForcedDedupByEventID(t *testing.T) {
	m := "mirror-mission"
	dup := "01KMKK29XX0DKV2D3T6CAZESB1"
	// Same event_id twice (canonical + a mirrored copy at a different source path/seq).
	e1 := forcedEvent(1, m, "WP01", dup)
	e2 := forcedEvent(2, m, "WP01", dup)
	e2.SourcePath = "kitty-specs/" + m + "/.worktrees/wp01/kitty-specs/" + m + "/status.events.jsonl"
	if findingListHas(buildFindings([]TimelineEvent{e1, e2}, nil), "repeatedly_forced_work_package") {
		t.Fatalf("duplicate event_id must not count as a repeat")
	}
	// Distinct event_ids on the same WP DO count.
	e3 := forcedEvent(3, m, "WP01", "01KMKK29XX0DKV2D3T6CAZESB2")
	if !findingListHas(buildFindings([]TimelineEvent{e1, e3}, nil), "repeatedly_forced_work_package") {
		t.Fatalf("two distinct forced overrides on one WP must flag")
	}
}

// TestRepeatedlyForcedRequiresMissionAndWP confirms an unattributable forced override (missing
// mission or wp) is not grouped into a bogus repeat, and that only forced_transition failures
// (not other failure IDs) feed the aggregate.
func TestRepeatedlyForcedRequiresMissionAndWP(t *testing.T) {
	// Two forced overrides with an empty WP — cannot attribute, must not flag.
	noWP := []TimelineEvent{
		{Seq: 1, Scope: Scope{Type: "mission", MissionSlug: "m"}, eventID: "01KMKK29XX0DKV2D3T6CAZESC1",
			Failures: []FailureFingerprint{{ID: "forced_transition"}}},
		{Seq: 2, Scope: Scope{Type: "mission", MissionSlug: "m"}, eventID: "01KMKK29XX0DKV2D3T6CAZESC2",
			Failures: []FailureFingerprint{{ID: "forced_transition"}}},
	}
	if findingListHas(buildFindings(noWP, nil), "repeatedly_forced_work_package") {
		t.Fatalf("forced overrides with no WP must not be grouped")
	}
	// A non-forced failure repeated on one WP must not feed the forced aggregate.
	other := []TimelineEvent{
		{Seq: 1, Scope: Scope{Type: "mission", MissionSlug: "m", WorkPackage: "WP01"}, eventID: "d1",
			Failures: []FailureFingerprint{{ID: "review_rejected"}}},
		{Seq: 2, Scope: Scope{Type: "mission", MissionSlug: "m", WorkPackage: "WP01"}, eventID: "d2",
			Failures: []FailureFingerprint{{ID: "review_rejected"}}},
	}
	if findingListHas(buildFindings(other, nil), "repeatedly_forced_work_package") {
		t.Fatalf("non-forced failures must not feed the forced aggregate")
	}
}

// TestRepeatedlyForcedRealJSONParsePath drives status-event JSON through the real parse path
// (parseFile → eventFromTextCtx), proving event_id is extracted from JSON — not just when the
// field is set by hand — and that a real forced-override reason classifies forced_transition
// end to end (Codex #48 code-review finding 1).
func TestRepeatedlyForcedRealJSONParsePath(t *testing.T) {
	path := "repo/kitty-specs/m/status.events.jsonl"
	kind := classifyPathKind(path)
	// Two distinct forced overrides on WP01 via real JSON → flagged.
	twoDistinct := `{"event_id":"01KMKK29XX0DKV2D3T6CAZESD1","force":true,"reason":"Force move to done","mission_slug":"m","wp_id":"WP01","from_lane":"planned","to_lane":"done"}
{"event_id":"01KMKK29XX0DKV2D3T6CAZESD2","force":true,"reason":"backward rewind: reopen","mission_slug":"m","wp_id":"WP01","from_lane":"done","to_lane":"in_progress"}`
	events, _ := parseFile(path, kind, []byte(twoDistinct), 0, newBuildState())
	for i := range events {
		events[i].Seq = i + 1
	}
	if !findingListHas(buildFindings(events, nil), "repeatedly_forced_work_package") {
		t.Fatalf("two forced overrides via real JSON must flag; parsed %d events", len(events))
	}

	// A canonical override plus its worktree mirror (same event_id, parsed from real JSON on
	// two paths) must NOT flag a repeat — proves the parse path populates event_id for dedup.
	line := `{"event_id":"01KMKK29XX0DKV2D3T6CAZESD1","force":true,"reason":"Force move to done","mission_slug":"m","wp_id":"WP02","from_lane":"planned","to_lane":"done"}`
	canon, _ := parseFile("repo/kitty-specs/m/status.events.jsonl", kind, []byte(line), 0, newBuildState())
	mirror, _ := parseFile("repo/.worktrees/wp02/kitty-specs/m/status.events.jsonl", kind, []byte(line), 0, newBuildState())
	all := append(canon, mirror...)
	for i := range all {
		all[i].Seq = i + 1
	}
	if findingListHas(buildFindings(all, nil), "repeatedly_forced_work_package") {
		t.Fatalf("a canonical override + its worktree mirror (same event_id) must not flag a repeat")
	}
}

// TestRepeatedlyForcedEvidencePrefersCanonical guards Codex #48 code-review finding 2: when a
// worktree mirror shares an event_id and sorts before the canonical line, the cited evidence
// provenance is the canonical status.events.jsonl, not the mirror copy.
func TestRepeatedlyForcedEvidencePrefersCanonical(t *testing.T) {
	m := "m"
	mirror := forcedEvent(1, m, "WP01", "01KMKK29XX0DKV2D3T6CAZESE1")
	mirror.SourcePath = "repo/.worktrees/wp01/kitty-specs/m/status.events.jsonl"
	canonical := forcedEvent(2, m, "WP01", "01KMKK29XX0DKV2D3T6CAZESE1") // same event_id, canonical path
	second := forcedEvent(3, m, "WP01", "01KMKK29XX0DKV2D3T6CAZESE2")    // distinct override → count=2
	fs := buildFindings([]TimelineEvent{mirror, canonical, second}, nil)
	f, ok := findingByID(fs, "repeatedly_forced_work_package")
	if !ok {
		t.Fatalf("expected repeatedly_forced_work_package finding")
	}
	if f.Count != 1 {
		t.Fatalf("Count = %d, want 1 (one WP flagged)", f.Count)
	}
	if isWorktreeMirrorPath(f.Evidence[0].SourcePath) {
		t.Fatalf("evidence must prefer canonical over worktree mirror; got %q", f.Evidence[0].SourcePath)
	}
}

// findingIDs is a small diagnostic helper listing finding ids.
func findingIDs(findings []Finding) []string {
	ids := make([]string, 0, len(findings))
	for _, f := range findings {
		ids = append(ids, f.ID)
	}
	return ids
}
