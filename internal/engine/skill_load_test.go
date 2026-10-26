package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"meno/internal/api"
	"meno/internal/config"
	"meno/internal/llm"
)

func writeSkill(t *testing.T, cwd, name, text string) {
	t.Helper()
	dir := filepath.Join(cwd, ".meno", "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNamedAndAutoSkillsLoadIntoTheFirstMessage(t *testing.T) {
	step := []llm.Event{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}}
	f := &fakeProvider{steps: [][]llm.Event{step, step}}
	e, _ := fixture(t, f)
	cwd := t.TempDir()
	writeSkill(t, cwd, "plan", "---\nname: plan\ndescription: plan things\n---\nMake a plan.")
	writeSkill(t, cwd, "always", "---\nname: always\ndescription: on\n---\nBe brief.")
	writeSkill(t, cwd, "off", "---\nname: off\ndescription: disabled\n---\nHidden.")
	paths := e.paths
	paths.Cwd = cwd
	store := config.NewStore(paths)
	list, err := store.LoadSkills()
	if err != nil {
		t.Fatal(err)
	}
	for i := range list {
		list[i].Auto = list[i].Name == "always" || list[i].Name == "off"
		list[i].Enabled = list[i].Name != "off"
	}
	if err = store.SaveSkills(list); err != nil {
		t.Fatal(err)
	}
	r, err := e.CreateSession(context.Background(), api.SessionCreateRequest{Cwd: cwd, ClientID: "c"})
	if err != nil {
		t.Fatal(err)
	}
	id := r.Session.ID
	if _, err = e.Message(context.Background(), id, api.MessageRequest{Text: "go <skill>plan</skill> and <skill>plan</skill>", ClientID: "c"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, e, id, func(s *runState) bool { return !s.running })
	snap, err := e.Snapshot(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	var skills []string
	user := ""
	for _, turn := range snap.Turns {
		switch {
		case turn.Kind == "skill":
			skills = append(skills, turn.Content)
		case turn.Role == "user" && turn.Kind == "":
			user = turn.Content
		}
	}
	if len(skills) != 2 || !strings.Contains(skills[0], `name="plan"`) || !strings.Contains(skills[0], "Make a plan.") || !strings.Contains(skills[1], "Be brief.") {
		t.Fatalf("skill turns: %q", skills)
	}
	if strings.Contains(skills[0], "description:") || strings.Contains(strings.Join(skills, ""), "Hidden.") {
		t.Fatalf("frontmatter or a disabled skill leaked: %q", skills)
	}
	if user != "go <skill>plan</skill> and <skill>plan</skill>\n\n<skill>always</skill>" {
		t.Fatalf("auto skill missing in the message: %q", user)
	}
	if snap.Session.Name != "go /skill:plan and /skill:plan" {
		t.Fatalf("auto references contaminated title: %q", snap.Session.Name)
	}
	// A later message loads only what it names, and never adds the auto skills again.
	if _, err = e.Message(context.Background(), id, api.MessageRequest{Text: "next", ClientID: "c"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, e, id, func(s *runState) bool { return !s.running })
	snap, _ = e.Snapshot(context.Background(), id)
	n := 0
	for _, turn := range snap.Turns {
		if turn.Kind == "skill" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("skill turns after a second message: %d", n)
	}
}
