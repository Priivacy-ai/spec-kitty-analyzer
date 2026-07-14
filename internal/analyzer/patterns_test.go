package analyzer

import (
	"strings"
	"testing"
)

// TestClassifyCodexReadCommand pins the conservative compound classifier (FR-003):
// a command is a pure read/inspection only when EVERY operator-split segment leads
// with a read command, with no write redirection and no mutating git subcommand.
// Any uncertainty resolves to false (scan) — recall-safe.
func TestClassifyCodexReadCommand(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		want bool
	}{
		// Pure reads (expect true).
		{"exec_command", "git diff", true},
		{"exec_command", "git show HEAD", true},
		{"exec_command", "git log --oneline", true},
		{"exec_command", "git status", true},
		{"exec_command", "rg foo | head", true},
		{"exec_command", "rg foo || true", true},
		{"exec_command", "cat a b", true},
		{"exec_command", "ls -la", true},
		{"exec_command", "stat x", true},
		{"exec_command", "grep -n foo file ; wc -l file", true},
		// Non-reads (expect false — scanned).
		{"exec_command", "git diff && go build", false},
		{"exec_command", "cat x > y", false},
		{"exec_command", "cat a >> b", false},
		{"exec_command", "rg foo && rm bar", false},
		{"exec_command", "sed -i s/a/b/ f", false},
		{"exec_command", "awk '{print}' f", false},
		{"exec_command", "go build ./...", false},
		{"exec_command", "git commit -m x", false},
		{"exec_command", "git add .", false},
		{"exec_command", "git", false},
		{"exec_command", "", false},
		// Command / process substitution hides a mutating command behind a read head —
		// must NOT be classified read (recall-critical, Codex WP01 review).
		{"exec_command", "cat `rm x`", false},
		{"exec_command", "cat \"$(rm x)\"", false},
		{"exec_command", "cat $(rm x)", false},
		{"exec_command", "cat <(rm x)", false},
		// `find` is a read only without a mutating action flag (-delete/-exec/...).
		{"exec_command", "find . -name x", true},
		{"exec_command", "find . -delete", false},
		{"exec_command", "find . -exec rm {} +", false},
		{"exec_command", "find . -type f -print", true},
		// git read subcommands with a write option (--output) are not reads.
		{"exec_command", "git diff --stat", true},
		{"exec_command", "git diff --output=out", false},
		// git options that run an external driver (--ext-diff/--textconv) or inline exec
		// config (-c ...) execute commands → not reads (Codex round-8). --no-ext-diff is safe.
		{"exec_command", "git diff --ext-diff", false},
		{"exec_command", "git --ext-diff diff", false},
		{"exec_command", "git diff --textconv file", false},
		{"exec_command", "git -c diff.external=rm diff", false},
		{"exec_command", "git diff --no-ext-diff", true},
		// sed: print-only invocations are reads (#37); any in-place/write/exec/transform
		// form stays scanned. Codex's dominant file read is `sed -n 'M,Np' file`.
		{"exec_command", "sed -n '1,260p' file", true},
		{"exec_command", "sed -n '/pat/p' file", true},
		{"exec_command", "sed -n '/error/,/done/p' file", true},
		{"exec_command", "sed '260q' file", true},
		{"exec_command", "sed -n 5p file", true},
		{"exec_command", "sed -ne 'p' file", true},
		{"exec_command", "sed -n -e '1,5p' file", true},
		{"exec_command", "rg foo | head ; sed -n '1,10p' file", true},
		// mutating / writing / executing / transforming sed → not a read (scan).
		{"exec_command", "sed -i s/a/b/ f", false},
		{"exec_command", "sed -i.bak 's/a/b/' f", false},
		{"exec_command", "sed --in-place 's/a/b/' f", false},
		{"exec_command", "sed -ni 'p' f", false},
		{"exec_command", "sed 's/a/b/w out' f", false},
		{"exec_command", "sed -e 'w out.txt' f", false},
		{"exec_command", "sed '/x/w captured.txt' f", false},
		{"exec_command", "sed '1e rm -rf x' f", false},
		{"exec_command", "sed 's/a/b/' f", false},
		{"exec_command", "sed -f script.sed f", false},
		{"exec_command", "sed -n '1,10p' f > out", false},
		{"exec_command", "sed -n '1,10p' f && rm f", false},
		// Newline and single-& (background) are command separators — a mutating
		// follow-on must not be hidden behind a read head (Codex round-3 review).
		{"exec_command", "cat x\nrm y", false},
		{"exec_command", "cat x & rm y", false},
		// Dangling/leading operators are shell syntax errors (a real failure) → scan,
		// not read (Codex round-7 review).
		{"exec_command", "cat x &&", false},
		{"exec_command", "cat x ||", false},
		{"exec_command", "cat x |", false},
		{"exec_command", "&& cat x", false},
		// `false` is a failing command, not a read; only `true` is a harmless no-op.
		{"exec_command", "rg foo || false", false},
		{"exec_command", "false", false},
		// rg --pre runs an external preprocessor command → not a read (Codex round-4).
		{"exec_command", "rg --pre rm pattern .", false},
		{"exec_command", "rg --context 3 foo", true},
		// Quoted / escaped dangerous options must still be detected — the shell strips
		// the quoting before argv, so we normalize before option inspection (Codex round-5).
		{"exec_command", "git diff \"--output=out\"", false},
		{"exec_command", "rg \"--pre=rm x\" pattern .", false},
		{"exec_command", "find . \"-delete\"", false},
		{"exec_command", "find . \\-delete", false},
		// Legit quoted read arguments still classify as reads (precision retained).
		{"exec_command", "rg \"foo bar\" .", true},
		{"exec_command", "cat \"my file\"", true},
		// A leading env assignment can inject command execution (LD_PRELOAD, PATH
		// shadowing, GIT_EXTERNAL_DIFF, RIPGREP_CONFIG_PATH) even ahead of a read head,
		// and we cannot prove the value inert without a shell parser → scan (Codex round-6).
		{"exec_command", "FOO=bar cat x", false},
		{"exec_command", "FOO=bar rm x", false},
		{"exec_command", "FOO=bar", false},
		{"exec_command", "GIT_EXTERNAL_DIFF=rm git diff", false},
		{"exec_command", "LC_ALL=C grep foo file", false},
		// Escaped quote inside double quotes must not desync the segmenter (still a read).
		{"exec_command", "grep \"a\\\"b\" file", true},
		// Non-exec tool names default to scan until a read-file tool is confirmed (FR-005).
		{"read_file_unknown_tool", "", false},
	}
	for _, c := range cases {
		if got := classifyCodexReadCommand(c.name, c.cmd); got != c.want {
			t.Errorf("classifyCodexReadCommand(%q, %q) = %v, want %v", c.name, c.cmd, got, c.want)
		}
	}
}

// TestParseCodexOutputEnvelope pins the envelope parser (FR-004): a well-formed body
// yields ok=true with the exit code, a header up to and including the status line, and
// bulk = everything after the Output: marker. A body without a status line → ok=false
// (caller scans). It must never panic on short/empty input.
func TestParseCodexOutputEnvelope(t *testing.T) {
	exit0 := "Chunk ID: 1\nWall time: 0.5 seconds\nProcess exited with code 0\n" +
		"Original token count: 10\nOutput:\ndiff --git a b\n+error exit code 2\n"
	header, bulk, code, ok := parseCodexOutputEnvelope(exit0)
	if !ok || code != 0 {
		t.Fatalf("exit0: ok=%v code=%d (want true, 0)", ok, code)
	}
	if !strings.Contains(header, "Process exited with code 0") {
		t.Errorf("exit0 header missing status line: %q", header)
	}
	if strings.Contains(header, "exit code 2") {
		t.Errorf("exit0 header leaked bulk content: %q", header)
	}
	if !strings.Contains(bulk, "exit code 2") {
		t.Errorf("exit0 bulk missing content: %q", bulk)
	}

	exit1 := "Process exited with code 1\nOutput:\nNo such file\n"
	header, _, code, ok = parseCodexOutputEnvelope(exit1)
	if !ok || code != 1 {
		t.Fatalf("exit1: ok=%v code=%d (want true, 1)", ok, code)
	}
	if !strings.Contains(header, "Process exited with code 1") {
		t.Errorf("exit1 header missing status line: %q", header)
	}

	// A stray "Output:" in the preamble (before the status line) must not be mistaken
	// for the real marker; bulk is taken after the marker line that follows the status
	// line (Codex WP01 review).
	preamble := "note Output: fake\nProcess exited with code 0\nOutput:\nrealbulk\n"
	_, bulk, code, ok = parseCodexOutputEnvelope(preamble)
	if !ok || code != 0 {
		t.Fatalf("preamble: ok=%v code=%d (want true, 0)", ok, code)
	}
	if bulk != "realbulk\n" {
		t.Errorf("preamble bulk = %q, want %q", bulk, "realbulk\n")
	}

	// The marker line must be a BARE "Output:"; a content line that merely begins with
	// "Output:" is skipped, so its content is not dropped (Codex round-4 review).
	inlineFake := "Process exited with code 0\nOutput: not the marker\nOutput:\nrealbulk\n"
	_, bulk, _, ok = parseCodexOutputEnvelope(inlineFake)
	if !ok {
		t.Fatalf("inlineFake: ok=false, want true")
	}
	if bulk != "realbulk\n" {
		t.Errorf("inlineFake bulk = %q, want %q", bulk, "realbulk\n")
	}

	for _, bad := range []string{"", "no envelope here", "Output:\njust content", "Process exited",
		"raw log line mentioning Process exited with code 0 mid-sentence",
		// Status line but no Output: marker → not the envelope shape → scan raw.
		"Process exited with code 0\nOriginal token count: 5\n",
		// Status phrase with trailing text on the line → not a real status line.
		"Process exited with code 0 is sample text\nOutput:\nx\n",
		// Only a non-bare "Output:" content line → no real marker → scan raw.
		"Process exited with code 0\nOutput: inline only\n"} {
		if _, _, _, ok := parseCodexOutputEnvelope(bad); ok {
			t.Errorf("expected ok=false for malformed input %q", bad)
		}
	}
}
