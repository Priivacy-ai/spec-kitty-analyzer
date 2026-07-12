package gogov

import (
	"fmt"
	"sort"
	"strings"
)

// RenderText produces a human-readable spec-kitty-go activity report.
func RenderText(rep Report) string {
	var b strings.Builder
	s := rep.Summary
	fmt.Fprintf(&b, "spec-kitty-go activity report (analyzer %s)\n", rep.Version)
	fmt.Fprintf(&b, "generated: %s\n", rep.GeneratedAt.Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(&b, "scanned:   %d log file(s), %d with spec-kitty-go activity\n", s.FilesScanned, s.FilesWithGoUsage)
	if s.FirstSeen != nil && s.LastSeen != nil {
		fmt.Fprintf(&b, "window:    %s -> %s\n", s.FirstSeen.Format("2006-01-02 15:04:05"), s.LastSeen.Format("2006-01-02 15:04:05"))
	}
	b.WriteString("\n")

	// Governance decisions -- the headline.
	fmt.Fprintf(&b, "Governance decisions (GovernedActions): %d\n", s.GovernedActions)
	if s.GovernedActions > 0 {
		fmt.Fprintf(&b, "  verdicts:       %s\n", formatCounts(s.Verdicts, verdictOrder))
		fmt.Fprintf(&b, "  governed tools: %s\n", formatCounts(s.GovernedTools, nil))
		fmt.Fprintf(&b, "  hook events:    %s\n", formatCounts(s.HookEvents, nil))
		if len(s.Adapters) > 0 {
			fmt.Fprintf(&b, "  adapters:       %s\n", formatCounts(s.Adapters, nil))
		}
		if len(s.ContextRefs) > 0 {
			fmt.Fprintf(&b, "  context refs:   %s\n", strings.Join(s.ContextRefs, ", "))
		}
		if s.Latency.Count > 0 {
			fmt.Fprintf(&b, "  hook latency:   min %dms  p50 %dms  p95 %dms  max %dms  mean %.1fms  (n=%d)\n",
				s.Latency.MinMs, s.Latency.P50Ms, s.Latency.P95Ms, s.Latency.MaxMs, s.Latency.MeanMs, s.Latency.Count)
		}
		if s.Denials+s.DecisionsNeeded+s.Errors > 0 {
			fmt.Fprintf(&b, "  attention:      %d denied, %d decision-required, %d errored\n", s.Denials, s.DecisionsNeeded, s.Errors)
		} else {
			b.WriteString("  attention:      none (all governed actions admitted cleanly)\n")
		}
	}
	b.WriteString("\n")

	// Direct CLI verb usage.
	fmt.Fprintf(&b, "Direct go-binary invocations (CLI): %d\n", s.CLIInvocations)
	if s.CLIInvocations > 0 {
		for _, line := range sortedCountLines(s.CLIVerbs) {
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}
	b.WriteString("\n")

	// Timeline.
	if len(rep.Events) > 0 {
		b.WriteString("Timeline:\n")
		for _, ev := range rep.Events {
			b.WriteString("  " + renderEventLine(ev) + "\n")
		}
	}

	for _, note := range rep.Notes {
		fmt.Fprintf(&b, "\nNote: %s\n", note)
	}
	return b.String()
}

func renderEventLine(ev Event) string {
	ts := "                   "
	if ev.Timestamp != nil {
		ts = ev.Timestamp.Format("2006-01-02 15:04:05")
	}
	if ev.Category == CategoryHook {
		detail := fmt.Sprintf("govern %-8s -> %-16s", ev.GovernedTool, ev.Verdict)
		if ev.DurationMs != nil {
			detail += fmt.Sprintf(" (%dms)", *ev.DurationMs)
		}
		if ev.HookEvent != "" {
			detail += " [" + ev.HookEvent + "]"
		}
		if ev.Stderr != "" {
			detail += " stderr=" + truncate(ev.Stderr, 60)
		}
		return fmt.Sprintf("%s  hook  %s", ts, detail)
	}
	verb := strings.TrimSpace(ev.Verb + " " + ev.Subcommand)
	return fmt.Sprintf("%s  cli   %s %s", ts, ev.Binary, verb)
}

var verdictOrder = []string{VerdictAdmit, VerdictDeny, VerdictDecisionRequired, VerdictError, VerdictUnknown}

// formatCounts renders a count map as "a=2, b=1". When order is non-nil, its
// keys lead (in the given order); any remaining keys follow, count-descending.
func formatCounts(m map[string]int, order []string) string {
	if len(m) == 0 {
		return "(none)"
	}
	seen := map[string]bool{}
	var parts []string
	for _, k := range order {
		if v, ok := m[k]; ok {
			parts = append(parts, fmt.Sprintf("%s=%d", k, v))
			seen[k] = true
		}
	}
	rest := make([]string, 0, len(m))
	for k := range m {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Slice(rest, func(i, j int) bool {
		if m[rest[i]] != m[rest[j]] {
			return m[rest[i]] > m[rest[j]]
		}
		return rest[i] < rest[j]
	})
	for _, k := range rest {
		parts = append(parts, fmt.Sprintf("%s=%d", k, m[k]))
	}
	return strings.Join(parts, ", ")
}

func sortedCountLines(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%4d  %s", m[k], k))
	}
	return lines
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
