package app

import (
	"reflect"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/config"
)

func withSkills(a *App) {
	a.st.Skills = []config.Skill{
		{Name: "plan", Desc: "write a plan", Enabled: true},
		{Name: "planet", Desc: "other", Enabled: true},
		{Name: "off", Desc: "disabled", Enabled: false},
		{Name: "new-thing", Desc: "clashes with /new", Enabled: true},
	}
}

func TestSkillRowsTable(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	withSkills(a)
	cases := []struct {
		q    string
		want []string
	}{
		{"pla", []string{"/skill:plan", "/skill:planet"}},
		{"PLAN", []string{"/skill:plan", "/skill:planet"}},
		{"of", nil},  // disabled skills are not offered
		{"new", nil}, // an exact command wins
		{"new-", []string{"/skill:new-thing"}},
		{"", nil},
		{"cd /tmp", nil}, // a command with arguments
		{"skill:pl", nil},
	}
	for _, c := range cases {
		if got := a.skillRows(c.q); !reflect.DeepEqual(got, c.want) {
			t.Errorf("skillRows(%q) = %v, want %v", c.q, got, c.want)
		}
	}
}

// "/pla" + Tab puts the skill into the draft as one object; nothing is run or
// sent, and the command popup closes.
func TestSlashSkillInsertsObjectAndDoesNotSend(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	withSkills(a)
	typeRunes(a, "/pla")
	if rows := commandRows(a); len(rows) != 2 || rows[0] != "/skill:plan" {
		t.Fatalf("rows %v", rows)
	}
	a.onKey(keyK(tcell.KeyEnter))
	if a.st.Running || a.st.sending != 0 {
		t.Fatal("selecting a skill sent the draft")
	}
	n := 0
	for _, r := range a.st.Input {
		if isObjRune(r) {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d objects in the draft: %q", n, string(a.st.Input))
	}
	if got := externalText(a.st.Input); got != "<skill>plan</skill> " {
		t.Fatalf("draft text %q", got)
	}
	if a.cmdActive() {
		t.Fatal("command popup still open")
	}
}
