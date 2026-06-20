package analyzer

import (
	"regexp"
	"strings"
)

var (
	slashCommandRE  = regexp.MustCompile(`(?i)(?:^|[\s"'(:` + "`" + `])(/spec-kitty[.\-][a-z][a-z0-9_-]*)\b`)
	specKittyCLIRe  = regexp.MustCompile(`(?m)(?:^|[\s$` + "`" + `;&|])((?:SPEC_KITTY_ENABLE_SAAS_SYNC=1\s+)?(?:(?:uv|uvx)\s+run\s+)?spec-kitty(?:\s+[^` + "`" + `\n\r;&|]+|$))`)
	skillPathRE     = regexp.MustCompile(`(?i)([A-Za-z0-9_./~@+\-]*((?:spk-[a-z0-9]+-[a-z0-9_.\-]+)|(?:spec-kitty-(?:bulk-edit-classification|charter-doctrine|git-workflow|glossary-context|implement-review|mission-review|mission-system|orchestrator-api-operator|program-orchestrate|runtime-next|runtime-review|setup-doctor|spdd-reasons|agent-surface-research|cli-orchestration|delegated-missions|docker-modes|monorepo-prep))|(?:spec-kitty(?:\.[a-z0-9_.\-]+)?)|ad-hoc-profile-load)/SKILL\.md)`)
	skillNameRE     = regexp.MustCompile(`(?i)(?:^|[\s"'(:` + "`" + `/])((?:spk-[a-z0-9]+-[a-z0-9_.\-]+)|(?:spec-kitty-(?:bulk-edit-classification|charter-doctrine|git-workflow|glossary-context|implement-review|mission-review|mission-system|orchestrator-api-operator|program-orchestrate|runtime-next|runtime-review|setup-doctor|spdd-reasons|agent-surface-research|cli-orchestration|delegated-missions|docker-modes|monorepo-prep))|(?:spec-kitty\.[a-z0-9_.\-]+)|ad-hoc-profile-load)\b`)
	missionPathRE   = regexp.MustCompile(`(?:^|[/\s])kitty-specs/([A-Za-z0-9][A-Za-z0-9_.\-]*)`)
	missionHandleRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.\-]{0,127}$`)
	opPathRE        = regexp.MustCompile(`(?:^|[/\s])kitty-ops/([A-Za-z0-9][A-Za-z0-9_.\-]*)\.jsonl`)
	wpRE            = regexp.MustCompile(`\bWP[0-9]{2,4}\b`)
	profileFlagRE   = regexp.MustCompile(`--profile\s+([A-Za-z0-9_.:\-]+)`)
	agentFlagRE     = regexp.MustCompile(`--agent\s+([A-Za-z0-9_.:\-]+)`)
)

var knownSlashActions = map[string]string{
	"accept":              "accept",
	"analyze":             "analyze",
	"charter":             "charter",
	"dashboard":           "dashboard",
	"implement":           "implement",
	"merge":               "merge",
	"plan":                "plan",
	"research":            "research",
	"review":              "review",
	"specify":             "specify",
	"status":              "status",
	"tasks":               "tasks",
	"tasks-finalize":      "tasks",
	"tasks-outline":       "tasks",
	"tasks-packages":      "tasks",
	"mission-review":      "mission_review",
	"runtime-next":        "next",
	"runtime-review":      "review",
	"implement-review":    "implement_review",
	"setup-doctor":        "doctor",
	"git-workflow":        "git",
	"program-orchestrate": "program",
}

func detectSlashCommands(text string) []SlashCommand {
	seen := map[string]bool{}
	var out []SlashCommand
	for _, m := range slashCommandRE.FindAllStringSubmatch(text, -1) {
		raw := strings.TrimSpace(m[1])
		name := strings.TrimPrefix(strings.ToLower(raw), "/")
		if seen[name] {
			continue
		}
		seen[name] = true
		action := ""
		if idx := strings.LastIndex(name, "."); idx >= 0 {
			action = knownSlashActions[name[idx+1:]]
		} else if idx := strings.LastIndex(name, "-"); idx >= 0 {
			action = knownSlashActions[name[idx+1:]]
		}
		out = append(out, SlashCommand{Name: name, Action: action, Raw: raw})
	}
	return out
}

func detectCLIInvocations(text string) []CLIInvocation {
	seen := map[string]bool{}
	var out []CLIInvocation
	for _, m := range specKittyCLIRe.FindAllStringSubmatch(text, -1) {
		raw := strings.TrimSpace(m[1])
		raw = strings.Trim(raw, `"'`)
		if raw == "" || seen[raw] {
			continue
		}
		seen[raw] = true
		out = append(out, parseCLIInvocation(raw))
	}
	return out
}

func parseCLIInvocation(raw string) CLIInvocation {
	fields := shellishFields(raw)
	inv := CLIInvocation{
		Raw:             raw,
		Args:            fields,
		SaaSSyncEnabled: strings.HasPrefix(raw, "SPEC_KITTY_ENABLE_SAAS_SYNC=1 "),
	}
	specIdx := -1
	for i, f := range fields {
		if f == "spec-kitty" {
			specIdx = i
			break
		}
	}
	if specIdx < 0 {
		return inv
	}
	args := fields[specIdx+1:]
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		args = args[1:]
	}
	if len(args) > 0 {
		inv.Verb = args[0]
	}
	if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		inv.Subcommand = inv.Verb + " " + args[1]
	}
	for i := 0; i < len(fields)-1; i++ {
		switch fields[i] {
		case "--mission", "--feature":
			inv.Mission = normalizeMissionHandle(fields[i+1])
		case "--agent":
			inv.Agent = trimShell(fields[i+1])
			parts := strings.Split(inv.Agent, ":")
			if len(parts) >= 3 {
				inv.Profile = parts[2]
			}
		case "--profile":
			inv.Profile = trimShell(fields[i+1])
		}
	}
	if wp := wpRE.FindString(raw); wp != "" {
		inv.WorkPackage = wp
	}
	return inv
}

func shellishFields(raw string) []string {
	raw = strings.ReplaceAll(raw, "\\\n", " ")
	fields := strings.Fields(raw)
	for i := range fields {
		fields[i] = trimShell(fields[i])
	}
	return fields
}

func trimShell(value string) string {
	return strings.Trim(value, `"',;`+"`")
}

func normalizeMissionHandle(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "<>{}$") || strings.Contains(raw, `\`) {
		return ""
	}
	raw = trimShell(raw)
	raw = strings.Trim(raw, "[]()")
	raw = strings.TrimRight(raw, ".,:;")
	if raw == "" || strings.ContainsAny(raw, " \t\r\n") || !missionHandleRE.MatchString(raw) {
		return ""
	}
	switch strings.ToLower(raw) {
	case "slug", "handle", "mission", "mission-slug", "mission_slug", "feature", "feature-slug", "feature_slug", "your-mission", "your-mission-slug", "my-mission", "text", "whitespace":
		return ""
	default:
		return raw
	}
}

func detectSkills(text string) []SkillUse {
	seen := map[string]bool{}
	var out []SkillUse
	for _, m := range skillPathRE.FindAllStringSubmatch(text, -1) {
		path := strings.TrimSpace(m[1])
		name := normalizeSkillName(m[2])
		if name == "" || seen[name+"|"+path] {
			continue
		}
		seen[name+"|"+path] = true
		out = append(out, SkillUse{Name: name, Path: path, Raw: path})
	}
	for _, m := range skillNameRE.FindAllStringSubmatch(text, -1) {
		raw := strings.TrimSpace(m[1])
		name := normalizeSkillName(raw)
		if name == "" || seen[name+"|"] {
			continue
		}
		seen[name+"|"] = true
		out = append(out, SkillUse{Name: name, Raw: raw})
	}
	return out
}

func normalizeSkillName(raw string) string {
	raw = strings.TrimSuffix(raw, "/SKILL.md")
	raw = strings.Trim(raw, `/ "'`+"`"+`()`)
	if idx := strings.LastIndex(raw, "/"); idx >= 0 {
		raw = raw[idx+1:]
	}
	return strings.ToLower(strings.ReplaceAll(raw, "_", "-"))
}

func detectAgentProfiles(text string) []AgentProfileUse {
	seen := map[string]bool{}
	var out []AgentProfileUse
	for _, m := range profileFlagRE.FindAllStringSubmatch(text, -1) {
		profile := trimShell(m[1])
		if profile == "" || seen["p:"+profile] {
			continue
		}
		seen["p:"+profile] = true
		out = append(out, AgentProfileUse{Profile: profile, Raw: "--profile " + profile})
	}
	for _, m := range agentFlagRE.FindAllStringSubmatch(text, -1) {
		agent := trimShell(m[1])
		if agent == "" {
			continue
		}
		parts := strings.Split(agent, ":")
		profile := ""
		role := ""
		if len(parts) >= 3 {
			profile = parts[2]
		}
		if len(parts) >= 4 {
			role = parts[3]
		}
		if profile == "" {
			profile = "unknown"
		}
		key := "a:" + agent + ":" + profile
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, AgentProfileUse{Profile: profile, Agent: agent, Role: role, Raw: "--agent " + agent})
	}
	return out
}

func actionFromCommand(inv CLIInvocation, slash []SlashCommand) string {
	if inv.Verb != "" {
		switch inv.Verb {
		case "specify", "plan", "tasks", "implement", "review", "accept", "merge", "next", "dispatch", "research":
			return inv.Verb
		case "agent":
			if strings.HasPrefix(inv.Subcommand, "agent action") {
				parts := strings.Fields(inv.Raw)
				for i, p := range parts {
					if p == "action" && i+1 < len(parts) {
						return trimShell(parts[i+1])
					}
				}
			}
		}
	}
	for _, s := range slash {
		if s.Action != "" {
			return s.Action
		}
	}
	return ""
}
