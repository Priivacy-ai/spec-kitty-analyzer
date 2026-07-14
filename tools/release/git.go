package main

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// ReleaseTag is a git tag that parsed as a valid "vX.Y.Z" release version.
type ReleaseTag struct {
	Raw     string
	Version Version
}

// parseReleaseTags turns the output of `git tag --list 'v*.*.*'` into release tags,
// excluding `exclude` (the tag currently being released, in tag mode), and sorts
// them descending by version. Split out from discoverReleaseTags so it is testable
// without a git repository.
func parseReleaseTags(gitOutput, exclude string) []ReleaseTag {
	var tags []ReleaseTag
	for _, line := range strings.Split(gitOutput, "\n") {
		tag := strings.TrimSpace(line)
		if tag == "" || tag == exclude {
			continue
		}
		v, err := ParseTag(tag)
		if err != nil {
			continue
		}
		tags = append(tags, ReleaseTag{Raw: tag, Version: v})
	}
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Version.Compare(tags[j].Version) > 0
	})
	return tags
}

// discoverReleaseTags reads the repository's release tags, excluding `exclude`.
func discoverReleaseTags(exclude string) ([]ReleaseTag, error) {
	out, err := exec.Command("git", "tag", "--list", "v*.*.*").Output()
	if err != nil {
		return nil, fmt.Errorf("could not list git tags (not a git work tree?): %w", err)
	}
	return parseReleaseTags(string(out), exclude), nil
}

// latestTag returns the highest release tag, or false when there are none.
func latestTag(tags []ReleaseTag) (ReleaseTag, bool) {
	if len(tags) == 0 {
		return ReleaseTag{}, false
	}
	return tags[0], true
}
