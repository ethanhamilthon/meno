package config

import "testing"

func TestBashStringsDefaultsAreUsable(t *testing.T) {
	d := DefaultSettings()
	if d.BashStrings != "off" || d.BashStringLines != 3 {
		t.Fatalf("defaults: %q %d", d.BashStrings, d.BashStringLines)
	}
	// Settings saved before the default existed hold an empty value.
	s := Settings{}
	if got := s.Get("bash_strings"); got != "" {
		t.Fatalf("zero value %q", got)
	}
	if err := s.Set("bash_strings", "lines"); err != nil || s.Get("bash_strings") != "lines" {
		t.Fatalf("set: %v %q", err, s.Get("bash_strings"))
	}
}
