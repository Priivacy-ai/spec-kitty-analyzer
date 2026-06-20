package analyzer

func normalizeReport(report *Report) {
	if report.Inputs == nil {
		report.Inputs = []InputFile{}
	}
	if report.Missions == nil {
		report.Missions = []MissionSummary{}
	}
	if report.Ops == nil {
		report.Ops = []OpSummary{}
	}
	if report.Timeline == nil {
		report.Timeline = []TimelineEvent{}
	}
	if report.Findings == nil {
		report.Findings = []Finding{}
	}
	if report.Notes == nil {
		report.Notes = []string{}
	}
	for i := range report.Timeline {
		if report.Timeline[i].SlashCommands == nil {
			report.Timeline[i].SlashCommands = []SlashCommand{}
		}
		if report.Timeline[i].CLIInvocations == nil {
			report.Timeline[i].CLIInvocations = []CLIInvocation{}
		}
		if report.Timeline[i].Skills == nil {
			report.Timeline[i].Skills = []SkillUse{}
		}
		if report.Timeline[i].AgentProfiles == nil {
			report.Timeline[i].AgentProfiles = []AgentProfileUse{}
		}
		if report.Timeline[i].Failures == nil {
			report.Timeline[i].Failures = []FailureFingerprint{}
		}
		if report.Timeline[i].RawJSONKeys == nil {
			report.Timeline[i].RawJSONKeys = []string{}
		}
	}
	for i := range report.Missions {
		if report.Missions[i].Files == nil {
			report.Missions[i].Files = []string{}
		}
		if report.Missions[i].WorkPackages == nil {
			report.Missions[i].WorkPackages = []WorkPackageState{}
		}
		if report.Missions[i].SlashCommands == nil {
			report.Missions[i].SlashCommands = []string{}
		}
		if report.Missions[i].CLIInvocations == nil {
			report.Missions[i].CLIInvocations = []string{}
		}
		if report.Missions[i].Skills == nil {
			report.Missions[i].Skills = []string{}
		}
		if report.Missions[i].FailureModes == nil {
			report.Missions[i].FailureModes = []string{}
		}
	}
	for i := range report.Ops {
		if report.Ops[i].Files == nil {
			report.Ops[i].Files = []string{}
		}
		if report.Ops[i].FailureModes == nil {
			report.Ops[i].FailureModes = []string{}
		}
	}
	for i := range report.Findings {
		if report.Findings[i].Scopes == nil {
			report.Findings[i].Scopes = []Scope{}
		}
		if report.Findings[i].Evidence == nil {
			report.Findings[i].Evidence = []FindingEvidence{}
		}
	}
}
