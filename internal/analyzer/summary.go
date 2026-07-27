package analyzer

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

func buildSummary(report Report) Summary {
	cmds := map[string]bool{}
	verbs := map[string]bool{}
	skills := map[string]bool{}
	profiles := map[string]bool{}
	failures := map[string]bool{}
	var s Summary
	s.InputFiles = len(report.Inputs)
	s.TimelineEvents = len(report.Timeline)
	s.Turns = len(report.Timeline)
	s.Missions = len(report.Missions)
	s.Ops = len(report.Ops)
	for _, op := range report.Ops {
		if op.Status == "open" {
			s.OpenOps++
		}
	}
	for _, event := range report.Timeline {
		switch event.Scope.Type {
		case "mission":
			s.MissionEvents++
		case "op":
			s.OpEvents++
		default:
			s.OutsideEvents++
		}
		for _, cmd := range event.SlashCommands {
			s.SlashCommands++
			cmds[cmd.Name] = true
		}
		for _, inv := range event.CLIInvocations {
			s.CLIInvocations++
			if inv.Verb != "" {
				verbs[inv.Verb] = true
			}
		}
		for _, skill := range event.Skills {
			s.Skills++
			skills[skill.Name] = true
		}
		for _, profile := range event.AgentProfiles {
			if profile.Profile != "" {
				s.AgentProfiles++
				profiles[profile.Profile] = true
			}
		}
		if len(event.Failures) > 0 {
			s.FailureEvents++
		}
		for _, failure := range event.Failures {
			failures[failure.ID] = true
		}
	}
	s.UniqueCommands = len(cmds)
	s.UniqueCLIVerbs = len(verbs)
	s.UniqueSkills = len(skills)
	s.FailureModes = len(failures)
	_ = profiles
	return s
}

// forcedOverrideRepeatThreshold is the per-work-package count of genuine forced_transition
// overrides at or above which a WP is flagged as repeatedly_forced_work_package (#48). A
// single forced override is routine recovery; repeated overrides mark a WP that kept failing
// to advance through the normal lane gate. Set to 2 from the corpus per-WP distribution (~90%
// of forced WPs are one-off; the >=2 tail is ~10%), mirroring spec-kitty's own merge-preflight
// repeat heuristic — but computed over the already-filtered genuine interventions
// (forced_transition), NOT the systemic-force-polluted status.json force_count.
const forcedOverrideRepeatThreshold = 2

// forcedWPKey identifies a work package for per-WP forced-override aggregation.
type forcedWPKey struct {
	mission string
	wp      string
}

// forcedWPAgg accumulates genuine forced_transition occurrences for one work package. The
// per-WP occurrence count is not recoverable from the aggregated forced_transition Finding
// (Finding.Count is global; Finding.Scopes are deduped), so #48 counts here from the raw
// per-event failures before any dedup.
type forcedWPAgg struct {
	scope        Scope           // normalized WP scope: Type=mission, no action/invocation
	count        int             // distinct genuine forced overrides on this WP
	seenEvent    map[string]bool // event_id dedup — a mirrored status.events.jsonl must not inflate count
	firstSeq     int             // earliest counted event seq (finding FirstSeq)
	lastSeq      int             // latest counted event seq (finding LastSeq)
	evSeq        int             // representative event seq (evidence)
	evSourcePath string          // representative event source (evidence provenance)
	evLine       int             // representative event line (evidence provenance)
}

// isWorktreeMirrorPath reports whether path is under a .worktrees/ tree — a per-WP checkout
// that mirrors the canonical status.events.jsonl. collectFiles does not exclude these, so the
// same status event can appear both canonically and mirrored under one repo.
func isWorktreeMirrorPath(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/.worktrees/")
}

// preferEvidenceSource reports whether event e is a better evidence source for agg than the one
// already retained: a canonical (non-.worktrees) source beats a mirror, and within the same
// class the earlier seq wins. Keeps the cited line deterministic and canonical even when a
// worktree mirror of the same event_id sorts ahead of the canonical copy.
func preferEvidenceSource(agg *forcedWPAgg, e TimelineEvent) bool {
	if agg.evSourcePath == "" {
		return true
	}
	curMirror := isWorktreeMirrorPath(agg.evSourcePath)
	newMirror := isWorktreeMirrorPath(e.SourcePath)
	if curMirror != newMirror {
		return curMirror // replace only when the retained source is the mirror
	}
	return e.Seq < agg.evSeq
}

// flaggedForcedWPs returns the work packages whose genuine forced_transition count meets
// forcedOverrideRepeatThreshold, sorted deterministically (count desc, then mission, then wp)
// so Scopes/Evidence order and FirstSeq/LastSeq are reproducible despite randomized Go map
// iteration order.
func flaggedForcedWPs(byWP map[forcedWPKey]*forcedWPAgg) []*forcedWPAgg {
	out := make([]*forcedWPAgg, 0, len(byWP))
	for _, agg := range byWP {
		if agg.count >= forcedOverrideRepeatThreshold {
			out = append(out, agg)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].count != out[j].count {
			return out[i].count > out[j].count
		}
		if out[i].scope.MissionSlug != out[j].scope.MissionSlug {
			return out[i].scope.MissionSlug < out[j].scope.MissionSlug
		}
		return out[i].scope.WorkPackage < out[j].scope.WorkPackage
	})
	return out
}

func buildFindings(events []TimelineEvent, ops []OpSummary) []Finding {
	byID := map[string]*Finding{}
	// forcedByWP accumulates genuine forced_transition overrides per work package for the
	// repeatedly_forced_work_package aggregate (#48), counted from the raw per-event failures
	// below because the aggregated forced_transition Finding cannot preserve a per-WP count.
	forcedByWP := map[forcedWPKey]*forcedWPAgg{}
	for _, event := range events {
		for _, failure := range event.Failures {
			f := byID[failure.ID]
			if f == nil {
				f = &Finding{
					ID:            failure.ID,
					Title:         failure.Title,
					Severity:      failure.Severity,
					FirstSeq:      event.Seq,
					Recovery:      failure.Recovery,
					Deterministic: failure.Deterministic,
				}
				byID[failure.ID] = f
			}
			f.Count++
			f.LastSeq = event.Seq
			f.Scopes = appendScopeUnique(f.Scopes, event.Scope)
			if len(f.Evidence) < 5 {
				f.Evidence = append(f.Evidence, FindingEvidence{
					Seq:        event.Seq,
					SourcePath: event.SourcePath,
					Line:       event.Line,
					Text:       event.TextPreview,
				})
			}

			// Per-WP aggregation for repeatedly_forced_work_package (#48). Only genuine
			// forced overrides (already filtered by the forced_transition detector) that
			// carry both a mission and a WP are attributable; dedup by event_id so a
			// duplicated status-event line (e.g. a .worktrees/ mirror, which collectFiles
			// does not exclude) cannot inflate one override into a false repeat.
			if failure.ID == "forced_transition" {
				mission := strings.TrimSpace(event.Scope.MissionSlug)
				wp := strings.TrimSpace(event.Scope.WorkPackage)
				if mission != "" && wp != "" {
					key := forcedWPKey{mission: mission, wp: wp}
					agg := forcedByWP[key]
					if agg == nil {
						agg = &forcedWPAgg{
							scope:     Scope{Type: "mission", MissionSlug: mission, WorkPackage: wp},
							seenEvent: map[string]bool{},
						}
						forcedByWP[key] = agg
					}
					// Count distinct genuine overrides — dedup by event_id so a mirrored line
					// cannot inflate the count — and track the counted-seq span for the finding.
					// Events without an event_id cannot be deduped and are counted as-is.
					if event.eventID == "" || !agg.seenEvent[event.eventID] {
						if event.eventID != "" {
							agg.seenEvent[event.eventID] = true
						}
						if agg.count == 0 || event.Seq < agg.firstSeq {
							agg.firstSeq = event.Seq
						}
						if event.Seq > agg.lastSeq {
							agg.lastSeq = event.Seq
						}
						agg.count++
					}
					// Pick the best evidence provenance across ALL occurrences (including
					// deduped duplicates) so a canonical copy replaces a worktree mirror even
					// when the mirror was counted first.
					if preferEvidenceSource(agg, event) {
						agg.evSeq = event.Seq
						agg.evSourcePath = event.SourcePath
						agg.evLine = event.Line
					}
				}
			}
		}
	}

	// repeatedly_forced_work_package: a WP force-overridden forcedOverrideRepeatThreshold+
	// times — a sharper reliability signal than a single override (it kept getting stuck).
	// Synthesized like the op findings below; the render allowlist (isSpecKittyFailureID)
	// carries the id for consistency though report.Findings render unconditionally.
	if flagged := flaggedForcedWPs(forcedByWP); len(flagged) > 0 {
		f := &Finding{
			ID:            "repeatedly_forced_work_package",
			Title:         "Work package forced repeatedly",
			Severity:      "medium",
			Recovery:      "A work package with multiple forced overrides repeatedly failed to advance through the normal lane gate. Investigate why it kept getting stuck (a dependency, review, or state-tracking fault) rather than treating each override in isolation — a repeatedly forced WP usually marks an underlying workflow problem.",
			Deterministic: true,
		}
		for i, agg := range flagged {
			f.Count++
			f.Scopes = append(f.Scopes, agg.scope)
			if i == 0 || agg.firstSeq < f.FirstSeq {
				f.FirstSeq = agg.firstSeq
			}
			if agg.lastSeq > f.LastSeq {
				f.LastSeq = agg.lastSeq
			}
			if len(f.Evidence) < 5 {
				f.Evidence = append(f.Evidence, FindingEvidence{
					Seq:        agg.evSeq,
					SourcePath: agg.evSourcePath,
					Line:       agg.evLine,
					Text:       fmt.Sprintf("%s in %s forced %d times", agg.scope.WorkPackage, agg.scope.MissionSlug, agg.count),
				})
			}
		}
		byID[f.ID] = f
	}
	// addOpFinding records one op-scoped finding, single-sourcing the byID upsert +
	// scope/evidence append shared by the op detectors below.
	addOpFinding := func(id, title, severity, recovery string, op OpSummary, evidence string) {
		f := byID[id]
		if f == nil {
			f = &Finding{ID: id, Title: title, Severity: severity, Recovery: recovery, Deterministic: true}
			byID[id] = f
		}
		f.Count++
		f.Scopes = appendScopeUnique(f.Scopes, Scope{Type: "op", InvocationID: op.InvocationID})
		if len(f.Evidence) < 5 {
			f.Evidence = append(f.Evidence, FindingEvidence{Text: evidence})
		}
	}
	for _, op := range ops {
		// Only flag ops backed by a real kitty-ops op log. Ops synthesized from an
		// invocation_id merely mentioned in transcript/prose (or a WP review/implement
		// invocation) are not dispatch Ops that close via profile-invocation complete.
		if !op.sawOpEvent {
			continue
		}
		switch {
		case op.Status == "open":
			addOpFinding("open_op_orphan", "Open Spec Kitty Op was not closed", "medium",
				"Close the Op with spec-kitty profile-invocation complete --invocation-id <id> --outcome <done|failed|abandoned>.",
				op, "kitty-ops/"+op.InvocationID+".jsonl has no completed event")
		case op.Outcome == "failed":
			// The Op closed reporting failure — the dispatched work did not succeed.
			addOpFinding("op_failed", "Spec Kitty Op completed with a failed outcome", "medium",
				"Inspect the Op's request and evidence; the dispatched work failed. Re-dispatch after fixing the cause, or record why the failure is acceptable.",
				op, "kitty-ops/"+op.InvocationID+".jsonl completed with outcome=failed")
		case op.Outcome == "abandoned":
			// An abandoned Op. closed_by distinguishes an explicit agent abandonment from
			// a doctor_sweep (the agent never closed it and the doctor swept it shut — the
			// sharper workflow-discipline fault). Folded into one finding to avoid
			// double-emitting for a single terminal state; closed_by rides the reason.
			recovery := "Confirm the Op was meant to be abandoned. If not, re-dispatch and close it explicitly with profile-invocation complete."
			evidence := "kitty-ops/" + op.InvocationID + ".jsonl completed with outcome=abandoned"
			if op.closedBy == "doctor_sweep" {
				evidence += " (closed_by=doctor_sweep: the agent never closed the Op; the doctor swept it shut)"
			} else if op.closedBy != "" {
				evidence += " (closed_by=" + op.closedBy + ")"
			}
			addOpFinding("op_abandoned", "Spec Kitty Op was abandoned", "medium", recovery, op, evidence)
		}
	}
	out := make([]Finding, 0, len(byID))
	for _, f := range byID {
		out = append(out, *f)
	}
	sort.Slice(out, func(i, j int) bool {
		if severityRank(out[i].Severity) != severityRank(out[j].Severity) {
			return severityRank(out[i].Severity) > severityRank(out[j].Severity)
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func appendScopeUnique(scopes []Scope, scope Scope) []Scope {
	for _, existing := range scopes {
		if existing == scope {
			return scopes
		}
	}
	return append(scopes, scope)
}

func severityRank(sev string) int {
	switch sev {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func defaultSurface() SpecKittySurface {
	return SpecKittySurface{
		TopLevelCommands: []string{
			"init", "accept", "config", "dashboard", "implement", "intake", "specify", "plan", "tasks", "lint", "materialize", "merge", "next", "research", "review", "safe-commit", "session-start", "session-stop", "upgrade", "validate-encoding", "validate-tasks", "verify-setup", "dispatch", "agent", "auth", "charter", "context", "doctor", "doctrine", "glossary", "migrate", "mission", "mission-type", "ops", "plugin", "orchestrator-api", "sync", "workflow", "profiles", "profile-invocation", "invocations", "retrospect",
		},
		SlashCommands: []string{
			"spec-kitty.specify", "spec-kitty.research", "spec-kitty.plan", "spec-kitty.tasks", "spec-kitty.implement", "spec-kitty.review", "spec-kitty.accept", "spec-kitty.merge", "spec-kitty.dashboard", "spec-kitty.charter", "spec-kitty.status", "spec-kitty.analyze",
		},
		SkillFamilies: []string{
			"spk-start-*", "spk-mission-*", "spk-run-*", "spk-gate-*", "spk-admin-*", "spk-team-*", "spk-doctrine-*", "spk-integrate-*", "spk-meta-*", "legacy spec-kitty-*",
		},
		DecisionKinds: []string{"step", "query", "decision_required", "blocked", "terminal"},
		MissionTypes:  []string{"software-dev", "research", "plan", "documentation"},
		WPLanes:       []string{"genesis", "planned", "claimed", "in_progress", "for_review", "in_review", "approved", "done", "blocked", "canceled"},
	}
}
