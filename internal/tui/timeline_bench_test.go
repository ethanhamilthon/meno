package tui

import (
	"fmt"
	"meno/internal/render"
	"strings"
	"testing"
)

// Stable history IDs exercise production cache hits rather than cold layouts.
func BenchmarkCachedTimeline(b *testing.B) {
	for _, n := range []int{600, 6000} {
		for _, mode := range []string{"layout", "draw", "live", "overlay"} {
			b.Run(fmt.Sprintf("%d/%s", n, mode), func(b *testing.B) {
				a, scr, _ := e2eApp(b, &recClient{})
				defer scr.Fini()
				for i := 0; i < n; i++ {
					a.st.Session.turns = append(a.st.Session.turns, render.Turn{ID: fmt.Sprint(i + 1), Role: render.RoleAssistant, Text: "## Result\n\n- one\n- two with `code`\n"})
				}
				if mode == "live" {
					a.st.LiveBase = n
					a.st.LiveText = strings.Repeat("Paragraph with **bold** and `code`.\n\n", 500)
					a.st.Running = true
				}
				if mode == "overlay" {
					a.st.Over = OverHelp
				}
				a.draw()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if mode == "layout" {
						_ = a.transcriptWithSelection(96)
					} else {
						a.draw()
					}
				}
			})
		}
	}
}

func BenchmarkToolOutputLayout(b *testing.B) {
	for _, n := range []int{100, 100000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			t := render.Turn{Role: render.RoleAssistant, Code: "echo output", Output: strings.Repeat("some output line\n", n)}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = render.BuildLines([]render.Turn{t}, 96, render.JSOpts{View: render.ViewFull})
			}
		})
	}
}

func BenchmarkChangingTimeline(b *testing.B) {
	for _, n := range []int{600, 6000} {
		for _, mode := range []string{"delta", "append", "composer", "panel"} {
			b.Run(fmt.Sprintf("%d/%s", n, mode), func(b *testing.B) {
				a, s, _ := e2eApp(b, &recClient{})
				defer s.Fini()
				for i := 0; i < n; i++ {
					a.st.Session.turns = append(a.st.Session.turns, render.Turn{ID: fmt.Sprint(i + 1), Role: render.RoleAssistant, Text: "## Result\n\n- one\n- two with `code`\n"})
				}
				a.st.LiveBase = n
				a.st.LiveText = strings.Repeat("Paragraph with **bold** and `code`.\n\n", 100)
				if mode == "panel" {
					a.openOverlay(OverModel)
				}
				a.draw()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					switch mode {
					case "delta":
						a.st.LiveText += "x"
					case "append":
						a.st.Session.changed(len(a.st.Session.turns))
						a.st.Session.turns = append(a.st.Session.turns, render.Turn{ID: fmt.Sprint(n + i + 1), Role: render.RoleAssistant, Text: "appended"})
					case "composer":
						a.setInput([]rune(fmt.Sprint(i)))
					case "panel":
						testQuery(a, fmt.Sprint(i))
					}
					a.draw()
				}
			})
		}
	}
}
