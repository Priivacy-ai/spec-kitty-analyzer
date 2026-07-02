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
	ext := strings.ToLower(filepath.Ext(path))
	inKittyOps := hasPathSegment(slash, "kitty-ops")
	inKittySpecs := hasPathSegment(slash, "kitty-specs")
	switch {
	case inKittyOps:
		return "op_jsonl"
	case inKittySpecs && base == "status.events.jsonl":
		return "mission_status_events"
	case inKittySpecs && base == "meta.json":
		return "mission_meta"
	case inKittySpecs && base == "status.json":
		return "mission_status_snapshot"
	case inKittySpecs && strings.HasPrefix(base, "WP") && strings.HasSuffix(base, ".md"):
		return "work_package"
	case inKittySpecs:
		return "mission_artifact"
	case strings.HasSuffix(base, ".jsonl"):
		return "jsonl_transcript"
	case strings.HasSuffix(base, ".json"):
		return "json"
	case ext == ".log":
		return "command_log"
	default:
		return "text"
	}
}

func hasPathSegment(slashPath, segment string) bool {
	for _, part := range strings.Split(slashPath, "/") {
		if part == segment {
			return true
		}
	}
	return false
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
			if event.Kind != "" && !skipArtifactMessage(kind, &event) {
				return []TimelineEvent{event}, startTurn + 1
			}
		}
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var events []TimelineEvent
	lineNo := 0
	turn := startTurn
	inWorkPackageFrontmatter := false
	for scanner.Scan() {
		lineNo++
		raw := bytes.TrimSpace(scanner.Bytes())
		frontmatterLine := false
		if kind == "work_package" {
			switch {
			case lineNo == 1 && bytes.Equal(raw, []byte("---")):
				inWorkPackageFrontmatter = true
			case inWorkPackageFrontmatter && bytes.Equal(raw, []byte("---")):
				inWorkPackageFrontmatter = false
			case inWorkPackageFrontmatter:
				frontmatterLine = true
			}
		}
		if len(raw) == 0 {
			continue
		}
		turn++
		var event TimelineEvent
		if obj, ok := decodeJSONObject(raw); ok {
			event = eventFromJSONObject(path, lineNo, turn, obj)
		} else {
			event = eventFromText(path, lineNo, turn, string(raw), nil)
		}
		// Single suppression gate (design issue-4 §5): every parseFile event — JSON
		// object line or plain text line — passes through skipArtifactMessage once,
		// before it joins the timeline / findings aggregation. (Previously the JSON
		// object-line branch bypassed the gate, so an artifact .md carrying a JSON
		// line could leak a suppressed failure; routing both branches through the
		// same predicate closes that and matches the §5 single-gate intent.)
		if frontmatterLine {
			addWorkPackageFrontmatterFailures(&event, string(raw))
		}
		if event.Kind != "" && !skipArtifactMessage(kind, &event) {
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

// skipArtifactMessage is the SINGLE suppression gate (design issue-4 §5). It is
// applied once per event in parseFile, AFTER classification and BEFORE the event
// reaches the timeline (and thus before findings aggregation in buildFindings), so
// an artifact/spec event never contributes a failure to the roll-up. The same gate
// point will later also gate Tier-3 anomalies (separate PR); routing every parseFile
// event through this one predicate keeps that future gating in lock-step.
//
// It drops (a) plain artifact "message" noise and (b) every non-whitelisted
// artifact-derived failure, applied UNIFORMLY across all four artifact kinds
// (work_package, mission_artifact, mission_meta, mission_status_snapshot — the set
// isArtifactKind keys on). Suppression is PER-FAILURE, not per-event: for an
// artifact event carrying failures, event.Failures is filtered down to the
// whitelisted IDs (artifactFailureWhitelist) in place; the rest are dropped. If no
// whitelisted failure remains, the event is suppressed entirely (return true), so it
// contributes nothing to findings. This is why the event is taken by pointer — the
// filtered slice must be observed by the caller before aggregation.
//
// The whitelist intentionally surfaces review_rejected only after the work-package
// frontmatter-specific detector adds it; arbitrary artifact prose cannot create it.
func skipArtifactMessage(kind string, event *TimelineEvent) bool {
	if !isArtifactKind(kind) {
		return false
	}
	if event.Kind == "message" {
		return true
	}
	if len(event.Failures) > 0 {
		before := len(event.Failures)
		event.Failures = retainWhitelistedArtifactFailures(kind, event.Failures)
		if len(event.Failures) == 0 {
			return true
		}
		// event.Title was derived from the pre-filter failures[0] (titleForEvent /
		// titleForKind) BEFORE this gate ran. If the filter dropped any failure but
		// at least one survives, that title may now name a dropped failure while
		// event.Failures lists only the whitelisted ones. Recompute the title from
		// the surviving failures so it stays in sync with failures[0]. Only fires on
		// the changed-and-non-empty artifact case: non-artifact events short-circuit
		// above, the fully-suppressed case already returned, and an unchanged filter
		// (before == len) leaves the title untouched.
		if len(event.Failures) != before {
			event.Title = titleForEvent(*event)
		}
	}
	return false
}

// reviewRejectedFrontmatterReason tags the only artifact-derived failure the
// analyzer intentionally surfaces: WP `review_status: has_feedback` frontmatter
// detected structurally in parseFile. Text-pattern review_rejected matches in
// artifact prose/output are suppressed like every other artifact-derived failure.
const reviewRejectedFrontmatterReason = "work package frontmatter review_status is has_feedback"

// retainWhitelistedArtifactFailures returns only structurally allowed artifact
// failures, preserving source order.
func retainWhitelistedArtifactFailures(kind string, failures []FailureFingerprint) []FailureFingerprint {
	kept := failures[:0]
	for _, f := range failures {
		if artifactFailureAllowed(kind, f) {
			kept = append(kept, f)
		}
	}
	return kept
}

func artifactFailureAllowed(kind string, failure FailureFingerprint) bool {
	return kind == "work_package" &&
		failure.ID == "review_rejected" &&
		failure.Reason == reviewRejectedFrontmatterReason
}

func addWorkPackageFrontmatterFailures(event *TimelineEvent, line string) {
	key, value, ok := frontmatterKeyValue(line)
	if !ok || !strings.EqualFold(key, "review_status") || !strings.EqualFold(value, "has_feedback") {
		return
	}
	failure, ok := fingerprintForRuleID("review_rejected", reviewRejectedFrontmatterReason)
	if !ok || failureListContains(event.Failures, failure.ID) {
		return
	}
	event.Failures = append(event.Failures, failure)
	event.Kind = "failure"
	event.Title = titleForEvent(*event)
}

func frontmatterKeyValue(line string) (key, value string, ok bool) {
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	value = strings.Trim(strings.TrimSpace(line[idx+1:]), `"'`)
	return key, value, key != ""
}

func failureListContains(failures []FailureFingerprint, id string) bool {
	for _, failure := range failures {
		if failure.ID == id {
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

	// Classification ordering (design issue-4 §5), made explicit here:
	//   (1) the §3a code-edit/file-read exclusion + §3c channel extraction happen
	//       in channelStringsForEvent (once per event, not per rule), producing the
	//       two cached strings;
	//   (2) structural obj-field rules run first inside classifyFailuresWithChannels;
	//   (3) per-pattern text rules then match each cached string for the pattern's
	//       scope (output-scoped → outputCh, diagnostic-scoped → diagnosticCh);
	//   (4) the generic_error fallback runs last, over outputCh only.
	// The single skipArtifactMessage suppression gate (parseFile) is applied AFTER
	// this returns, before findings aggregation — see channelStringsForEvent and the
	// parseFile gate comment.
	outCh, diagCh := channelStringsForEvent(path, text, obj)
	// The source kind (from the path, §3d vocabulary) gates the structural
	// review_rejected detector to spec-kitty live-event streams only.
	failures := classifyFailuresWithChannels(outCh, diagCh, classifyPathKind(path), obj, cli)
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
		outputCh:       outCh,
		diagnosticCh:   diagCh,
	}
}

// channelStringsForEvent computes the (output, diagnostic) channel strings for one
// event, once per event rather than once per failure rule (design issue-4 §3c/§3d,
// §5; NFR-002 — the cost is the channel extraction, not O(rules × object size)).
//
//   - obj != nil: derive both channels from the typed §3c extraction (channels.go,
//     WP01), which already applies the §3a code-edit/file-read exclusion. A code edit
//     or file read therefore contributes to NEITHER string.
//   - obj == nil: the line is raw, non-JSON text with no harness structure to route,
//     so resolve it by SOURCE KIND (§3d), not by content.
func channelStringsForEvent(path, text string, obj map[string]any) (outCh, diagCh string) {
	if obj != nil {
		// §3c typed extraction (WP01) — already applies the §3a code-edit/file-read
		// exclusion, so a code edit or file read reaches NEITHER channel. Call the
		// extraction ONCE and derive both cached strings from the single result: a
		// repeated outputText+diagnosticText pair would walk the object twice and emit
		// the channels.go "unmapped event shape" stderr log twice for one event. The
		// two strings are byte-identical to the old two-call results: outputText joins
		// ct.output; diagnosticText joins ct.output followed by ct.narrative.
		return channelTextPair(obj)
	}

	// §3d plain-text model: a raw, non-JSON line has no harness structure to route,
	// so resolve it by SOURCE KIND (classifyPathKind), not by content.
	switch kind := classifyPathKind(path); {
	case kind == "text":
		// Generic standalone .txt/.md/.yaml files outside kitty-specs are explicitly
		// unsupported for now. There is no source kind that proves the bytes are
		// command output, and silently treating them as output would reopen the
		// false-positive class this mission closes. They contribute no channel text
		// and are not classified — documented here, not silently mishandled.
		return "", ""

	case kind == "command_log":
		return text, text

	case isArtifactKind(kind):
		// Artifact/spec prose (spec/plan/research/WP markdown, mission meta/status
		// snapshots) is diagnostic-only. Output-scoped text rules cannot fire on it;
		// distinctive diagnostic rules may classify pre-gate, then the single artifact
		// suppression gate drops non-whitelisted artifact failures before aggregation.
		return "", text

	default:
		// Transcript-derived stray non-JSON line (a rare line in a .jsonl event log
		// that did not parse as an object): best-effort OUTPUT-ELIGIBLE — treat the
		// raw bytes as both output and narrative (Contract D, second row).
		return text, text
	}
}

// isArtifactKind reports whether a source kind is a non-transcript artifact/spec
// kind whose plain text is prose, not command output (design issue-4 §3d). This is
// the SAME set the skipArtifactMessage suppression gate keys on; both call this
// helper so the artifact-kind definition lives in one place and cannot drift.
func isArtifactKind(kind string) bool {
	switch kind {
	case "work_package", "mission_artifact", "mission_meta", "mission_status_snapshot":
		return true
	default:
		return false
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
