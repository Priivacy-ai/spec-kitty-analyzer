package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/priivacy-ai/spec-kitty-analyzer/internal/analyzer"
	"github.com/priivacy-ai/spec-kitty-analyzer/internal/discovery"
	"github.com/priivacy-ai/spec-kitty-analyzer/internal/reports"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return errors.New("missing command")
	}
	switch args[0] {
	case "analyze":
		return runAnalyze(args[1:])
	case "version", "--version", "-v":
		fmt.Println("spec-kitty-analyzer " + analyzer.Version)
		return nil
	case "help", "--help", "-h":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runAnalyze(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	out := fs.String("out", "spec-kitty-analyzer-report.json", "path to write structured JSON")
	md := fs.String("md", "", "path to write markdown report (default: derived from --out)")
	html := fs.String("html", "", "path to write HTML report (default: derived from --out)")
	pdf := fs.String("pdf", "", "path to write PDF report (default: derived from --out)")
	mission := fs.String("mission", "", "mission slug to resolve from cached harness logs")
	cachePath := fs.String("cache", "", "cache path (default: ~/.spec-kitty-analyzer/cache.json)")
	cacheBust := fs.Bool("cache-bust", false, "rescan every harness log instead of reusing unchanged cache entries")
	recentLimit := fs.Int("recent", 10, "number of recent harness logs to show when no mission/path is provided")
	jsonOnly := fs.Bool("json-only", false, "write only JSON")
	var logRoots multiFlag
	fs.Var(&logRoots, "log-root", "additional harness log root to scan (repeatable)")
	if err := fs.Parse(reorderAnalyzeArgs(args)); err != nil {
		return err
	}
	paths := fs.Args()

	reportPaths := paths
	reportMission := strings.TrimSpace(*mission)
	if reportMission == "" && len(paths) == 1 && !pathExists(paths[0]) {
		reportMission = paths[0]
		reportPaths = nil
	}

	var report analyzer.Report
	var err error
	switch {
	case reportMission != "":
		cache, stats, err := refreshHarnessCache(*cachePath, logRoots, *cacheBust)
		if err != nil {
			return err
		}
		fmt.Println(discovery.StatsLine(stats))
		reportPaths = discovery.FilesForMission(cache, reportMission)
		if len(reportPaths) == 0 {
			return fmt.Errorf("no cached harness logs found for mission %q", reportMission)
		}
		report, err = analyzer.AnalyzeMission(reportPaths, reportMission)
	case len(reportPaths) == 0:
		cache, stats, err := refreshHarnessCache(*cachePath, logRoots, *cacheBust)
		if err != nil {
			return err
		}
		fmt.Println(discovery.StatsLine(stats))
		selected, err := promptRecentLog(os.Stdin, os.Stdout, cache, *recentLimit)
		if err != nil {
			return err
		}
		reportPaths = []string{selected.Path}
		report, err = analyzer.Analyze(reportPaths)
	default:
		report, err = analyzer.Analyze(reportPaths)
	}
	if err != nil {
		return err
	}
	mdPath, htmlPath, pdfPath := "", "", ""
	if !*jsonOnly {
		mdPath = derive(*out, *md, ".md")
		htmlPath = derive(*out, *html, ".html")
		pdfPath = derive(*out, *pdf, ".pdf")
	}
	if err := reports.WriteAll(report, *out, mdPath, htmlPath, pdfPath); err != nil {
		return err
	}
	fmt.Printf("Wrote JSON: %s\n", *out)
	if mdPath != "" {
		fmt.Printf("Wrote Markdown: %s\n", mdPath)
	}
	if htmlPath != "" {
		fmt.Printf("Wrote HTML: %s\n", htmlPath)
	}
	if pdfPath != "" {
		fmt.Printf("Wrote PDF: %s\n", pdfPath)
	}
	fmt.Printf("Timeline events: %d, missions: %d, ops: %d, failure modes: %d\n", report.Summary.TimelineEvents, report.Summary.Missions, report.Summary.Ops, report.Summary.FailureModes)
	return nil
}

func refreshHarnessCache(cachePath string, roots multiFlag, force bool) (*discovery.MissionCache, discovery.ScanStats, error) {
	var harnessRoots []discovery.HarnessRoot
	if len(roots) > 0 {
		harnessRoots = discovery.CustomHarnessRoots(roots)
	}
	return discovery.RefreshCache(discovery.RefreshOptions{
		CachePath: cachePath,
		Roots:     harnessRoots,
		Force:     force,
	})
}

func promptRecentLog(in io.Reader, out io.Writer, cache *discovery.MissionCache, limit int) (discovery.CachedFile, error) {
	candidates := discovery.RecentLogs(cache, limit, false)
	if len(candidates) == 0 {
		return discovery.CachedFile{}, errors.New("no harness logs found")
	}
	fmt.Fprintln(out, "Recent harness logs:")
	for i, file := range candidates {
		missions := "no missions detected"
		if len(file.MissionRefs) > 0 {
			missions = formatMissionRefs(file.MissionRefs)
		} else if len(file.Missions) > 0 {
			missions = strings.Join(file.Missions, ", ")
		}
		fmt.Fprintf(out, "%2d. %s  %-8s  %s\n", i+1, file.ModTime.Format("2006-01-02 15:04:05"), file.Harness, missions)
		fmt.Fprintf(out, "    %s\n", file.Path)
	}
	fmt.Fprintf(out, "Select log [1-%d]: ", len(candidates))
	var raw string
	if _, err := fmt.Fscan(in, &raw); err != nil {
		return discovery.CachedFile{}, err
	}
	idx, err := strconv.Atoi(raw)
	if err != nil || idx < 1 || idx > len(candidates) {
		return discovery.CachedFile{}, fmt.Errorf("invalid selection %q", raw)
	}
	return candidates[idx-1], nil
}

func formatMissionRefs(refs []discovery.MissionRef) string {
	parts := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.ShortTitle == "" || ref.ShortTitle == ref.Slug {
			parts = append(parts, ref.Slug)
			continue
		}
		parts = append(parts, ref.Slug+" ("+ref.ShortTitle+")")
	}
	return strings.Join(parts, ", ")
}

func derive(jsonPath, explicit, ext string) string {
	if explicit != "" {
		return explicit
	}
	base := strings.TrimSuffix(jsonPath, filepath.Ext(jsonPath))
	return base + ext
}

func reorderAnalyzeArgs(args []string) []string {
	valueFlags := map[string]bool{"--out": true, "--md": true, "--html": true, "--pdf": true, "--mission": true, "--cache": true, "--recent": true, "--log-root": true}
	var flagsPart []string
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagsPart = append(flagsPart, arg)
			if strings.Contains(arg, "=") {
				continue
			}
			if valueFlags[arg] && i+1 < len(args) {
				i++
				flagsPart = append(flagsPart, args[i])
			}
			continue
		}
		positional = append(positional, arg)
	}
	return append(flagsPart, positional...)
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type multiFlag []string

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func usage() {
	fmt.Println(`Usage: spec-kitty-analyzer COMMAND [ARGS]...

Commands:
  analyze [mission-slug]  Resolve mission logs across harnesses and report.
  analyze [paths...]      Analyze explicit files/directories directly.
  version             Print version.

Analyze examples:
  spec-kitty-analyzer analyze task-workflow-bug-fixes-01KV69BZ --out report.json
  spec-kitty-analyzer analyze task-workflow-bug-fixes-01KV69BZ --cache-bust --out report.json
  spec-kitty-analyzer analyze --mission task-workflow-bug-fixes-01KV69BZ --out report.json
  spec-kitty-analyzer analyze
  spec-kitty-analyzer analyze /path/to/mission-or-log --json-only`)
}
