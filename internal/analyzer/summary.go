package analyzer

import "sort"

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

func buildFindings(events []TimelineEvent, ops []OpSummary) []Finding {
	byID := map[string]*Finding{}
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
		}
	}
	for _, op := range ops {
		if op.Status == "open" {
			id := "open_op_orphan"
			f := byID[id]
			if f == nil {
				f = &Finding{
					ID:            id,
					Title:         "Open Spec Kitty Op was not closed",
					Severity:      "medium",
					Recovery:      "Close the Op with spec-kitty profile-invocation complete --invocation-id <id> --outcome <done|failed|abandoned>.",
					Deterministic: true,
				}
				byID[id] = f
			}
			f.Count++
			f.Scopes = appendScopeUnique(f.Scopes, Scope{Type: "op", InvocationID: op.InvocationID})
			if len(f.Evidence) < 5 {
				f.Evidence = append(f.Evidence, FindingEvidence{Text: "kitty-ops/" + op.InvocationID + ".jsonl has no completed event"})
			}
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
