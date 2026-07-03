package analyzer

import (
	"encoding/json"
	"testing"
)

// TestCurrentBuildDefaults locks the local/dev sentinels (FR-004). A build with
// no -ldflags injection must report dev/none/unknown, never a real version.
func TestCurrentBuildDefaults(t *testing.T) {
	b := CurrentBuild()
	if b.Version != "dev" || b.Commit != "none" || b.BuildDate != "unknown" {
		t.Fatalf("CurrentBuild()=%+v, want {dev none unknown}", b)
	}
}

// TestReportJSONHasNestedBuildAndNoTopLevelVersion guards the C1 breaking
// contract in both directions (FR-002 + FR-005): a nested `build` object with
// the three fields, and NO top-level `version` key.
func TestReportJSONHasNestedBuildAndNoTopLevelVersion(t *testing.T) {
	data, err := json.Marshal(Report{Build: CurrentBuild()})
	if err != nil {
		t.Fatalf("marshal Report: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal Report: %v", err)
	}
	if _, ok := m["version"]; ok {
		t.Errorf("top-level \"version\" must be absent, got: %s", data)
	}
	raw, ok := m["build"]
	if !ok {
		t.Fatalf("nested \"build\" object missing, got: %s", data)
	}
	var b Build
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("unmarshal build: %v", err)
	}
	if b.Version != "dev" || b.Commit != "none" || b.BuildDate != "unknown" {
		t.Errorf("build=%+v, want {dev none unknown}", b)
	}
}
