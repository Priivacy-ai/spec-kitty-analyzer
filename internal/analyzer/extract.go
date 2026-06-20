package analyzer

import (
	"bufio"
	"bytes"
	"os"
	"sort"
)

// ExtractMissionsFromFile performs the lightweight pass used by the harness-log
// cache. It intentionally avoids building a full timeline report.
func ExtractMissionsFromFile(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxInputFileBytes {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	addMissionFromPath(path, seen)

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		if obj, ok := decodeJSONObject(raw); ok {
			scope := scopeFromJSON(obj)
			addMission(scope.MissionSlug, seen)
			addMissionsFromText(path, flattenJSON(obj), seen)
			continue
		}
		addMissionsFromText(path, string(raw), seen)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return sortedMissionKeys(seen), nil
}

func addMissionFromPath(path string, seen map[string]bool) {
	if mission := missionSlugFromPath(path); mission != "" {
		addMission(mission, seen)
	}
}

func addMissionsFromText(path, text string, seen map[string]bool) {
	if m := missionPathRE.FindAllStringSubmatch(text, -1); len(m) > 0 {
		for _, match := range m {
			if len(match) > 1 {
				addMission(match[1], seen)
			}
		}
	}
	cli := detectCLIInvocations(text)
	for _, inv := range cli {
		addMission(inv.Mission, seen)
	}
	scope := scopeFromPathAndText(path, text, cli, detectSlashCommands(text))
	addMission(scope.MissionSlug, seen)
}

func addMission(slug string, seen map[string]bool) {
	slug = normalizeMissionHandle(slug)
	if slug != "" {
		seen[slug] = true
	}
}

func sortedMissionKeys(seen map[string]bool) []string {
	out := make([]string, 0, len(seen))
	for slug := range seen {
		out = append(out, slug)
	}
	sort.Strings(out)
	return out
}
