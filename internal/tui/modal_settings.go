package tui

import (
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/config"
)

type settingRow struct {
	label string
	key   string
}

var settingRows = []settingRow{
	{"system prompt", "system_prompt"}, {"view", "view"}, {"notify", "notify"},
	{"notify sound", "notify_sound"}, {"notify volume", "notify_volume"},
	{"notify command", "notify_command"}, {"auto compact", "compact_percent"},
}

func (a *App) settingsKey(k *tcell.EventKey) {
	switch {
	case k.Key() == tcell.KeyDown || isKey(k, 'j'):
		a.st.Sel = min(a.st.Sel+1, len(settingRows)-1)
		return
	case k.Key() == tcell.KeyUp || isKey(k, 'k'):
		a.st.Sel = max(a.st.Sel-1, 0)
		return
	case config.TextSetting(settingRows[clampSel(a.st.Sel, len(settingRows))].key) && (k.Key() == tcell.KeyEnter || k.Rune() == ' '):
		key := settingRows[clampSel(a.st.Sel, len(settingRows))].key
		a.editSetting(key)
	case k.Key() == tcell.KeyLeft || k.Rune() == ' ' || k.Key() == tcell.KeyEnter:
		step := 1
		if k.Key() == tcell.KeyLeft {
			step = -1
		}
		a.cycleSetting(step)
	case k.Key() == tcell.KeyRight:
		a.cycleSetting(1)
	case isKey(k, 's'):
		a.st.LocalScope = !a.st.LocalScope
		return
	case isKey(k, 'r'):
		a.resetLocal()
		return
	case k.Rune() == '+' || k.Rune() == '=' || k.Rune() == '-':
		key := settingRows[clampSel(a.st.Sel, len(settingRows))].key
		if config.NumOption(key) == "" {
			return
		}
		delta := 1
		if k.Rune() == '-' {
			delta = -1
		}
		a.st.Settings.AdjustNum(key, delta)
		a.persistSettings()
	}
}

// editSetting types a free-text setting, such as the search key, in the composer.
func (a *App) editSetting(key string) {
	label := key
	for _, r := range settingRows {
		if r.key == key {
			label = r.label
		}
	}
	a.beginEntry(entry{label: label + " ", hint: "type the value", done: func(text string) error {
		if err := a.st.Settings.Set(key, strings.TrimSpace(text)); err != nil {
			return err
		}
		a.noteSettingEdit(key)
		if key == "notify_command" {
			a.previewNotifySound()
		}
		a.persistSettings()
		return nil
	}}, a.st.Settings.Get(key))
}

func (a *App) settingChoices(key string) []string {
	switch key {
	case "system_prompt":
		// The built-in prompt, then the files of system-prompts/. A name that
		// is set but has no file stays listed, so the row shows what is stored.
		out := append([]string{config.DefaultSystemPrompt}, a.paths.ListSystemPrompts()...)
		if cur := a.st.Settings.SystemPrompt; cur != "" && !slices.Contains(out, cur) {
			out = append(out, cur)
		}
		return out
	default:
		return config.Choices(key)
	}
}

func (a *App) cycleSetting(step int) {
	key := settingRows[clampSel(a.st.Sel, len(settingRows))].key
	opts := a.settingChoices(key)
	if len(opts) == 0 {
		return
	}
	cur := a.st.Settings.Get(key)
	idx := 0
	for i, o := range opts {
		if o == cur {
			idx = i
			break
		}
	}
	n := len(opts)
	next := opts[((idx+step)%n+n)%n]
	_ = a.st.Settings.Set(key, next)
	a.noteSettingEdit(key)
	switch key {
	case "notify_sound":
		a.st.Settings.NotifyCommand = ""
		a.noteSettingEdit("notify_command")
		a.previewNotifySound()
	case "notify_volume":
		a.previewNotifySound()
	}
	a.persistSettings()
}
