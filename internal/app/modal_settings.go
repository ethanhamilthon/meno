package app

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/config"
	"meno/internal/ui"
)

type settingRow struct {
	label string
	key   string
}

type settingTab struct {
	name string
	rows []settingRow
}

var settingTabs = []settingTab{
	{
		name: "General",
		rows: []settingRow{
			{"theme", "theme"},
			{"images", "images"},
		},
	},
	{
		name: "Display",
		rows: []settingRow{
			{"js code", "js_code"},
			{"js output", "js_output"},
			{"js strings", "js_strings"},
			{"js timing", "js_timing"},
			{"thinking", "thinking"},
			{"turn summary", "turn_summary"},
		},
	},
	{
		name: "Web",
		rows: []settingRow{
			{"search", "websearch"},
			{"search key", "websearch_key"},
		},
	},
	{
		name: "Notify",
		rows: []settingRow{
			{"notify", "notify"},
			{"notify sound", "notify_sound"},
			{"notify volume", "notify_volume"},
			{"notify command", "notify_command"},
		},
	},
	{
		name: "Agents",
		rows: []settingRow{
			{"subagent provider", "subagent_provider"},
			{"subagent model", "subagent_model"},
			{"subagent effort", "subagent_effort"},
		},
	},
	{
		name: "Context",
		rows: []settingRow{
			{"auto compact", "compact_percent"},
			{"js timeout", "js_timeout"},
		},
	},
}

var settingRows = flattenSettingRows()

func flattenSettingRows() []settingRow {
	var out []settingRow
	for _, t := range settingTabs {
		out = append(out, t.rows...)
	}
	return out
}

func settingTabFor(sel int) (tabIdx, tabStart, rowInTab int, tab settingTab) {
	sel = clampSel(sel, len(settingRows))
	offset := 0
	for i, t := range settingTabs {
		if sel < offset+len(t.rows) {
			return i, offset, sel - offset, t
		}
		offset += len(t.rows)
	}
	last := len(settingTabs) - 1
	t := settingTabs[last]
	return last, offset - len(t.rows), 0, t
}

func settingTabStart(tabIdx int) int {
	n := len(settingTabs)
	tabIdx = ((tabIdx % n) + n) % n
	offset := 0
	for i := 0; i < tabIdx; i++ {
		offset += len(settingTabs[i].rows)
	}
	return offset
}

func (a *App) settingsKey(k *tcell.EventKey) {
	if a.st.Editing {
		a.settingTextKey(k)
		return
	}
	tabIdx, tabStart, rowInTab, tab := settingTabFor(a.st.Sel)
	switch {
	case k.Key() == tcell.KeyTab || k.Rune() == ']':
		a.st.Sel = settingTabStart(tabIdx + 1)
		return
	case k.Key() == tcell.KeyBacktab || k.Rune() == '[':
		a.st.Sel = settingTabStart(tabIdx - 1)
		return
	case k.Key() == tcell.KeyDown || isKey(k, 'j'):
		a.st.Sel = tabStart + (rowInTab+1)%len(tab.rows)
		return
	case k.Key() == tcell.KeyUp || isKey(k, 'k'):
		a.st.Sel = tabStart + (rowInTab-1+len(tab.rows))%len(tab.rows)
		return
	case config.TextSetting(settingRows[clampSel(a.st.Sel, len(settingRows))].key) && (k.Key() == tcell.KeyEnter || k.Rune() == ' '):
		key := settingRows[clampSel(a.st.Sel, len(settingRows))].key
		a.beginEdit(a.st.Settings.Get(key), false, false)
	case k.Key() == tcell.KeyLeft || k.Rune() == ' ' || k.Key() == tcell.KeyEnter:
		step := 1
		if k.Key() == tcell.KeyLeft {
			step = -1
		}
		a.cycleSetting(step)
	case k.Key() == tcell.KeyRight:
		a.cycleSetting(1)
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

// settingTextKey edits a free-text setting such as the search key.
func (a *App) settingTextKey(k *tcell.EventKey) {
	key := settingRows[clampSel(a.st.Sel, len(settingRows))].key
	switch {
	case k.Key() == tcell.KeyEsc:
		a.endEdit()
	case k.Key() == tcell.KeyEnter:
		if err := a.st.Settings.Set(key, strings.TrimSpace(a.st.Edit.String())); err != nil {
			a.notifyErr(err.Error())
			return
		}
		a.endEdit()
		if key == "notify_command" {
			a.previewNotifySound()
		}
		a.persistSettings()
	default:
		a.st.Edit.Handle(k)
	}
}

func (a *App) settingChoices(key string) []string {
	switch key {
	case "theme":
		themes := ui.Themes()
		out := make([]string, len(themes))
		for i, t := range themes {
			out[i] = t.Name
		}
		return out
	case "subagent_provider":
		out := []string{""}
		seen := map[string]bool{"": true}
		for _, p := range a.st.Providers {
			if p.Name != "" && !seen[p.Name] {
				out = append(out, p.Name)
				seen[p.Name] = true
			}
		}
		if cur := a.st.Settings.SubagentProvider; !seen[cur] {
			out = append(out, cur)
		}
		return out
	case "subagent_model":
		out := []string{""}
		seen := map[string]bool{"": true}
		add := func(id string) {
			if id != "" && !seen[id] {
				out = append(out, id)
				seen[id] = true
			}
		}
		targetProv := a.st.Settings.SubagentProvider
		if targetProv != "" {
			for _, m := range a.st.Models[targetProv] {
				add(m.ID)
			}
		} else {
			for _, m := range a.modelList() {
				add(m.ID)
			}
		}
		if a.st.Session != nil {
			add(a.st.Session.Model)
		}
		add(a.st.Settings.SubagentModel)
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
	switch key {
	case "theme":
		if !ui.SetTheme(a.st.Settings.Theme) {
			ui.SetTheme(ui.DefaultTheme)
		}
	case "notify_sound":
		a.st.Settings.NotifyCommand = ""
		a.previewNotifySound()
	case "notify_volume":
		a.previewNotifySound()
	}
	a.persistSettings()
}
