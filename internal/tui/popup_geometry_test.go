package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"meno/internal/api"
	"meno/internal/config"
	"meno/internal/render"

	"github.com/gdamore/tcell/v2"
)

func TestModalGeometryFitsTheScreen(t *testing.T) {
	for _, size := range [][2]int{{112, 36}, {96, 30}, {80, 24}, {60, 20}, {48, 16}, {40, 12}, {1, 1}, {0, 0}} {
		for _, requested := range []int{0, 66, 73, 80, 660} {
			b := centered(size[0], size[1], requested, 50)
			if b.w < 0 || b.w > size[0] || b.oh != min(52, size[1]) || b.h != max(0, b.oh-2) || b.x != (size[0]-b.w)/2 || b.oy != (size[1]-b.oh)/2 || b.y != b.oy+1 {
				t.Fatalf("size %v request %d: %+v", size, requested, b)
			}
			if size[0] >= 100 && b.w < modalWidth(size[0]) {
				t.Fatalf("a wide screen keeps at least the half-screen width: size %v %+v", size, b)
			}
			if size[0] >= 20 && size[0] < 100 && b.w < size[0]-4 {
				t.Fatalf("a narrow screen uses nearly the full row: size %v %+v", size, b)
			}
			if size[0] >= 100 && requested > 0 && requested <= size[0]-4 && b.w < requested {
				t.Fatalf("content that needs %d columns must get them: size %v %+v", requested, size, b)
			}
		}
	}
}

func TestAutocompleteGridLayout(t *testing.T) {
	for _, tc := range []struct{ count, w, rows int }{
		{0, 100, 0}, {1, 100, 1}, {5, 100, 5}, {6, 100, 3}, {7, 100, 4}, {40, 100, 20}, {60, 100, 25}, {200, 100, 25}, {10, 30, 10},
	} {
		if got := acRowCount(tc.count, tc.w); got != tc.rows {
			t.Fatalf("count %d width %d: rows %d, want %d", tc.count, tc.w, got, tc.rows)
		}
	}
}

func skillsN(n int) []config.Skill {
	var out []config.Skill
	for i := 0; i < n; i++ {
		out = append(out, config.Skill{Name: fmt.Sprintf("p%02d", i), Desc: fmt.Sprintf("desc%02d", i), Enabled: true})
	}
	return out
}

func TestAutocompleteTwoColumnsFillDownThenRight(t *testing.T) {
	a, s := visualFixture(t, 100, 40)
	a.st.Mode = render.ModeInput
	a.st.Skills = skillsN(8)
	a.setInput([]rune("/skill:"))
	a.drawAutocompleteRows(render.Canvas{S: s}, 0, 0, 100, 10)
	// 8 items: 4 rows, left column p00..p03, right column p04..p07.
	for row := 0; row < 4; row++ {
		line := visualRow(s, row)
		l, r := fmt.Sprintf("/skill:p%02d", row), fmt.Sprintf("/skill:p%02d", row+4)
		li, ri := strings.Index(line, l), strings.Index(line, r)
		if li < 0 || ri < 0 || li >= 50 || ri < 50 {
			t.Fatalf("row %d: %q (left %d right %d)", row, line, li, ri)
		}
	}
	if strings.TrimSpace(visualRow(s, 4)) != "" {
		t.Fatalf("list too tall: %q", visualRow(s, 4))
	}
	// Down walks the left column, then continues at the top of the right one.
	for i := 0; i < 4; i++ {
		a.acKey(keyK(tcell.KeyDown))
	}
	if a.st.Sel != 4 {
		t.Fatalf("selection %d", a.st.Sel)
	}
	a.drawAutocompleteRows(render.Canvas{S: s}, 0, 0, 100, 10)
	if line := visualRow(s, 0); !strings.Contains(line[50:], "▸") || strings.Contains(line[:50], "▸") {
		t.Fatalf("marker not at the top of the right column: %q", line)
	}
}

func TestAutocompleteShortListUsesFullWidth(t *testing.T) {
	a, s := visualFixture(t, 100, 40)
	a.st.Skills = skillsN(5)
	a.setInput([]rune("/skill:"))
	a.drawAutocompleteRows(render.Canvas{S: s}, 0, 0, 100, 10)
	_, _, st, _ := s.GetContent(99, 0)
	_, bg, _ := st.Decompose()
	if bg != render.BgPopup && bg != render.BgPopupSel {
		t.Fatalf("list does not span the full width: bg %v", bg)
	}
	if strings.Contains(visualRow(s, 5), "p") {
		t.Fatal("five items must take five rows")
	}
}

func TestAutocompleteScrollsByPages(t *testing.T) {
	a, s := visualFixture(t, 100, 40)
	a.st.Mode = render.ModeInput
	a.st.Skills = skillsN(40)
	a.setInput([]rune("/skill:"))
	for i := 0; i < 30; i++ {
		a.acKey(keyK(tcell.KeyDown))
	}
	if a.st.Sel != 30 {
		t.Fatalf("selection %d", a.st.Sel)
	}
	a.drawAutocompleteRows(render.Canvas{S: s}, 0, 0, 100, 5)
	// 5 rows x 2 columns = pages of 10: sel 30 is on the page p30..p39.
	if !strings.Contains(visualRow(s, 0), "/skill:p30") || !strings.Contains(visualRow(s, 4), "/skill:p34") {
		t.Fatalf("selected completion not in viewport: %q", visualRow(s, 0))
	}
	a.acKey(keyK(tcell.KeyEnter))
	if labelText(a.st.Input) != "/skill:p30 " {
		t.Fatalf("input %q", labelText(a.st.Input))
	}
}

func TestModalDimsEntireBackdrop(t *testing.T) {
	_, s := visualFixture(t, 80, 24)
	c := render.Canvas{S: s}
	c.Fill(0, 0, 80, 24, render.BgUser)
	c.Text(0, 0, 80, "background 界", render.Style{FG: render.FgWhite, BG: render.BgUser, Bold: true}.TCell())
	b := centered(80, 24, 73, 8)
	b.frame(c)
	for _, p := range [][2]int{{0, 0}, {79, 0}, {0, 23}, {79, 23}} {
		_, _, st, _ := s.GetContent(p[0], p[1])
		_, bg, _ := st.Decompose()
		r, g, blue := render.BgUser.RGB()
		if bg != tcell.NewRGBColor(r/2, g/2, blue/2) {
			t.Fatalf("undimmed corner %v", p)
		}
	}
	r, _, st, _ := s.GetContent(0, 0)
	_, _, attrs := st.Decompose()
	if r != 'b' || attrs&tcell.AttrBold == 0 {
		t.Fatal("dimming lost content or attributes")
	}
	_, _, st, _ = s.GetContent(b.x, b.y)
	_, bg, _ := st.Decompose()
	if bg != render.BgPopup {
		t.Fatal("modal surface was dimmed")
	}
}

func TestSessionSearchTitleDatePathAndClear(t *testing.T) {
	a, s := visualFixture(t, 120, 36)
	a.globalSessions = []api.SessionItem{{ID: "test", Name: "Fix compiler", Cwd: "/work/compiler", UpdatedAt: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)}}
	for _, q := range []string{"COMPILER", "/work", "2026-10-16"} {
		a.st.Query.Set(q)
		if len(sessionIdx(a)) != 1 {
			t.Fatalf("query %q not matched", q)
		}
	}
	a.sessionsKey(keyK(tcell.KeyCtrlU))
	if !a.st.Query.Empty() {
		t.Fatal("query not cleared")
	}
	a.drawSessions(render.Canvas{S: s}, 120, 36)
	for _, text := range []string{"sessions", "search title, date or path", "Fix compiler", "/work/compiler"} {
		if !strings.Contains(screenText(s), text) {
			t.Fatalf("missing %q", text)
		}
	}
}

func sessionIdx(a *App) []int {
	_, idx := a.sessionsFiltered()
	return idx
}
