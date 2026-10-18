package app

import (
	"runtime"
	"strings"

	"meno/internal/config"
)

// notifyCommand implements :notify. Without arguments it shows the setting;
// "test" makes a signal; "command <cmd>" sets the command ("command" alone
// restores the default, "command bell" is the terminal bell).
func (a *App) notifyCommand(arg string) {
	word, rest, _ := strings.Cut(arg, " ")
	rest = strings.TrimSpace(rest)
	switch word {
	case "":
		a.notify("notify: " + a.st.Settings.Notify + " · command: " + a.describeNotifyCommand())
	case "test":
		event := "done"
		if rest != "" {
			event = rest
		}
		saved := a.st.Settings.Notify
		a.st.Settings.Notify = "done"
		a.signal(event)
		a.st.Settings.Notify = saved
	case "sounds":
		a.notify("notification sounds: " + strings.Join(config.Choices("notify_sound"), ", "))
	case "sound":
		if err := a.st.Settings.Set("notify_sound", rest); err != nil {
			a.notifyErr("usage: :notify sound <name> · :notify sounds lists choices")
			return
		}
		a.st.Settings.NotifyCommand = ""
		a.persistSettings()
		a.previewNotifySound()
		a.notify("notify sound: " + rest)
	case "command":
		a.st.Settings.NotifyCommand = rest
		a.persistSettings()
		a.notify("notify command: " + a.describeNotifyCommand())
	default:
		if err := a.st.Settings.Set("notify", word); err != nil {
			a.notifyErr("usage: :notify [off|question|done|test|sounds|sound <name>|command <cmd>]")
			return
		}
		a.persistSettings()
		a.notify("notify: " + word)
	}
}

func (a *App) describeNotifyCommand() string {
	switch cmd := a.st.Settings.NotifyCommand; cmd {
	case "":
		if d := soundCommand(a.st.Settings.NotifySound); d != "" {
			return d + " (default)"
		}
		return "terminal bell (default)"
	case "bell":
		return "terminal bell"
	default:
		return cmd
	}
}

// soundCommand keeps the legacy platform default while offering portable cues.
func soundCommand(name string) string {
	switch name {
	case "", "default":
		return defaultNotifyCommand(runtime.GOOS)
	case "bell":
		return "bell"
	default:
		return "sound:" + name
	}
}

// Preview ignores the notification mode but does not change it.
func (a *App) previewNotifySound() {
	saved := a.st.Settings.Notify
	a.st.Settings.Notify = "done"
	a.signal("done")
	a.st.Settings.Notify = saved
}
