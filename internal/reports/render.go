package reports

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/priivacy-ai/spec-kitty-analyzer/internal/analyzer"
)

func WriteAll(report analyzer.Report, jsonPath, markdownPath, htmlPath, pdfPath string) error {
	if jsonPath != "" {
		if err := WriteJSON(report, jsonPath); err != nil {
			return err
		}
	}
	if markdownPath != "" {
		if err := WriteMarkdown(report, markdownPath); err != nil {
			return err
		}
	}
	if htmlPath != "" {
		if err := WriteHTML(report, htmlPath); err != nil {
			return err
		}
	}
	if pdfPath != "" {
		if err := WritePDF(report, pdfPath); err != nil {
			return err
		}
	}
	return nil
}

func WriteJSON(report analyzer.Report, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, append(data, '\n'))
}

func WriteMarkdown(report analyzer.Report, path string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# Spec Kitty Analyzer Report\n\n")
	fmt.Fprintf(&b, "- Generated: `%s`\n", report.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"))
	fmt.Fprintf(&b, "- Inputs: `%d`\n", report.Summary.InputFiles)
	fmt.Fprintf(&b, "- Timeline events: `%d`\n", report.Summary.TimelineEvents)
	fmt.Fprintf(&b, "- Missions: `%d`, Ops: `%d`, Open Ops: `%d`\n", report.Summary.Missions, report.Summary.Ops, report.Summary.OpenOps)
	fmt.Fprintf(&b, "- Failures: `%d` events across `%d` modes\n\n", report.Summary.FailureEvents, report.Summary.FailureModes)

	b.WriteString("## Findings\n\n")
	if len(report.Findings) == 0 {
		b.WriteString("No deterministic failure fingerprints found.\n\n")
	} else {
		b.WriteString("| Severity | Count | ID | Title | Recovery |\n|---|---:|---|---|---|\n")
		for _, f := range report.Findings {
			fmt.Fprintf(&b, "| %s | %d | `%s` | %s | %s |\n", f.Severity, f.Count, f.ID, escapeMD(f.Title), escapeMD(f.Recovery))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Missions\n\n")
	if len(report.Missions) == 0 {
		b.WriteString("No mission-scoped events found.\n\n")
	} else {
		b.WriteString("| Mission | Type | Target | Events | Failures | Commands | Skills |\n|---|---|---|---:|---:|---|---|\n")
		for _, m := range report.Missions {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %d | %d | %s | %s |\n", m.Slug, m.MissionType, m.TargetBranch, m.EventCount, m.FailureCount, joinCode(m.SlashCommands), joinCode(m.Skills))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Ops\n\n")
	if len(report.Ops) == 0 {
		b.WriteString("No Op-scoped events found.\n\n")
	} else {
		b.WriteString("| Invocation | Status | Profile | Action | Outcome | Events | Failures |\n|---|---|---|---|---|---:|---|\n")
		for _, op := range report.Ops {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | `%s` | `%s` | %d | %s |\n", op.InvocationID, op.Status, op.ProfileID, op.Action, op.Outcome, op.EventCount, joinCode(op.FailureModes))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Timeline\n\n")
	timeline := FilteredTimeline(report)
	fmt.Fprintf(&b, "_Showing %d Spec Kitty timeline events; filtered out %d harness-only events._\n\n", len(timeline), len(report.Timeline)-len(timeline))
	b.WriteString("| Seq | Scope | Kind | Command/Skill/Failure | Source |\n|---:|---|---|---|---|\n")
	for _, e := range timeline {
		fmt.Fprintf(&b, "| %d | %s | `%s` | %s | `%s:%d` |\n", e.Seq, scopeText(e.Scope), e.Kind, escapeMD(eventSignal(e)), e.SourcePath, e.Line)
	}
	return writeFile(path, []byte(b.String()))
}

func WriteHTML(report analyzer.Report, path string) error {
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>Spec Kitty Analyzer</title>")
	b.WriteString(`<style>body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;margin:32px;color:#17202a}table{border-collapse:collapse;width:100%;font-size:13px}th,td{border:1px solid #d8dee4;padding:6px 8px;text-align:left;vertical-align:top}th{background:#f6f8fa}.pill{display:inline-block;border-radius:6px;padding:2px 6px;background:#eef2f7}.high{color:#b42318;font-weight:700}.medium{color:#9a6700;font-weight:700}.low{color:#316dca}.muted{color:#57606a}code{background:#f6f8fa;padding:1px 4px;border-radius:4px}</style>`)
	b.WriteString("</head><body>")
	b.WriteString("<h1>Spec Kitty Analyzer Report</h1>")
	fmt.Fprintf(&b, "<p class=\"muted\">Generated %s</p>", html.EscapeString(report.GeneratedAt.Format("2006-01-02T15:04:05Z07:00")))
	fmt.Fprintf(&b, "<p><span class=\"pill\">%d timeline events</span> <span class=\"pill\">%d missions</span> <span class=\"pill\">%d ops</span> <span class=\"pill\">%d failure modes</span></p>", report.Summary.TimelineEvents, report.Summary.Missions, report.Summary.Ops, report.Summary.FailureModes)

	b.WriteString("<h2>Findings</h2>")
	if len(report.Findings) == 0 {
		b.WriteString("<p>No deterministic failure fingerprints found.</p>")
	} else {
		b.WriteString("<table><thead><tr><th>Severity</th><th>Count</th><th>ID</th><th>Title</th><th>Recovery</th></tr></thead><tbody>")
		for _, f := range report.Findings {
			fmt.Fprintf(&b, "<tr><td class=\"%s\">%s</td><td>%d</td><td><code>%s</code></td><td>%s</td><td>%s</td></tr>", f.Severity, f.Severity, f.Count, html.EscapeString(f.ID), html.EscapeString(f.Title), html.EscapeString(f.Recovery))
		}
		b.WriteString("</tbody></table>")
	}

	b.WriteString("<h2>Missions</h2>")
	b.WriteString("<table><thead><tr><th>Mission</th><th>Type</th><th>Target</th><th>Events</th><th>Failures</th><th>Commands</th><th>Skills</th></tr></thead><tbody>")
	for _, m := range report.Missions {
		fmt.Fprintf(&b, "<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td>%d</td><td>%d</td><td>%s</td><td>%s</td></tr>", html.EscapeString(m.Slug), html.EscapeString(m.MissionType), html.EscapeString(m.TargetBranch), m.EventCount, m.FailureCount, htmlList(m.SlashCommands), htmlList(m.Skills))
	}
	b.WriteString("</tbody></table>")

	b.WriteString("<h2>Ops</h2>")
	b.WriteString("<table><thead><tr><th>Invocation</th><th>Status</th><th>Profile</th><th>Action</th><th>Outcome</th><th>Events</th><th>Failures</th></tr></thead><tbody>")
	for _, op := range report.Ops {
		fmt.Fprintf(&b, "<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%d</td><td>%s</td></tr>", html.EscapeString(op.InvocationID), html.EscapeString(op.Status), html.EscapeString(op.ProfileID), html.EscapeString(op.Action), html.EscapeString(op.Outcome), op.EventCount, htmlList(op.FailureModes))
	}
	b.WriteString("</tbody></table>")

	b.WriteString("<h2>Timeline</h2>")
	timeline := FilteredTimeline(report)
	fmt.Fprintf(&b, "<p class=\"muted\">Showing %d Spec Kitty timeline events; filtered out %d harness-only events.</p>", len(timeline), len(report.Timeline)-len(timeline))
	b.WriteString("<table><thead><tr><th>Seq</th><th>Scope</th><th>Kind</th><th>Signal</th><th>Source</th></tr></thead><tbody>")
	for _, e := range timeline {
		fmt.Fprintf(&b, "<tr><td>%d</td><td>%s</td><td><code>%s</code></td><td>%s</td><td><code>%s:%d</code></td></tr>", e.Seq, html.EscapeString(scopeText(e.Scope)), html.EscapeString(e.Kind), html.EscapeString(eventSignal(e)), html.EscapeString(e.SourcePath), e.Line)
	}
	b.WriteString("</tbody></table></body></html>")
	return writeFile(path, []byte(b.String()))
}

func WritePDF(report analyzer.Report, path string) error {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("Spec Kitty Analyzer Report", false)
	pdf.SetMargins(12, 14, 12)
	pdf.SetAutoPageBreak(true, 14)
	pdf.SetFooterFunc(func() {
		pdf.SetY(-10)
		pdf.SetFont("Helvetica", "", 7)
		pdf.SetTextColor(87, 96, 106)
		pdf.CellFormat(0, 5, fmt.Sprintf("Spec Kitty Analyzer Report  |  Page %d", pdf.PageNo()), "", 0, "C", false, 0, "")
	})
	pdf.AddPage()

	pdf.SetTextColor(23, 32, 42)
	pdf.SetFont("Helvetica", "B", 19)
	pdf.CellFormat(0, 9, "Spec Kitty Analyzer Report", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(87, 96, 106)
	pdf.CellFormat(0, 5, "Generated "+report.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"), "", 1, "L", false, 0, "")
	pdf.Ln(4)
	drawPDFPills(pdf, []string{
		fmt.Sprintf("%d timeline events", report.Summary.TimelineEvents),
		fmt.Sprintf("%d missions", report.Summary.Missions),
		fmt.Sprintf("%d ops", report.Summary.Ops),
		fmt.Sprintf("%d failure modes", report.Summary.FailureModes),
		fmt.Sprintf("%d skills", report.Summary.Skills),
	})
	pdf.Ln(7)

	drawPDFSection(pdf, "Findings")
	if len(report.Findings) == 0 {
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(23, 32, 42)
		pdf.MultiCell(0, 5, "No deterministic failure fingerprints found.", "", "", false)
	} else {
		rows := make([][]pdfCell, 0, len(report.Findings))
		for _, f := range report.Findings {
			rows = append(rows, []pdfCell{
				{Text: f.Severity, Style: f.Severity},
				{Text: fmt.Sprint(f.Count), Style: "normal"},
				{Text: f.ID, Style: "code"},
				{Text: f.Title, Style: "normal"},
				{Text: f.Recovery, Style: "muted"},
			})
		}
		drawPDFTable(pdf, []string{"Severity", "Count", "ID", "Title", "Recovery"}, []float64{22, 14, 31, 43, 76}, rows)
	}

	pdf.Ln(5)
	drawPDFSection(pdf, "Missions")
	if len(report.Missions) == 0 {
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(23, 32, 42)
		pdf.MultiCell(0, 5, "No mission-scoped events found.", "", "", false)
	} else {
		rows := make([][]pdfCell, 0, len(report.Missions))
		for _, m := range report.Missions {
			rows = append(rows, []pdfCell{
				{Text: m.Slug, Style: "code"},
				{Text: m.MissionType, Style: "normal"},
				{Text: m.TargetBranch, Style: "normal"},
				{Text: fmt.Sprint(m.EventCount), Style: "normal"},
				{Text: fmt.Sprint(m.FailureCount), Style: failureCountStyle(m.FailureCount)},
				{Text: pdfList(m.SlashCommands, 5), Style: "code"},
				{Text: pdfList(m.Skills, 5), Style: "code"},
			})
		}
		drawPDFTable(pdf, []string{"Mission", "Type", "Target", "Events", "Failures", "Commands", "Skills"}, []float64{50, 17, 19, 13, 14, 35, 38}, rows)
	}

	pdf.Ln(5)
	drawPDFSection(pdf, "Ops")
	if len(report.Ops) == 0 {
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(23, 32, 42)
		pdf.MultiCell(0, 5, "No Op-scoped events found.", "", "", false)
	} else {
		rows := make([][]pdfCell, 0, len(report.Ops))
		for _, op := range report.Ops {
			rows = append(rows, []pdfCell{
				{Text: op.InvocationID, Style: "code"},
				{Text: op.Status, Style: "normal"},
				{Text: op.ProfileID, Style: "normal"},
				{Text: op.Action, Style: "normal"},
				{Text: op.Outcome, Style: "normal"},
				{Text: fmt.Sprint(op.EventCount), Style: "normal"},
				{Text: pdfList(op.FailureModes, 4), Style: "code"},
			})
		}
		drawPDFTable(pdf, []string{"Invocation", "Status", "Profile", "Action", "Outcome", "Events", "Failures"}, []float64{53, 17, 29, 21, 18, 12, 36}, rows)
	}

	pdf.Ln(5)
	drawPDFSection(pdf, "Timeline")
	timeline := FilteredTimeline(report)
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(87, 96, 106)
	pdf.MultiCell(0, 4.5, fmt.Sprintf("Showing %d Spec Kitty timeline events; filtered out %d harness-only events.", len(timeline), len(report.Timeline)-len(timeline)), "", "", false)
	pdf.Ln(1)
	rows := make([][]pdfCell, 0, len(timeline))
	for _, e := range timeline {
		rows = append(rows, []pdfCell{
			{Text: fmt.Sprintf("%04d", e.Seq), Style: "muted"},
			{Text: scopeText(e.Scope), Style: "normal"},
			{Text: e.Kind, Style: "code"},
			{Text: pdfClamp(eventSignal(e), 240), Style: signalStyle(e)},
			{Text: fmt.Sprintf("%s:%d", filepath.Base(e.SourcePath), e.Line), Style: "code"},
		})
	}
	drawPDFTable(pdf, []string{"Seq", "Scope", "Kind", "Signal", "Source"}, []float64{13, 42, 24, 75, 32}, rows)
	return pdf.OutputFileAndClose(path)
}

type TimelineFilterSummary struct {
	Mode     string `json:"mode"`
	Included int    `json:"included"`
	Excluded int    `json:"excluded"`
}

func TimelineFilter(report analyzer.Report) TimelineFilterSummary {
	included := len(FilteredTimeline(report))
	return TimelineFilterSummary{
		Mode:     "spec-kitty-positive-signals",
		Included: included,
		Excluded: len(report.Timeline) - included,
	}
}

func FilteredTimeline(report analyzer.Report) []analyzer.TimelineEvent {
	out := make([]analyzer.TimelineEvent, 0, len(report.Timeline))
	for _, event := range report.Timeline {
		if IsSpecKittyTimelineEvent(event) {
			out = append(out, event)
		}
	}
	return out
}

func IsSpecKittyTimelineEvent(event analyzer.TimelineEvent) bool {
	if event.Scope.Type == "mission" || event.Scope.Type == "op" {
		return true
	}
	if len(event.SlashCommands) > 0 || len(event.CLIInvocations) > 0 || len(event.Skills) > 0 || len(event.AgentProfiles) > 0 {
		return true
	}
	if isSpecKittyToolName(event.ToolName) {
		return true
	}
	for _, failure := range event.Failures {
		if isSpecKittyFailureID(failure.ID) {
			return true
		}
	}
	return false
}

func isSpecKittyToolName(name string) bool {
	name = strings.ToLower(name)
	return strings.Contains(name, "spec-kitty") || strings.Contains(name, "spk-")
}

func isSpecKittyFailureID(id string) bool {
	switch id {
	case "branch_worktree_confusion",
		"circular_dependencies",
		"completed_not_terminal_runtime_bug",
		"config_yaml_invalid",
		"dirty_worktree_ref_advance",
		"guard_failure",
		"manifest_drift",
		"merge_conflict",
		"merge_operation_failed",
		"missing_artifact",
		"namespace_package_import",
		"no_code_commits",
		"null_prompt_step_runtime_bug",
		"ref_advance_non_fast_forward",
		"review_rejected",
		"reviewer_failed",
		"runtime_blocked",
		"runtime_not_initialized",
		"saas_sync_flag_missing",
		"skill_surface_missing",
		"stale_agent",
		"sync_auth_required",
		"sync_boundary_preflight",
		"tracker_binding_missing",
		"worktree_linkage_broken",
		"wrong_cli_surface":
		return true
	default:
		return false
	}
}

type pdfCell struct {
	Text  string
	Style string
}

func drawPDFSection(pdf *fpdf.Fpdf, title string) {
	ensurePDFSpace(pdf, 12)
	pdf.SetFillColor(246, 248, 250)
	pdf.SetDrawColor(216, 222, 228)
	pdf.SetTextColor(23, 32, 42)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(0, 8, title, "B", 1, "L", true, 0, "")
	pdf.Ln(2)
}

func drawPDFPills(pdf *fpdf.Fpdf, pills []string) {
	x := pdf.GetX()
	y := pdf.GetY()
	maxX := 198.0
	for _, pill := range pills {
		text := pdfText(pill)
		pdf.SetFont("Helvetica", "", 8.5)
		w := pdf.GetStringWidth(text) + 7
		if x+w > maxX {
			x = 12
			y += 8
		}
		pdf.SetXY(x, y)
		pdf.SetFillColor(238, 242, 247)
		pdf.SetDrawColor(216, 222, 228)
		pdf.SetTextColor(23, 32, 42)
		pdf.RoundedRect(x, y, w, 6, 1.2, "1234", "DF")
		pdf.SetXY(x+3.5, y+1.1)
		pdf.CellFormat(w-7, 4, text, "", 0, "C", false, 0, "")
		x += w + 3
	}
	pdf.SetXY(12, y+8)
}

func drawPDFTable(pdf *fpdf.Fpdf, headers []string, widths []float64, rows [][]pdfCell) {
	drawPDFTableHeader(pdf, headers, widths)
	for i, row := range rows {
		height := pdfRowHeight(pdf, widths, row)
		if ensurePDFSpace(pdf, height) {
			drawPDFTableHeader(pdf, headers, widths)
		}
		drawPDFTableRow(pdf, widths, row, i%2 == 1, height)
	}
	pdf.Ln(1.5)
}

func drawPDFTableHeader(pdf *fpdf.Fpdf, headers []string, widths []float64) {
	ensurePDFSpace(pdf, 9)
	x := pdf.GetX()
	y := pdf.GetY()
	pdf.SetFillColor(246, 248, 250)
	pdf.SetDrawColor(216, 222, 228)
	pdf.SetTextColor(23, 32, 42)
	pdf.SetFont("Helvetica", "B", 8)
	for i, header := range headers {
		pdf.Rect(x, y, widths[i], 7, "DF")
		pdf.SetXY(x+1.2, y+1.8)
		pdf.CellFormat(widths[i]-2.4, 3.5, pdfText(header), "", 0, "L", false, 0, "")
		x += widths[i]
	}
	pdf.SetXY(12, y+7)
}

func drawPDFTableRow(pdf *fpdf.Fpdf, widths []float64, row []pdfCell, shaded bool, height float64) {
	x := pdf.GetX()
	y := pdf.GetY()
	if shaded {
		pdf.SetFillColor(251, 252, 253)
	} else {
		pdf.SetFillColor(255, 255, 255)
	}
	pdf.SetDrawColor(216, 222, 228)
	for i, cell := range row {
		pdf.Rect(x, y, widths[i], height, "DF")
		pdf.SetXY(x+1.2, y+1.3)
		applyPDFCellStyle(pdf, cell.Style)
		pdf.MultiCell(widths[i]-2.4, 3.8, pdfText(cell.Text), "", "L", false)
		x += widths[i]
		pdf.SetXY(x, y)
	}
	pdf.SetXY(12, y+height)
}

func pdfRowHeight(pdf *fpdf.Fpdf, widths []float64, row []pdfCell) float64 {
	maxLines := 1
	for i, cell := range row {
		applyPDFCellStyle(pdf, cell.Style)
		lines := pdf.SplitText(pdfText(cell.Text), widths[i]-2.4)
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
	}
	height := float64(maxLines)*3.8 + 2.6
	if height < 6.4 {
		return 6.4
	}
	return height
}

func applyPDFCellStyle(pdf *fpdf.Fpdf, style string) {
	switch style {
	case "code":
		pdf.SetFont("Courier", "", 7)
		pdf.SetTextColor(23, 32, 42)
	case "high":
		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetTextColor(180, 35, 24)
	case "medium":
		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetTextColor(154, 103, 0)
	case "low":
		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetTextColor(49, 109, 202)
	case "muted":
		pdf.SetFont("Helvetica", "", 7.5)
		pdf.SetTextColor(87, 96, 106)
	default:
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(23, 32, 42)
	}
}

func ensurePDFSpace(pdf *fpdf.Fpdf, height float64) bool {
	_, pageHeight := pdf.GetPageSize()
	if pdf.GetY()+height <= pageHeight-14 {
		return false
	}
	pdf.AddPage()
	return true
}

func failureCountStyle(count int) string {
	if count > 0 {
		return "high"
	}
	return "muted"
}

func signalStyle(e analyzer.TimelineEvent) string {
	if len(e.Failures) == 0 {
		return "normal"
	}
	switch e.Failures[0].Severity {
	case "high", "medium", "low":
		return e.Failures[0].Severity
	default:
		return "medium"
	}
}

func pdfList(items []string, limit int) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) <= limit {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:limit], ", ") + fmt.Sprintf(", +%d more", len(items)-limit)
}

func pdfClamp(s string, limit int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "..."
}

func pdfText(s string) string {
	replacer := strings.NewReplacer(
		"\n", " ",
		"\r", " ",
		"\t", " ",
		"✓", "OK",
		"⚠️", "WARNING",
		"⚠", "WARNING",
		"→", "->",
		"—", "-",
		"–", "-",
		"“", `"`,
		"”", `"`,
		"‘", "'",
		"’", "'",
		"…", "...",
	)
	s = replacer.Replace(s)
	return strings.Map(func(r rune) rune {
		if r < 32 {
			return ' '
		}
		if r > 126 {
			return '?'
		}
		return r
	}, s)
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func scopeText(scope analyzer.Scope) string {
	switch scope.Type {
	case "mission":
		if scope.WorkPackage != "" {
			return "mission:" + scope.MissionSlug + "/" + scope.WorkPackage
		}
		return "mission:" + scope.MissionSlug
	case "op":
		return "op:" + scope.InvocationID
	default:
		return "outside"
	}
}

func eventSignal(e analyzer.TimelineEvent) string {
	if len(e.Failures) > 0 {
		return e.Failures[0].ID + ": " + e.Failures[0].Reason
	}
	if len(e.CLIInvocations) > 0 {
		return e.CLIInvocations[0].Raw
	}
	if len(e.SlashCommands) > 0 {
		return "/" + e.SlashCommands[0].Name
	}
	if len(e.Skills) > 0 {
		return e.Skills[0].Name
	}
	if e.ToolName != "" {
		return e.ToolName
	}
	return e.TextPreview
}

func escapeMD(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func joinCode(items []string) string {
	if len(items) == 0 {
		return ""
	}
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = "`" + escapeMD(item) + "`"
	}
	return strings.Join(out, ", ")
}

func htmlList(items []string) string {
	if len(items) == 0 {
		return ""
	}
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = "<code>" + html.EscapeString(item) + "</code>"
	}
	return strings.Join(out, ", ")
}
