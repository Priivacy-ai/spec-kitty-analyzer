package main

import (
	"strings"
	"testing"
)

const sampleChangelog = `# Changelog

## [Unreleased]

## [0.3.0] - 2026-07-14

### Added
- Something new.

## [0.2.0] - 2026-07-03

### Improved
- Precision.

## [0.1.0] - 2026-06-20

### Added
- Initial release.

[Unreleased]: https://example.com/compare/v0.3.0...HEAD
[0.3.0]: https://example.com/compare/v0.2.0...v0.3.0
[0.2.0]: https://example.com/compare/v0.1.0...v0.2.0
[0.1.0]: https://example.com/releases/tag/v0.1.0
`

func mustParse(t *testing.T, text string) []Section {
	t.Helper()
	s, err := ParseChangelog(text)
	if err != nil {
		t.Fatalf("ParseChangelog error: %v", err)
	}
	return s
}

func TestTopReleasedVersion(t *testing.T) {
	sections := mustParse(t, sampleChangelog)
	v, ok := TopReleasedVersion(sections)
	if !ok || v.Canonical() != "0.3.0" {
		t.Fatalf("TopReleasedVersion = %v, %v; want 0.3.0", v.Canonical(), ok)
	}
}

func TestExtractSection(t *testing.T) {
	sections := mustParse(t, sampleChangelog)

	got := ExtractSection(sections, "0.3.0")
	if !strings.Contains(got, "Something new.") || strings.Contains(got, "## [0.2.0]") {
		t.Errorf("extract 0.3.0 wrong body:\n%s", got)
	}
	// Extraction must stop at the next heading (no bleed into 0.2.0).
	if strings.Contains(got, "Precision.") {
		t.Errorf("extract 0.3.0 bled into 0.2.0:\n%s", got)
	}

	// Missing version -> default text, no panic.
	miss := ExtractSection(sections, "9.9.9")
	if !strings.Contains(miss, "No changelog entry found") {
		t.Errorf("extract missing = %q, want default message", miss)
	}
}

func TestExtractLinkRefsNotHeadings(t *testing.T) {
	// The bottom [x.y.z]: link-reference lines must not be parsed as version
	// headings, so only 3 released sections + Unreleased exist.
	sections := mustParse(t, sampleChangelog)
	released := 0
	unreleased := 0
	for _, s := range sections {
		if s.IsUnreleased {
			unreleased++
		} else {
			released++
		}
	}
	if released != 3 || unreleased != 1 {
		t.Errorf("got released=%d unreleased=%d; want 3 and 1 (link refs must not be headings)", released, unreleased)
	}
}

func TestIsPopulated(t *testing.T) {
	empty := "# Changelog\n\n## [0.3.0] - 2026-07-14\n\n## [0.2.0] - 2026-07-03\n\n### Added\n- x\n"
	sections := mustParse(t, empty)
	// 0.3.0 has only blank lines before 0.2.0 -> not populated.
	if sections[0].IsPopulated() {
		t.Error("empty 0.3.0 section should not be populated")
	}
	if !sections[1].IsPopulated() {
		t.Error("0.2.0 section should be populated")
	}
}

func TestMalformedHeadingIsError(t *testing.T) {
	bad := []string{
		"## [0.3] - 2026-07-14\n\n- x\n",           // too few components
		"## [v0.3.0] - 2026-07-14\n\n- x\n",        // leading v
		"## [draft]\n\n- x\n",                      // arbitrary word
		"## [0.3.0] 2026-07-14\n\n- x\n",           // missing separator
		"## [0.3.0] (2026-07-14)\n\n- x\n",         // wrong date syntax
		"## [0.3.0]: https://example.com\n\n- x\n", // link-ref typo as heading
		"## [0.3.0]\n\n- x\n",                      // released heading missing date
		"## [0.3.0] - someday\n\n- x\n",            // released heading malformed date
		"## [0.3.0] - 2026-99-99\n\n- x\n",         // impossible date
		"## [Unreleased] - 2026-07-14\n\n- x\n",    // Unreleased must stay undated
		"## 0.3.0 - 2026-07-14\n\n- x\n",           // missing brackets must not hide a release
	}
	for _, text := range bad {
		if _, err := ParseChangelog(text); err == nil {
			t.Errorf("ParseChangelog(%q) expected malformed-heading error, got nil", text)
		}
	}
}

func TestUnreleasedNotReleased(t *testing.T) {
	sections := mustParse(t, "## [Unreleased]\n\n- wip\n")
	if _, ok := TopReleasedVersion(sections); ok {
		t.Error("Unreleased-only changelog must have no released version")
	}
}

func TestLinkReferencesDoNotPopulateSection(t *testing.T) {
	sections := mustParse(t, `# Changelog

## [0.1.0] - 2026-06-20

[0.1.0]: https://example.com/releases/tag/v0.1.0
`)
	if sections[0].IsPopulated() {
		t.Error("link-reference-only section should not count as populated release notes")
	}
}
