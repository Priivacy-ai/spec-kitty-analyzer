package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// stage is the prerelease stage of a version. Its integer value is the ordering
// rank: alpha < beta < rc < stable.
type stage int

const (
	stageAlpha  stage = iota // "aN"
	stageBeta                // "bN"
	stageRC                  // "rcN"
	stageStable              // no prerelease suffix
)

// Version is a parsed release version. Only the compact prerelease spelling
// (X.Y.Z{a|b|rc}N) is accepted — the dotted "-rc.N" form is deliberately rejected
// to mirror the reference tool and avoid a parity-comparison ambiguity.
type Version struct {
	Major    int
	Minor    int
	Patch    int
	Stage    stage
	StageNum int
}

var versionRe = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:(a|b|rc)(\d+))?$`)

// ParseVersion parses a canonical release version such as "0.3.0" or "0.4.0rc1".
func ParseVersion(s string) (Version, error) {
	m := versionRe.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("invalid version %q (expected X.Y.Z or X.Y.Z{a|b|rc}N)", s)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	st := stageStable
	num := 0
	if m[4] != "" {
		switch m[4] {
		case "a":
			st = stageAlpha
		case "b":
			st = stageBeta
		case "rc":
			st = stageRC
		}
		num, _ = strconv.Atoi(m[5])
		if num == 0 {
			return Version{}, fmt.Errorf("invalid version %q (prerelease number must be positive)", s)
		}
	}
	v := Version{Major: major, Minor: minor, Patch: patch, Stage: st, StageNum: num}
	if s != v.Canonical() {
		return Version{}, fmt.Errorf("invalid version %q (must use canonical spelling %q)", s, v.Canonical())
	}
	return v, nil
}

// ParseTag parses a "vX.Y.Z" git tag into a Version.
func ParseTag(tag string) (Version, error) {
	if !strings.HasPrefix(tag, "v") {
		return Version{}, fmt.Errorf("tag %q must start with 'v'", tag)
	}
	return ParseVersion(tag[1:])
}

// Canonical returns the canonical string spelling of the version.
func (v Version) Canonical() string {
	base := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	switch v.Stage {
	case stageAlpha:
		return base + "a" + strconv.Itoa(v.StageNum)
	case stageBeta:
		return base + "b" + strconv.Itoa(v.StageNum)
	case stageRC:
		return base + "rc" + strconv.Itoa(v.StageNum)
	default:
		return base
	}
}

// Compare returns -1, 0, or 1 as v is less than, equal to, or greater than o,
// ordering by (major, minor, patch, stage, stageNum).
func (v Version) Compare(o Version) int {
	for _, p := range [][2]int{
		{v.Major, o.Major},
		{v.Minor, o.Minor},
		{v.Patch, o.Patch},
		{int(v.Stage), int(o.Stage)},
		{v.StageNum, o.StageNum},
	} {
		if p[0] < p[1] {
			return -1
		}
		if p[0] > p[1] {
			return 1
		}
	}
	return 0
}
