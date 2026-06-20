package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/priivacy-ai/spec-kitty-analyzer/internal/analyzer"
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
	jsonOnly := fs.Bool("json-only", false, "write only JSON")
	if err := fs.Parse(reorderAnalyzeArgs(args)); err != nil {
		return err
	}
	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	report, err := analyzer.Analyze(paths)
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

func derive(jsonPath, explicit, ext string) string {
	if explicit != "" {
		return explicit
	}
	base := strings.TrimSuffix(jsonPath, filepath.Ext(jsonPath))
	return base + ext
}

func reorderAnalyzeArgs(args []string) []string {
	valueFlags := map[string]bool{"--out": true, "--md": true, "--html": true, "--pdf": true}
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

func usage() {
	fmt.Println(`Usage: spec-kitty-analyzer COMMAND [ARGS]...

Commands:
  analyze [paths...]  Analyze Spec Kitty missions, Ops, and agent logs.
  version             Print version.

Analyze examples:
  spec-kitty-analyzer analyze kitty-specs/ kitty-ops/ ~/.codex/sessions --out report.json
  spec-kitty-analyzer analyze /path/to/mission-or-log --json-only`)
}
