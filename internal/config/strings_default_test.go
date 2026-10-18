package config

import "testing"

func TestJSStringsDefaultsAreUsable(t *testing.T) {
	d := DefaultSettings()
	if d.JSStrings != "off" || d.JSStringLines != 3 {
		t.Fatalf("defaults: %q %d", d.JSStrings, d.JSStringLines)
	}
	// Settings saved before the default existed hold an empty value.
	s := Settings{}
	if got := s.Get("js_strings"); got != "" {
		t.Fatalf("zero value %q", got)
	}
	if err := s.Set("js_strings", "lines"); err != nil || s.Get("js_strings") != "lines" {
		t.Fatalf("set: %v %q", err, s.Get("js_strings"))
	}
}
