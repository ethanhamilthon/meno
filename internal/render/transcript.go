package render

import (
	"html"

	"strconv"
	"strings"
	"time"
)

// Gutter is the number of columns kept free of text on the left and the right
// of every row. Backgrounds, rules and icons may use them; text may not.
const Gutter = 2

const padX = Gutter

// ViewMode is how much of the agent's work the timeline shows.
type ViewMode int

const (
	// ViewFull shows thinking and tool calls; output is cut to OutputRows.
	ViewFull ViewMode = iota
	// ViewBashOnly shows the commands only, cut to CodeRows.
	ViewBashOnly
	// ViewCompact hides thinking and tool calls from the timeline.
	ViewCompact
)

// Row limits of the views.
const (
	OutputRows = 5
	CodeRows   = 5
)

type JSOpts struct {
	View ViewMode
	// Running paints code blocks as live (streaming or executing).
	Running bool
	// Summary shows the line that closes a run.
	Summary bool
	// Exec marks a call that is executing now (not only streaming): its footer
	// counts Elapsed up; Spin picks the spinner frame.
	Exec    bool
	Elapsed time.Duration
	Spin    int
	// Images configures inline pictures for attachments and view().
	Images ImageOpts
	// Streaming: the call's source is still arriving, so it is shown plain.
	Streaming bool
}

func BuildLines(turns []Turn, w int, js JSOpts) []Line {
	var out []Line
	wrapW := TranscriptTextWidth(w)
	separate := false
	for _, t := range turns {
		think := ""
		if js.View == ViewFull && t.Thinking != "" {
			think = strings.TrimSpace(t.Thinking)
		}
		visible := t.Role == RoleUser || t.Code != "" || t.Role == RoleAssistant && (t.Text != "" || think != "")
		if visible && len(out) > 0 && separate {
			out = append(out, Band(BgTerm))
		}
		if visible {
			separate = TurnHasMessage(t, js)
		}
		if think != "" && t.Role == RoleAssistant {
			out = append(out, thinkingLines(think, wrapW)...)
			if t.Text != "" || t.Code != "" {
				out = append(out, Band(BgTerm))
			}
		}
		switch {
		case t.Role == RoleUser && t.Kind == KindEvent:
			out = append(out, transcriptRows(eventSummary(t.Text), "⇢", w)...)
		case t.Role == RoleUser && t.Kind == KindSkill:
			out = append(out, transcriptRows("/skill:"+skillName(t.Text)+" loaded", "◌", w)...)
		case t.Role == RoleUser && t.Kind == KindView:
			out = append(out, transcriptRows(t.Text, "◌", w)...)
			if js.View != ViewCompact {
				for _, at := range t.Attachments {
					out = append(out, imageBlock(at, "view()", w, js.Images)...)
				}
			}
		case t.Role == RoleUser:
			out = append(out, Band(BgUser))
			shown := t.Text
			out = append(out, WrapLines(withObjectLabels(shown), wrapW, userLineMaker())...)
			out = append(out, Band(BgUser))
			if js.View != ViewCompact {
				for _, at := range t.Attachments {
					out = append(out, imageBlock(at, "attachment", w, js.Images)...)
				}
			}
		case t.Code != "":
			if t.Text != "" {
				out = append(out, MarkdownLines(t.Text, wrapW, "◇")...)
			}
			block := jsBlock(t, w, js)
			if t.Text != "" && len(block) > 0 {
				out = append(out, Band(BgTerm))
			}
			out = append(out, block...)
		case t.Role == RoleAssistant && t.Text != "":
			out = append(out, MarkdownLines(t.Text, wrapW, "◆")...)
		}
		if t.Run != nil {
			if len(out) > 0 {
				out = append(out, Band(BgTerm))
			}
			out = append(out, transcriptSummary(t.Run, w)...)
		}
	}
	return out
}

func jsBlock(t Turn, w int, js JSOpts) []Line {
	if js.View == ViewCompact {
		return nil
	}
	out := []Line{bashHeader(t, w, js)}
	{
		max := 0
		if js.View == ViewBashOnly {
			max = CodeRows
		}
		// No background in any state: a color that changes when the call starts or
		// ends repaints the whole block.
		lines := CodeLinesOpt(t.Code, BgTerm, TranscriptTextWidth(w), max, CodeOpts{Plain: js.Streaming})
		for i := range lines {
			lines[i].Segs = append([]Seg{{Text: "  ", FG: FgDim}}, lines[i].Segs...)
		}
		out = append(out, lines...)
	}
	if js.View == ViewFull {
		out = append(out, limitRows(outputLines(t, w), OutputRows)...)
	}
	return out
}

func outputLines(t Turn, w int) []Line {
	if t.Err != "" {
		return WrapLines("error: "+t.Err, TranscriptTextWidth(w), func(s string) Line {
			return Plain("  "+s, Red, BgTerm)
		})
	}
	if t.Output == "" {
		return nil
	}
	return WrapLines(t.Output, TranscriptTextWidth(w), func(s string) Line {
		return Plain("  "+s, FgDim, BgTerm)
	})
}

// thinkingLines renders model reasoning dimmed, with a glyph on the first row.
func thinkingLines(text string, w int) []Line {
	rows := Wrap(text, max(1, w))
	var out []Line
	for i, l := range rows {
		prefix := "  "
		if i == 0 {
			prefix = "∴ "
		}
		out = append(out, Line{Segs: []Seg{{Text: prefix, FG: Purple}, {Text: l, FG: FgDim}}, BG: BgTerm})
	}
	return out
}

// limitRows keeps the first n rows and marks the cut; n <= 0 keeps everything.
func limitRows(rows []Line, n int) []Line {
	if n <= 0 || len(rows) <= n {
		return rows
	}
	more := Line{Segs: []Seg{{Text: "  … " + itoa(len(rows)-n) + " more", FG: FgDim}}, BG: BgTerm}
	return append(append([]Line(nil), rows[:n]...), more)
}

func itoa(n int) string { return strconv.Itoa(n) }

// eventSummary is the one-line form of a rendered steering event: the source
// and the first line of its text.
func eventSummary(rendered string) string {
	if body, ok := strings.CutPrefix(rendered, "<system-event>"); ok {
		// What the event is about: the tag of its first line ("<date-changed>…")
		// or, for a plain note, the word before its colon ("date-changed: …").
		body = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(body), "</system-event>"))
		if _, rest, ok := strings.Cut(body, "\n<"); ok {
			body = "<" + rest
		}
		if rest, ok := strings.CutPrefix(body, "<"); ok {
			name, _, _ := strings.Cut(rest, ">")
			return "system: " + name
		}
		name, _, _ := strings.Cut(body, ":")
		return "system: " + strings.Join(strings.Fields(name), " ")
	}
	head, body, _ := strings.Cut(rendered, ">\n")
	src, from := attr(head, "source"), attr(head, "from")
	if from != "" {
		src += " " + from
	}
	first, _, _ := strings.Cut(html.UnescapeString(strings.TrimSuffix(body, "\n</event>")), "\n")
	return html.UnescapeString(src) + ": " + first
}

func attr(head, name string) string {
	_, rest, ok := strings.Cut(head, " "+name+"=\"")
	if !ok {
		return ""
	}
	v, _, _ := strings.Cut(rest, "\"")
	return v
}
