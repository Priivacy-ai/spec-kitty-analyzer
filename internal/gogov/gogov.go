// Package gogov extracts a record of what the spec-kitty-go binary actually
// DID during agent sessions, from Claude Code / OpenCode / Codex transcript
// logs.
//
// The existing analyzer engine is built for the Python Spec Kitty surface
// (/spec-kitty.* slash commands, `spec-kitty ...` CLI verbs, spk-* skills). It
// is deliberately channel-scoped and, critically, its typed channel dispatch
// drops any log line it does not recognize -- including the Claude Code
// `type:"attachment"` hook events that carry spec-kitty-go's governance
// verdicts. Those verdicts are the single most direct evidence of the go
// tool's runtime behavior, so this package reads them first-class.
//
// spec-kitty-go is the ground-up "governed-operation platform" rewrite: it
// wraps each unit of agent work in a GovernedAction envelope, admits it
// deny-by-default, and returns one of ADMIT | DENY | DECISION_REQUIRED before
// any side effect. When wired into Claude Code as a PreToolUse hook
// (`spec-kitty hook run --adapter claude-code --event PreToolUse ...`) every
// governed tool call leaves a structured attachment in the transcript:
//
//	{"type":"attachment","attachment":{
//	   "type":"hook_success","hookName":"PreToolUse:Bash","hookEvent":"PreToolUse",
//	   "content":"ADMIT","stdout":"ADMIT\n","stderr":"","exitCode":0,
//	   "command":".../spec-kitty hook run --adapter claude-code --event PreToolUse --governance-context-ref ctx/...",
//	   "durationMs":49}}
//
// This package turns those events -- plus direct invocations of the go binary's
// own verb surface (hook/review/space/ledger/seal/governance/config +
// witness-sidecar) -- into a deterministic activity report.
package gogov

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/priivacy-ai/spec-kitty-analyzer/internal/analyzer"
)

// maxInputFileBytes bounds how much of any single log we will read (mirrors the
// analyzer engine's own cap).
const maxInputFileBytes = 50 << 20

// Governance verdicts emitted by spec-kitty-go's admission decision.
const (
	VerdictAdmit            = "ADMIT"
	VerdictDeny             = "DENY"
	VerdictDecisionRequired = "DECISION_REQUIRED"
	VerdictError            = "ERROR"   // hook failed to run / non-zero exit
	VerdictUnknown          = "UNKNOWN" // ran, but no recognizable verdict token
)

// Event categories.
const (
	CategoryHook = "hook" // a governance decision captured as a transcript hook attachment
	CategoryCLI  = "cli"  // a direct invocation of the go binary's verb surface
)

// Event is one observed spec-kitty-go action.
type Event struct {
	Seq        int        `json:"seq"`
	Timestamp  *time.Time `json:"timestamp,omitempty"`
	SourcePath string     `json:"source_path"`
	Line       int        `json:"line"`
	Category   string     `json:"category"` // hook | cli

	// Hook (governance decision) fields.
	HookEvent     string `json:"hook_event,omitempty"`     // PreToolUse | PostToolUse | ...
	GovernedTool  string `json:"governed_tool,omitempty"`  // Read | Bash | Edit | ...
	Verdict       string `json:"verdict,omitempty"`        // ADMIT | DENY | DECISION_REQUIRED | ERROR | UNKNOWN
	Reason        string `json:"reason,omitempty"`         // DENY/DECISION_REQUIRED summary (text after the verdict token)
	ToolUseID     string `json:"tool_use_id,omitempty"`    // links this decision to the governed tool call
	GovernedInput string `json:"governed_input,omitempty"` // what that tool call was about to do (e.g. the Bash command)
	DurationMs    *int   `json:"duration_ms,omitempty"`
	ExitCode      *int   `json:"exit_code,omitempty"`
	Adapter       string `json:"adapter,omitempty"`     // claude-code | opencode | ...
	ContextRef    string `json:"context_ref,omitempty"` // --governance-context-ref value
	Stderr        string `json:"stderr,omitempty"`

	// CLI (verb-surface) fields.
	Binary     string `json:"binary,omitempty"`     // spec-kitty | witness-sidecar
	Verb       string `json:"verb,omitempty"`       // hook | review | space | ledger | seal | ...
	Subcommand string `json:"subcommand,omitempty"` // run | verify | evaluate | accept | admit | ...

	Raw string `json:"raw,omitempty"` // representative command / verdict text
}

// LatencyStats summarizes governance-hook durations (spec-kitty-go's hot path).
type LatencyStats struct {
	Count  int     `json:"count"`
	MinMs  int     `json:"min_ms"`
	P50Ms  int     `json:"p50_ms"`
	P95Ms  int     `json:"p95_ms"`
	MaxMs  int     `json:"max_ms"`
	MeanMs float64 `json:"mean_ms"`
}

// Summary is the rolled-up view of spec-kitty-go behavior.
type Summary struct {
	GovernedActions  int            `json:"governed_actions"`
	Verdicts         map[string]int `json:"verdicts"`
	GovernedTools    map[string]int `json:"governed_tools"`
	HookEvents       map[string]int `json:"hook_events"`
	Adapters         map[string]int `json:"adapters,omitempty"`
	ContextRefs      []string       `json:"context_refs,omitempty"`
	Latency          LatencyStats   `json:"latency_ms"`
	Denials          int            `json:"denials"`
	DecisionsNeeded  int            `json:"decisions_required"`
	Errors           int            `json:"errors"`
	Reasons          []string       `json:"deny_decision_reasons,omitempty"` // distinct DENY/DECISION reasons
	PreToolHooks     int            `json:"pre_tool_hooks"`
	PostToolHooks    int            `json:"post_tool_hooks"`
	PrePostPaired    int            `json:"pre_post_paired"` // governed tool calls seen with both a Pre and Post hook
	CLIInvocations   int            `json:"cli_invocations"`
	CLIVerbs         map[string]int `json:"cli_verbs,omitempty"`
	Binaries         map[string]int `json:"binaries,omitempty"`
	Ledger           LedgerActivity `json:"ledger"`
	FilesScanned     int            `json:"files_scanned"`
	FilesWithGoUsage int            `json:"files_with_go_activity"`
	FirstSeen        *time.Time     `json:"first_seen,omitempty"`
	LastSeen         *time.Time     `json:"last_seen,omitempty"`
}

// LedgerActivity summarizes what reached spec-kitty-go's tamper-evident ledger.
//
// The ledger lives out-of-band in ledger.db (`.kittify/`), not in the harness
// transcript, so this reports two things: OBSERVED ledger/seal CLI operations
// in the logs, plus the DERIVED count of admission-decision appends that the
// hook contract guarantees. Per cmd/spec-kitty/hook.go, every governed
// admission durably appends OperationRequested, OperationClassified,
// GovernanceContextResolved and the final AdmissionDecision to ledger.db before
// the verdict is returned -- so each governance decision here corresponds to a
// recorded ledger decision, even though the record itself is not in the log.
type LedgerActivity struct {
	AdmissionDecisionsRecorded int  `json:"admission_decisions_recorded"` // derived: one per governance decision
	LedgerCLIOps               int  `json:"ledger_cli_ops"`               // observed: `spec-kitty ledger ...`
	SealOps                    int  `json:"seal_ops"`                     // observed: `spec-kitty seal` / auto-seal CLI
	Derived                    bool `json:"derived"`                      // true: appends are inferred from the hook contract, not read from ledger.db
}

// Report is the top-level result of an activity scan.
type Report struct {
	Tool        string    `json:"tool"`    // always "spec-kitty-go"
	Version     string    `json:"version"` // analyzer version
	GeneratedAt time.Time `json:"generated_at"`
	Inputs      []string  `json:"inputs"`
	Summary     Summary   `json:"summary"`
	Events      []Event   `json:"events"`
	Notes       []string  `json:"notes,omitempty"`
}

// distinctiveGoVerbs are go-only verbs: safe to attribute to spec-kitty-go even
// when the binary token is bare `spec-kitty` (the Python CLI shares the binary
// name but not these verbs; it uses dispatch/next/specify/plan/tasks/...).
var distinctiveGoVerbs = map[string]bool{
	"hook":        true,
	"space":       true,
	"ledger":      true,
	"seal":        true,
	"governance":  true,
	"composition": true,
	"review":      true,
}

// ambiguousGoVerbs are accepted only when the binary is clearly the go build
// (path-prefixed bin/spec-kitty, cmd/spec-kitty, or `go run ./cmd/spec-kitty`).
var ambiguousGoVerbs = map[string]bool{
	"config":  true,
	"version": true,
	"charter": true,
	"init":    true,
}

// goSubcommands lists the known second-level verbs per top-level verb. A token
// following the verb is only recorded as a subcommand when it is in this set;
// otherwise it is a positional argument (a filename, project name, etc.) and is
// left off the aggregation key. This keeps rollups clean
// (`witness-sidecar verify-provenance`, not `... verify-provenance att`) while
// the full command text is still preserved on each event's Raw field.
var goSubcommands = map[string]map[string]bool{
	"hook":       {"run": true},
	"ledger":     {"verify": true, "append": true, "list": true, "show": true, "export": true},
	"space":      {"admit": true, "list": true, "show": true},
	"review":     {"evaluate": true, "accept": true},
	"config":     {"get": true, "set": true, "list": true, "show": true},
	"governance": {"status": true, "show": true},
	"charter":    {"status": true, "sync": true, "context": true},
}

// goCLIRe matches an invocation of the spec-kitty-go binary surface inside a
// command string. Group 1 = optional path prefix, 2 = binary, 3 = verb,
// 4 = optional subcommand.
var goCLIRe = regexp.MustCompile(`((?:\S*/)?(?:cmd/)?)(spec-kitty|witness-sidecar)\s+([a-z][a-z-]+)(?:\s+([a-z][a-z-]+))?`)

var contextRefRe = regexp.MustCompile(`--governance-context-ref[= ]([^\s"]+)`)
var adapterRe = regexp.MustCompile(`--adapter[= ]([^\s"]+)`)
var eventFlagRe = regexp.MustCompile(`--event[= ]([^\s"]+)`)

// AnalyzeFiles scans the given transcript files and returns a spec-kitty-go
// activity report. now is passed in so callers control the generated_at stamp
// (and tests stay deterministic).
func AnalyzeFiles(paths []string, now time.Time) Report {
	rep := Report{
		Tool:        "spec-kitty-go",
		Version:     analyzer.Version,
		GeneratedAt: now,
		Inputs:      append([]string{}, paths...),
	}
	var events []Event
	filesWith := 0
	for _, path := range paths {
		before := len(events)
		events = append(events, extractFromFile(path)...)
		if len(events) > before {
			filesWith++
		}
	}
	sort.SliceStable(events, func(i, j int) bool {
		ti, tj := events[i].Timestamp, events[j].Timestamp
		switch {
		case ti != nil && tj != nil && !ti.Equal(*tj):
			return ti.Before(*tj)
		case (ti == nil) != (tj == nil):
			return ti != nil // timestamped events first
		case events[i].SourcePath != events[j].SourcePath:
			return events[i].SourcePath < events[j].SourcePath
		default:
			return events[i].Line < events[j].Line
		}
	})
	for i := range events {
		events[i].Seq = i + 1
	}
	rep.Events = events
	rep.Summary = buildSummary(events, len(paths), filesWith)
	if len(events) == 0 {
		rep.Notes = append(rep.Notes, "No spec-kitty-go activity detected. Scans for governance-hook attachments (spec-kitty hook run verdicts) and direct go-binary verb invocations.")
	}
	return rep
}

// extractFromFile reads one transcript file and returns its spec-kitty-go events.
func extractFromFile(path string) []Event {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if len(data) > maxInputFileBytes {
		data = data[:maxInputFileBytes]
	}
	scrubbed, _ := analyzer.Scrub(data)
	lines := strings.Split(string(scrubbed), "\n")

	// Pass 1: index every tool_use block by its id so a hook attachment can be
	// linked to the exact tool call it governed (the attachment carries only the
	// toolUseID, not the tool's name/input).
	toolUses := indexToolUses(lines)

	// Pass 2: extract governance decisions and CLI invocations.
	var events []Event
	for i, rawLine := range lines {
		lineNo := i + 1
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(line), &obj) == nil {
			ts := parseTimestamp(obj)
			if hook, ok := hookEventFrom(obj); ok {
				hook.SourcePath = path
				hook.Line = lineNo
				hook.Timestamp = ts
				if ref, found := toolUses[hook.ToolUseID]; found {
					if hook.GovernedTool == "" {
						hook.GovernedTool = ref.name
					}
					hook.GovernedInput = ref.summary
				}
				events = append(events, hook)
			}
			for _, cmd := range commandStrings(obj) {
				for _, ev := range cliEventsFrom(cmd) {
					ev.SourcePath = path
					ev.Line = lineNo
					ev.Timestamp = ts
					events = append(events, ev)
				}
			}
			continue
		}
		// Non-JSON (plain .log/.txt) line: treat the whole line as a command.
		for _, ev := range cliEventsFrom(line) {
			ev.SourcePath = path
			ev.Line = lineNo
			events = append(events, ev)
		}
	}
	return events
}

type toolRef struct {
	name    string
	summary string
}

// indexToolUses scans transcript lines for tool_use blocks and returns a map
// from tool-use id to a short description of what that tool call was doing.
func indexToolUses(lines []string) map[string]toolRef {
	index := map[string]toolRef{}
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || !strings.Contains(line, "tool_use") {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(line), &obj) != nil {
			continue
		}
		m, ok := obj["message"].(map[string]any)
		if !ok {
			continue
		}
		content, ok := m["content"].([]any)
		if !ok {
			continue
		}
		for _, item := range content {
			block, ok := item.(map[string]any)
			if !ok || asString(block["type"]) != "tool_use" {
				continue
			}
			id := asString(block["id"])
			if id == "" {
				continue
			}
			name := asString(block["name"])
			input, _ := block["input"].(map[string]any)
			index[id] = toolRef{name: name, summary: summarizeToolInput(name, input)}
		}
	}
	return index
}

// summarizeToolInput renders a one-line description of a governed tool call.
func summarizeToolInput(name string, input map[string]any) string {
	if input == nil {
		return ""
	}
	for _, key := range []string{"command", "file_path", "path", "pattern", "url", "prompt"} {
		if v := asString(input[key]); v != "" {
			return truncate(v, 120)
		}
	}
	return ""
}

// hookEventFrom recognizes a Claude Code hook attachment that carries a
// spec-kitty-go governance verdict and builds a hook Event from it.
func hookEventFrom(obj map[string]any) (Event, bool) {
	if asString(obj["type"]) != "attachment" {
		return Event{}, false
	}
	att, ok := obj["attachment"].(map[string]any)
	if !ok {
		return Event{}, false
	}
	hookType := asString(att["type"])
	if !strings.HasPrefix(hookType, "hook") {
		return Event{}, false
	}
	command := asString(att["command"])
	// Attribute to spec-kitty-go only when the hook command runs its binary.
	if !isSpecKittyGoHookCommand(command) {
		return Event{}, false
	}
	ev := Event{
		Category:     CategoryHook,
		HookEvent:    asString(att["hookEvent"]),
		GovernedTool: governedToolFromHookName(asString(att["hookName"]), asString(att["hookEvent"])),
		ToolUseID:    asString(att["toolUseID"]),
		Stderr:       strings.TrimSpace(asString(att["stderr"])),
	}
	if ev.HookEvent == "" {
		ev.HookEvent = firstSubmatch(eventFlagRe, command)
	}
	if code, ok := asInt(att["exitCode"]); ok {
		ev.ExitCode = &code
	}
	if dur, ok := asInt(att["durationMs"]); ok {
		ev.DurationMs = &dur
	}
	ev.Verdict, ev.Reason = verdictFrom(asString(att["content"]), asString(att["stdout"]), hookType, ev.ExitCode)
	ev.Adapter = firstSubmatch(adapterRe, command)
	ev.ContextRef = firstSubmatch(contextRefRe, command)
	ev.Raw = strings.TrimSpace(command)
	return ev, true
}

// isSpecKittyGoHookCommand reports whether a hook command line runs the go
// binary's `hook run` surface.
func isSpecKittyGoHookCommand(command string) bool {
	if command == "" {
		return false
	}
	return strings.Contains(command, "spec-kitty") && strings.Contains(command, "hook run")
}

// commandStrings pulls command-bearing strings out of a decoded transcript
// object: Bash/Shell tool_use inputs, a top-level "message" string, and codex
// function_call arguments. It deliberately excludes assistant/user narrative
// text so an agent merely *discussing* a command does not register.
func commandStrings(obj map[string]any) []string {
	var out []string
	if msg := asString(obj["message"]); msg != "" {
		out = append(out, msg)
	}
	// Claude: message.content[] tool_use blocks.
	if m, ok := obj["message"].(map[string]any); ok {
		if content, ok := m["content"].([]any); ok {
			for _, item := range content {
				block, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if asString(block["type"]) != "tool_use" {
					continue
				}
				if !isShellTool(asString(block["name"])) {
					continue
				}
				input, ok := block["input"].(map[string]any)
				if !ok {
					continue
				}
				if cmd := asString(input["command"]); cmd != "" {
					out = append(out, cmd)
				}
			}
		}
	}
	// Codex: {"payload":{"type":"function_call","name":"shell","arguments":"{...command...}"}}.
	if payload, ok := obj["payload"].(map[string]any); ok {
		if asString(payload["type"]) == "function_call" && isShellTool(asString(payload["name"])) {
			if args := asString(payload["arguments"]); args != "" {
				out = append(out, args)
			}
		}
	}
	return out
}

func isShellTool(name string) bool {
	switch strings.ToLower(name) {
	case "bash", "shell", "local_shell", "exec", "run":
		return true
	}
	return false
}

// cliEventsFrom finds every spec-kitty-go binary invocation in one command
// string (a chained command can hold several).
func cliEventsFrom(command string) []Event {
	var events []Event
	for _, m := range goCLIRe.FindAllStringSubmatch(command, -1) {
		prefix, binary, verb, sub := m[1], m[2], m[3], m[4]
		if !acceptGoInvocation(prefix, binary, verb) {
			continue
		}
		if !goSubcommands[verb][sub] {
			sub = "" // a positional argument, not a real subcommand
		}
		events = append(events, Event{
			Category:   CategoryCLI,
			Binary:     binary,
			Verb:       verb,
			Subcommand: sub,
			Raw:        strings.TrimSpace(m[0]),
		})
	}
	return events
}

// acceptGoInvocation gates a regex match down to real spec-kitty-go usage.
func acceptGoInvocation(prefix, binary, verb string) bool {
	if binary == "witness-sidecar" {
		return true // witness-sidecar is a go-only binary
	}
	// binary == "spec-kitty" (shared name with the Python CLI).
	if distinctiveGoVerbs[verb] {
		return true
	}
	// Ambiguous verbs need an unambiguous go-build path prefix.
	pathPrefixed := strings.Contains(prefix, "bin/") || strings.Contains(prefix, "cmd/")
	return pathPrefixed && ambiguousGoVerbs[verb]
}

// governedToolFromHookName splits "PreToolUse:Bash" -> "Bash".
func governedToolFromHookName(hookName, hookEvent string) string {
	if hookName == "" {
		return ""
	}
	if idx := strings.LastIndex(hookName, ":"); idx >= 0 {
		return strings.TrimSpace(hookName[idx+1:])
	}
	if hookName == hookEvent {
		return ""
	}
	return hookName
}

// verdictFrom derives the governance verdict and its human reason from a hook's
// output. It follows spec-kitty-go's exact `hook run` contract
// (cmd/spec-kitty/hook.go): stdout is "ADMIT" | "DENY: <summary>" |
// "DECISION_REQUIRED: <summary>", and the exit code is ADMIT=0, DENY=1,
// usage/error=2, DECISION_REQUIRED=3. The stdout token is authoritative; the
// exit code is the fallback when a harness recorded no textual content.
func verdictFrom(content, stdout, hookType string, exitCode *int) (verdict, reason string) {
	raw := strings.TrimSpace(content)
	if raw == "" {
		raw = strings.TrimSpace(stdout)
	}
	upper := strings.ToUpper(raw)
	switch {
	case strings.Contains(upper, VerdictDecisionRequired):
		return VerdictDecisionRequired, reasonAfterToken(raw, VerdictDecisionRequired)
	case strings.Contains(upper, VerdictDeny):
		return VerdictDeny, reasonAfterToken(raw, VerdictDeny)
	case strings.Contains(upper, VerdictAdmit):
		return VerdictAdmit, ""
	}
	// No verdict token: fall back to the exit-code contract.
	if exitCode != nil {
		switch *exitCode {
		case 0:
			return VerdictUnknown, ""
		case 1:
			return VerdictDeny, ""
		case 3:
			return VerdictDecisionRequired, ""
		default: // 2 (usage/error) or any other non-zero
			return VerdictError, ""
		}
	}
	if lt := strings.ToLower(hookType); strings.Contains(lt, "error") || strings.Contains(lt, "block") {
		return VerdictError, ""
	}
	return VerdictUnknown, ""
}

// reasonAfterToken returns the "<summary>" in "DENY: <summary>" (case- and
// spacing-tolerant); "" when the verdict carried no trailing reason.
func reasonAfterToken(raw, token string) string {
	idx := strings.Index(strings.ToUpper(raw), token)
	if idx < 0 {
		return ""
	}
	rest := raw[idx+len(token):]
	rest = strings.TrimLeft(rest, ": \t")
	return strings.TrimSpace(rest)
}

func buildSummary(events []Event, filesScanned, filesWith int) Summary {
	s := Summary{
		Verdicts:         map[string]int{},
		GovernedTools:    map[string]int{},
		HookEvents:       map[string]int{},
		Adapters:         map[string]int{},
		CLIVerbs:         map[string]int{},
		Binaries:         map[string]int{},
		FilesScanned:     filesScanned,
		FilesWithGoUsage: filesWith,
	}
	refs := map[string]bool{}
	reasonSeen := map[string]bool{}
	// track Pre/Post hooks per governed tool-use id for pairing.
	type prePost struct{ pre, post bool }
	byToolUse := map[string]*prePost{}
	var durations []int
	for _, ev := range events {
		if ev.Timestamp != nil {
			if s.FirstSeen == nil || ev.Timestamp.Before(*s.FirstSeen) {
				t := *ev.Timestamp
				s.FirstSeen = &t
			}
			if s.LastSeen == nil || ev.Timestamp.After(*s.LastSeen) {
				t := *ev.Timestamp
				s.LastSeen = &t
			}
		}
		switch ev.Category {
		case CategoryHook:
			s.GovernedActions++
			s.Verdicts[ev.Verdict]++
			if ev.GovernedTool != "" {
				s.GovernedTools[ev.GovernedTool]++
			}
			if ev.HookEvent != "" {
				s.HookEvents[ev.HookEvent]++
			}
			switch ev.HookEvent {
			case "PreToolUse":
				s.PreToolHooks++
			case "PostToolUse":
				s.PostToolHooks++
			}
			if ev.ToolUseID != "" {
				pp := byToolUse[ev.ToolUseID]
				if pp == nil {
					pp = &prePost{}
					byToolUse[ev.ToolUseID] = pp
				}
				switch ev.HookEvent {
				case "PreToolUse":
					pp.pre = true
				case "PostToolUse":
					pp.post = true
				}
			}
			if ev.Adapter != "" {
				s.Adapters[ev.Adapter]++
			}
			if ev.ContextRef != "" {
				refs[ev.ContextRef] = true
			}
			if ev.DurationMs != nil {
				durations = append(durations, *ev.DurationMs)
			}
			if ev.Reason != "" && !reasonSeen[ev.Reason] {
				reasonSeen[ev.Reason] = true
				s.Reasons = append(s.Reasons, ev.Reason)
			}
			switch ev.Verdict {
			case VerdictDeny:
				s.Denials++
			case VerdictDecisionRequired:
				s.DecisionsNeeded++
			case VerdictError:
				s.Errors++
			}
		case CategoryCLI:
			s.CLIInvocations++
			if ev.Binary != "" {
				s.Binaries[ev.Binary]++
			}
			key := ev.Binary + " " + ev.Verb
			if ev.Subcommand != "" {
				key += " " + ev.Subcommand
			}
			s.CLIVerbs[strings.TrimSpace(key)]++
			if ev.Binary == "spec-kitty" {
				switch ev.Verb {
				case "ledger":
					s.Ledger.LedgerCLIOps++
				case "seal":
					s.Ledger.SealOps++
				}
			}
		}
	}
	for _, pp := range byToolUse {
		if pp.pre && pp.post {
			s.PrePostPaired++
		}
	}
	for ref := range refs {
		s.ContextRefs = append(s.ContextRefs, ref)
	}
	sort.Strings(s.ContextRefs)
	sort.Strings(s.Reasons)
	s.Latency = latencyStats(durations)
	// Every governed admission durably appends its decision to ledger.db per the
	// hook contract; that append is not in the transcript, so mark it derived.
	s.Ledger.AdmissionDecisionsRecorded = s.GovernedActions
	s.Ledger.Derived = true
	return s
}

func latencyStats(durations []int) LatencyStats {
	if len(durations) == 0 {
		return LatencyStats{}
	}
	sorted := append([]int{}, durations...)
	sort.Ints(sorted)
	sum := 0
	for _, d := range sorted {
		sum += d
	}
	return LatencyStats{
		Count:  len(sorted),
		MinMs:  sorted[0],
		P50Ms:  percentile(sorted, 50),
		P95Ms:  percentile(sorted, 95),
		MaxMs:  sorted[len(sorted)-1],
		MeanMs: float64(sum) / float64(len(sorted)),
	}
}

// percentile returns the p-th percentile (nearest-rank) of a pre-sorted slice.
func percentile(sorted []int, p int) int {
	if len(sorted) == 0 {
		return 0
	}
	rank := (p * len(sorted)) / 100
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}

// --- small decoding helpers (kept local so the delicate analyzer package is untouched) ---

func firstSubmatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i), true
		}
	}
	return 0, false
}

func parseTimestamp(obj map[string]any) *time.Time {
	raw := ""
	for _, key := range []string{"timestamp", "created_at", "time", "ts"} {
		if s := asString(obj[key]); s != "" {
			raw = s
			break
		}
	}
	if raw == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return &t
		}
	}
	return nil
}
