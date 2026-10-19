package app

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"meno/internal/llm"
	"meno/internal/ui"
)

func visualFixture(t *testing.T, w, h int) (*App, tcell.SimulationScreen) {
	t.Helper()
	a, s := newTestApp(t)
	t.Cleanup(s.Fini)
	s.SetSize(w, h)
	a.paths.Cwd = "~/projects/api"
	a.st.Settings.Model = "gpt-5.5"
	a.st.Settings.Provider = "openai"
	a.st.Settings.Effort = "high"
	a.st.Providers = []llm.Provider{{Name: "openai"}}
	a.st.Pricing = map[string]llm.ModelInfo{"openai/gpt-5.5": {Context: 400000, Input: 1.25, Output: 10}}
	a.st.Context = 1400
	return a, s
}

func visualRow(s tcell.SimulationScreen, y int) string {
	w, _ := s.Size()
	var b strings.Builder
	for x := 0; x < w; x++ {
		r, _, _, _ := s.GetContent(x, y)
		b.WriteRune(r)
	}
	return b.String()
}

func TestVisualReferenceBottom(t *testing.T) {
	a, s := visualFixture(t, 112, 36)
	a.draw()
	if a.bottomHeight() != 4 {
		t.Fatalf("bottom = %d, want 3 composer + 1 status", a.bottomHeight())
	}
	for y := 32; y < 35; y++ {
		_, _, st, _ := s.GetContent(0, y)
		_, bg, _ := st.Decompose()
		if bg != ui.BgUser {
			t.Fatalf("composer row %d background %v", y, bg)
		}
	}
	_, _, st, _ := s.GetContent(2, 35)
	fg, bg, attrs := st.Decompose()
	if fg != ui.FgBlack || bg != ui.Green || attrs&tcell.AttrReverse != 0 {
		t.Fatalf("badge fg=%v bg=%v attrs=%v", fg, bg, attrs)
	}
	row := visualRow(s, 35)
	center := a.statusUsageText()
	index := strings.Index(row, center)
	if index < 0 {
		t.Fatalf("status metrics missing %q: %s", center, row)
	}
	if got := ui.TextWidth(row[:index]); got != (112-ui.TextWidth(center))/2 {
		t.Fatalf("center at %d: %s", got, row)
	}
	if !strings.Contains(row, "gpt-5.5 (high)") || strings.ContainsAny(row, "○●◆") {
		t.Fatal("idle bar shows the model only on the right:", row)
	}
	a.st.Running = true
	a.draw()
	if !strings.Contains(visualRow(s, 35), "1x◆") {
		t.Fatal("running activity not indicated")
	}
	exportVisual(t, s, "main")
}

// logoCellLit reports whether a cell of the wordmark box is lit: a letter
// pixel or its shadow one cell down and to the right.
func logoCellLit(ri, i int) bool {
	if i >= 0 && i < logoW && ri >= 0 && ri < logoH && []rune(logoRows[ri])[i] != ' ' {
		return true
	}
	if ri >= 1 && ri-1 < logoH && i >= 1 && i-1 < logoW {
		return []rune(logoRows[ri-1])[i-1] != ' '
	}
	return false
}

func TestVisualWelcomeAlignment(t *testing.T) {
	a, s := visualFixture(t, 112, 36)
	a.draw()
	top := (32 - welcomeBlockH) / 2
	x := (112 - logoW) / 2
	for ri, want := range logoRows {
		got := []rune(visualRow(s, top+3+ri))[x : x+logoW]
		for i, r := range []rune(want) {
			if lit := got[i] != ' '; lit != logoCellLit(ri, i) {
				t.Fatalf("logo shape row %d col %d: lit=%v want %v (main %v)", ri, i, lit, logoCellLit(ri, i), r != ' ')
			}
		}
	}
	if row := visualRow(s, top); !strings.Contains(row, tagline) {
		t.Fatal(row)
	}
	if row := visualRow(s, top+18); !strings.Contains(row, ":help") || !strings.Contains(row, "commands") {
		t.Fatal(row)
	}
	exportVisual(t, s, "welcome")
}

func TestWelcomeWaveAnimates(t *testing.T) {
	a, s := visualFixture(t, 112, 36)
	a.draw()
	if !a.waveOn.Load() {
		t.Fatal("welcome screen does not request animation")
	}
	first := screenText(s)
	time.Sleep(1200 * time.Millisecond)
	a.draw()
	if screenText(s) == first {
		t.Fatal("background wave did not move")
	}
}

// logoFrame returns the lit mask and the glyphs of the wordmark on screen.
func logoFrame(s tcell.SimulationScreen, top, x int) (mask, glyphs string) {
	for ri, want := range logoRows {
		row := []rune(visualRow(s, top+3+ri))
		for i := range []rune(want) {
			if row[x+i] != ' ' {
				mask += "1"
				glyphs += string(row[x+i])
			} else {
				mask += "0"
			}
		}
	}
	return
}

func TestWelcomeLogoScramblesWithoutChangingShape(t *testing.T) {
	a, s := visualFixture(t, 112, 36)
	top := (32 - welcomeBlockH) / 2
	x := (112 - logoW) / 2
	a.draw()
	mask1, glyphs1 := logoFrame(s, top, x)
	time.Sleep(250 * time.Millisecond)
	a.draw()
	mask2, glyphs2 := logoFrame(s, top, x)
	if mask1 != mask2 {
		t.Fatalf("wordmark shape changed:\n%s\n%s", mask1, mask2)
	}
	if glyphs1 == glyphs2 {
		t.Fatalf("the glyphs did not scramble: %q", glyphs1)
	}
	if strings.Trim(glyphs1, " ") == "" {
		t.Fatal("no lit pixels")
	}
}

func TestVisualModalGridAndSelection(t *testing.T) {
	a, s := visualFixture(t, 112, 36)
	a.openOverlay(OverSettings)
	a.draw()
	_, _, _, tab := settingTabFor(a.st.Sel)
	b := centered(112, 36, 66, 4+len(tab.rows)+1)
	_, _, st, _ := s.GetContent(b.x, b.y)
	_, bg, _ := st.Decompose()
	if bg != ui.BgPopup {
		t.Fatal("settings not centered on cell grid")
	}
	_, _, st, _ = s.GetContent(b.x-1, b.y)
	_, bg, _ = st.Decompose()
	if bg == ui.BgPopup {
		t.Fatal("settings expands beyond reference width")
	}
	if !strings.Contains(visualRow(s, 35), "SETTINGS") {
		t.Fatal("overlay badge missing")
	}
	exportVisual(t, s, "settings")
	a.st.Models = map[string][]llm.Model{"openai": {}}
	for i := 0; i < 20; i++ {
		a.st.Models["openai"] = append(a.st.Models["openai"], llm.Model{ID: fmt.Sprintf("model-%02d", i), Provider: "openai"})
	}
	a.openOverlay(OverModel)
	a.st.Sel = -1
	for i, model := range a.modelList() {
		if model.ID == "model-19" && model.Provider == "openai" {
			a.st.Sel = i
			break
		}
	}
	if a.st.Sel < 0 {
		t.Fatal("model-19 missing from merged catalog")
	}
	a.draw()
	if !strings.Contains(screenText(s), "model-19") {
		t.Fatal("selected model outside viewport")
	}
	exportVisual(t, s, "model")
}

func TestVisualHelpRetainsDetailedMetrics(t *testing.T) {
	a, s := visualFixture(t, 112, 36)
	a.openOverlay(OverHelp)
	var visible strings.Builder
	for scroll := 0; ; scroll++ {
		a.st.Sel = scroll
		a.draw()
		visible.WriteString(screenText(s))
		visible.WriteByte('\n')
		if a.st.Sel < scroll {
			break
		}
	}
	text := strings.Join(strings.Fields(visible.String()), " ")
	for _, want := range []string{"usage · last request", "last in", "cache", "session in", "spent"} {
		if !strings.Contains(text, want) {
			t.Fatalf("help metrics missing %q", want)
		}
	}
	exportVisual(t, s, "help")
}

func TestVisualResponsiveComposer(t *testing.T) {
	for _, size := range [][2]int{{40, 12}, {20, 7}, {8, 4}, {1, 1}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			a, s := visualFixture(t, size[0], size[1])
			a.st.Input = []rune(strings.Repeat("界", 80))
			a.draw()
			if size[1] > 1 && size[0] > 2 && !cursorVisible(s) {
				t.Fatal("long composer loses cursor")
			}
			if size[0] == 40 {
				exportVisual(t, s, "narrow")
			}
		})
	}
}

// Opt-in exports stay outside the repository. SVG preserves exact terminal cells
// and colors while allowing a viewer to choose its own monospace font.
func exportVisual(t *testing.T, s tcell.SimulationScreen, name string) {
	t.Helper()
	if os.Getenv("MENO_VISUAL_EXPORT") == "" {
		return
	}
	dir, err := os.MkdirTemp("", "meno-visual-"+name+"-")
	if err != nil {
		t.Fatal(err)
	}
	w, h := s.Size()
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, w*9, h*18, w*9, h*18)
	b.WriteString(`<g font-family="Menlo,DejaVu Sans Mono,monospace" font-size="14">`)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, comb, st, cw := s.GetContent(x, y)
			fg, bg, _ := st.Decompose()
			color := func(c tcell.Color) string { r, g, b := c.RGB(); return fmt.Sprintf("#%02x%02x%02x", r, g, b) }
			fmt.Fprintf(&b, `<rect x="%d" y="%d" width="9" height="18" fill="%s"/>`, x*9, y*18, color(bg))
			if r != ' ' && r != 0 && cw > 0 {
				text := string(append([]rune{r}, comb...))
				fmt.Fprintf(&b, `<text x="%d" y="%d" fill="%s">%s</text>`, x*9, y*18+14, color(fg), html.EscapeString(text))
			}
		}
	}
	b.WriteString(`</g></svg>`)
	path := filepath.Join(dir, name+".svg")
	if err := os.WriteFile(path, []byte(b.String()), 0600); err != nil {
		t.Fatal(err)
	}
	t.Log("visual snapshot:", path)
}
