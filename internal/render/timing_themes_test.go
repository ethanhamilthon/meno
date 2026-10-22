package render

import (
	"fmt"
	"testing"
)

func TestErrorBackgroundExistsInEveryTheme(t *testing.T) {
	for _, th := range Themes() {
		p := th.Palette
		if p.BgJSErr == 0 || p.BgJSErr == p.BgTerm || p.BgJSErr == p.BgJSDone {
			t.Errorf("%s: BgJSErr %v", th.Name, p.BgJSErr)
		}
		if c := contrast(p.FgWhite, p.BgJSErr); c < 4.5 {
			t.Errorf("%s: text on the error background %.1f", th.Name, c)
		}
		if c := contrast(p.Red, p.BgJSErr); c < 2.5 {
			t.Errorf("%s: red on the error background %.1f", th.Name, c)
		}
		if c := contrast(p.Orange, p.BgPopup); c < 2.5 {
			t.Errorf("%s: inline code %.1f", th.Name, c)
		}
		if c := contrast(p.Green, p.BgJSDone); c < 2.5 {
			t.Errorf("%s: added line marker %.1f", th.Name, c)
		}
	}
	defer SetTheme(DefaultTheme)
	SetTheme(DefaultTheme)
	if BgJSErr == 0 || BgJSErr == BgTerm {
		t.Fatal("theme switch did not set the error background")
	}
	SetTheme(DefaultTheme)
	if fmt.Sprint(BgJSErr) != fmt.Sprint(Themes()[0].BgJSErr) {
		t.Fatal("default restored")
	}
}
