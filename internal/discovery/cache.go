package discovery

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/priivacy-ai/spec-kitty-analyzer/internal/analyzer"
)

const cacheVersion = 1

type HarnessRoot struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type MissionCache struct {
	Version   int                    `json:"version"`
	LastRunAt time.Time              `json:"last_run_at"`
	Roots     []HarnessRoot          `json:"roots"`
	Files     map[string]CachedFile  `json:"files"`
	Missions  map[string]MissionLogs `json:"missions"`
}

type CachedFile struct {
	Path        string       `json:"path"`
	Harness     string       `json:"harness"`
	Size        int64        `json:"size"`
	ModTime     time.Time    `json:"mod_time"`
	ScannedAt   time.Time    `json:"scanned_at"`
	Missions    []string     `json:"missions"`
	MissionRefs []MissionRef `json:"mission_refs"`
	ScanError   string       `json:"scan_error,omitempty"`
}

type MissionRef struct {
	Slug       string `json:"slug"`
	ShortTitle string `json:"short_title"`
}

type MissionLogs struct {
	Slug       string    `json:"slug"`
	ShortTitle string    `json:"short_title"`
	Files      []string  `json:"files"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type ScanStats struct {
	CachePath    string
	LogFiles     int
	Scanned      int
	Reused       int
	Pruned       int
	Errored      int
	MissionCount int
}

type RefreshOptions struct {
	CachePath string
	Roots     []HarnessRoot
	Force     bool
	Now       time.Time
}

func DefaultCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".spec-kitty-analyzer", "cache.json"), nil
}

func DefaultHarnessRoots() []HarnessRoot {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	candidates := []HarnessRoot{
		{Name: "claude", Path: filepath.Join(home, ".claude", "projects")},
		{Name: "codex", Path: filepath.Join(home, ".codex", "sessions")},
		{Name: "codex", Path: filepath.Join(home, ".codex", "logs")},
		{Name: "codex", Path: filepath.Join(home, ".codex", "projects")},
		{Name: "codex", Path: filepath.Join(home, ".codex", "threads")},
		{Name: "agents", Path: filepath.Join(home, ".agents", "logs")},
		{Name: "opencode", Path: filepath.Join(home, ".local", "share", "opencode")},
	}
	return existingRoots(candidates)
}

func CustomHarnessRoots(paths []string) []HarnessRoot {
	roots := make([]HarnessRoot, 0, len(paths))
	for _, raw := range paths {
		path := expandHome(raw)
		name := classifyHarness(path)
		roots = append(roots, HarnessRoot{Name: name, Path: filepath.Clean(path)})
	}
	return existingRoots(roots)
}

func RefreshCache(opts RefreshOptions) (*MissionCache, ScanStats, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	cachePath := opts.CachePath
	if cachePath == "" {
		var err error
		cachePath, err = DefaultCachePath()
		if err != nil {
			return nil, ScanStats{}, err
		}
	}
	roots := opts.Roots
	if len(roots) == 0 {
		roots = DefaultHarnessRoots()
	}
	stats := ScanStats{CachePath: cachePath}
	if len(roots) == 0 {
		return nil, stats, errors.New("no harness log roots found; pass --log-root to scan a custom location")
	}

	old := emptyCache()
	if !opts.Force {
		loaded, err := loadCache(cachePath)
		if err == nil {
			old = loaded
		}
	}

	files, err := collectHarnessLogs(roots)
	if err != nil {
		return nil, stats, err
	}
	stats.LogFiles = len(files)

	next := emptyCache()
	next.Roots = roots
	next.LastRunAt = now
	for _, found := range files {
		oldEntry, ok := old.Files[found.Path]
		if ok && oldEntry.Size == found.Size && oldEntry.ModTime.Equal(found.ModTime) {
			oldEntry.Harness = found.Harness
			oldEntry.MissionRefs = missionRefsForSlugs(oldEntry.Missions)
			next.Files[found.Path] = oldEntry
			stats.Reused++
			continue
		}
		entry := CachedFile{
			Path:      found.Path,
			Harness:   found.Harness,
			Size:      found.Size,
			ModTime:   found.ModTime,
			ScannedAt: now,
		}
		missions, scanErr := analyzer.ExtractMissionsFromFile(found.Path)
		if scanErr != nil {
			entry.ScanError = scanErr.Error()
			stats.Errored++
		}
		entry.Missions = missions
		entry.MissionRefs = missionRefsForSlugs(missions)
		next.Files[found.Path] = entry
		stats.Scanned++
	}
	for path := range old.Files {
		if _, ok := next.Files[path]; !ok {
			stats.Pruned++
		}
	}
	rebuildMissionIndex(next)
	stats.MissionCount = len(next.Missions)
	if err := saveCache(cachePath, next); err != nil {
		return nil, stats, err
	}
	return next, stats, nil
}

func FilesForMission(cache *MissionCache, slug string) []string {
	mission, ok := cache.Missions[slug]
	if !ok {
		return nil
	}
	out := append([]string(nil), mission.Files...)
	sort.Strings(out)
	return out
}

func RecentLogs(cache *MissionCache, limit int, missionOnly bool) []CachedFile {
	if limit <= 0 {
		limit = 10
	}
	files := make([]CachedFile, 0, len(cache.Files))
	for _, file := range cache.Files {
		if missionOnly && len(file.Missions) == 0 {
			continue
		}
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].ModTime.Equal(files[j].ModTime) {
			return files[i].Path < files[j].Path
		}
		return files[i].ModTime.After(files[j].ModTime)
	})
	if len(files) > limit {
		files = files[:limit]
	}
	return files
}

func loadCache(path string) (*MissionCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cache MissionCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	if cache.Version != cacheVersion || cache.Files == nil || cache.Missions == nil {
		return nil, errors.New("cache version mismatch or invalid cache")
	}
	return &cache, nil
}

func saveCache(path string, cache *MissionCache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func emptyCache() *MissionCache {
	return &MissionCache{
		Version:  cacheVersion,
		Files:    map[string]CachedFile{},
		Missions: map[string]MissionLogs{},
	}
}

func rebuildMissionIndex(cache *MissionCache) {
	cache.Missions = map[string]MissionLogs{}
	for _, file := range cache.Files {
		for _, slug := range file.Missions {
			entry := cache.Missions[slug]
			entry.Slug = slug
			entry.ShortTitle = shortMissionTitle(slug)
			entry.Files = append(entry.Files, file.Path)
			if file.ModTime.After(entry.LastSeenAt) {
				entry.LastSeenAt = file.ModTime
			}
			cache.Missions[slug] = entry
		}
	}
	for slug, entry := range cache.Missions {
		sort.Strings(entry.Files)
		cache.Missions[slug] = entry
	}
}

func missionRefsForSlugs(slugs []string) []MissionRef {
	refs := make([]MissionRef, 0, len(slugs))
	for _, slug := range slugs {
		refs = append(refs, MissionRef{Slug: slug, ShortTitle: shortMissionTitle(slug)})
	}
	return refs
}

type foundLog struct {
	Path    string
	Harness string
	Size    int64
	ModTime time.Time
}

func collectHarnessLogs(roots []HarnessRoot) ([]foundLog, error) {
	seen := map[string]bool{}
	var files []foundLog
	for _, root := range roots {
		err := filepath.WalkDir(root.Path, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if skipHarnessDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !isHarnessLog(path) || seen[path] {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			seen[path] = true
			files = append(files, foundLog{
				Path:    path,
				Harness: root.Name,
				Size:    info.Size(),
				ModTime: info.ModTime().UTC(),
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func isHarnessLog(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jsonl", ".log", ".txt", ".ndjson":
		return true
	default:
		return false
	}
}

func skipHarnessDir(name string) bool {
	switch name {
	case ".git", ".venv", "node_modules", "vendor", "dist", "build", "__pycache__", ".pytest_cache", ".ruff_cache":
		return true
	default:
		return false
	}
}

func existingRoots(candidates []HarnessRoot) []HarnessRoot {
	var roots []HarnessRoot
	seen := map[string]bool{}
	for _, root := range candidates {
		root.Path = filepath.Clean(root.Path)
		if root.Name == "" {
			root.Name = classifyHarness(root.Path)
		}
		info, err := os.Stat(root.Path)
		if err != nil || !info.IsDir() || seen[root.Path] {
			continue
		}
		seen[root.Path] = true
		roots = append(roots, root)
	}
	return roots
}

func classifyHarness(path string) string {
	slash := filepath.ToSlash(path)
	switch {
	case strings.Contains(slash, "/.claude/"):
		return "claude"
	case strings.Contains(slash, "/.codex/"):
		return "codex"
	case strings.Contains(slash, "/.agents/"):
		return "agents"
	case strings.Contains(strings.ToLower(slash), "opencode"):
		return "opencode"
	default:
		return "custom"
	}
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func StatsLine(stats ScanStats) string {
	return fmt.Sprintf("Cache: %s | logs=%d scanned=%d reused=%d pruned=%d errors=%d missions=%d", stats.CachePath, stats.LogFiles, stats.Scanned, stats.Reused, stats.Pruned, stats.Errored, stats.MissionCount)
}

func shortMissionTitle(slug string) string {
	parts := strings.FieldsFunc(slug, func(r rune) bool {
		return r == '-' || r == '_' || r == '.'
	})
	parts = trimMissionSlugTokens(parts)
	if len(parts) == 0 {
		return slug
	}
	if len(parts) == 1 && looksLikeMissionID(parts[0]) {
		return slug
	}
	for i, part := range parts {
		parts[i] = titleWord(part)
	}
	return strings.Join(parts, " ")
}

func trimMissionSlugTokens(parts []string) []string {
	for len(parts) > 0 && isNumericToken(parts[0]) {
		parts = parts[1:]
	}
	for len(parts) > 1 && looksLikeMissionID(parts[len(parts)-1]) {
		parts = parts[:len(parts)-1]
	}
	return parts
}

func isNumericToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func looksLikeMissionID(s string) bool {
	if len(s) < 4 || !strings.HasPrefix(s, "01") {
		return false
	}
	hasDigit := false
	hasUpper := false
	for _, r := range s {
		switch {
		case unicode.IsDigit(r):
			hasDigit = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		default:
			return false
		}
	}
	return hasDigit && hasUpper
}

func titleWord(s string) string {
	if s == "" {
		return s
	}
	lower := strings.ToLower(s)
	runes := []rune(lower)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
