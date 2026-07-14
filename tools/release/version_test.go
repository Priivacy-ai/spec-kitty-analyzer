package main

import "testing"

func TestParseVersion(t *testing.T) {
	valid := map[string]string{ // input -> canonical
		"0.3.0":    "0.3.0",
		"1.2.3":    "1.2.3",
		"0.4.0a1":  "0.4.0a1",
		"0.4.0b2":  "0.4.0b2",
		"0.4.0rc1": "0.4.0rc1",
		"10.20.30": "10.20.30",
	}
	for in, want := range valid {
		v, err := ParseVersion(in)
		if err != nil {
			t.Errorf("ParseVersion(%q) unexpected error: %v", in, err)
			continue
		}
		if got := v.Canonical(); got != want {
			t.Errorf("ParseVersion(%q).Canonical() = %q, want %q", in, got, want)
		}
	}

	invalid := []string{
		"0.4.0-rc.1", // dotted prerelease is rejected
		"0.4.0-rc1",
		"01.2.3",    // leading zero in major
		"1.02.3",    // leading zero in minor
		"1.2.03",    // leading zero in patch
		"0.4.0rc0",  // prerelease number must be positive
		"0.4.0rc01", // leading zero in prerelease number
		"0.3",       // too few components
		"v0.3.0",    // leading v is a tag, not a version
		"0.3.0.1",   // too many components
		"1.2.x",
		"",
		"Unreleased",
	}
	for _, in := range invalid {
		if _, err := ParseVersion(in); err == nil {
			t.Errorf("ParseVersion(%q) expected error, got nil", in)
		}
	}
}

func TestParseTag(t *testing.T) {
	v, err := ParseTag("v0.3.0")
	if err != nil || v.Canonical() != "0.3.0" {
		t.Fatalf("ParseTag(v0.3.0) = %v, %v", v.Canonical(), err)
	}
	if _, err := ParseTag("0.3.0"); err == nil {
		t.Error("ParseTag(0.3.0) expected error (missing v)")
	}
	if _, err := ParseTag("v01.2.3"); err == nil {
		t.Error("ParseTag(v01.2.3) expected error (non-canonical version)")
	}
}

func TestVersionCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.3.0", "0.2.0", 1},
		{"0.2.0", "0.3.0", -1},
		{"0.3.0", "0.3.0", 0},
		{"0.4.0rc1", "0.4.0", -1}, // rc precedes stable
		{"0.4.0", "0.4.0rc1", 1},
		{"0.4.0a1", "0.4.0b1", -1}, // alpha < beta
		{"0.4.0b1", "0.4.0rc1", -1},
		{"0.4.0rc1", "0.4.0rc2", -1},
		{"1.0.0", "0.9.9", 1},
	}
	for _, c := range cases {
		a, _ := ParseVersion(c.a)
		b, _ := ParseVersion(c.b)
		if got := a.Compare(b); got != c.want {
			t.Errorf("Compare(%s,%s) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
