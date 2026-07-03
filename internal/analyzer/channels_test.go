package analyzer

import (
	"io"
	"os"
	"strings"
	"testing"
)

// channelExpectation declares where a signature string must land for one event
// shape, per the §3c per-harness schema matrix.
type channelExpectation int

const (
	expectOutput    channelExpectation = iota // in outputText (and diagnosticText)
	expectNarrative                           // in diagnosticText only, NOT outputText
	expectNeither                             // in neither (excluded)
)

// TestChannelExtractionMatrix is the golden table proving every known harness
// shape routes to output / narrative / excluded exactly as the §3c matrix and
// the channel-classification contract dictate (Contract A).
func TestChannelExtractionMatrix(t *testing.T) {
	cases := []struct {
		name string
		obj  map[string]any
		sig  string
		want channelExpectation
	}{
		{
			name: "ClaudeMessageText_narrative",
			obj: map[string]any{
				"message": map[string]any{
					"role": "assistant",
					"content": []any{
						map[string]any{"type": "text", "text": "SIG_MSG catch the AssertionError and log it"},
					},
				},
			},
			sig:  "SIG_MSG",
			want: expectNarrative,
		},
		{
			name: "ClaudeMessagePlainString_narrative",
			obj: map[string]any{
				"message": map[string]any{
					"role":    "user",
					"content": "SIG_MSGSTR please handle the error defensively",
				},
			},
			sig:  "SIG_MSGSTR",
			want: expectNarrative,
		},
		{
			name: "TopLevelStringMessage_narrative",
			obj: map[string]any{
				"message": "SIG_TOPMSG the coordination worktree points at a different main checkout than the target branch",
			},
			sig:  "SIG_TOPMSG",
			want: expectNarrative,
		},
		{
			name: "ToolUseResultStdout_output",
			obj: map[string]any{
				"toolUseResult": map[string]any{"stdout": "SIG_STDOUT all good", "stderr": ""},
			},
			sig:  "SIG_STDOUT",
			want: expectOutput,
		},
		{
			name: "ToolUseResultStderr_output",
			obj: map[string]any{
				"toolUseResult": map[string]any{"stderr": "SIG_STDERR E AssertionError: boom"},
			},
			sig:  "SIG_STDERR",
			want: expectOutput,
		},
		{
			name: "ToolUseResultBareString_output",
			obj: map[string]any{
				"toolUseResult": "SIG_BARE raw command output line",
			},
			sig:  "SIG_BARE",
			want: expectOutput,
		},
		{
			name: "ToolUseResultJSONString_output",
			obj: map[string]any{
				"toolUseResult": `{"stdout":"SIG_JSONSTR decoded output","stderr":"","interrupted":false}`,
			},
			sig:  "SIG_JSONSTR",
			want: expectOutput,
		},
		{
			name: "ClaudeToolResultBlockString_output",
			obj: map[string]any{
				"message": map[string]any{
					"role": "user",
					"content": []any{
						map[string]any{"type": "tool_result", "content": "SIG_TOOLRESULT stderr boom"},
					},
				},
			},
			sig:  "SIG_TOOLRESULT",
			want: expectOutput,
		},
		{
			name: "ClaudeToolResultBlockArray_output",
			obj: map[string]any{
				"message": map[string]any{
					"role": "user",
					"content": []any{
						map[string]any{
							"type": "tool_result",
							"content": []any{
								map[string]any{"type": "text", "text": "SIG_TOOLRESULTARR exit status 1"},
							},
						},
					},
				},
			},
			sig:  "SIG_TOOLRESULTARR",
			want: expectOutput,
		},
		{
			name: "ClaudeEditWrite_excluded",
			obj: map[string]any{
				"toolUseResult": map[string]any{
					"filePath":  "/repo/x.py",
					"oldString": "pass",
					"newString": "raise AssertionError('SIG_EDIT boom')",
				},
			},
			sig:  "SIG_EDIT",
			want: expectNeither,
		},
		{
			name: "ClaudeEditStructuredPatch_excluded",
			obj: map[string]any{
				"toolUseResult": map[string]any{
					"filePath": "/repo/x.py",
					"structuredPatch": []any{
						map[string]any{"lines": []any{"+raise AssertionError('SIG_PATCH')"}},
					},
				},
			},
			sig:  "SIG_PATCH",
			want: expectNeither,
		},
		{
			name: "ClaudeRead_excluded",
			obj: map[string]any{
				"toolUseResult": map[string]any{
					"file": map[string]any{
						"filePath": "/repo/x.py",
						"content":  "SIG_READ raise AssertionError('boom')",
					},
				},
			},
			sig:  "SIG_READ",
			want: expectNeither,
		},
		{
			name: "ClaudeToolResultBlockOutputString_output",
			obj: map[string]any{
				"message": map[string]any{
					"role": "user",
					"content": []any{
						map[string]any{"type": "tool_result", "output": "SIG_TOOLRESULTOUT exit status 2"},
					},
				},
			},
			sig:  "SIG_TOOLRESULTOUT",
			want: expectOutput,
		},
		{
			name: "ClaudeToolResultBlockOutputArray_output",
			obj: map[string]any{
				"message": map[string]any{
					"role": "user",
					"content": []any{
						map[string]any{
							"type": "tool_result",
							"output": []any{
								map[string]any{"type": "text", "text": "SIG_TOOLRESULTOUTARR command not found"},
							},
						},
					},
				},
			},
			sig:  "SIG_TOOLRESULTOUTARR",
			want: expectOutput,
		},
		{
			name: "CodexFunctionCallOutput_output",
			obj: map[string]any{
				"payload": map[string]any{
					"type":   "function_call_output",
					"output": "SIG_CODEXOUT command failed with exit status 1",
				},
			},
			sig:  "SIG_CODEXOUT",
			want: expectOutput,
		},
		{
			name: "CodexReasoning_narrative",
			obj: map[string]any{
				"payload": map[string]any{
					"type": "reasoning",
					"content": []any{
						map[string]any{"type": "reasoning_text", "text": "SIG_CODEXREASON handle AssertionError defensively"},
					},
				},
			},
			sig:  "SIG_CODEXREASON",
			want: expectNarrative,
		},
		{
			name: "CodexMessage_narrative",
			obj: map[string]any{
				"payload": map[string]any{
					"type":    "message",
					"content": "SIG_CODEXMSG I will now discuss the merge failed scenario",
				},
			},
			sig:  "SIG_CODEXMSG",
			want: expectNarrative,
		},
		{
			name: "CodexAgentMessage_narrative",
			obj: map[string]any{
				"payload": map[string]any{
					"type":    "agent_message",
					"message": "SIG_CODEXAGENT reviewing the merge failed scenario before fixing",
				},
			},
			sig:  "SIG_CODEXAGENT",
			want: expectNarrative,
		},
		{
			name: "CodexTokenCount_excluded",
			obj: map[string]any{
				"payload": map[string]any{
					"type": "token_count",
					"info": map[string]any{
						"note": "SIG_CODEXTOKENS exit code 1 traceback rejected",
					},
				},
			},
			sig:  "SIG_CODEXTOKENS",
			want: expectNeither,
		},
		{
			name: "CodexTaskComplete_narrative",
			obj: map[string]any{
				"payload": map[string]any{
					"type":               "task_complete",
					"last_agent_message": "SIG_CODEXTASKDONE summary of the completed turn",
				},
			},
			sig:  "SIG_CODEXTASKDONE",
			want: expectNarrative,
		},
		{
			// review #4 guard: a reasoning/message payload that ALSO carries a stray
			// top-level `message` string must NOT have that string read — only
			// agent_message keys its text under payload.message. The content text lands
			// in narrative; SIG_STRAYMSG (in the message field) must reach NEITHER channel.
			name: "CodexMessageStrayMessageField_excluded",
			obj: map[string]any{
				"payload": map[string]any{
					"type":    "message",
					"content": "discussing the plan",
					"message": "SIG_STRAYMSG must not be read for a message-type payload",
				},
			},
			sig:  "SIG_STRAYMSG",
			want: expectNeither,
		},
		{
			name: "TopLevelError_output",
			obj: map[string]any{
				"error": "SIG_TOPERR something blew up",
			},
			sig:  "SIG_TOPERR",
			want: expectOutput,
		},
		{
			name: "NestedStructuredError_output",
			obj: map[string]any{
				"exception": map[string]any{
					"type":    "RuntimeError",
					"message": "SIG_NESTEDERR boom in subprocess",
				},
			},
			sig:  "SIG_NESTEDERR",
			want: expectOutput,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, diag := channelTextPair(tc.obj)

			// Invariant: diagnosticText ⊇ outputText (output is a prefix).
			if !strings.HasPrefix(diag, out) {
				t.Fatalf("diagnosticText must contain outputText as a prefix\n out=%q\ndiag=%q", out, diag)
			}

			inOutput := strings.Contains(out, tc.sig)
			inDiag := strings.Contains(diag, tc.sig)

			switch tc.want {
			case expectOutput:
				if !inOutput {
					t.Errorf("signature %q expected in outputText, got %q", tc.sig, out)
				}
				if !inDiag {
					t.Errorf("signature %q expected in diagnosticText, got %q", tc.sig, diag)
				}
			case expectNarrative:
				if inOutput {
					t.Errorf("signature %q must NOT be in outputText (narrative-only), got %q", tc.sig, out)
				}
				if !inDiag {
					t.Errorf("signature %q expected in diagnosticText, got %q", tc.sig, diag)
				}
			case expectNeither:
				if inOutput {
					t.Errorf("signature %q must be excluded from outputText, got %q", tc.sig, out)
				}
				if inDiag {
					t.Errorf("signature %q must be excluded from diagnosticText, got %q", tc.sig, diag)
				}
			}
		})
	}
}

// TestChannelStructuralVsTextOrdering pins the §7.4 precedence WP03 relies on: an
// object whose toolUseResult is a source read (excluded) but which ALSO carries a
// top-level structured error must surface the error in the output channel while
// the file content stays excluded.
func TestChannelStructuralVsTextOrdering(t *testing.T) {
	obj := map[string]any{
		"toolUseResult": map[string]any{
			"file": map[string]any{
				"filePath": "/repo/x.py",
				"content":  "SIG_FILECONTENT raise AssertionError('boom')",
			},
		},
		"error": "SIG_TOPLEVELERR command exited with status 1",
	}

	out, diag := channelTextPair(obj)

	if !strings.Contains(out, "SIG_TOPLEVELERR") {
		t.Errorf("top-level error must appear in outputText, got %q", out)
	}
	if strings.Contains(out, "SIG_FILECONTENT") {
		t.Errorf("source-read file content must be excluded from outputText, got %q", out)
	}
	if strings.Contains(diag, "SIG_FILECONTENT") {
		t.Errorf("source-read file content must be excluded from diagnosticText, got %q", diag)
	}
	if !strings.HasPrefix(diag, out) {
		t.Fatalf("diagnosticText must contain outputText as a prefix\n out=%q\ndiag=%q", out, diag)
	}
}

// TestChannelReadEditExclusionPreservesSiblingOutput catches mixed tool-result
// objects: file/edit payloads are excluded, but sibling stderr/error fields are
// still real output and must remain visible to failure classification.
func TestChannelReadEditExclusionPreservesSiblingOutput(t *testing.T) {
	obj := map[string]any{
		"toolUseResult": map[string]any{
			"file": map[string]any{
				"filePath": "/repo/x.py",
				"content":  "SIG_FILECONTENT raise AssertionError('boom')",
			},
			"newString": "SIG_EDIT raise AssertionError('edit')",
			"stderr":    "SIG_STDERR E AssertionError: boom",
			"error":     map[string]any{"message": "SIG_ERROR command failed"},
		},
	}

	out, diag := channelTextPair(obj)

	for _, sig := range []string{"SIG_STDERR", "SIG_ERROR"} {
		if !strings.Contains(out, sig) {
			t.Errorf("%s must remain in outputText for mixed tool result, got %q", sig, out)
		}
		if !strings.Contains(diag, sig) {
			t.Errorf("%s must remain in diagnosticText for mixed tool result, got %q", sig, diag)
		}
	}
	for _, sig := range []string{"SIG_FILECONTENT", "SIG_EDIT"} {
		if strings.Contains(out, sig) || strings.Contains(diag, sig) {
			t.Errorf("%s must remain excluded from both channels; out=%q diag=%q", sig, out, diag)
		}
	}
}

// TestChannelDeterminism confirms identical input yields identical channel
// strings across repeated extraction, including for an object whose nested
// structured error has multiple string leaves (sorted-key traversal, FR-006).
func TestChannelDeterminism(t *testing.T) {
	obj := map[string]any{
		"toolUseResult": map[string]any{
			"stdout": "alpha line",
			"stderr": "beta line",
		},
		"error": map[string]any{
			"code":    "E_ONE",
			"message": "gamma failure",
			"detail":  "delta context",
		},
	}

	firstOut, firstDiag := channelTextPair(obj)
	for i := 0; i < 5; i++ {
		gotOut, gotDiag := channelTextPair(obj)
		if gotOut != firstOut {
			t.Fatalf("outputText not deterministic: %q != %q", gotOut, firstOut)
		}
		if gotDiag != firstDiag {
			t.Fatalf("diagnosticText not deterministic: %q != %q", gotDiag, firstDiag)
		}
	}
}

// TestChannelNilObject confirms a nil object yields empty channel strings (the
// obj==nil plain-text routing is the caller's responsibility, design §3d).
func TestChannelNilObject(t *testing.T) {
	if out := outputText(nil); out != "" {
		t.Errorf("outputText(nil) = %q, want empty", out)
	}
	if diag := diagnosticText(nil); diag != "" {
		t.Errorf("diagnosticText(nil) = %q, want empty", diag)
	}
}

// TestTopLevelStringMessageNeverOutput pins the holistic recall fix: a pure
// top-level string `message` event (the testdata/fixture/session.jsonl shape) is
// narrative — its text reaches diagnosticText but the output channel stays empty
// (a string message is never command/tool output).
func TestTopLevelStringMessageNeverOutput(t *testing.T) {
	obj := map[string]any{
		"message": "Run /spec-kitty.specify for sample-mission",
	}
	if out := outputText(obj); out != "" {
		t.Errorf("outputText = %q, want empty (string message is narrative, never output)", out)
	}
	diag := diagnosticText(obj)
	if !strings.Contains(diag, "Run /spec-kitty.specify for sample-mission") {
		t.Errorf("diagnosticText must contain the narrative string message, got %q", diag)
	}
}

// TestCodexKnownTypeMissingFieldLogsAndExcludes pins §3c schema-drift handling: a
// KNOWN codex payload.type whose expected field is absent (or yields no text) is
// logged on stderr (matrix-growth signal) and excluded from BOTH channels — it is
// never silently dropped and never leaks into output. Stderr is captured to prove
// the logged-and-excluded path is exercised, not just the absence of leakage.
func TestCodexKnownTypeMissingFieldLogsAndExcludes(t *testing.T) {
	cases := []struct {
		name string
		obj  map[string]any
	}{
		{
			name: "function_call_output missing output",
			obj: map[string]any{
				"payload": map[string]any{
					"type":   "function_call_output",
					"callId": "call_42",
				},
			},
		},
		{
			name: "reasoning empty content",
			obj: map[string]any{
				"payload": map[string]any{
					"type":    "reasoning",
					"content": []any{},
				},
			},
		},
		{
			name: "agent_message missing message",
			obj: map[string]any{
				"payload": map[string]any{
					"type":   "agent_message",
					"phase":  "commentary",
					"callId": "x",
				},
			},
		},
		{
			name: "task_complete missing last_agent_message",
			obj: map[string]any{
				"payload": map[string]any{
					"type":    "task_complete",
					"turn_id": "abc",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logged := captureStderr(t, func() {
				out, diag := channelTextPair(tc.obj)
				if out != "" {
					t.Errorf("outputText = %q, want empty (excluded)", out)
				}
				if diag != "" {
					t.Errorf("diagnosticText = %q, want empty (excluded)", diag)
				}
			})
			if !strings.Contains(logged, "unmapped event shape") {
				t.Errorf("expected schema-drift log on stderr, got %q", logged)
			}
			if !strings.Contains(logged, "codex payload.type=") {
				t.Errorf("expected codex payload.type detail in log, got %q", logged)
			}
			if count := strings.Count(logged, "unmapped event shape"); count != 1 {
				t.Errorf("expected exactly one schema-drift log from paired extraction, got %d: %q", count, logged)
			}
		})
	}
}

// Codex payload types that are now MAPPED (agent_message and task_complete →
// narrative; token_count → excluded metadata) must NOT emit the unmapped-shape
// matrix-growth log — that is the noise the §3c mapping removes. Pinning silence
// guards against a regression that re-floods stderr for these known types.
func TestCodexMappedTypesNotLogged(t *testing.T) {
	cases := []struct {
		name string
		obj  map[string]any
	}{
		{
			name: "agent_message",
			obj: map[string]any{
				"payload": map[string]any{"type": "agent_message", "message": "narrative prose"},
			},
		},
		{
			name: "token_count",
			obj: map[string]any{
				"payload": map[string]any{"type": "token_count", "info": map[string]any{"total_tokens": 10}},
			},
		},
		{
			name: "task_complete",
			obj: map[string]any{
				"payload": map[string]any{"type": "task_complete", "last_agent_message": "done"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logged := captureStderr(t, func() {
				_, _ = channelTextPair(tc.obj)
			})
			if strings.Contains(logged, "unmapped event shape") {
				t.Errorf("mapped codex type %q should not log unmapped-shape, got %q", tc.name, logged)
			}
		})
	}
}

// captureStderr redirects os.Stderr for the duration of fn and returns what was
// written. Used to assert the logUnmappedShape (stderr) path is exercised.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(data)
}

// codexOutputObj builds a codex function_call_output event object, optionally with a
// correlation id (call_id).
func codexOutputObj(callID, output string) map[string]any {
	p := map[string]any{"type": "function_call_output", "output": output}
	if callID != "" {
		p["call_id"] = callID
	}
	return map[string]any{"payload": p}
}

// ctxWith builds a channelContext registry from the given calls (keyed by callID).
func ctxWith(calls ...codexCall) channelContext {
	m := make(map[string]codexCall, len(calls))
	for _, c := range calls {
		m[c.callID] = c
	}
	return channelContext{codexCalls: m}
}

// TestCodexReadOutputGating is the golden channel-matrix for the read-output gating
// (contracts/channel-matrix.md rows 1–7): a read command's output is excluded from BOTH
// channels envelope-aware, while a real/unknown command is scanned (recall-safe).
func TestCodexReadOutputGating(t *testing.T) {
	readGitDiff := codexCall{callID: "c1", name: "exec_command", cmd: "git diff", isRead: true}
	readCat := codexCall{callID: "c2", name: "exec_command", cmd: "cat missing", isRead: true}
	realBuild := codexCall{callID: "c3", name: "exec_command", cmd: "go build ./...", isRead: false}
	compound := codexCall{callID: "c4", name: "exec_command", cmd: "git diff && go build", isRead: false}
	readPipe := codexCall{callID: "c5", name: "exec_command", cmd: "rg foo | head", isRead: true}
	readShow := codexCall{callID: "c7", name: "exec_command", cmd: "git show", isRead: true}
	ctx := ctxWith(readGitDiff, readCat, realBuild, compound, readPipe, readShow)

	// Row 1: read, exit 0, diff content with failure-like tokens → excluded (both channels).
	out, diag := channelTextPairCtx(codexOutputObj("c1", "Process exited with code 0\nOutput:\ndiff --git a b\n+SIG_R1 error exit code 2\n"), ctx)
	if out != "" || diag != "" {
		t.Errorf("row1 read exit-0 must be excluded; out=%q diag=%q", out, diag)
	}

	// Row 2: read, exit 1 → keep the status header only, drop the bulk.
	out, _ = channelTextPairCtx(codexOutputObj("c2", "Process exited with code 1\nOutput:\nNo such file SIG_R2BULK\n"), ctx)
	if !strings.Contains(out, "Process exited with code 1") {
		t.Errorf("row2 must keep status header; out=%q", out)
	}
	if strings.Contains(out, "SIG_R2BULK") {
		t.Errorf("row2 must drop bulk content; out=%q", out)
	}

	// Row 3: real command, exit 1 → scanned in full.
	out, _ = channelTextPairCtx(codexOutputObj("c3", "Process exited with code 1\nOutput:\nSIG_R3 build failed\n"), ctx)
	if !strings.Contains(out, "SIG_R3") {
		t.Errorf("row3 real command must be scanned; out=%q", out)
	}

	// Row 4: compound (not all-read) → scanned.
	out, _ = channelTextPairCtx(codexOutputObj("c4", "Process exited with code 1\nOutput:\nSIG_R4 build failed\n"), ctx)
	if !strings.Contains(out, "SIG_R4") {
		t.Errorf("row4 compound must be scanned; out=%q", out)
	}

	// Row 5: all-read pipeline → excluded.
	out, diag = channelTextPairCtx(codexOutputObj("c5", "Process exited with code 0\nOutput:\nSIG_R5 match error\n"), ctx)
	if out != "" || diag != "" {
		t.Errorf("row5 read pipeline must be excluded; out=%q diag=%q", out, diag)
	}

	// Row 6: no matching call_id → scanned (unknown intent).
	out, _ = channelTextPairCtx(codexOutputObj("nomatch", "Process exited with code 0\nOutput:\nSIG_R6 exit code 2\n"), ctx)
	if !strings.Contains(out, "SIG_R6") {
		t.Errorf("row6 unknown id must be scanned; out=%q", out)
	}

	// Row 7: camelCase callId, read, exit 0 → excluded.
	obj7 := map[string]any{"payload": map[string]any{
		"type": "function_call_output", "callId": "c7",
		"output": "Process exited with code 0\nOutput:\nSIG_R7 error\n",
	}}
	out, diag = channelTextPairCtx(obj7, ctx)
	if out != "" || diag != "" {
		t.Errorf("row7 callId read exit-0 must be excluded; out=%q diag=%q", out, diag)
	}

	// FR-005 fallback: a read-correlated output that is NOT a string can't be envelope-
	// parsed → scan it (collectStringLeaves), never silently exclude.
	out, _ = channelTextPairCtx(map[string]any{"payload": map[string]any{
		"type": "function_call_output", "call_id": "c1",
		"output": map[string]any{"note": "SIG_NONSTR scanned"},
	}}, ctx)
	if !strings.Contains(out, "SIG_NONSTR") {
		t.Errorf("non-string read output must be scanned; out=%q", out)
	}

	// FR-005 fallback: a read-correlated output whose envelope is unparseable (no status
	// line) → scan the raw output (recall-safe).
	out, _ = channelTextPairCtx(codexOutputObj("c1", "SIG_UNPARSE no envelope here exit code 2"), ctx)
	if !strings.Contains(out, "SIG_UNPARSE") {
		t.Errorf("unparseable read envelope must be scanned; out=%q", out)
	}

	// Back-compat: the stateless entrypoint (no registry) scans a function_call_output.
	out, _ = channelTextPair(codexOutputObj("c1", "Process exited with code 0\nOutput:\nSIG_BC scanned\n"))
	if !strings.Contains(out, "SIG_BC") {
		t.Errorf("stateless path must scan (empty ctx); out=%q", out)
	}
}

// TestCodexPayloadMapping pins the FR-006 payload-type mapping and guards that the
// already-mapped types are unchanged (R5 regression guard).
func TestCodexPayloadMapping(t *testing.T) {
	empty := channelContext{}
	cases := []struct {
		name string
		obj  map[string]any
		sig  string
		want channelExpectation
	}{
		{
			name: "FunctionCall_excluded",
			obj:  map[string]any{"payload": map[string]any{"type": "function_call", "name": "exec_command", "arguments": `{"cmd":"git diff"}`, "call_id": "c1", "extra": "SIG_FC hidden"}},
			sig:  "SIG_FC", want: expectNeither,
		},
		{
			name: "TaskStarted_excluded",
			obj:  map[string]any{"payload": map[string]any{"type": "task_started", "info": "SIG_TS exit code 2"}},
			sig:  "SIG_TS", want: expectNeither,
		},
		{
			name: "UserMessage_narrative_content",
			obj:  map[string]any{"payload": map[string]any{"type": "user_message", "content": "SIG_UM please fix the merge failed step"}},
			sig:  "SIG_UM", want: expectNarrative,
		},
		{
			name: "UserMessage_narrative_message",
			obj:  map[string]any{"payload": map[string]any{"type": "user_message", "message": "SIG_UM2 run the build again"}},
			sig:  "SIG_UM2", want: expectNarrative,
		},
		{
			name: "EmptyType_excluded",
			obj:  map[string]any{"payload": map[string]any{"note": "SIG_ET exit code 2"}},
			sig:  "SIG_ET", want: expectNeither,
		},
		{
			name: "Reasoning_unchanged_narrative",
			obj:  map[string]any{"payload": map[string]any{"type": "reasoning", "content": []any{map[string]any{"type": "reasoning_text", "text": "SIG_RZ keep mapped"}}}},
			sig:  "SIG_RZ", want: expectNarrative,
		},
		{
			name: "AgentMessage_unchanged_narrative",
			obj:  map[string]any{"payload": map[string]any{"type": "agent_message", "message": "SIG_AM unchanged"}},
			sig:  "SIG_AM", want: expectNarrative,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, diag := channelTextPairCtx(tc.obj, empty)
			if !strings.HasPrefix(diag, out) {
				t.Fatalf("diag must contain out as prefix; out=%q diag=%q", out, diag)
			}
			inOut, inDiag := strings.Contains(out, tc.sig), strings.Contains(diag, tc.sig)
			switch tc.want {
			case expectNarrative:
				if inOut {
					t.Errorf("%s: %q must NOT be in output; out=%q", tc.name, tc.sig, out)
				}
				if !inDiag {
					t.Errorf("%s: %q expected in diagnostic; diag=%q", tc.name, tc.sig, diag)
				}
			case expectNeither:
				if inOut || inDiag {
					t.Errorf("%s: %q must be excluded; out=%q diag=%q", tc.name, tc.sig, out, diag)
				}
			}
		})
	}
}
