package query

import (
	"sort"
	"strings"
	"time"

	"github.com/priivacy-ai/spec-kitty-analyzer/internal/analyzer"
	"github.com/priivacy-ai/spec-kitty-analyzer/internal/reports"
)

type Options struct {
	Include    []string `json:"include,omitempty"`
	FailureIDs []string `json:"failure_ids,omitempty"`
	Commands   []string `json:"commands,omitempty"`
	Skills     []string `json:"skills,omitempty"`
	Profiles   []string `json:"profiles,omitempty"`
	Scopes     []string `json:"scopes,omitempty"`
	Contains   []string `json:"contains,omitempty"`
	Limit      int      `json:"limit,omitempty"`
}

type Result struct {
	Version        string                        `json:"version"`
	GeneratedAt    time.Time                     `json:"generated_at"`
	Mission        string                        `json:"mission,omitempty"`
	ShortTitle     string                        `json:"short_title,omitempty"`
	Cache          CacheInfo                     `json:"cache"`
	Query          Options                       `json:"query"`
	Summary        MatchSummary                  `json:"summary"`
	TimelineFilter reports.TimelineFilterSummary `json:"timeline_filter"`
	Inputs         []analyzer.InputFile          `json:"inputs,omitempty"`
	Missions       []analyzer.MissionSummary     `json:"missions,omitempty"`
	Ops            []analyzer.OpSummary          `json:"ops,omitempty"`
	Findings       []analyzer.Finding            `json:"findings,omitempty"`
	Timeline       []analyzer.TimelineEvent      `json:"timeline,omitempty"`
	Signals        *Signals                      `json:"signals,omitempty"`
	Surface        *analyzer.SpecKittySurface    `json:"spec_kitty_surface,omitempty"`
	Notes          []string                      `json:"notes,omitempty"`
}

type CacheInfo struct {
	Path         string    `json:"path,omitempty"`
	LastRunAt    time.Time `json:"last_run_at"`
	LogFiles     int       `json:"log_files"`
	Scanned      int       `json:"scanned"`
	Reused       int       `json:"reused"`
	Pruned       int       `json:"pruned"`
	Errored      int       `json:"errored"`
	MissionCount int       `json:"mission_count"`
	MissionFiles []string  `json:"mission_files,omitempty"`
}

type MatchSummary struct {
	InputFiles             int `json:"input_files"`
	ReportTimelineEvents   int `json:"report_timeline_events"`
	FilteredTimelineEvents int `json:"filtered_timeline_events"`
	MatchedTimelineEvents  int `json:"matched_timeline_events"`
	Missions               int `json:"missions"`
	Ops                    int `json:"ops"`
	Findings               int `json:"findings"`
	SlashCommandHits       int `json:"slash_command_hits"`
	CLIInvocationHits      int `json:"cli_invocation_hits"`
	SkillHits              int `json:"skill_hits"`
	AgentProfileHits       int `json:"agent_profile_hits"`
	FailureHits            int `json:"failure_hits"`
}

type Signals struct {
	SlashCommands  []SlashCommandHit  `json:"slash_commands,omitempty"`
	CLIInvocations []CLIInvocationHit `json:"cli_invocations,omitempty"`
	Skills         []SkillHit         `json:"skills,omitempty"`
	AgentProfiles  []AgentProfileHit  `json:"agent_profiles,omitempty"`
	Failures       []FailureHit       `json:"failures,omitempty"`
}

type EventRef struct {
	Seq        int            `json:"seq"`
	Turn       int            `json:"turn,omitempty"`
	Timestamp  *time.Time     `json:"timestamp,omitempty"`
	SourcePath string         `json:"source_path"`
	Line       int            `json:"line,omitempty"`
	Kind       string         `json:"kind"`
	Scope      analyzer.Scope `json:"scope"`
	Title      string         `json:"title,omitempty"`
	ToolName   string         `json:"tool_name,omitempty"`
	Failures   []string       `json:"failures,omitempty"`
	Preview    string         `json:"text_preview,omitempty"`
}

type SlashCommandHit struct {
	Event   EventRef              `json:"event"`
	Command analyzer.SlashCommand `json:"command"`
}

type CLIInvocationHit struct {
	Event      EventRef               `json:"event"`
	Invocation analyzer.CLIInvocation `json:"invocation"`
}

type SkillHit struct {
	Event EventRef          `json:"event"`
	Skill analyzer.SkillUse `json:"skill"`
}

type AgentProfileHit struct {
	Event   EventRef                 `json:"event"`
	Profile analyzer.AgentProfileUse `json:"profile"`
}

type FailureHit struct {
	Event   EventRef                    `json:"event"`
	Failure analyzer.FailureFingerprint `json:"failure"`
}

func Build(report analyzer.Report, mission string, shortTitle string, opts Options) Result {
	opts = normalizeOptions(opts)
	filteredTimeline := reports.FilteredTimeline(report)
	matchedTimeline := filterTimeline(filteredTimeline, opts)
	if opts.Limit > 0 && len(matchedTimeline) > opts.Limit {
		matchedTimeline = matchedTimeline[:opts.Limit]
	}
	signals := buildSignals(matchedTimeline)
	summary := MatchSummary{
		InputFiles:             len(report.Inputs),
		ReportTimelineEvents:   len(report.Timeline),
		FilteredTimelineEvents: len(filteredTimeline),
		MatchedTimelineEvents:  len(matchedTimeline),
		Missions:               len(report.Missions),
		Ops:                    len(report.Ops),
		Findings:               len(filterFindings(report.Findings, opts)),
		SlashCommandHits:       len(signals.SlashCommands),
		CLIInvocationHits:      len(signals.CLIInvocations),
		SkillHits:              len(signals.Skills),
		AgentProfileHits:       len(signals.AgentProfiles),
		FailureHits:            len(signals.Failures),
	}
	result := Result{
		Version:        report.Version,
		GeneratedAt:    report.GeneratedAt,
		Mission:        mission,
		ShortTitle:     shortTitle,
		Query:          opts,
		Summary:        summary,
		TimelineFilter: reports.TimelineFilter(report),
		Notes:          report.Notes,
	}
	include := includeSet(opts.Include)
	if include["all"] || include["inputs"] {
		result.Inputs = report.Inputs
	}
	if include["all"] || include["missions"] {
		result.Missions = report.Missions
	}
	if include["all"] || include["ops"] {
		result.Ops = report.Ops
	}
	if include["all"] || include["findings"] || include["failures"] {
		result.Findings = filterFindings(report.Findings, opts)
	}
	if include["all"] || include["timeline"] {
		result.Timeline = matchedTimeline
	}
	if include["all"] || include["signals"] || include["commands"] || include["skills"] || include["profiles"] || include["failures"] {
		result.Signals = signals
	}
	if include["all"] || include["surface"] {
		surface := report.Surface
		result.Surface = &surface
	}
	return result
}

func normalizeOptions(opts Options) Options {
	opts.Include = normalizeList(opts.Include)
	if len(opts.Include) == 0 {
		opts.Include = []string{"all"}
	}
	opts.FailureIDs = normalizeList(opts.FailureIDs)
	opts.Commands = normalizeList(opts.Commands)
	opts.Skills = normalizeList(opts.Skills)
	opts.Profiles = normalizeList(opts.Profiles)
	opts.Scopes = normalizeList(opts.Scopes)
	opts.Contains = normalizeContains(opts.Contains)
	return opts
}

func normalizeList(items []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range items {
		for _, part := range strings.Split(item, ",") {
			part = strings.TrimSpace(strings.ToLower(part))
			if part == "" || seen[part] {
				continue
			}
			seen[part] = true
			out = append(out, part)
		}
	}
	sort.Strings(out)
	return out
}

func normalizeContains(items []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		key := strings.ToLower(item)
		if item == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}

func includeSet(items []string) map[string]bool {
	out := map[string]bool{}
	for _, item := range items {
		out[item] = true
	}
	return out
}

func filterTimeline(events []analyzer.TimelineEvent, opts Options) []analyzer.TimelineEvent {
	out := make([]analyzer.TimelineEvent, 0, len(events))
	for _, event := range events {
		if eventMatches(event, opts) {
			out = append(out, event)
		}
	}
	return out
}

func eventMatches(event analyzer.TimelineEvent, opts Options) bool {
	if len(opts.FailureIDs) > 0 && !eventHasFailure(event, opts.FailureIDs) {
		return false
	}
	if len(opts.Commands) > 0 && !eventHasCommand(event, opts.Commands) {
		return false
	}
	if len(opts.Skills) > 0 && !eventHasSkill(event, opts.Skills) {
		return false
	}
	if len(opts.Profiles) > 0 && !eventHasProfile(event, opts.Profiles) {
		return false
	}
	if len(opts.Scopes) > 0 && !eventHasScope(event, opts.Scopes) {
		return false
	}
	if len(opts.Contains) > 0 && !eventContains(event, opts.Contains) {
		return false
	}
	return true
}

func eventHasFailure(event analyzer.TimelineEvent, needles []string) bool {
	for _, failure := range event.Failures {
		if anyMatch(failure.ID, needles) || anyMatch(failure.Title, needles) {
			return true
		}
	}
	return false
}

func eventHasCommand(event analyzer.TimelineEvent, needles []string) bool {
	if anyMatch(event.Scope.Action, needles) {
		return true
	}
	for _, slash := range event.SlashCommands {
		if anyMatch(slash.Name, needles) || anyMatch(slash.Action, needles) || anyMatch(slash.Raw, needles) {
			return true
		}
	}
	for _, inv := range event.CLIInvocations {
		if anyMatch(inv.Raw, needles) || anyMatch(inv.Verb, needles) || anyMatch(inv.Subcommand, needles) || anyMatch(inv.Mission, needles) || anyMatch(inv.WorkPackage, needles) || anyMatch(inv.Agent, needles) || anyMatch(inv.Profile, needles) {
			return true
		}
	}
	for _, failure := range event.Failures {
		if anyMatch(failure.ID, needles) || anyMatch(failure.Title, needles) {
			return true
		}
	}
	return false
}

func eventHasSkill(event analyzer.TimelineEvent, needles []string) bool {
	for _, skill := range event.Skills {
		if anyMatch(skill.Name, needles) || anyMatch(skill.Path, needles) || anyMatch(skill.Raw, needles) {
			return true
		}
	}
	return false
}

func eventHasProfile(event analyzer.TimelineEvent, needles []string) bool {
	for _, profile := range event.AgentProfiles {
		if anyMatch(profile.Profile, needles) || anyMatch(profile.Agent, needles) || anyMatch(profile.Role, needles) || anyMatch(profile.Raw, needles) {
			return true
		}
	}
	for _, inv := range event.CLIInvocations {
		if anyMatch(inv.Profile, needles) || anyMatch(inv.Agent, needles) {
			return true
		}
	}
	return false
}

func eventHasScope(event analyzer.TimelineEvent, needles []string) bool {
	values := []string{event.Scope.Type, scopeString(event.Scope), event.Scope.MissionSlug, event.Scope.InvocationID, event.Scope.WorkPackage, event.Scope.Action}
	for _, value := range values {
		if anyMatch(value, needles) {
			return true
		}
	}
	return false
}

func eventContains(event analyzer.TimelineEvent, needles []string) bool {
	values := []string{event.Title, event.TextPreview, event.ToolName, event.Kind, scopeString(event.Scope)}
	for _, slash := range event.SlashCommands {
		values = append(values, slash.Name, slash.Action, slash.Raw)
	}
	for _, inv := range event.CLIInvocations {
		values = append(values, inv.Raw, inv.Verb, inv.Subcommand, inv.Mission, inv.WorkPackage, inv.Agent, inv.Profile)
	}
	for _, skill := range event.Skills {
		values = append(values, skill.Name, skill.Path, skill.Raw)
	}
	for _, profile := range event.AgentProfiles {
		values = append(values, profile.Profile, profile.Agent, profile.Role, profile.Raw)
	}
	for _, failure := range event.Failures {
		values = append(values, failure.ID, failure.Title, failure.Reason, failure.Recovery)
	}
	for _, value := range values {
		if anyMatch(value, needles) {
			return true
		}
	}
	return false
}

func anyMatch(value string, needles []string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false
	}
	for _, needle := range needles {
		needle = strings.ToLower(strings.TrimSpace(needle))
		if needle == "" {
			continue
		}
		if value == needle || strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func buildSignals(events []analyzer.TimelineEvent) *Signals {
	signals := &Signals{}
	for _, event := range events {
		ref := eventRef(event)
		for _, slash := range event.SlashCommands {
			signals.SlashCommands = append(signals.SlashCommands, SlashCommandHit{Event: ref, Command: slash})
		}
		for _, inv := range event.CLIInvocations {
			signals.CLIInvocations = append(signals.CLIInvocations, CLIInvocationHit{Event: ref, Invocation: inv})
		}
		for _, skill := range event.Skills {
			signals.Skills = append(signals.Skills, SkillHit{Event: ref, Skill: skill})
		}
		for _, profile := range event.AgentProfiles {
			signals.AgentProfiles = append(signals.AgentProfiles, AgentProfileHit{Event: ref, Profile: profile})
		}
		for _, failure := range event.Failures {
			signals.Failures = append(signals.Failures, FailureHit{Event: ref, Failure: failure})
		}
	}
	return signals
}

func eventRef(event analyzer.TimelineEvent) EventRef {
	failures := make([]string, 0, len(event.Failures))
	for _, failure := range event.Failures {
		failures = append(failures, failure.ID)
	}
	return EventRef{
		Seq:        event.Seq,
		Turn:       event.Turn,
		Timestamp:  event.Timestamp,
		SourcePath: event.SourcePath,
		Line:       event.Line,
		Kind:       event.Kind,
		Scope:      event.Scope,
		Title:      event.Title,
		ToolName:   event.ToolName,
		Failures:   failures,
		Preview:    event.TextPreview,
	}
}

func filterFindings(findings []analyzer.Finding, opts Options) []analyzer.Finding {
	if len(opts.FailureIDs) == 0 {
		return findings
	}
	out := make([]analyzer.Finding, 0, len(findings))
	for _, finding := range findings {
		if anyMatch(finding.ID, opts.FailureIDs) || anyMatch(finding.Title, opts.FailureIDs) {
			out = append(out, finding)
		}
	}
	return out
}

func scopeString(scope analyzer.Scope) string {
	switch scope.Type {
	case "mission":
		if scope.WorkPackage != "" {
			return "mission:" + scope.MissionSlug + "/" + scope.WorkPackage
		}
		return "mission:" + scope.MissionSlug
	case "op":
		return "op:" + scope.InvocationID
	default:
		return "outside"
	}
}
