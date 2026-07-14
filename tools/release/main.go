// Command release is maintainer-only tooling for the curated CHANGELOG release
// pipeline. It is NOT part of the shipped spec-kitty-analyzer binary.
//
//	go run ./tools/release extract <version>
//	go run ./tools/release validate --mode branch
//	go run ./tools/release validate --mode tag --tag vX.Y.Z
//
// It reads CHANGELOG.md from the working directory and (for validate) the local
// git tags. Standard library only.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const changelogPath = "CHANGELOG.md"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "extract":
		os.Exit(cmdExtract(os.Args[2:]))
	case "validate":
		os.Exit(cmdValidate(os.Args[2:]))
	case "-h", "--help", "help":
		usage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage:
  release extract <version>                  print the CHANGELOG section for <version>
  release validate --mode branch             check release readiness for a PR/branch
  release validate --mode tag --tag vX.Y.Z   check tag ↔ changelog ↔ progression parity
`)
}

// cmdExtract prints the changelog section for a version to stdout.
func cmdExtract(args []string) int {
	if len(args) != 1 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(os.Stderr, "usage: release extract <version>")
		return 2
	}
	text, err := os.ReadFile(changelogPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read %s: %v\n", changelogPath, err)
		return 1
	}
	sections, err := ParseChangelog(string(text))
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot parse %s: %v\n", changelogPath, err)
		return 1
	}
	fmt.Println(ExtractSection(sections, args[0]))
	return 0
}

// cmdValidate runs the release-readiness checks.
func cmdValidate(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	mode := fs.String("mode", "", "validation mode: branch | tag")
	tag := fs.String("tag", "", "tag to validate in tag mode (default $GITHUB_REF_NAME)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *mode != "branch" && *mode != "tag" {
		fmt.Fprintln(os.Stderr, "--mode must be 'branch' or 'tag'")
		return 2
	}
	tagVal := *tag
	if *mode == "tag" && tagVal == "" {
		tagVal = os.Getenv("GITHUB_REF_NAME")
	}
	if *mode == "tag" && tagVal == "" {
		fmt.Fprintln(os.Stderr, "tag mode requires --tag or $GITHUB_REF_NAME")
		return 2
	}

	text, err := os.ReadFile(changelogPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read %s: %v\n", changelogPath, err)
		return 1
	}
	sections, err := ParseChangelog(string(text))
	if err != nil {
		fmt.Fprintf(os.Stderr, "- %v\n", err)
		return 1
	}

	top, ok := TopReleasedVersion(sections)
	if !ok {
		fmt.Fprintln(os.Stderr, "- no released version section found in CHANGELOG.md")
		return 1
	}
	var issues []string
	if !sectionPopulated(sections, top) {
		issues = append(issues, fmt.Sprintf("changelog section [%s] is empty (not populated)", top.Canonical()))
	}

	// In tag mode the tag under release is already pushed; exclude it from the
	// existing-tag set so the strict-progression check does not compare against self.
	exclude := ""
	if *mode == "tag" {
		exclude = tagVal
	}
	tags, err := discoverReleaseTags(exclude)
	if err != nil {
		fmt.Fprintf(os.Stderr, "- %v\n", err)
		return 1
	}

	var state string
	switch *mode {
	case "branch":
		issues = append(issues, validateBranch(top, tags)...)
		state = branchStateLabel(top, tags)
	case "tag":
		issues = append(issues, validateTag(top, tags, tagVal)...)
	}

	if len(issues) > 0 {
		for _, m := range issues {
			fmt.Fprintf(os.Stderr, "- %s\n", m)
		}
		return 1
	}
	if *mode == "branch" {
		latest := "none"
		if lt, ok := latestTag(tags); ok {
			latest = lt.Raw
		}
		fmt.Fprintf(os.Stderr, "release readiness OK: %s (mode=branch, state=%s, latest tag=%s)\n", top.Canonical(), state, latest)
	} else {
		fmt.Fprintf(os.Stderr, "release readiness OK: %s (mode=tag)\n", top.Canonical())
	}
	return 0
}

// sectionPopulated reports whether the released section for v is populated.
func sectionPopulated(sections []Section, v Version) bool {
	for _, s := range sections {
		if !s.IsUnreleased && s.Version.Compare(v) == 0 {
			return s.IsPopulated()
		}
	}
	return false
}

// validateBranch applies state-aware monotonicity for a PR/branch: a new version
// being prepared (V>T) must be strictly greater; an inter-release state (V==T) is
// fine; a changelog behind the tags (V<T) is an error.
func validateBranch(top Version, tags []ReleaseTag) []string {
	lt, ok := latestTag(tags)
	if !ok {
		return nil
	}
	if top.Compare(lt.Version) < 0 {
		return []string{fmt.Sprintf("changelog top released version %s is behind the latest tag %s", top.Canonical(), lt.Raw)}
	}
	return nil
}

// branchStateLabel classifies the branch state for the success summary.
func branchStateLabel(top Version, tags []ReleaseTag) string {
	lt, ok := latestTag(tags)
	if !ok {
		return "release-prep"
	}
	switch {
	case top.Compare(lt.Version) > 0:
		return "release-prep"
	default:
		return "inter-release"
	}
}

// validateTag checks tag↔changelog parity plus strict progression beyond the
// latest prior tag (the tag under release having been excluded upstream).
func validateTag(top Version, tags []ReleaseTag, tag string) []string {
	tv, err := ParseTag(tag)
	if err != nil {
		return []string{fmt.Sprintf("invalid --tag %q: %v", tag, err)}
	}
	var issues []string
	if tv.Compare(top) != 0 {
		issues = append(issues, fmt.Sprintf("tag %s does not match top released changelog version %s", tag, top.Canonical()))
	}
	if lt, ok := latestTag(tags); ok && top.Compare(lt.Version) <= 0 {
		issues = append(issues, fmt.Sprintf("version %s does not advance beyond latest tag %s", top.Canonical(), lt.Raw))
	}
	return issues
}
