package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Markdown renders outside engine locks and never inspects provider settings.
func (s ExportSnapshot) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Session: %s\n\nSession ID: %s\n\nWorking directory: %s\n\nCutoff: %s\n\n", s.Title, s.ID, s.Cwd, s.Cutoff.Format(time.RFC3339Nano))
	fmt.Fprintf(&b, "Current session settings (not historical request settings): provider %s; model %s; effort %s.\n\n", value(s.Provider), value(s.Model), value(s.Effort))
	for _, w := range s.Warnings {
		fmt.Fprintf(&b, "> Warning: %s\n\n", w)
	}
	b.WriteString("## Model requests\n\nA step is a logical request; retries are separate attempts. Input includes cache reads and writes; do not add them again. `unavailable` is not zero.\n\n")
	b.WriteString("| Step / attempt | Purpose | Provider | Model | Effort | Configuration | Outcome | Input | Output | Cached input | Cache write |\n| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, r := range s.Requests {
		in, out, cached, written := "unavailable", "unavailable", "unavailable", "unavailable"
		if r.Usage != nil {
			in, out = fmt.Sprint(r.Usage.In), fmt.Sprint(r.Usage.Out)
			if r.Usage.CacheReported {
				cached, written = fmt.Sprint(r.Usage.Cached), fmt.Sprint(r.Usage.CacheWrite)
			}
		}
		config := "unavailable"
		if r.Config != "" {
			config = fmt.Sprintf("[%s](#configuration-%s)", r.Config[:min(12, len(r.Config))], r.Config)
		}
		fmt.Fprintf(&b, "| %s / %d | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |\n", cell(value(r.Step)), r.Attempt, cell(value(r.Purpose)), cell(value(r.Provider)), cell(value(r.Model)), cell(value(r.Effort)), config, cell(r.Outcome), in, out, cached, written)
	}
	if len(s.Requests) == 0 {
		b.WriteString("\nRequest details unavailable.\n")
	}
	b.WriteString("\n## Request boundaries\n\n")
	for _, r := range s.Requests {
		fmt.Fprintf(&b, "- Request `%s`, step `%s`, attempt %d: before history ID %d; started %s; ended %s.\n", r.ID, r.Step, r.Attempt, r.Boundary, stamp(r.Started), stamp(r.Ended))
	}
	b.WriteString("\n## Request configurations\n\n")
	for _, c := range s.Configurations {
		fmt.Fprintf(&b, "### Configuration %s\n\nSystem prompt:\n\n", c.ID)
		fence(&b, "text", c.System)
		b.WriteString("Function definition:\n\n")
		if len(c.Tool) == 0 {
			b.WriteString("Tools disabled.\n\n")
		} else {
			var v any
			if json.Unmarshal(c.Tool, &v) == nil {
				raw, _ := json.MarshalIndent(v, "", "  ")
				fence(&b, "json", string(raw))
			}
		}
	}
	b.WriteString("## Conversation\n\n")
	for _, t := range s.Turns {
		renderExportTurn(&b, t)
	}
	return b.String()
}

func value(s string) string {
	if s == "" {
		return "unavailable"
	}
	return s
}
func cell(s string) string { return strings.NewReplacer("|", "\\|", "\n", " ", "\r", " ").Replace(s) }
func stamp(t time.Time) string {
	if t.IsZero() {
		return "unavailable"
	}
	return t.Format(time.RFC3339Nano)
}

func fence(b *strings.Builder, language, text string) {
	f := "```"
	for strings.Contains(text, f) {
		f += "`"
	}
	fmt.Fprintf(b, "%s%s\n%s\n%s\n\n", f, language, text, f)
}

func renderExportTurn(b *strings.Builder, t Turn) {
	fmt.Fprintf(b, "### %s — history %d\n\n", strings.Title(t.Role), t.HID)
	if t.System != "" {
		b.WriteString("System event:\n\n")
		fence(b, "text", t.System)
	}
	if t.Kind == KindView {
		b.WriteString("*Image view*\n\n")
	}
	if t.Text != "" {
		b.WriteString(t.Text + "\n\n")
	}
	if t.Thinking != "" {
		b.WriteString("Thinking:\n\n")
		fence(b, "text", t.Thinking)
	}
	if t.CallID != "" {
		fmt.Fprintf(b, "Tool call: `%s`\n\n", t.CallID)
	}
	if t.Code != "" {
		fence(b, "sh", t.Code)
	}
	if t.Output != "" {
		fence(b, "text", t.Output)
	}
	if t.Err != "" {
		b.WriteString("Error:\n\n")
		fence(b, "text", t.Err)
	}
	for _, a := range t.Attachments {
		fmt.Fprintf(b, "Attachment: image #%d (%s, %dx%d; bytes not embedded)\n\n", a.N, a.Mime, a.W, a.H)
	}
}
