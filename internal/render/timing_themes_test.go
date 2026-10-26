package render

import (
	"fmt"
	"testing"
)

func TestErrorBackgroundExistsInEveryTheme(t *testing.T) {
	for _, th := range Themes() {
		p := th.Palette
		if p.BgDiffDel == 0 || p.BgDiffDel == p.BgTerm || p.BgDiffDel == p.BgDiffAdd {
			t.Errorf("%s: BgDiffDel %v", th.Name, p.BgDiffDel)
		}
		if c := contrast(p.FgWhite, p.BgDiffDel); c < 4.5 {
			t.Errorf("%s: text on the error background %.1f", th.Name, c)
		}
		if c := contrast(p.Red, p.BgDiffDel); c < 2.5 {
			t.Errorf("%s: red on the error background %.1f", th.Name, c)
		}
		if c := contrast(p.Orange, p.BgPopup); c < 2.5 {
			t.Errorf("%s: inline code %.1f", th.Name, c)
		}
		if c := contrast(p.Green, p.BgDiffAdd); c < 2.5 {
			t.Errorf("%s: added line marker %.1f", th.Name, c)
		}
	}
	defer SetTheme(DefaultTheme)
	SetTheme(DefaultTheme)
	if BgDiffDel == 0 || BgDiffDel == BgTerm {
		t.Fatal("theme switch did not set the error background")
	}
	SetTheme(DefaultTheme)
	if fmt.Sprint(BgDiffDel) != fmt.Sprint(Themes()[0].BgDiffDel) {
		t.Fatal("default restored")
	}
}
