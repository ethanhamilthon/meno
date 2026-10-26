package tui

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestHelpListsEveryCommand keeps /help in step with runCommand: each command
// the switch accepts has a row, so a new command cannot ship undocumented.
func TestHelpListsEveryCommand(t *testing.T) {
	src, err := os.ReadFile("commands.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	body = body[strings.Index(body, "func (a *App) runCommand"):]
	body = body[:strings.Index(body, "default:")]
	keys := map[string]bool{}
	for _, r := range helpRows {
		for _, k := range strings.Fields(strings.ReplaceAll(r.key, ",", " ")) {
			keys[k] = true
		}
	}
	names := 0
	for _, m := range regexp.MustCompile(`case ((?:"[a-z]+"(?:, )?)+):`).FindAllStringSubmatch(body, -1) {
		for _, n := range regexp.MustCompile(`"([a-z]+)"`).FindAllStringSubmatch(m[1], -1) {
			names++
			if !keys["/"+n[1]] {
				t.Errorf("/help has no row for /%s", n[1])
			}
		}
	}
	if names < 15 {
		t.Fatalf("parsed only %d commands from runCommand", names)
	}
}

var modeWord = regexp.MustCompile(`(?i)\bmodes?\b`)

func TestHelpHasNoModes(t *testing.T) {
	for _, r := range helpRows {
		if modeWord.MatchString(r.desc) {
			t.Fatalf("help names a mode that no longer exists: %q", r.desc)
		}
	}
	for _, want := range []string{"@", "Ctrl+V", "Enter"} {
		found := false
		for _, r := range helpRows {
			found = found || strings.HasPrefix(r.key, want)
		}
		if !found {
			t.Fatalf("help has no row for %s", want)
		}
	}
}

// The help list is long. On a normal terminal it must leave the status bar
// visible: it carries the counters (running, done) that a user waits for.
func TestHelpLeavesTheStatusBarVisible(t *testing.T) {
	a, s := visualFixture(t, 100, 32)
	a.openOverlay(OverHelp)
	a.draw()
	last := strings.TrimSpace(visualRow(s, 31))
	if !strings.Contains(last, "gpt-5.5") {
		t.Fatalf("the status bar is covered by help; last row: %q", last)
	}
	if first := strings.TrimSpace(visualRow(s, 0)); first != "" {
		t.Fatalf("help reaches the top row: %q", first)
	}
}
