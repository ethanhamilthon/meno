package render

import (
	"encoding/json"
	"strings"
	"testing"
)

func rawThemes(t *testing.T, s string) []json.RawMessage {
	t.Helper()
	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

const goodTheme = `{"name":"mine","dark":true,"bg":"101010","fg":"#EEEEEE","dim":"777777","blue":"5555FF","purple":"AA55FF","green":"55FF55","yellow":"FFFF55","red":"FF5555","orange":"FFAA55"}`

func TestThemeFileAddsThemesWithoutPanicking(t *testing.T) {
	defer SetExtraThemes(nil)
	got, warns := ThemesFromJSON(rawThemes(t, `[`+goodTheme+`,
		{"name":"default","bg":"000000"},
		{"name":"broken","bg":"zzzzzz","fg":"111111","dim":"111111","blue":"111111","purple":"111111","green":"111111","yellow":"111111","red":"111111","orange":"111111"},
		{"bg":"000000"},
		42]`))
	if len(got) != 1 || got[0].Name != "mine" || len(warns) != 4 {
		t.Fatalf("%d themes, warnings: %v", len(got), warns)
	}
	if !strings.Contains(strings.Join(warns, "|"), "default is ignored") {
		t.Fatalf("a built-in name must be refused: %v", warns)
	}
	SetExtraThemes(got)
	if !SetTheme("mine") || CurrentTheme() != "mine" {
		t.Fatal("theme from the file cannot be activated")
	}
	if last := Themes()[len(Themes())-1]; last.Name != "mine" {
		t.Fatalf("file themes come after the built-in ones, last is %q", last.Name)
	}
	SetTheme(DefaultTheme)
}
