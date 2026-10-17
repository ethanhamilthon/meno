package ui

import (
	"strconv"
	"strings"
	"time"

	"meno/internal/agent"
	"meno/internal/llm"
)

const padX = 2

type JSMode int

const (
	JSFull JSMode = iota
	JSLines
	JSOff
)

type JSOpts struct {
	Mode  JSMode
	Lines int
	// OutMode/OutLines are the same all/lines/off choice for tool output.
	OutMode  JSMode
	OutLines int
	// ThinkMode/ThinkLines are the same all/lines/off choice for model reasoning.
	ThinkMode  JSMode
	ThinkLines int
	// Running paints code blocks as live (streaming or executing).
	Running bool
	// Timing is which finished js calls show their duration; Summary shows the
	// line that closes a run.
	Timing  TimingMode
	Summary bool
	// Exec marks a call that is executing now (not only streaming): its footer
	// counts Elapsed up towards Limit (0 = no limit); Spin picks the spinner frame.
	Exec    bool
	Elapsed time.Duration
	Limit   time.Duration
	Spin    int
}

func BuildLines(turns []agent.Turn, w int, js JSOpts) []Line {
	var out []Line
	wrapW := w - padX*2
	for _, t := range turns {
		think := ""
		if js.ThinkMode != JSOff && t.Replay != nil {
			think = strings.TrimSpace(t.Replay.Thought)
		}
		visible := t.Role == llm.RoleUser || t.Code != "" || t.Role == llm.RoleAssistant && (t.Text != "" || think != "")
		if visible && len(out) > 0 {
			out = append(out, Band(BgTerm))
		}
		if think != "" && t.Role == llm.RoleAssistant {
			out = append(out, thinkingLines(think, wrapW-2, js)...)
			if t.Text != "" || t.Code != "" {
				out = append(out, Band(BgTerm))
			}
		}
		switch {
		case t.Role == llm.RoleUser && t.Kind == agent.KindView:
			out = append(out, WrapLines("◌ "+t.Text, wrapW-2, func(s string) Line {
				return Plain("  "+s, FgDim, BgTerm)
			})...)
		case t.Role == llm.RoleUser:
			out = append(out, Band(BgUser))
			out = append(out, WrapLines(t.Text, wrapW-2, func(s string) Line {
				return Plain("  "+s, FgWhite, BgUser)
			})...)
			out = append(out, Band(BgUser))
		case t.Code != "":
			if t.Text != "" {
				out = append(out, MarkdownLines(t.Text, wrapW-2, "◇")...)
			}
			block := jsBlock(t, wrapW-2, js)
			if t.Text != "" && len(block) > 0 {
				out = append(out, Band(BgTerm))
			}
			out = append(out, block...)
		case t.Role == llm.RoleAssistant && t.Text != "":
			out = append(out, MarkdownLines(t.Text, wrapW-2, "◆")...)
		}
		if js.Summary && t.Run != nil {
			if len(out) > 0 {
				out = append(out, Band(BgTerm))
			}
			out = append(out, SummaryLines(t.Run, wrapW)...)
		}
	}
	return out
}

func jsBlock(t agent.Turn, w int, js JSOpts) []Line {
	var out []Line
	if js.Mode != JSOff {
		max := 0
		if js.Mode == JSLines {
			max = js.Lines
		}
		bg := BgJSDone
		switch {
		case js.Running:
			bg = BgJSRun
		case t.Err != "" || t.TimedOut:
			bg = BgJSErr
		}
		lines := CodeLines(t.Code, bg, w-2, max)
		for i := range lines {
			lines[i].Segs = append([]Seg{{Text: "  ", FG: FgDim}}, lines[i].Segs...)
		}
		out = append(out, lines...)
		// The timing belongs under its code; with the code hidden only the
		// news that matter (a failure, a timeout) are worth a line of their own.
		if foot, ok := timingFooter(t, js); ok {
			out = append(out, Line{Segs: append([]Seg{{Text: "  "}}, fitFooter(foot, w-2)...), BG: bg})
		}
	} else if foot, ok := timingFooter(t, js); ok && (t.Err != "" || t.TimedOut) {
		out = append(out, Line{Segs: append([]Seg{{Text: "  "}}, fitFooter(foot, w-2)...), BG: BgJSErr})
	}
	if js.OutMode != JSOff {
		limit := 0
		if js.OutMode == JSLines {
			limit = js.OutLines
		}
		out = append(out, limitRows(outputLines(t, w), limit)...)
	}
	return out
}

func outputLines(t agent.Turn, w int) []Line {
	if t.Err != "" {
		return WrapLines("error: "+t.Err, w, func(s string) Line {
			return Plain("  "+s, Red, BgTerm)
		})
	}
	if t.Output == "" {
		return nil
	}
	return WrapLines(t.Output, w, func(s string) Line {
		return Plain("  "+s, FgDim, BgTerm)
	})
}

// thinkingLines renders model reasoning dimmed, with a glyph on the first row.
// In "lines" mode only the newest rows are kept: the end of the reasoning is what
// the model acted on, and the view stays steady while it streams.
func thinkingLines(text string, w int, js JSOpts) []Line {
	rows := Wrap(text, max(1, w))
	hidden := 0
	if js.ThinkMode == JSLines && js.ThinkLines > 0 && len(rows) > js.ThinkLines {
		hidden = len(rows) - js.ThinkLines
		rows = rows[hidden:]
	}
	var out []Line
	if hidden > 0 {
		out = append(out, Line{Segs: []Seg{{Text: "∴ ", FG: Purple}, {Text: "… " + itoa(hidden) + " earlier", FG: FgDim}}, BG: BgTerm})
	}
	for i, l := range rows {
		prefix := "  "
		if i == 0 && hidden == 0 {
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
