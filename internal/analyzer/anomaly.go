package analyzer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Tier-3 unclassified-anomaly trap (issue #15).
//
// Tier 3 is the deliberate, deterministic counterweight to the precision work in
// #4/#11: it re-captures output/structured signals that clearly indicate a problem
// but match NO existing fingerprint (neither a Tier-1 rule nor the Tier-2
// generic_error fallback), and reports them SEGREGATED — never as a confirmed
// failure, never counted in the failure roll-up. The residual set is deliberately
// tight (research.md D1, re-derived against current code) so it cannot re-admit the
// benign-chatter false positives #4 closed:
//
//   - a STRUCTURED top-level `exit_status` with a non-zero value — the one structured
//     indicator jsonHasError does not cover, and which never reaches the output channel
//     in a Tier-2-catchable text form; read top-level-only (no recursive walk) so it
//     stays deterministic and inside the post-#13 channel-exclusion model; and
//   - the OUTPUT crash signatures `panic:`, `segmentation fault`, `core dumped` —
//     not fingerprinted and not in genericFailureSignals. (`Traceback (most recent
//     call last):` is deliberately excluded: it is already Tier-1/Tier-2.)
//
// Emission is residual-only and non-artifact — see the parseFile append sites.

const (
	kindStructuredExitStatus = "structured_exit_status"
	kindCrashPanic           = "crash_panic"
	kindCrashSegfault        = "crash_segfault"
	kindCrashCoreDumped      = "crash_core_dumped"

	channelOutput     = "output"
	channelStructured = "structured"

	// maxAnomalyEvidence bounds the per-group evidence list; Count still totals all.
	maxAnomalyEvidence = 5
	// anomalySnippetMax bounds a snippet's length.
	anomalySnippetMax = 200
)

// crashSignal pairs a residual output crash-signature regex with its anomaly kind.
type crashSignal struct {
	kind string
	re   *regexp.Regexp
}

var crashSignals = []crashSignal{
	{kindCrashPanic, regexp.MustCompile(`(?i)panic:`)},
	{kindCrashSegfault, regexp.MustCompile(`(?i)segmentation fault`)},
	{kindCrashCoreDumped, regexp.MustCompile(`(?i)core dumped`)},
}

// anomalyCandidate is the in-memory Tier-3 signal detected for one event, stashed
// on TimelineEvent and consumed by buildAnomalies. Never serialized.
type anomalyCandidate struct {
	kind    string // structured_exit_status | crash_panic | crash_segfault | crash_core_dumped
	channel string // output | structured
	token   string // FULL matched signal — the hash input (grouping key), never truncated
	snippet string // bounded excerpt for report evidence only (never hashed)
}

// detectAnomalies returns zero or more residual anomaly candidates for one event.
// An event may carry several (e.g. a non-zero exit_status AND a panic — M2).
//
//   - obj is the decoded top-level event object (nil for a plain-text line); the
//     structured read is TOP-LEVEL ONLY (obj["exit_status"]), never a recursive walk
//     (research.md H3) — so it is deterministic and cannot reach into content the
//     post-#13 channel model excluded.
//   - outputCh is the already-extracted output channel string (narrative, codex-read,
//     and file/edit content have been excluded upstream), scanned for crash signatures.
func detectAnomalies(obj map[string]any, outputCh string) []anomalyCandidate {
	var out []anomalyCandidate

	// Structured: a top-level non-zero exit_status (the residual structured indicator).
	if obj != nil {
		if v, ok := obj["exit_status"]; ok {
			if n, ok := asNumber(v); ok && n != 0 {
				sig := fmt.Sprintf("exit_status=%d", int64(n))
				out = append(out, anomalyCandidate{
					kind:    kindStructuredExitStatus,
					channel: channelStructured,
					token:   sig,
					snippet: sig,
				})
			}
		}
	}

	// Output crash signatures: at most one candidate per kind, snippet = first match line.
	if strings.TrimSpace(outputCh) != "" {
		for _, sig := range crashSignals {
			if line, ok := firstMatchingLine(outputCh, sig.re); ok {
				out = append(out, anomalyCandidate{
					kind:    sig.kind,
					channel: channelOutput,
					token:   line,                             // full matched line → hash input
					snippet: preview(line, anomalySnippetMax), // bounded → evidence only
				})
			}
		}
	}

	return out
}

// asNumber coerces a decoded JSON scalar to a float64 (encoding/json decodes numbers
// as float64). Non-numeric values return ok=false.
func asNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

// firstMatchingLine returns the first line of s matching re (trimmed), or ok=false.
func firstMatchingLine(s string, re *regexp.Regexp) (string, bool) {
	for _, line := range strings.Split(s, "\n") {
		if re.MatchString(line) {
			return strings.TrimSpace(line), true
		}
	}
	return "", false
}

var (
	// digitRun collapses to a single placeholder so a shape groups regardless of
	// incidental numbers (e.g. panic index [5] vs [9]).
	digitRun = regexp.MustCompile(`\d+`)
	// pathOrHexRun collapses path-like and long hex/id runs to a placeholder.
	pathOrHexRun = regexp.MustCompile(`(?:[/\\][^\s]+)|(?:\b[0-9a-fA-F]{8,}\b)`)
)

// normalizeToken canonicalizes a snippet so incidental variation collapses: it is
// lowercased, path/hex runs become a placeholder, and digit runs become '#'. This is
// the token component of the signature hash.
func normalizeToken(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = pathOrHexRun.ReplaceAllString(s, "§")
	s = digitRun.ReplaceAllString(s, "#")
	return s
}

// signatureHash is the full 64-char sha256 hex over (channel, tool, kind,
// normalizedToken). It is the group key AND the ignore-registry key — no truncation,
// so a collision cannot suppress an unrelated anomaly (research.md M3, FR-005).
func signatureHash(channel, tool, kind, token string) string {
	sum := sha256.Sum256([]byte(channel + "\x00" + tool + "\x00" + kind + "\x00" + normalizeToken(token)))
	return hex.EncodeToString(sum[:])
}

// ignoredAnomalySignatures is the checked-in ignore registry: signature hash → the
// human reason it is confirmed benign. It starts empty. This is the "ignore" arm of
// the promote → refine → ignore self-improvement loop (FR-006): a maintainer pastes a
// report's full `signature_hash` here to suppress it on the next run. Richer triage
// tooling (dashboards, promotion automation) is out of scope for v1 (C-007).
var ignoredAnomalySignatures = map[string]string{}

func isIgnoredSignature(hash string) bool {
	_, ok := ignoredAnomalySignatures[hash]
	return ok
}

// anomalyTitle is the human label for an anomaly group.
func anomalyTitle(kind string) string {
	switch kind {
	case kindStructuredExitStatus:
		return "Unclassified anomaly: non-zero structured exit_status"
	case kindCrashPanic:
		return "Unclassified anomaly: panic in command output"
	case kindCrashSegfault:
		return "Unclassified anomaly: segmentation fault in command output"
	case kindCrashCoreDumped:
		return "Unclassified anomaly: core dumped in command output"
	default:
		return "Unclassified anomaly"
	}
}

// buildAnomalies aggregates the stashed per-event candidates into segregated,
// deterministically-ordered anomaly groups: keyed by signature hash, ignored hashes
// dropped, with count + first/last occurrence and a bounded evidence list. It never
// touches Findings or the failure roll-up (segregation — INV-1). Output ordering is
// stable — sorted by (signature_hash, first_seq) — with evidence sorted by seq (NFR-002).
func buildAnomalies(events []TimelineEvent) []Anomaly {
	groups := map[string]*Anomaly{}
	// allEvidence holds every occurrence per group; capped only AFTER a seq sort so
	// the retained evidence is the lowest-seq subset regardless of event order.
	allEvidence := map[string][]AnomalyEvidence{}
	var order []string
	for _, ev := range events {
		for _, c := range ev.anomalyCandidates {
			// Hash the FULL token (never the bounded snippet) so genuinely-different
			// long lines never collide into one signature/ignore key.
			hash := signatureHash(c.channel, ev.ToolName, c.kind, c.token)
			if isIgnoredSignature(hash) {
				continue
			}
			g, ok := groups[hash]
			if !ok {
				g = &Anomaly{
					SignatureHash: hash,
					Kind:          c.kind,
					Channel:       c.channel,
					Title:         anomalyTitle(c.kind),
					FirstSeq:      ev.Seq,
					LastSeq:       ev.Seq,
				}
				groups[hash] = g
				order = append(order, hash)
			}
			g.Count++
			if ev.Seq < g.FirstSeq {
				g.FirstSeq = ev.Seq
			}
			if ev.Seq > g.LastSeq {
				g.LastSeq = ev.Seq
			}
			allEvidence[hash] = append(allEvidence[hash], AnomalyEvidence{
				Seq:        ev.Seq,
				SourcePath: ev.SourcePath,
				Line:       ev.Line,
				Snippet:    c.snippet,
			})
		}
	}

	out := make([]Anomaly, 0, len(order))
	for _, h := range order {
		g := groups[h]
		ev := allEvidence[h]
		sort.SliceStable(ev, func(i, j int) bool { return ev[i].Seq < ev[j].Seq })
		if len(ev) > maxAnomalyEvidence {
			ev = ev[:maxAnomalyEvidence] // keep the lowest-seq occurrences
		}
		g.Evidence = ev
		out = append(out, *g)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SignatureHash != out[j].SignatureHash {
			return out[i].SignatureHash < out[j].SignatureHash
		}
		return out[i].FirstSeq < out[j].FirstSeq
	})
	return out
}
