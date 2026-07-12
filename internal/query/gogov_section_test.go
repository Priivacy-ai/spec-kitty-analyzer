package query

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWantsSpecKittyGo(t *testing.T) {
	cases := []struct {
		include []string
		want    bool
	}{
		{nil, false},                    // default include is applied by the caller, not here
		{[]string{"timeline"}, false},   // unrelated section
		{[]string{"go"}, true},          // explicit
		{[]string{"all"}, true},         // all subsumes go
		{[]string{"GO"}, true},          // case-insensitive via normalizeList
		{[]string{"signals", "go"}, true},
		{[]string{"spec-kitty-go"}, true},
	}
	for _, c := range cases {
		if got := WantsSpecKittyGo(c.include); got != c.want {
			t.Errorf("WantsSpecKittyGo(%v) = %v, want %v", c.include, got, c.want)
		}
	}
}

// TestResultMarshalsSpecKittyGoSection guards the query JSON contract: when a
// spec-kitty-go report is attached it serializes under the "spec_kitty_go" key,
// and when absent the key is omitted (omitempty).
func TestResultMarshalsSpecKittyGoSection(t *testing.T) {
	var absent Result
	data, err := json.Marshal(absent)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "spec_kitty_go") {
		t.Fatalf("expected spec_kitty_go to be omitted when nil, got: %s", data)
	}
}
