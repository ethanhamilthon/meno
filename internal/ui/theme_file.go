package ui

import (
	"encoding/json"
	"fmt"
	"strings"
)

// extraThemes are the themes of theme.json. They are shown after the built-in
// ones and never replace them.
var extraThemes []Theme

// SetExtraThemes replaces the themes loaded from theme.json.
func SetExtraThemes(ts []Theme) { extraThemes = append([]Theme(nil), ts...) }

type themeSpec struct {
	Name   string `json:"name"`
	Dark   bool   `json:"dark"`
	Bg     string `json:"bg"`
	Fg     string `json:"fg"`
	Dim    string `json:"dim"`
	Blue   string `json:"blue"`
	Purple string `json:"purple"`
	Green  string `json:"green"`
	Yellow string `json:"yellow"`
	Red    string `json:"red"`
	Orange string `json:"orange"`
}

// parseHex6 accepts "RRGGBB" or "#RRGGBB" and returns it without the "#".
func parseHex6(s string) (string, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return "", false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return "", false
		}
	}
	return s, true
}

// ThemesFromJSON builds themes from the entries of theme.json. Each entry is
// {"name", "dark", "bg", "fg", "dim", "blue", "purple", "green", "yellow",
// "red", "orange"} with colors as RRGGBB. Invalid entries and names that are
// already taken are skipped and reported as warnings; nothing here panics.
func ThemesFromJSON(raw []json.RawMessage) ([]Theme, []string) {
	var out []Theme
	var warns []string
	taken := map[string]bool{}
	for _, t := range themes {
		taken[t.Name] = true
	}
	for i, r := range raw {
		var s themeSpec
		if err := json.Unmarshal(r, &s); err != nil {
			warns = append(warns, fmt.Sprintf("theme.json: entry %d: %v", i+1, err))
			continue
		}
		name := strings.TrimSpace(s.Name)
		if name == "" {
			warns = append(warns, fmt.Sprintf("theme.json: entry %d: name is required", i+1))
			continue
		}
		if taken[name] {
			warns = append(warns, fmt.Sprintf("theme.json: %s is ignored: a theme with this name exists", name))
			continue
		}
		cols := map[string]string{"bg": s.Bg, "fg": s.Fg, "dim": s.Dim, "blue": s.Blue, "purple": s.Purple,
			"green": s.Green, "yellow": s.Yellow, "red": s.Red, "orange": s.Orange}
		bad := ""
		for _, k := range []string{"bg", "fg", "dim", "blue", "purple", "green", "yellow", "red", "orange"} {
			h, ok := parseHex6(cols[k])
			if !ok {
				bad = k
				break
			}
			cols[k] = h
		}
		if bad != "" {
			warns = append(warns, fmt.Sprintf("theme.json: %s: %s must be a color like RRGGBB", name, bad))
			continue
		}
		taken[name] = true
		out = append(out, derive(name, s.Dark, cols["bg"], cols["fg"], cols["dim"], cols["blue"], cols["purple"],
			cols["green"], cols["yellow"], cols["red"], cols["orange"]))
	}
	return out, warns
}
