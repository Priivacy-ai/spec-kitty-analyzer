package main

import "testing"

func tagsFrom(t *testing.T, output, exclude string) []ReleaseTag {
	t.Helper()
	return parseReleaseTags(output, exclude)
}

func TestParseReleaseTagsSortAndExclude(t *testing.T) {
	out := "v0.1.0\nv0.2.0\nv0.1.1\nnot-a-tag\nv0.3.0\n"
	tags := tagsFrom(t, out, "")
	if len(tags) != 4 {
		t.Fatalf("got %d tags, want 4 (non-tag filtered)", len(tags))
	}
	if tags[0].Raw != "v0.3.0" {
		t.Errorf("latest = %s, want v0.3.0 (descending sort)", tags[0].Raw)
	}
	// Exclude the tag under release.
	ex := tagsFrom(t, out, "v0.3.0")
	if lt, ok := latestTag(ex); !ok || lt.Raw != "v0.2.0" {
		t.Errorf("with v0.3.0 excluded, latest = %v, want v0.2.0", lt.Raw)
	}
}

func mustV(t *testing.T, s string) Version {
	t.Helper()
	v, err := ParseVersion(s)
	if err != nil {
		t.Fatalf("ParseVersion(%q): %v", s, err)
	}
	return v
}

func TestValidateBranchStateAware(t *testing.T) {
	tags := tagsFrom(t, "v0.1.0\nv0.2.0\n", "")

	// release-prep: 0.3.0 > v0.2.0 -> OK
	if issues := validateBranch(mustV(t, "0.3.0"), tags); len(issues) != 0 {
		t.Errorf("release-prep should pass, got %v", issues)
	}
	if s := branchStateLabel(mustV(t, "0.3.0"), tags); s != "release-prep" {
		t.Errorf("state = %q, want release-prep", s)
	}

	// inter-release: 0.2.0 == v0.2.0 -> OK (the Codex-R4 regression trap)
	if issues := validateBranch(mustV(t, "0.2.0"), tags); len(issues) != 0 {
		t.Errorf("inter-release (V==T) MUST pass, got %v", issues)
	}
	if s := branchStateLabel(mustV(t, "0.2.0"), tags); s != "inter-release" {
		t.Errorf("state = %q, want inter-release", s)
	}

	// behind: 0.1.5 < v0.2.0 -> error
	if issues := validateBranch(mustV(t, "0.1.5"), tags); len(issues) == 0 {
		t.Error("changelog behind tags (V<T) should error")
	}

	// no tags yet -> OK, release-prep
	if issues := validateBranch(mustV(t, "0.1.0"), nil); len(issues) != 0 {
		t.Errorf("no tags should pass, got %v", issues)
	}
}

func TestValidateTag(t *testing.T) {
	// Tag mode: the tag under release is excluded from the set upstream, so pass
	// the prior tags here.
	prior := tagsFrom(t, "v0.1.0\nv0.2.0\n", "")

	// parity + advance: releasing v0.3.0 with top released 0.3.0 -> OK
	if issues := validateTag(mustV(t, "0.3.0"), prior, "v0.3.0"); len(issues) != 0 {
		t.Errorf("valid tag release should pass, got %v", issues)
	}

	// parity mismatch: tag v0.3.0 but changelog top is 0.2.5
	if issues := validateTag(mustV(t, "0.2.5"), prior, "v0.3.0"); len(issues) == 0 {
		t.Error("tag/changelog mismatch should error")
	}

	// non-advancing: releasing v0.2.0 again (top 0.2.0), prior still has v0.2.0
	if issues := validateTag(mustV(t, "0.2.0"), prior, "v0.2.0"); len(issues) == 0 {
		t.Error("non-advancing tag should error")
	}

	// bad tag
	if issues := validateTag(mustV(t, "0.3.0"), prior, "0.3.0"); len(issues) == 0 {
		t.Error("tag without leading v should error")
	}
}
