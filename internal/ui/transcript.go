package ui

import (
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
	Mode   JSMode
	Lines  int
	Output bool
}

func BuildLines(turns []agent.Turn, w int, js JSOpts, thinking bool) []Line {
	var out []Line
	wrapW := w - padX*2
	for _, t := range turns {
		switch {
		case t.Role == llm.RoleUser:
			out = append(out, Band(BgUser))
			out = append(out, WrapLines(t.Text, wrapW-2, func(s string) Line {
				return Plain("  "+s, FgWhite, BgUser)
			})...)
			out = append(out, Band(BgUser))
		case t.Code != "":
			if t.Text != "" {
				out = append(out, WrapLines(t.Text, wrapW-2, func(s string) Line {
					return Glyphed("◇", Purple, s, FgSoft, BgTerm)
				})...)
			}
			out = append(out, jsBlock(t, wrapW, js)...)
		case t.Role == llm.RoleAssistant && t.Text != "":
			out = append(out, WrapLines(t.Text, wrapW-2, func(s string) Line {
				return Glyphed("◆", Purple, s, FgWhite, BgTerm)
			})...)
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
		out = append(out, CodeLines(t.Code, BgJSDone, max)...)
	}
	if js.Output {
		out = append(out, outputLines(t, w)...)
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
