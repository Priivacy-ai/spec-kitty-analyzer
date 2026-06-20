package analyzer

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type buildState struct {
	mission map[string]*MissionSummary
	opMap   map[string]*OpSummary
}

func newBuildState() *buildState {
	return &buildState{
		mission: map[string]*MissionSummary{},
		opMap:   map[string]*OpSummary{},
	}
}

func (s *buildState) missionFor(slug string) *MissionSummary {
	if slug == "" {
		slug = "unknown"
	}
	if s.mission[slug] == nil {
		s.mission[slug] = &MissionSummary{Slug: slug}
	}
	return s.mission[slug]
}

func (s *buildState) opFor(id string) *OpSummary {
	if id == "" {
		id = "unknown"
	}
	if s.opMap[id] == nil {
		s.opMap[id] = &OpSummary{InvocationID: id, Status: "open"}
	}
	return s.opMap[id]
}

func (s *buildState) readMissionMeta(path string, data []byte) {
	var obj map[string]any
	if json.Unmarshal(bytes.TrimSpace(data), &obj) != nil {
		return
	}
	slug := firstJSONStringByKey(obj, "slug")
	slug = normalizeMissionHandle(slug)
	if slug == "" {
		slug = missionSlugFromPath(path)
	}
	m := s.missionFor(slug)
	m.MissionType = firstJSONStringByKey(obj, "mission", "mission_type")
	m.TargetBranch = firstJSONStringByKey(obj, "target_branch", "merge_target_branch")
	m.Files = appendUnique(m.Files, path)
}

func (s *buildState) readWorkPackage(path string, data []byte) {
	slug := missionSlugFromPath(path)
	if slug == "" {
		return
	}
	wp := WorkPackageState{ID: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), SourcePath: path}
	if id := wpRE.FindString(wp.ID); id != "" {
		wp.ID = id
	}
	frontmatter := parseFrontmatter(string(data))
	wp.Lane = frontmatter["lane"]
	if wp.Lane == "" {
		wp.Lane = frontmatter["status"]
	}
	wp.ReviewStatus = frontmatter["review_status"]
	wp.Dependencies = splitList(frontmatter["dependencies"])
	m := s.missionFor(slug)
	m.Files = appendUnique(m.Files, path)
	replaced := false
	for i := range m.WorkPackages {
		if m.WorkPackages[i].ID == wp.ID {
			m.WorkPackages[i] = wp
			replaced = true
			break
		}
	}
	if !replaced {
		m.WorkPackages = append(m.WorkPackages, wp)
	}
}

func (s *buildState) absorbTimeline(events []TimelineEvent) {
	for _, event := range events {
		if event.Scope.MissionSlug != "" {
			m := s.missionFor(event.Scope.MissionSlug)
			m.Files = appendUnique(m.Files, event.SourcePath)
			m.EventCount++
			for _, sc := range event.SlashCommands {
				m.SlashCommands = appendUnique(m.SlashCommands, sc.Name)
			}
			for _, inv := range event.CLIInvocations {
				m.CLIInvocations = appendUnique(m.CLIInvocations, inv.Raw)
			}
			for _, skill := range event.Skills {
				m.Skills = appendUnique(m.Skills, skill.Name)
			}
			for _, failure := range event.Failures {
				m.FailureModes = appendUnique(m.FailureModes, failure.ID)
				m.FailureCount++
			}
		}
		if event.Scope.InvocationID != "" {
			op := s.opFor(event.Scope.InvocationID)
			op.Files = appendUnique(op.Files, event.SourcePath)
			op.EventCount++
			for _, failure := range event.Failures {
				op.FailureModes = appendUnique(op.FailureModes, failure.ID)
			}
			s.absorbOpEvent(op, event)
		}
	}
}

func (s *buildState) absorbOpEvent(op *OpSummary, event TimelineEvent) {
	text := strings.ToLower(event.TextPreview)
	for _, profile := range event.AgentProfiles {
		if profile.Profile != "" && profile.Profile != "unknown" {
			op.ProfileID = profile.Profile
		}
	}
	if event.Scope.Action != "" {
		op.Action = event.Scope.Action
	}
	if event.Timestamp != nil {
		if strings.Contains(text, "started") && op.StartedAt == nil {
			op.StartedAt = event.Timestamp
		}
		if strings.Contains(text, "completed") || strings.Contains(text, "done") || strings.Contains(text, "failed") || strings.Contains(text, "abandoned") {
			op.CompletedAt = event.Timestamp
		}
	}
	if strings.Contains(text, "completed") || strings.Contains(text, "outcome done") || strings.Contains(text, `"outcome":"done"`) {
		op.Status = "completed"
		op.Outcome = "done"
	}
	if strings.Contains(text, "outcome failed") || strings.Contains(text, `"outcome":"failed"`) {
		op.Status = "completed"
		op.Outcome = "failed"
	}
	if strings.Contains(text, "outcome abandoned") || strings.Contains(text, `"outcome":"abandoned"`) {
		op.Status = "completed"
		op.Outcome = "abandoned"
	}
}

func (s *buildState) missions() []MissionSummary {
	out := make([]MissionSummary, 0, len(s.mission))
	for _, m := range s.mission {
		sort.Strings(m.Files)
		sort.Strings(m.SlashCommands)
		sort.Strings(m.CLIInvocations)
		sort.Strings(m.Skills)
		sort.Strings(m.FailureModes)
		sort.Slice(m.WorkPackages, func(i, j int) bool { return m.WorkPackages[i].ID < m.WorkPackages[j].ID })
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

func (s *buildState) opSummaries() []OpSummary {
	out := make([]OpSummary, 0, len(s.opMap))
	for _, op := range s.opMap {
		sort.Strings(op.Files)
		sort.Strings(op.FailureModes)
		out = append(out, *op)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].InvocationID < out[j].InvocationID })
	return out
}

func missionSlugFromPath(path string) string {
	slash := filepath.ToSlash(path)
	if m := missionPathRE.FindStringSubmatch(slash); len(m) > 1 {
		return m[1]
	}
	return ""
}

func parseFrontmatter(text string) map[string]string {
	out := map[string]string{}
	if !strings.HasPrefix(text, "---") {
		return out
	}
	lines := strings.Split(text, "\n")
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "---" {
			break
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.Trim(strings.TrimSpace(line[idx+1:]), `"'`)
			out[key] = val
		}
	}
	return out
}

func splitList(raw string) []string {
	raw = strings.Trim(raw, "[] ")
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' })
	var out []string
	for _, p := range parts {
		p = strings.Trim(p, `"'`)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func appendUnique(items []string, value string) []string {
	if value == "" {
		return items
	}
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
