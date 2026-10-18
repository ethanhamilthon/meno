package app

import (
	"testing"

	"meno/internal/config"
)

func TestNotifySoundSelectionAndPreview(t *testing.T) {
	a, scr, store := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	var s signals
	a.notifier = s.record
	a.st.Settings.NotifyCommand = "custom"
	a.runCommand("notify sound chime")
	if a.st.Settings.NotifySound != "chime" || a.st.Settings.NotifyCommand != "" || a.st.Settings.Notify != "off" {
		t.Fatalf("%+v", a.st.Settings)
	}
	if len(s.cmds) != 1 || s.cmds[0] != "sound:chime" {
		t.Fatalf("%v", s.cmds)
	}
	saved, err := config.NewStore(store).LoadSettings()
	if err != nil || saved.NotifySound != "chime" {
		t.Fatalf("%+v %v", saved, err)
	}
	a.runCommand("notify sound unknown")
	if a.st.Settings.NotifySound != "chime" || len(s.cmds) != 1 {
		t.Fatal("invalid choice changed settings")
	}
	a.runCommand("notify command custom")
	a.runCommand("notify test")
	if s.cmds[len(s.cmds)-1] != "custom" {
		t.Fatal("custom command must override sound")
	}
	a.runCommand("notify command")
	a.runCommand("notify test")
	if s.cmds[len(s.cmds)-1] != "sound:chime" {
		t.Fatal("reset must restore selected sound")
	}
	a.runCommand("notify sounds")
	if a.lastNotice() == "" {
		t.Fatal("missing sound list")
	}
}

func TestSettingsSoundCyclePreviewsAndPersists(t *testing.T) {
	a, scr, store := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	var s signals
	a.notifier = s.record
	for i, row := range settingRows {
		if row.key == "notify_sound" {
			a.st.Sel = i
		}
	}
	a.st.Settings.NotifyCommand = "custom"
	a.cycleSetting(1)
	if a.st.Settings.NotifySound != "bell" || a.st.Settings.NotifyCommand != "" || a.st.Settings.Notify != "off" {
		t.Fatalf("%+v", a.st.Settings)
	}
	if len(s.cmds) != 1 || s.cmds[0] != "bell" {
		t.Fatalf("%v", s.cmds)
	}
	saved, err := config.NewStore(store).LoadSettings()
	if err != nil || saved.NotifySound != "bell" {
		t.Fatalf("%+v %v", saved, err)
	}
	a.cycleSetting(-1)
	a.cycleSetting(-1)
	opts := config.Choices("notify_sound")
	if a.st.Settings.NotifySound != opts[len(opts)-1] {
		t.Fatal("cycle must wrap")
	}
}
