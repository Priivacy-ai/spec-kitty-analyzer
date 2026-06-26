package analyzer

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// Channel extraction (design issue-4 §3a/§3c).
//
// Given one parsed event object, this module returns the text that belongs to
// each channel class, with code-edit and file-read content excluded. It does
// NOT classify failures; it only produces the two strings the failure rules
// will later match against:
//
//   - outputText(obj)     — real command/tool output + structured error text.
//   - diagnosticText(obj) — output PLUS narrative (assistant/user message text,
//     codex reasoning/message). Invariant: diagnosticText ⊇ outputText.
//
// Extraction is a TYPED dispatch over the §3c per-harness schema matrix, never a
// blind flatten — that is what lets §3a exclude code edits / file reads
// absolutely (they are never part of output OR narrative). Unmapped shapes are
// logged (a matrix-growth signal) and default to excluded-from-output; they are
// never silently treated as output.

// channelText accumulates the per-channel fragments extracted from one event.
// Fragments are kept in deterministic source order so identical input yields
// identical output strings (FR-006).
type channelText struct {
	output    []string
	narrative []string
}

// outputText returns the real command/tool output and structured error text for
// one event object (the "output" channel). Returns "" for a nil object.
func outputText(obj map[string]any) string {
	ct := extractChannels(obj)
	return strings.Join(ct.output, " ")
}

// diagnosticText returns the output channel PLUS narrative (the "diagnostic"
// channel = output ∪ narrative). The output fragments are emitted first and in
// the same order as outputText, so outputText is always a prefix of
// diagnosticText — guaranteeing diagnosticText ⊇ outputText.
func diagnosticText(obj map[string]any) string {
	ct := extractChannels(obj)
	frags := make([]string, 0, len(ct.output)+len(ct.narrative))
	frags = append(frags, ct.output...)
	frags = append(frags, ct.narrative...)
	return strings.Join(frags, " ")
}

// extractChannels performs the single typed extraction pass over an event
// object, routing each known harness shape (§3c) to output / narrative /
// excluded. Structured top-level error fields are extracted independently of the
// tool-result routing, so a source-read object that also carries a top-level
// error still surfaces that error in the output channel (§7.4 ordering).
func extractChannels(obj map[string]any) channelText {
	var ct channelText
	if obj == nil {
		return ct
	}

	// Claude assistant/user message: message.content[] text → narrative;
	// embedded tool_result content blocks → output. A top-level string `message`
	// (older transcript shape, e.g. testdata/fixture/session.jsonl) is assistant/
	// user prose → narrative, so it reaches diagnosticText but never outputText.
	if msg, ok := obj["message"].(map[string]any); ok {
		extractMessageContent(msg, &ct)
	} else if s, ok := obj["message"].(string); ok {
		appendFragment(&ct.narrative, s)
	}

	// Claude tool result: toolUseResult (map or string), with §3a exclusion.
	if tur, exists := obj["toolUseResult"]; exists {
		extractToolUseResult(tur, &ct, true)
	}

	// Codex payload: typed path keyed on payload.type (not a bare key scan).
	if payload, ok := obj["payload"].(map[string]any); ok {
		extractCodexPayload(payload, &ct)
	}

	// Structured error: top-level error/exception/traceback (incl. nested
	// objects) → string leaf values → output. Independent of the routing above.
	for _, key := range []string{"error", "exception", "traceback"} {
		if v, ok := obj[key]; ok {
			collectStringLeaves(v, &ct.output)
		}
	}

	return ct
}

// extractMessageContent handles a Claude `message` object. Text content blocks
// are narrative; embedded `tool_result` content blocks are output. A plain
// string content (older transcript shape) is treated as message text →
// narrative.
func extractMessageContent(msg map[string]any, ct *channelText) {
	content, ok := msg["content"]
	if !ok {
		return
	}
	switch typed := content.(type) {
	case string:
		appendFragment(&ct.narrative, typed)
	case []any:
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			blockType, _ := block["type"].(string)
			switch blockType {
			case "text":
				if t, ok := block["text"].(string); ok {
					appendFragment(&ct.narrative, t)
				}
			case "tool_result":
				extractToolResultBlock(block, ct)
			case "tool_use", "thinking", "redacted_thinking", "image",
				"server_tool_use", "web_search_tool_result":
				// Known non-narrative-text blocks: the agent invoking a tool, its
				// private reasoning, or media. Per the §3c matrix only `text`
				// blocks are narrative output text, so these are skipped (not
				// logged — they are mapped, just not extracted).
			default:
				logUnmappedShape("message content block type=" + quote(blockType))
			}
		}
	}
}

// extractToolResultBlock handles a Claude `tool_result` content block. Per the
// §3c matrix ("block content/output"), the text may live under `content` OR
// `output`; either key is a string or an array of {type:"text", text:...}
// blocks, and either way the text is real tool output → output channel. Both
// keys are read (in fixed content-then-output order for determinism) so a block
// carrying both includes both.
func extractToolResultBlock(block map[string]any, ct *channelText) {
	for _, key := range []string{"content", "output"} {
		if v, ok := block[key]; ok {
			appendToolResultText(v, ct)
		}
	}
}

// appendToolResultText routes a tool_result block field value (string or array
// of {type:"text", text:...} blocks, or bare strings) to the output channel.
func appendToolResultText(v any, ct *channelText) {
	switch typed := v.(type) {
	case string:
		appendFragment(&ct.output, typed)
	case []any:
		for _, item := range typed {
			switch sub := item.(type) {
			case string:
				appendFragment(&ct.output, sub)
			case map[string]any:
				if t, ok := sub["text"].(string); ok {
					appendFragment(&ct.output, t)
				}
			}
		}
	}
}

// extractToolUseResult handles the top-level `toolUseResult` value, which is
// either a structured map or a bare string. When reDecode is true and a bare
// string looks like JSON ("{"-prefixed), it is decoded ONCE and re-routed
// through the map handler (bounded recursion — the re-decoded content is not
// itself re-decoded). On decode failure the raw string is treated as output.
func extractToolUseResult(v any, ct *channelText, reDecode bool) {
	switch typed := v.(type) {
	case map[string]any:
		extractToolUseResultMap(typed, ct)
	case string:
		if reDecode && strings.HasPrefix(strings.TrimSpace(typed), "{") {
			if decoded, ok := decodeJSONObject([]byte(typed)); ok {
				extractToolUseResultMap(decoded, ct)
				return
			}
		}
		appendFragment(&ct.output, typed)
	}
}

// extractToolUseResultMap applies the §3a exclusions then extracts the output
// channels of a structured tool result. §3a is absolute: a file read
// (file.content) or a code edit (newString/oldString/structuredPatch) yields
// nothing in either channel. Otherwise stdout/stderr/output strings and any
// nested error/exception/traceback are output. A tool-result map that matches
// none of these is an unmapped shape: logged for matrix growth, excluded.
func extractToolUseResultMap(result map[string]any, ct *channelText) {
	// §3a file-read exclusion (generalizes jsonLooksLikeSourceRead).
	if file, ok := result["file"].(map[string]any); ok {
		if _, hasContent := file["content"].(string); hasContent {
			return
		}
	}
	// §3a code-edit exclusion.
	if hasAnyKey(result, "newString", "oldString", "structuredPatch") {
		return
	}

	matched := false
	for _, key := range []string{"stdout", "stderr", "output"} {
		if s, ok := result[key].(string); ok {
			matched = true
			appendFragment(&ct.output, s)
		}
	}
	for _, key := range []string{"error", "exception", "traceback"} {
		if v, ok := result[key]; ok {
			matched = true
			collectStringLeaves(v, &ct.output)
		}
	}
	if !matched {
		logUnmappedShape("toolUseResult map keys=" + strings.Join(jsonKeys(result), ","))
	}
}

// extractCodexPayload handles a codex `payload` object via a typed path keyed on
// payload.type (§3c — not a bare key allowlist). function_call_output payloads
// carry real tool output; reasoning/message payloads carry narrative. Any other
// payload.type is unmapped: logged for matrix growth, excluded from output.
//
// A KNOWN payload.type whose expected field is absent — or present but yielding
// no text fragments — is schema drift: it is logged (so the matrix can grow) and
// excluded, never silently dropped. Logging fires only when the known type
// produced zero fragments; a successful extraction is not logged.
func extractCodexPayload(payload map[string]any, ct *channelText) {
	ptype, _ := payload["type"].(string)
	switch ptype {
	case "function_call_output":
		before := len(ct.output)
		if v, ok := payload["output"]; ok {
			collectStringLeaves(v, &ct.output)
		}
		if len(ct.output) == before {
			logUnmappedShape("codex payload.type=" + quote(ptype) + " keys=" + strings.Join(jsonKeys(payload), ","))
		}
	case "reasoning", "message":
		before := len(ct.narrative)
		if v, ok := payload["content"]; ok {
			collectTextLeaves(v, &ct.narrative)
		}
		if len(ct.narrative) == before {
			logUnmappedShape("codex payload.type=" + quote(ptype) + " keys=" + strings.Join(jsonKeys(payload), ","))
		}
	default:
		logUnmappedShape("codex payload.type=" + quote(ptype))
	}
}

// collectTextLeaves gathers narrative text from a codex content value: a bare
// string, an array of content blocks, or a nested object exposing a `text` or
// `content` field. Array order is preserved for determinism.
func collectTextLeaves(v any, dst *[]string) {
	switch typed := v.(type) {
	case string:
		appendFragment(dst, typed)
	case []any:
		for _, item := range typed {
			collectTextLeaves(item, dst)
		}
	case map[string]any:
		if t, ok := typed["text"].(string); ok {
			appendFragment(dst, t)
			return
		}
		if c, ok := typed["content"]; ok {
			collectTextLeaves(c, dst)
		}
	}
}

// collectStringLeaves gathers every string leaf under a value (used for output
// channels and nested structured-error objects). Map keys are visited in sorted
// order so identical input yields identical output (FR-006).
func collectStringLeaves(v any, dst *[]string) {
	switch typed := v.(type) {
	case string:
		appendFragment(dst, typed)
	case []any:
		for _, item := range typed {
			collectStringLeaves(item, dst)
		}
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for k := range typed {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			collectStringLeaves(typed[k], dst)
		}
	}
}

// appendFragment appends a non-empty fragment, applying the same per-string
// truncation flattenJSON uses so a single pathological field cannot dominate the
// matched text.
func appendFragment(dst *[]string, s string) {
	if strings.TrimSpace(s) == "" {
		return
	}
	if len(s) > maxFlattenJSONStringBytes {
		s = s[:maxFlattenJSONStringBytes] + " [truncated]"
	}
	*dst = append(*dst, s)
}

// hasAnyKey reports whether the map contains any of the given keys.
func hasAnyKey(m map[string]any, keys ...string) bool {
	for _, k := range keys {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

// quote renders a value for log detail; empty strings are made visible.
func quote(s string) string {
	if s == "" {
		return "(empty)"
	}
	return s
}

// logUnmappedShape records an event shape the §3c extraction matrix does not yet
// cover. The shape is excluded from output (never silently treated as output);
// the log line is the matrix-growth signal (design §3c/§3e). It writes to
// stderr, matching the only existing log sink in this codebase
// (cmd/spec-kitty-analyzer/main.go: fmt.Fprintln(os.Stderr, ...)).
func logUnmappedShape(detail string) {
	fmt.Fprintln(os.Stderr, "analyzer: unmapped event shape (excluded from output):", detail)
}
