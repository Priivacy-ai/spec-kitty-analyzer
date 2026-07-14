package analyzer

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	slashCommandRE  = regexp.MustCompile(`(?i)(?:^|[\s"'(:` + "`" + `])(/spec-kitty[.\-][a-z][a-z0-9_-]*)\b`)
	specKittyCLIRe  = regexp.MustCompile(`(?m)(?:^|[\s$` + "`" + `;&|])((?:SPEC_KITTY_ENABLE_SAAS_SYNC=1\s+)?(?:(?:uv|uvx)\s+run\s+)?spec-kitty(?:\s+[^` + "`" + `\n\r;&|]+|$))`)
	skillPathRE     = regexp.MustCompile(`(?i)([A-Za-z0-9_./~@+\-]*((?:spk-[a-z0-9]+-[a-z0-9_.\-]+)|(?:spec-kitty-(?:bulk-edit-classification|charter-doctrine|git-workflow|glossary-context|implement-review|mission-review|mission-system|orchestrator-api-operator|program-orchestrate|runtime-next|runtime-review|setup-doctor|spdd-reasons|agent-surface-research|cli-orchestration|delegated-missions|docker-modes|monorepo-prep))|(?:spec-kitty(?:\.[a-z0-9_.\-]+)?)|ad-hoc-profile-load)/SKILL\.md)`)
	skillNameRE     = regexp.MustCompile(`(?i)(?:^|[\s"'(:` + "`" + `/])((?:spk-[a-z0-9]+-[a-z0-9_.\-]+)|(?:spec-kitty-(?:bulk-edit-classification|charter-doctrine|git-workflow|glossary-context|implement-review|mission-review|mission-system|orchestrator-api-operator|program-orchestrate|runtime-next|runtime-review|setup-doctor|spdd-reasons|agent-surface-research|cli-orchestration|delegated-missions|docker-modes|monorepo-prep))|(?:spec-kitty\.[a-z0-9_.\-]+)|ad-hoc-profile-load)\b`)
	missionPathRE   = regexp.MustCompile(`(?:^|[/\s])kitty-specs/([A-Za-z0-9][A-Za-z0-9_.\-]*)`)
	missionHandleRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.\-]{0,127}$`)
	opPathRE        = regexp.MustCompile(`(?:^|[/\s])kitty-ops/([A-Za-z0-9][A-Za-z0-9_.\-]*)\.jsonl`)
	// invocationIDRE validates a token pulled from free text before it is trusted
	// as a Spec Kitty invocation id: either a 32-char hex digest (e.g. a review
	// invocation) or a 26-char Crockford ULID (e.g. a mission id). This rejects
	// stray tokens like the struct-tag word "omitempty" that a bare substring scan
	// would otherwise capture.
	invocationIDRE = regexp.MustCompile(`(?i)^(?:[0-9a-f]{32}|[0-9A-HJKMNP-TV-Z]{26})$`)
	wpRE           = regexp.MustCompile(`\bWP[0-9]{2,4}\b`)
	profileFlagRE  = regexp.MustCompile(`--profile\s+([A-Za-z0-9_.:\-]+)`)
	agentFlagRE    = regexp.MustCompile(`--agent\s+([A-Za-z0-9_.:\-]+)`)
)

var knownSlashActions = map[string]string{
	"accept":              "accept",
	"analyze":             "analyze",
	"charter":             "charter",
	"dashboard":           "dashboard",
	"implement":           "implement",
	"merge":               "merge",
	"plan":                "plan",
	"research":            "research",
	"review":              "review",
	"specify":             "specify",
	"status":              "status",
	"tasks":               "tasks",
	"tasks-finalize":      "tasks",
	"tasks-outline":       "tasks",
	"tasks-packages":      "tasks",
	"mission-review":      "mission_review",
	"runtime-next":        "next",
	"runtime-review":      "review",
	"implement-review":    "implement_review",
	"setup-doctor":        "doctor",
	"git-workflow":        "git",
	"program-orchestrate": "program",
}

func detectSlashCommands(text string) []SlashCommand {
	seen := map[string]bool{}
	var out []SlashCommand
	for _, m := range slashCommandRE.FindAllStringSubmatch(text, -1) {
		raw := strings.TrimSpace(m[1])
		name := strings.TrimPrefix(strings.ToLower(raw), "/")
		if seen[name] {
			continue
		}
		seen[name] = true
		action := ""
		if idx := strings.LastIndex(name, "."); idx >= 0 {
			action = knownSlashActions[name[idx+1:]]
		} else if idx := strings.LastIndex(name, "-"); idx >= 0 {
			action = knownSlashActions[name[idx+1:]]
		}
		out = append(out, SlashCommand{Name: name, Action: action, Raw: raw})
	}
	return out
}

func detectCLIInvocations(text string) []CLIInvocation {
	seen := map[string]bool{}
	var out []CLIInvocation
	for _, m := range specKittyCLIRe.FindAllStringSubmatch(text, -1) {
		raw := strings.TrimSpace(m[1])
		raw = strings.Trim(raw, `"'`)
		if raw == "" || seen[raw] {
			continue
		}
		seen[raw] = true
		out = append(out, parseCLIInvocation(raw))
	}
	return out
}

func parseCLIInvocation(raw string) CLIInvocation {
	fields := shellishFields(raw)
	inv := CLIInvocation{
		Raw:             raw,
		Args:            fields,
		SaaSSyncEnabled: strings.HasPrefix(raw, "SPEC_KITTY_ENABLE_SAAS_SYNC=1 "),
	}
	specIdx := -1
	for i, f := range fields {
		if f == "spec-kitty" {
			specIdx = i
			break
		}
	}
	if specIdx < 0 {
		return inv
	}
	args := fields[specIdx+1:]
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		args = args[1:]
	}
	if len(args) > 0 {
		inv.Verb = args[0]
	}
	if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		inv.Subcommand = inv.Verb + " " + args[1]
	}
	for i := 0; i < len(fields)-1; i++ {
		switch fields[i] {
		case "--mission", "--feature":
			inv.Mission = normalizeMissionHandle(fields[i+1])
		case "--agent":
			inv.Agent = trimShell(fields[i+1])
			parts := strings.Split(inv.Agent, ":")
			if len(parts) >= 3 {
				inv.Profile = parts[2]
			}
		case "--profile":
			inv.Profile = trimShell(fields[i+1])
		}
	}
	if wp := wpRE.FindString(raw); wp != "" {
		inv.WorkPackage = wp
	}
	return inv
}

func shellishFields(raw string) []string {
	raw = strings.ReplaceAll(raw, "\\\n", " ")
	fields := strings.Fields(raw)
	for i := range fields {
		fields[i] = trimShell(fields[i])
	}
	return fields
}

func trimShell(value string) string {
	return strings.Trim(value, `"',;`+"`")
}

func normalizeMissionHandle(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "<>{}$") || strings.Contains(raw, `\`) {
		return ""
	}
	raw = trimShell(raw)
	raw = strings.Trim(raw, "[]()")
	raw = strings.TrimRight(raw, ".,:;-_")
	if raw == "" || strings.ContainsAny(raw, " \t\r\n") || !missionHandleRE.MatchString(raw) || looksLikeStandaloneMissionID(raw) {
		return ""
	}
	switch strings.ToLower(raw) {
	case "slug", "handle", "mission", "mission-slug", "mission_slug", "feature", "feature-slug", "feature_slug", "your-mission", "your-mission-slug", "my-mission", "text", "whitespace":
		return ""
	default:
		return raw
	}
}

func looksLikeStandaloneMissionID(raw string) bool {
	if len(raw) < 4 || !strings.HasPrefix(raw, "01") {
		return false
	}
	hasDigit := false
	hasUpper := false
	for _, r := range raw {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		default:
			return false
		}
	}
	return hasDigit && hasUpper
}

func detectSkills(text string) []SkillUse {
	seen := map[string]bool{}
	var out []SkillUse
	for _, m := range skillPathRE.FindAllStringSubmatch(text, -1) {
		path := strings.TrimSpace(m[1])
		name := normalizeSkillName(m[2])
		if name == "" || seen[name+"|"+path] {
			continue
		}
		seen[name+"|"+path] = true
		out = append(out, SkillUse{Name: name, Path: path, Raw: path})
	}
	for _, m := range skillNameRE.FindAllStringSubmatch(text, -1) {
		raw := strings.TrimSpace(m[1])
		name := normalizeSkillName(raw)
		if name == "" || seen[name+"|"] {
			continue
		}
		seen[name+"|"] = true
		out = append(out, SkillUse{Name: name, Raw: raw})
	}
	return out
}

func normalizeSkillName(raw string) string {
	raw = strings.TrimSuffix(raw, "/SKILL.md")
	raw = strings.Trim(raw, `/ "'`+"`"+`()`)
	if idx := strings.LastIndex(raw, "/"); idx >= 0 {
		raw = raw[idx+1:]
	}
	return strings.ToLower(strings.ReplaceAll(raw, "_", "-"))
}

func detectAgentProfiles(text string) []AgentProfileUse {
	seen := map[string]bool{}
	var out []AgentProfileUse
	for _, m := range profileFlagRE.FindAllStringSubmatch(text, -1) {
		profile := trimShell(m[1])
		if profile == "" || seen["p:"+profile] {
			continue
		}
		seen["p:"+profile] = true
		out = append(out, AgentProfileUse{Profile: profile, Raw: "--profile " + profile})
	}
	for _, m := range agentFlagRE.FindAllStringSubmatch(text, -1) {
		agent := trimShell(m[1])
		if agent == "" {
			continue
		}
		parts := strings.Split(agent, ":")
		profile := ""
		role := ""
		if len(parts) >= 3 {
			profile = parts[2]
		}
		if len(parts) >= 4 {
			role = parts[3]
		}
		if profile == "" {
			profile = "unknown"
		}
		key := "a:" + agent + ":" + profile
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, AgentProfileUse{Profile: profile, Agent: agent, Role: role, Raw: "--agent " + agent})
	}
	return out
}

func actionFromCommand(inv CLIInvocation, slash []SlashCommand) string {
	if inv.Verb != "" {
		switch inv.Verb {
		case "specify", "plan", "tasks", "implement", "review", "accept", "merge", "next", "dispatch", "research":
			return inv.Verb
		case "agent":
			if strings.HasPrefix(inv.Subcommand, "agent action") {
				parts := strings.Fields(inv.Raw)
				for i, p := range parts {
					if p == "action" && i+1 < len(parts) {
						return trimShell(parts[i+1])
					}
				}
			}
		}
	}
	for _, s := range slash {
		if s.Action != "" {
			return s.Action
		}
	}
	return ""
}

// Codex read-command classification (issue-#13 / codex read-output scoping).
//
// These are pure, deterministic helpers consumed by the channel layer to decide
// whether a codex `function_call_output` carries file/inspection CONTENT (source,
// diffs, doc bodies) rather than real command-failure output. The governing
// invariant is recall-safe: ANY uncertainty — unknown command, unbalanced quotes,
// a missing exit-code line — resolves to the SCANNING default (false / ok=false),
// never to exclusion. A wrong exclusion silently suppresses a real failure; a wrong
// inclusion only costs a benign extra scan. (FR-003, FR-004, C-003 — no shell parser.)

// readCommandSet is the allowlist of commands whose output is inspection CONTENT,
// not command-failure output. Kept deliberately minimal (recall over reach). `awk` is
// excluded on purpose (it can mutate/redirect), so it is not a pure read. `sed` is NOT
// in this bare set either: it is classified by segmentIsRead with option/script
// inspection (a print-only `sed -n 'M,Np'` is a read; any in-place/write/exec/transform
// form is scanned — #37). `true`/`false` are harmless shell no-ops that appear in
// read pipelines (e.g. `rg foo || true`) and produce no content and no real failure.
// `find` is deliberately NOT in this set: it can mutate (`-delete`, `-exec`) so it is
// classified by segmentIsRead with option inspection, not by a bare head lookup.
// `true` is the only shell no-op included (harmless, appears in `rg foo || true`
// idioms); `false` is excluded because it is a failing command, not a read.
var readCommandSet = map[string]bool{
	"cat": true, "head": true, "tail": true, "nl": true, "wc": true,
	"rg": true, "grep": true, "egrep": true, "fgrep": true,
	"ls": true, "stat": true, "file": true,
	"true": true,
}

// findMutatingFlags are `find` actions that write or execute rather than list.
var findMutatingFlags = map[string]bool{
	"-delete": true, "-exec": true, "-execdir": true, "-ok": true, "-okdir": true,
	"-fprint": true, "-fprintf": true, "-fprint0": true, "-fls": true,
}

// gitReadSubcommands are the git subcommands that only read/inspect. A `git` command
// is a read only when its first non-flag argument is one of these; every other git
// subcommand (add, commit, checkout, restore, reset, push, merge, …) mutates → scan.
var gitReadSubcommands = map[string]bool{
	"show": true, "diff": true, "log": true, "blame": true, "status": true,
}

// codexReadFileTools maps codex tool `name`s (when the call is NOT `exec_command`)
// that are pure file reads. It is intentionally empty until a read-file tool name is
// confirmed against a real corpus (WP04); an unknown tool name therefore defaults to
// scan (recall-safe, FR-005). The lookup hook stays so the set can grow without a
// signature change.
var codexReadFileTools = map[string]bool{}

// classifyCodexReadCommand reports whether a codex call is a pure read/inspection
// (FR-003). For a non-`exec_command` tool it classifies by tool name; for
// `exec_command` it requires EVERY operator-split (`&&`/`||`/`;`/`|`) segment to lead
// with a read command (or a read-only `git` subcommand), with no write redirection
// (`>`/`>>`) and balanced quotes. Any uncertainty → false.
func classifyCodexReadCommand(name, cmd string) bool {
	if name != "exec_command" {
		return codexReadFileTools[name]
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}
	// Command / process substitution can hide a mutating command behind a read head
	// (e.g. `cat "$(rm x)"`, `cat <(rm x)`). We cannot cheaply prove the substitution is
	// itself a read without a shell parser (C-003), so any substitution → scan
	// (recall-safe). `>(` is also caught by the write-redirection check below.
	if strings.Contains(cmd, "$(") || strings.ContainsRune(cmd, '`') ||
		strings.Contains(cmd, "<(") || strings.Contains(cmd, ">(") {
		return false
	}
	segments, hasRedirect, balanced := analyzeShellCommand(cmd)
	if hasRedirect || !balanced {
		return false
	}
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			// A leading/trailing/doubled operator (e.g. `cat x &&`, `| grep y`) leaves an
			// empty segment; that command is a shell syntax error — a real failure — so
			// it must be scanned, not read.
			return false
		}
		if !segmentIsRead(seg) {
			return false
		}
	}
	return true
}

// segmentIsRead reports whether one already-split command segment leads with a read
// command. Leading `VAR=value` environment assignments are skipped before the head is
// taken (so `FOO=bar cat x` is still a read). A bare `git` (no subcommand), or a
// segment that is only assignments, is not a read.
func segmentIsRead(seg string) bool {
	// Normalize away shell quoting/escaping BEFORE inspecting heads and option flags:
	// the shell strips quotes/backslashes before the command sees argv, so a dangerous
	// option can be hidden as `"--output=out"` or `\-delete`. Normalizing reveals it.
	raw := strings.Fields(seg)
	fields := make([]string, 0, len(raw))
	for _, f := range raw {
		if n := shellNormalizeToken(f); n != "" {
			fields = append(fields, n)
		}
	}
	if len(fields) == 0 {
		return false
	}
	// A leading env assignment can inject command execution ahead of an allowlisted read
	// head — LD_PRELOAD/DYLD_INSERT_LIBRARIES, PATH shadowing the read binary,
	// GIT_EXTERNAL_DIFF/GIT_PAGER, RIPGREP_CONFIG_PATH (which can carry --pre) — and we
	// cannot prove the value inert without a shell parser (C-003). So any env-prefixed
	// command → scan (recall-safe; over-scanning a benign `LC_ALL=C grep` is acceptable).
	if isEnvAssignment(fields[0]) {
		return false
	}
	head := fields[0]
	rest := fields[1:]
	switch head {
	case "git":
		// Options that write a file (`--output`) or run an external driver
		// (`--ext-diff`, `--textconv`) make an otherwise read-only subcommand mutate or
		// execute, so they are not reads. Inline exec config (`git -c diff.external=…`)
		// is already caught below: the config token is read as the "subcommand" and is
		// not a read subcommand. (Repo-config-based exec such as a preset
		// `diff.external` in .gitconfig is invisible in the command string and is a
		// documented non-goal — the analyzer classifies the output of already-run
		// commands, not adversarial repository configuration.)
		for _, f := range rest {
			if f == "--ext-diff" || f == "--textconv" ||
				f == "--output" || strings.HasPrefix(f, "--output=") {
				return false
			}
		}
		for _, f := range rest {
			if strings.HasPrefix(f, "-") {
				continue
			}
			return gitReadSubcommands[f]
		}
		return false
	case "find":
		for _, f := range rest {
			if findMutatingFlags[f] {
				return false
			}
		}
		return true
	case "rg":
		// `rg --pre <CMD>` runs an external preprocessor command per file, so it can
		// execute a mutating command — not a read.
		for _, f := range rest {
			if f == "--pre" || strings.HasPrefix(f, "--pre=") {
				return false
			}
		}
		return true
	case "sed":
		// `sed` is a read ONLY when it just prints/extracts. It is excluded from the
		// bare allowlist because it can edit in place (`-i`), write/read files
		// (`w`/`W`/`r`/`R`, `s///w`), execute a shell (`e`, `s///e`), or transform
		// content (`s`/`y`). Codex's most common file read is `sed -n 'M,Np' <file>`,
		// so recognizing the print-only forms removes a large false-positive class
		// (#37) while keeping every mutating form scanned (recall-safe, #13 posture).
		return sedIsRead(rest)
	default:
		return readCommandSet[head]
	}
}

// sedRegexAddr matches a `/regex/` sed address (honoring `\/` escapes) so its body is
// not mistaken for a sed command when scanning for write/exec commands.
var sedRegexAddr = regexp.MustCompile(`/(?:\\.|[^/\\])*/`)

// sedSafeShortFlags are single-letter sed options that take NO argument and cannot
// edit/write/execute: -n (quiet), -E/-r (extended regex), -s (separate), -z (null
// data), -u (unbuffered). Any OTHER short flag may take an argument (e.g. GNU
// `-l N`) or mutate, so it fails closed.
var sedSafeShortFlags = map[rune]bool{
	'n': true, 'E': true, 'r': true, 's': true, 'z': true, 'u': true,
}

// sedSafeLongFlags are argumentless long sed options that are inert.
var sedSafeLongFlags = map[string]bool{
	"--quiet": true, "--silent": true, "--regexp-extended": true, "--separate": true,
	"--null-data": true, "--unbuffered": true, "--posix": true, "--sandbox": true,
	"--debug": true,
	// NB: argument-taking long options (e.g. --line-length=N, --expression handled
	// above) are deliberately absent so they fail closed.
}

// sedIsRead reports whether a `sed` invocation only reads/prints — never edits in
// place, writes/reads files, executes a shell, or transforms via `s`/`y`. It is
// FAIL-CLOSED: any flag not proven inert (unknown short/long option, `-i`/`--in-place`,
// `-f`/`--file`, or a script with a mutating/executing command) resolves to false
// (scan), matching the recall-critical codex-read scoping posture (#13/C-003). A
// false-allow here would wrongly exclude a real command failure, so the parser refuses
// anything it cannot model — including attached expressions (`-e1wout`) and
// argument-taking options (`-l N`) that could smuggle a write past a naive bundle scan.
func sedIsRead(rest []string) bool {
	var scripts []string
	for i := 0; i < len(rest); {
		f := rest[i]
		switch {
		case f == "-" || !strings.HasPrefix(f, "-"):
			// Bare operand: the first is the script (when none gathered via -e); any
			// later bare operands are input files.
			if len(scripts) == 0 {
				scripts = append(scripts, f)
			}
			i++
		case f == "--in-place" || strings.HasPrefix(f, "--in-place"):
			return false // edits the file in place
		case f == "--file" || strings.HasPrefix(f, "--file="):
			return false // external script we cannot inspect
		case f == "--expression":
			if i+1 >= len(rest) {
				return false
			}
			scripts = append(scripts, rest[i+1])
			i += 2
		case strings.HasPrefix(f, "--expression="):
			scripts = append(scripts, strings.TrimPrefix(f, "--expression="))
			i++
		case strings.HasPrefix(f, "--"):
			if !sedSafeLongFlags[f] {
				return false // unknown/argument-taking long flag → fail closed
			}
			i++
		default:
			// Short-flag bundle, parsed char by char so an attached expression
			// (`-e1wout`) or an argument-taking flag cannot slip through.
			bundle := []rune(f[1:])
			advanced := 1
			for j := 0; j < len(bundle); j++ {
				c := bundle[j]
				if c == 'i' || c == 'f' {
					return false // in-place edit / external script
				}
				if c == 'e' {
					// `-e` takes the rest of the bundle as its expression, or the next arg.
					if j+1 < len(bundle) {
						scripts = append(scripts, string(bundle[j+1:]))
					} else {
						if i+1 >= len(rest) {
							return false
						}
						scripts = append(scripts, rest[i+1])
						advanced = 2
					}
					break
				}
				if !sedSafeShortFlags[c] {
					return false // unknown short flag (may take an arg / mutate) → fail closed
				}
			}
			i += advanced
		}
	}
	if len(scripts) == 0 {
		return false
	}
	for _, s := range scripts {
		if !sedScriptIsPrintOnly(s) {
			return false
		}
	}
	return true
}

// sedScriptIsPrintOnly reports whether a sed script contains no command that writes,
// reads, executes, or transforms — i.e. only prints/extracts. `/regex/` addresses are
// stripped first so their contents are not read as commands; any of the mutating/
// executing/transforming command letters remaining (`s y w W r R e a c i`) → false.
func sedScriptIsPrintOnly(s string) bool {
	stripped := sedRegexAddr.ReplaceAllString(s, "/")
	for _, r := range stripped {
		switch r {
		case 's', 'y', 'w', 'W', 'r', 'R', 'e', 'a', 'c', 'i':
			return false
		}
	}
	return true
}

// shellNormalizeToken removes shell quoting and backslash escaping from a single token
// so head and option-flag matching sees what the command actually receives as argv.
// It strips `'`/`"` and unescapes `\x`→`x`. This is intentionally lossy (not a shell
// parser, C-003): it only needs to reveal read heads and dangerous option flags. Any
// residual imprecision biases toward NOT-read (over-scan), which is recall-safe.
func shellNormalizeToken(t string) string {
	var b strings.Builder
	for i := 0; i < len(t); i++ {
		c := t[i]
		switch {
		case c == '\'' || c == '"':
			// drop quote characters
		case c == '\\' && i+1 < len(t):
			i++
			b.WriteByte(t[i])
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// isEnvAssignment reports whether tok is a shell environment assignment (NAME=value)
// with a valid identifier name. A leading assignment marks the segment as not-read
// (see segmentIsRead) because env vars can inject command execution.
func isEnvAssignment(tok string) bool {
	eq := strings.IndexByte(tok, '=')
	if eq <= 0 {
		return false
	}
	for j, r := range tok[:eq] {
		switch {
		case r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
		case j > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// analyzeShellCommand splits cmd on the shell sequence/pipe operators `&&`, `||`,
// `;`, and `|` that appear OUTSIDE single/double quotes, and reports whether any
// out-of-quote write redirection (`>`/`>>`) is present and whether quotes are
// balanced. This is a conservative segmenter, NOT a shell parser (C-003): it exists
// only to bound the leading-token check, and it errs toward "not a read" on anything
// it cannot cleanly split (unbalanced quotes surface via balanced=false).
func analyzeShellCommand(cmd string) (segments []string, hasRedirect, balanced bool) {
	var b strings.Builder
	inSingle, inDouble := false, false
	flush := func() {
		segments = append(segments, b.String())
		b.Reset()
	}
	runes := []rune(cmd)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if inSingle {
			if c == '\'' {
				inSingle = false
			}
			b.WriteRune(c)
			continue
		}
		if inDouble {
			// A backslash escapes the next rune inside double quotes, so an escaped
			// quote (\") does not close the string.
			if c == '\\' && i+1 < len(runes) {
				b.WriteRune(c)
				i++
				b.WriteRune(runes[i])
				continue
			}
			if c == '"' {
				inDouble = false
			}
			b.WriteRune(c)
			continue
		}
		switch c {
		case '\'':
			inSingle = true
			b.WriteRune(c)
		case '"':
			inDouble = true
			b.WriteRune(c)
		case '>':
			hasRedirect = true
			b.WriteRune(c)
		case ';', '\n', '\r':
			// `;` and newlines are command separators; a mutating follow-on must not
			// hide behind a read head.
			flush()
		case '&':
			// `&&` (and) and a single `&` (background) both end the current command.
			if i+1 < len(runes) && runes[i+1] == '&' {
				i++
			}
			flush()
		case '|':
			if i+1 < len(runes) && runes[i+1] == '|' {
				i++
			}
			flush()
		default:
			b.WriteRune(c)
		}
	}
	flush()
	balanced = !inSingle && !inDouble
	return segments, hasRedirect, balanced
}

// codexExitCodeRE matches the codex output-envelope status line. It is anchored to a
// FULL line (`(?m)^…[ \t\r]*$`) so neither a mid-content phrase nor a line with trailing
// prose ("…code 0 is sample text") is mistaken for a real status line. A negative code
// is accepted (interrupted/timed-out calls surface as non-zero).
var codexExitCodeRE = regexp.MustCompile(`(?m)^Process exited with code (-?\d+)[ \t\r]*$`)

// parseCodexOutputEnvelope splits a codex `function_call_output` string into its
// status header (up to and including the "Process exited with code N" line), its bulk
// content (everything after the `Output:` marker line), and the exit code (FR-004).
// It returns ok=false when the envelope shape is absent, so the caller scans the raw
// output (recall-safe). It never panics on short or empty input.
//
// Invariant: the input is, by construction, a codex `function_call_output.output`
// string — codex wraps every exec result in this envelope — so the first full-line
// status + bare `Output:` marker are the real envelope boundaries, and any
// envelope-shaped lines inside the wrapped command output fall in `bulk` (after the
// real marker), never dropped. The parser is only consulted for a read-classified
// command, where dropping exit-0 bulk is the intended exclusion; a non-zero read keeps
// the header. WP04 corpus validation confirms the real envelope shape.
func parseCodexOutputEnvelope(output string) (header, bulk string, exitCode int, ok bool) {
	m := codexExitCodeRE.FindStringSubmatchIndex(output)
	if m == nil {
		return "", "", 0, false
	}
	code, err := strconv.Atoi(output[m[2]:m[3]])
	if err != nil {
		return "", "", 0, false
	}
	// header ends at the newline that terminates the status line (or EOF).
	headerEnd := len(output)
	if nl := strings.IndexByte(output[m[1]:], '\n'); nl >= 0 {
		headerEnd = m[1] + nl
	}
	header = output[:headerEnd]
	// A real envelope has an `Output:` marker line after the status line. Requiring it
	// (rather than accepting any lone status line) keeps a non-envelope body that merely
	// starts with the status phrase from being parsed as an envelope — content lives
	// only after `Output:`, so no content is ever dropped by returning ok=false here.
	bulk, hasMarker := codexBulkAfterOutput(output[headerEnd:])
	if !hasMarker {
		return "", "", 0, false
	}
	return header, bulk, code, true
}

// codexBulkAfterOutput returns everything after the first BARE `Output:` marker line,
// and whether such a line was found. The marker must occupy its own line — at a line
// start with only whitespace after it — so neither a mid-line `Output:` nor a content
// line that merely begins with `Output:` (e.g. "Output: important text") is treated as
// the marker (which would drop that line's content).
func codexBulkAfterOutput(region string) (bulk string, found bool) {
	const marker = "Output:"
	for idx := 0; ; {
		rel := strings.Index(region[idx:], marker)
		if rel < 0 {
			return "", false
		}
		pos := idx + rel
		atLineStart := pos == 0 || region[pos-1] == '\n'
		afterMarker := pos + len(marker)
		lineRest := region[afterMarker:]
		bulkStart := len(region)
		tail := lineRest
		if nl := strings.IndexByte(lineRest, '\n'); nl >= 0 {
			tail = lineRest[:nl]
			bulkStart = afterMarker + nl + 1
		}
		if atLineStart && strings.TrimSpace(tail) == "" {
			return region[bulkStart:], true
		}
		idx = afterMarker
	}
}
