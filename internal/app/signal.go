package app

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// notifyTimeout bounds how long a notify command may run.
const notifyTimeout = 5 * time.Second

// signalEvents are the events a signal can announce.
//
//	question: the agent asks the user something
//	done:     a turn ended
//	error:    a turn ended with an error
//
// Which of them make a signal depends on the notify setting.
func wantSignal(mode, event string) bool {
	switch mode {
	case "question":
		return event == "question"
	case "done":
		return true
	}
	return false
}

// defaultNotifyCommand is what an empty notify_command means on goos; "" is the
// terminal bell.
func defaultNotifyCommand(goos string) string {
	if goos == "darwin" {
		return "afplay /System/Library/Sounds/Glass.aiff"
	}
	return ""
}

// signal tells the user that the current session needs them (see wantSignal).
func (a *App) signal(event string) {
	s := a.st.Settings
	if !wantSignal(s.Notify, event) {
		return
	}
	cmd := s.NotifyCommand
	if cmd == "" {
		cmd = defaultNotifyCommand(runtime.GOOS)
	}
	env := []string{"MENO_NOTIFY_EVENT=" + event}
	if a.st.Session != nil {
		env = append(env, "MENO_NOTIFY_SESSION="+a.st.Session.ID, "MENO_NOTIFY_TITLE="+a.st.Session.DisplayTitle())
	}
	if a.notifier != nil {
		a.notifier(cmd, env)
		return
	}
	a.notifyNow(cmd, env)
}

// notifyNow rings the bell or starts the command in the background.
func (a *App) notifyNow(cmd string, env []string) {
	if cmd == "" || cmd == "bell" {
		_ = a.scr.Beep()
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
		defer cancel()
		c := exec.CommandContext(ctx, "sh", "-c", cmd)
		c.Env = append(os.Environ(), env...)
		_ = c.Run()
	}()
}

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
	case "command":
		a.st.Settings.NotifyCommand = rest
		a.persistSettings()
		a.notify("notify command: " + a.describeNotifyCommand())
	default:
		if err := a.st.Settings.Set("notify", word); err != nil {
			a.notifyErr("usage: :notify [off|question|done|test|command <cmd>]")
			return
		}
		a.persistSettings()
		a.notify("notify: " + word)
	}
}

func (a *App) describeNotifyCommand() string {
	switch cmd := a.st.Settings.NotifyCommand; cmd {
	case "":
		if d := defaultNotifyCommand(runtime.GOOS); d != "" {
			return d + " (default)"
		}
		return "terminal bell (default)"
	case "bell":
		return "terminal bell"
	default:
		return cmd
	}
}
