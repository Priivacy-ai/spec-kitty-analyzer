package main

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/priivacy-ai/spec-kitty-analyzer/internal/analyzer"
)

// TestVersionCommandOutput covers FR-001: the `version` command prints version,
// commit, and build date on one line. On an un-injected (test) build these are
// the sentinels.
func TestVersionCommandOutput(t *testing.T) {
	out := captureStdout(t, func() {
		if err := run([]string{"version"}); err != nil {
			t.Fatalf("run(version): %v", err)
		}
	})
	want := "spec-kitty-analyzer dev (commit none, built unknown)\n"
	if out != want {
		t.Fatalf("version output = %q, want %q", out, want)
	}
}

// TestMissionsResultJSONShape covers FR-002 + FR-005 for the missions surface:
// nested `build` object present, top-level `version` absent.
func TestMissionsResultJSONShape(t *testing.T) {
	data, err := json.Marshal(missionsResult{Build: analyzer.CurrentBuild()})
	if err != nil {
		t.Fatalf("marshal missionsResult: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["version"]; ok {
		t.Errorf("top-level \"version\" must be absent, got: %s", data)
	}
	raw, ok := m["build"]
	if !ok {
		t.Fatalf("nested \"build\" object missing, got: %s", data)
	}
	var b analyzer.Build
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("unmarshal build: %v", err)
	}
	if b.Version != "dev" || b.Commit != "none" || b.BuildDate != "unknown" {
		t.Errorf("build=%+v, want {dev none unknown}", b)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()
	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(data)
}
