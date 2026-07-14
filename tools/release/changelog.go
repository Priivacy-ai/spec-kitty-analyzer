package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// headingRe matches a candidate Keep-a-Changelog H2 heading:
//
//	## [<content>]            or   ## [<content>] - <date>
//
// Bottom link-reference lines ("[x.y.z]: https://…") do NOT match — they lack the
// leading "## ". <content> is classified downstream as Unreleased, a valid version,
// or (a hard error) neither.
var headingRe = regexp.MustCompile(`^##\s+\[([^\]]+)\]\s*(?:-\s*(.*))?$`)
var h2HeadingCandidateRe = regexp.MustCompile(`^##\s+`)

// Section is one changelog section: either the Unreleased sentinel or a released
// version, plus the body lines up to the next heading.
type Section struct {
	IsUnreleased bool
	Version      Version // valid only when !IsUnreleased
	Date         string
	Body         []string
}

// ParseChangelog parses the changelog text into ordered sections (top to bottom).
// A "## [...]" heading whose bracket content is neither "Unreleased" nor a valid
// version is a hard error (never silently skipped) so a typo'd heading cannot hide
// the real top section.
func ParseChangelog(text string) ([]Section, error) {
	lines := strings.Split(text, "\n")
	var sections []Section
	cur := -1
	for _, line := range lines {
		if m := headingRe.FindStringSubmatch(line); m != nil {
			content := strings.TrimSpace(m[1])
			sec := Section{Date: strings.TrimSpace(m[2])}
			if strings.EqualFold(content, "Unreleased") {
				if sec.Date != "" {
					return nil, fmt.Errorf("malformed changelog heading %q: Unreleased must not have a date", strings.TrimSpace(line))
				}
				sec.IsUnreleased = true
			} else {
				v, err := ParseVersion(content)
				if err != nil {
					return nil, fmt.Errorf("malformed changelog heading %q: bracket content is neither \"Unreleased\" nor a valid version", strings.TrimSpace(line))
				}
				if !validDate(sec.Date) {
					return nil, fmt.Errorf("malformed changelog heading %q: released versions require a YYYY-MM-DD date", strings.TrimSpace(line))
				}
				sec.Version = v
			}
			sections = append(sections, sec)
			cur = len(sections) - 1
			continue
		}
		if h2HeadingCandidateRe.MatchString(line) {
			return nil, fmt.Errorf("malformed changelog heading %q: expected \"## [Unreleased]\" or \"## [X.Y.Z] - YYYY-MM-DD\"", strings.TrimSpace(line))
		}
		if cur >= 0 {
			sections[cur].Body = append(sections[cur].Body, line)
		}
	}
	return sections, nil
}

// trimBlank drops leading and trailing blank lines.
func trimBlank(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// IsPopulated reports whether the section has at least one non-blank body line.
func (s Section) IsPopulated() bool {
	for _, line := range trimBlank(s.Body) {
		t := strings.TrimSpace(line)
		if t == "" || isLinkReference(t) {
			continue
		}
		return true
	}
	return false
}

func isLinkReference(line string) bool {
	return strings.HasPrefix(line, "[") && strings.Contains(line, "]:")
}

func validDate(s string) bool {
	t, err := time.Parse("2006-01-02", s)
	return err == nil && t.Format("2006-01-02") == s
}

// TopReleasedVersion returns the first (topmost) released section's version.
func TopReleasedVersion(sections []Section) (Version, bool) {
	for _, s := range sections {
		if !s.IsUnreleased {
			return s.Version, true
		}
	}
	return Version{}, false
}

// ExtractSection returns the trimmed body of the section for versionStr, or a safe
// default message when no populated matching section exists. It never calls git.
func ExtractSection(sections []Section, versionStr string) string {
	want, err := ParseVersion(versionStr)
	if err == nil {
		for _, s := range sections {
			if s.IsUnreleased || s.Version.Compare(want) != 0 {
				continue
			}
			body := trimBlank(s.Body)
			if len(body) > 0 {
				return strings.Join(body, "\n")
			}
		}
	}
	return fmt.Sprintf("Release %s\n\nNo changelog entry found for this version.", versionStr)
}
