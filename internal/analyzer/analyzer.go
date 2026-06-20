package analyzer

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const maxInputFileBytes int64 = 50 * 1024 * 1024

func Analyze(paths []string) (Report, error) {
	if len(paths) == 0 {
		paths = []string{"."}
	}
	files, err := collectFiles(paths)
	if err != nil {
		return Report{}, err
	}
	if len(files) == 0 {
		return Report{}, errors.New("no supported input files found")
	}

	state := newBuildState()
	report := Report{
		Version:     Version,
		GeneratedAt: time.Now().UTC(),
		Redactions:  map[string]int{},
		Surface:     defaultSurface(),
	}
	turn := 0
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		input := InputFile{Path: path, Kind: classifyPathKind(path), Bytes: info.Size()}
		report.Inputs = append(report.Inputs, input)
		if info.Size() > maxInputFileBytes {
			report.Notes = append(report.Notes, fmt.Sprintf("skipped %s: larger than %d bytes", path, maxInputFileBytes))
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			report.Notes = append(report.Notes, fmt.Sprintf("skipped %s: %v", path, err))
			continue
		}
		scrubbed, redactions := Scrub(data)
		mergeCounts(report.Redactions, redactions)
		events, nextTurn := parseFile(path, input.Kind, scrubbed, turn, state)
		turn = nextTurn
		report.Timeline = append(report.Timeline, events...)
	}

	report.Timeline = sortTimeline(report.Timeline)
	for i := range report.Timeline {
		report.Timeline[i].Seq = i + 1
	}
	state.absorbTimeline(report.Timeline)
	report.Missions = state.missions()
	report.Ops = state.opSummaries()
	report.Findings = buildFindings(report.Timeline, report.Ops)
	report.Summary = buildSummary(report)
	normalizeReport(&report)
	return report, nil
}

func AnalyzeMission(paths []string, slug string) (Report, error) {
	slug = normalizeMissionHandle(slug)
	if slug == "" {
		return Report{}, errors.New("mission slug is required")
	}
	report, err := Analyze(paths)
	if err != nil {
		return Report{}, err
	}
	filtered := filterReportByMission(report, slug)
	if len(filtered.Timeline) == 0 {
		return Report{}, fmt.Errorf("no timeline events found for mission %q", slug)
	}
	return filtered, nil
}

func filterReportByMission(report Report, slug string) Report {
	keptSources := map[string]bool{}
	timeline := make([]TimelineEvent, 0, len(report.Timeline))
	for _, event := range report.Timeline {
		if eventReferencesMission(event, slug) {
			timeline = append(timeline, event)
			keptSources[event.SourcePath] = true
		}
	}
	for i := range timeline {
		timeline[i].Seq = i + 1
	}

	inputs := make([]InputFile, 0, len(report.Inputs))
	for _, input := range report.Inputs {
		if keptSources[input.Path] {
			inputs = append(inputs, input)
		}
	}

	state := newBuildState()
	state.absorbTimeline(timeline)
	report.Inputs = inputs
	report.Timeline = timeline
	report.Missions = state.missions()
	report.Ops = state.opSummaries()
	report.Findings = buildFindings(report.Timeline, report.Ops)
	report.Summary = buildSummary(report)
	report.Notes = append(report.Notes, fmt.Sprintf("filtered to mission %s", slug))
	normalizeReport(&report)
	return report
}

func eventReferencesMission(event TimelineEvent, slug string) bool {
	if event.Scope.MissionSlug == slug {
		return true
	}
	if event.Scope.Type == "mission" && event.Scope.MissionSlug != "" {
		return false
	}
	for _, inv := range event.CLIInvocations {
		if inv.Mission == slug {
			return true
		}
	}
	return strings.Contains(event.TextPreview, slug)
}

func collectFiles(paths []string) ([]string, error) {
	seen := map[string]bool{}
	var files []string
	for _, raw := range paths {
		path := filepath.Clean(raw)
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if supportedFile(path) && !seen[path] {
				seen[path] = true
				files = append(files, path)
			}
			continue
		}
		err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if skipDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if supportedFile(p) && !seen[p] {
				seen[p] = true
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}

func skipDir(name string) bool {
	switch name {
	case ".git", ".venv", "node_modules", "vendor", "dist", "build", "__pycache__", ".pytest_cache", ".ruff_cache":
		return true
	default:
		return false
	}
}

func supportedFile(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jsonl", ".json", ".log", ".txt", ".md", ".yaml", ".yml":
		return true
	}
	return base == "status.events.jsonl" || base == "meta.json" || base == "status.json" || base == "lanes.json"
}

func classifyPathKind(path string) string {
	slash := filepath.ToSlash(path)
	base := filepath.Base(path)
	switch {
	case strings.Contains(slash, "/kitty-ops/"):
		return "op_jsonl"
	case strings.Contains(slash, "/kitty-specs/") && base == "status.events.jsonl":
		return "mission_status_events"
	case strings.Contains(slash, "/kitty-specs/") && base == "meta.json":
		return "mission_meta"
	case strings.Contains(slash, "/kitty-specs/") && base == "status.json":
		return "mission_status_snapshot"
	case strings.Contains(slash, "/kitty-specs/") && strings.HasPrefix(base, "WP") && strings.HasSuffix(base, ".md"):
		return "work_package"
	case strings.Contains(slash, "/kitty-specs/"):
		return "mission_artifact"
	case strings.HasSuffix(base, ".jsonl"):
		return "jsonl_transcript"
	case strings.HasSuffix(base, ".json"):
		return "json"
	default:
		return "text"
	}
}

func parseFile(path, kind string, data []byte, startTurn int, state *buildState) ([]TimelineEvent, int) {
	if kind == "mission_meta" {
		state.readMissionMeta(path, data)
	}
	if kind == "work_package" {
		state.readWorkPackage(path, data)
	}
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		if obj, ok := decodeJSONObject(bytes.TrimSpace(data)); ok {
			event := eventFromJSONObject(path, 1, startTurn+1, obj)
			if event.Kind != "" && !skipArtifactMessage(kind, event) {
				return []TimelineEvent{event}, startTurn + 1
			}
		}
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var events []TimelineEvent
	lineNo := 0
	turn := startTurn
	for scanner.Scan() {
		lineNo++
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		turn++
		if obj, ok := decodeJSONObject(raw); ok {
			events = append(events, eventFromJSONObject(path, lineNo, turn, obj))
			continue
		}
		text := string(raw)
		event := eventFromText(path, lineNo, turn, text, nil)
		if event.Kind != "" && !skipArtifactMessage(kind, event) {
			events = append(events, event)
		}
	}
	return events, turn
}

func eventFromJSONObject(path string, line, turn int, obj map[string]any) TimelineEvent {
	text := flattenJSON(obj)
	if strings.TrimSpace(text) == "" {
		encoded, _ := json.Marshal(obj)
		text = string(encoded)
	}
	event := eventFromText(path, line, turn, text, obj)
	event.Timestamp = parseJSONTime(obj)
	event.RawJSONKeys = jsonKeys(obj)
	event.ToolName = firstJSONStringByKey(obj, "tool", "tool_name", "name")
	if profile := firstJSONStringByKey(obj, "profile_id", "profile"); profile != "" {
		event.AgentProfiles = append(event.AgentProfiles, AgentProfileUse{Profile: profile, Raw: "profile_id:" + profile})
	}
	event.Scope = mergeScope(event.Scope, scopeFromJSON(obj))
	if event.Kind == "message" {
		event.Kind = kindFromJSON(path, obj, event)
	}
	event.Title = titleForEvent(event)
	return event
}

func skipArtifactMessage(kind string, event TimelineEvent) bool {
	switch kind {
	case "work_package", "mission_artifact", "mission_meta", "mission_status_snapshot":
		if event.Kind == "message" {
			return true
		}
		if (kind == "work_package" || kind == "mission_artifact") && len(event.Failures) > 0 {
			return !artifactFailureAllowed(event)
		}
		return false
	default:
		return false
	}
}

func artifactFailureAllowed(event TimelineEvent) bool {
	for _, failure := range event.Failures {
		switch failure.ID {
		case "review_rejected":
			return true
		}
	}
	return false
}

func eventFromText(path string, line, turn int, text string, obj map[string]any) TimelineEvent {
	slash := detectSlashCommands(text)
	cli := detectCLIInvocations(text)
	skills := detectSkills(text)
	profiles := detectAgentProfiles(text)
	scope := scopeFromPathAndText(path, text, cli, slash)
	action := actionFromCommand(firstInvocation(cli), slash)
	if scope.Action == "" {
		scope.Action = action
	}
	failures := classifyFailures(text, obj, cli)
	kind := "message"
	switch {
	case len(failures) > 0:
		kind = "failure"
	case len(cli) > 0:
		kind = "cli_invocation"
	case len(slash) > 0:
		kind = "slash_command"
	case len(skills) > 0:
		kind = "skill_read"
	case len(profiles) > 0:
		kind = "agent_profile"
	case scope.Type == "mission" && strings.Contains(filepath.Base(path), "status.events"):
		kind = "mission_event"
	case scope.Type == "op":
		kind = "op_event"
	}
	return TimelineEvent{
		Turn:           turn,
		SourcePath:     path,
		Line:           line,
		Kind:           kind,
		Title:          titleForKind(kind, text, slash, cli, skills, failures),
		Scope:          scope,
		TextPreview:    preview(text, 320),
		SlashCommands:  slash,
		CLIInvocations: cli,
		Skills:         skills,
		AgentProfiles:  profiles,
		Failures:       failures,
	}
}

func kindFromJSON(path string, obj map[string]any, event TimelineEvent) string {
	eventType := strings.ToLower(firstJSONStringByKey(obj, "event", "event_type", "type", "kind", "status", "outcome"))
	switch {
	case strings.Contains(filepath.ToSlash(path), "/kitty-ops/"):
		return "op_event"
	case strings.Contains(filepath.Base(path), "status.events"):
		return "mission_event"
	case eventType == "blocked" || len(event.Failures) > 0:
		return "failure"
	case strings.Contains(eventType, "tool"):
		return "tool"
	case eventType != "":
		return eventType
	default:
		return "message"
	}
}

func scopeFromPathAndText(path, text string, invocations []CLIInvocation, slash []SlashCommand) Scope {
	scope := Scope{Type: "outside"}
	slashPath := filepath.ToSlash(path)
	if m := opPathRE.FindStringSubmatch(slashPath); len(m) > 1 {
		scope.Type = "op"
		scope.InvocationID = m[1]
	}
	if m := missionPathRE.FindStringSubmatch(slashPath); len(m) > 1 {
		scope.Type = "mission"
		scope.MissionSlug = m[1]
	}
	for _, inv := range invocations {
		if inv.Mission != "" {
			scope.Type = "mission"
			scope.MissionSlug = inv.Mission
		}
		if inv.WorkPackage != "" && scope.WorkPackage == "" {
			scope.WorkPackage = inv.WorkPackage
		}
	}
	if scope.MissionSlug == "" {
		if idx := strings.Index(text, "--mission "); idx >= 0 {
			fields := strings.Fields(text[idx:])
			if len(fields) >= 2 {
				mission := normalizeMissionHandle(fields[1])
				if mission != "" {
					scope.Type = "mission"
					scope.MissionSlug = mission
				}
			}
		}
	}
	if scope.InvocationID == "" {
		if id := firstInvocationIDText(text); id != "" {
			scope.InvocationID = id
			if scope.Type == "outside" {
				scope.Type = "op"
			}
		}
	}
	if scope.WorkPackage == "" {
		scope.WorkPackage = wpRE.FindString(text)
	}
	if scope.Action == "" {
		scope.Action = actionFromCommand(firstInvocation(invocations), slash)
	}
	return scope
}

func scopeFromJSON(obj map[string]any) Scope {
	scope := Scope{Type: "outside"}
	if mission := firstJSONStringByKey(obj, "mission_slug", "feature_slug"); mission != "" {
		if mission = normalizeMissionHandle(mission); mission != "" {
			scope.Type = "mission"
			scope.MissionSlug = mission
		}
	}
	if wp := firstJSONStringByKey(obj, "wp_id", "work_package_id", "work_package"); wp != "" {
		scope.WorkPackage = wp
	}
	if action := firstJSONStringByKey(obj, "action", "step_id"); action != "" {
		scope.Action = action
	}
	if inv := firstJSONStringByKey(obj, "invocation_id"); inv != "" {
		scope.InvocationID = inv
		if scope.Type == "outside" {
			scope.Type = "op"
		}
	}
	return scope
}

func mergeScope(a, b Scope) Scope {
	out := a
	if out.Type == "" || out.Type == "outside" {
		out.Type = b.Type
	}
	if b.MissionSlug != "" {
		out.MissionSlug = b.MissionSlug
		if out.Type == "outside" {
			out.Type = "mission"
		}
	}
	if b.InvocationID != "" {
		out.InvocationID = b.InvocationID
		if out.Type == "outside" {
			out.Type = "op"
		}
	}
	if b.WorkPackage != "" {
		out.WorkPackage = b.WorkPackage
	}
	if b.Action != "" {
		out.Action = b.Action
	}
	if out.Type == "" {
		out.Type = "outside"
	}
	return out
}

func firstInvocationIDText(text string) string {
	key := "invocation_id"
	idx := strings.Index(text, key)
	if idx < 0 {
		return ""
	}
	tail := text[idx+len(key):]
	tail = strings.TrimLeft(tail, ` "':=`)
	fields := strings.Fields(tail)
	if len(fields) == 0 {
		return ""
	}
	return trimShell(fields[0])
}

func firstInvocation(inv []CLIInvocation) CLIInvocation {
	if len(inv) == 0 {
		return CLIInvocation{}
	}
	return inv[0]
}

func titleForEvent(event TimelineEvent) string {
	return titleForKind(event.Kind, event.TextPreview, event.SlashCommands, event.CLIInvocations, event.Skills, event.Failures)
}

func titleForKind(kind, text string, slash []SlashCommand, cli []CLIInvocation, skills []SkillUse, failures []FailureFingerprint) string {
	switch {
	case len(failures) > 0:
		return failures[0].Title
	case len(cli) > 0:
		return "CLI: " + cli[0].Raw
	case len(slash) > 0:
		return "Command: /" + slash[0].Name
	case len(skills) > 0:
		return "Skill: " + skills[0].Name
	case kind == "mission_event":
		return "Mission event"
	case kind == "op_event":
		return "Op event"
	default:
		return preview(text, 80)
	}
}

func preview(text string, limit int) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "..."
}

func sortTimeline(events []TimelineEvent) []TimelineEvent {
	sort.SliceStable(events, func(i, j int) bool {
		ti, tj := events[i].Timestamp, events[j].Timestamp
		if ti != nil && tj != nil && !ti.Equal(*tj) {
			return ti.Before(*tj)
		}
		if ti != nil && tj == nil {
			return true
		}
		if ti == nil && tj != nil {
			return false
		}
		if events[i].SourcePath != events[j].SourcePath {
			return events[i].SourcePath < events[j].SourcePath
		}
		return events[i].Line < events[j].Line
	})
	return events
}

func mergeCounts(dst, src map[string]int) {
	for k, v := range src {
		dst[k] += v
	}
}
