package app

import (
	"testing"

	"meno/internal/config"
)

func TestSettingsSoundAndVolumeCyclePreviewsAndPersists(t *testing.T) {
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

	// Cycling notify_volume previews the sound and persists the volume.
	for i, row := range settingRows {
		if row.key == "notify_volume" {
			a.st.Sel = i
		}
	}
	before := len(s.cmds)
	a.cycleSetting(1)
	if a.st.Settings.NotifyVolume != 20 || len(s.cmds) != before+1 {
		t.Fatalf("volume=%d cmds=%v", a.st.Settings.NotifyVolume, s.cmds)
	}
	saved, err = config.NewStore(store).LoadSettings()
	if err != nil || saved.NotifyVolume != 20 {
		t.Fatalf("volume not saved: %+v %v", saved, err)
	}
}
