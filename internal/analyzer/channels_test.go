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
			name: "CodexTaskComplete_excluded",
			obj: map[string]any{
				"payload": map[string]any{
					"type":               "task_complete",
					"last_agent_message": "SIG_CODEXTASKDONE review failed: verdict: rejected",
				},
			},
			sig:  "SIG_CODEXTASKDONE",
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
			out := outputText(tc.obj)
			diag := diagnosticText(tc.obj)

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

	out := outputText(obj)
	diag := diagnosticText(obj)

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

	out := outputText(obj)
	diag := diagnosticText(obj)

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

	firstOut := outputText(obj)
	firstDiag := diagnosticText(obj)
	for i := 0; i < 5; i++ {
		if got := outputText(obj); got != firstOut {
			t.Fatalf("outputText not deterministic: %q != %q", got, firstOut)
		}
		if got := diagnosticText(obj); got != firstDiag {
			t.Fatalf("diagnosticText not deterministic: %q != %q", got, firstDiag)
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logged := captureStderr(t, func() {
				if out := outputText(tc.obj); out != "" {
					t.Errorf("outputText = %q, want empty (excluded)", out)
				}
				if diag := diagnosticText(tc.obj); diag != "" {
					t.Errorf("diagnosticText = %q, want empty (excluded)", diag)
				}
			})
			if !strings.Contains(logged, "unmapped event shape") {
				t.Errorf("expected schema-drift log on stderr, got %q", logged)
			}
			if !strings.Contains(logged, "codex payload.type=") {
				t.Errorf("expected codex payload.type detail in log, got %q", logged)
			}
		})
	}
}

// Codex payload types that are now MAPPED (agent_message → narrative;
// token_count/task_complete → excluded metadata) must NOT emit the unmapped-shape
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
				_ = outputText(tc.obj)
				_ = diagnosticText(tc.obj)
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
