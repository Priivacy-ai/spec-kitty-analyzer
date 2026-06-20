package discovery

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRefreshCacheIncrementalAndCacheBust(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "session.jsonl")
	writeLog(t, logPath, "alpha-01KV")

	cachePath := filepath.Join(dir, "cache.json")
	root := HarnessRoot{Name: "test", Path: dir}

	cache, stats, err := RefreshCache(RefreshOptions{
		CachePath: cachePath,
		Roots:     []HarnessRoot{root},
		Now:       time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("RefreshCache initial failed: %v", err)
	}
	if stats.Scanned != 1 || stats.Reused != 0 || stats.MissionCount != 1 {
		t.Fatalf("initial stats=%#v", stats)
	}
	if got := FilesForMission(cache, "alpha-01KV"); len(got) != 1 || got[0] != logPath {
		t.Fatalf("alpha files=%#v", got)
	}
	if title := cache.Missions["alpha-01KV"].ShortTitle; title != "Alpha" {
		t.Fatalf("short title=%q want Alpha", title)
	}

	_, stats, err = RefreshCache(RefreshOptions{
		CachePath: cachePath,
		Roots:     []HarnessRoot{root},
		Now:       time.Date(2026, 6, 20, 10, 1, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("RefreshCache reuse failed: %v", err)
	}
	if stats.Scanned != 0 || stats.Reused != 1 {
		t.Fatalf("reuse stats=%#v", stats)
	}

	_, stats, err = RefreshCache(RefreshOptions{
		CachePath: cachePath,
		Roots:     []HarnessRoot{root},
		Force:     true,
		Now:       time.Date(2026, 6, 20, 10, 2, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("RefreshCache force failed: %v", err)
	}
	if stats.Scanned != 1 || stats.Reused != 0 {
		t.Fatalf("force stats=%#v", stats)
	}

	writeLog(t, logPath, "beta-01KV")
	cache, stats, err = RefreshCache(RefreshOptions{
		CachePath: cachePath,
		Roots:     []HarnessRoot{root},
		Now:       time.Date(2026, 6, 20, 10, 3, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("RefreshCache changed failed: %v", err)
	}
	if stats.Scanned != 1 || stats.Reused != 0 || stats.MissionCount != 1 {
		t.Fatalf("changed stats=%#v", stats)
	}
	if got := FilesForMission(cache, "alpha-01KV"); len(got) != 0 {
		t.Fatalf("alpha should be gone, got %#v", got)
	}
	if got := FilesForMission(cache, "beta-01KV"); len(got) != 1 || got[0] != logPath {
		t.Fatalf("beta files=%#v", got)
	}
}

func TestShortMissionTitle(t *testing.T) {
	tests := map[string]string{
		"task-workflow-bug-fixes-01KV69BZ":            "Task Workflow Bug Fixes",
		"002-lightweight-pypi-release":                "Lightweight Pypi Release",
		"01KV69BZEHXDSGGMR6J3QN0J2E":                  "01KV69BZEHXDSGGMR6J3QN0J2E",
		"cyrillic-doctrine-internal-helpers-01KVDZ6S": "Cyrillic Doctrine Internal Helpers",
	}
	for slug, want := range tests {
		if got := shortMissionTitle(slug); got != want {
			t.Fatalf("shortMissionTitle(%q)=%q want %q", slug, got, want)
		}
	}
}

func writeLog(t *testing.T, path, mission string) {
	t.Helper()
	content := `{"timestamp":"2026-06-20T10:00:00Z","message":"spec-kitty next --mission ` + mission + `"}`
	if err := os.WriteFile(path, []byte(content+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}
