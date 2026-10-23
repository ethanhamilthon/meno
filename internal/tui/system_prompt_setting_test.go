package tui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"meno/internal/config"
)

// The settings row lists the built-in prompt and the files of system-prompts/,
// and cycling through it stores a name the daemon can resolve.
func TestSystemPromptSettingListsTheFilesAndCycles(t *testing.T) {
	paths := config.Paths{Home: t.TempDir()}
	if err := os.MkdirAll(paths.SystemPromptsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"writer.md", "reviewer.md", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(paths.SystemPromptsDir(), f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a := &App{paths: paths, st: NewState()}
	if got := a.settingChoices("system_prompt"); !slices.Equal(got, []string{"default", "reviewer", "writer"}) {
		t.Fatalf("choices: %v", got)
	}
	a.st.Settings.SystemPrompt = "gone"
	if got := a.settingChoices("system_prompt"); !slices.Contains(got, "gone") {
		t.Fatalf("a stored name without a file must stay visible: %v", got)
	}
	a.st.Settings.SystemPrompt = ""
	if cur := a.st.Settings.Get("system_prompt"); cur != "default" {
		t.Fatalf("current: %q", cur)
	}
	if err := a.st.Settings.Set("system_prompt", "writer"); err != nil || a.st.Settings.SystemPrompt != "writer" {
		t.Fatalf("%+v %v", a.st.Settings, err)
	}
	if err := a.st.Settings.Set("system_prompt", "default"); err != nil || a.st.Settings.SystemPrompt != "" {
		t.Fatalf("%+v %v", a.st.Settings, err)
	}
}
