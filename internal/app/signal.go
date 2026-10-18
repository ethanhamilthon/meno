package app

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"meno/internal/notify"
	"meno/internal/safe"
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
		cmd = soundCommand(s.NotifySound)
	}
	volume := s.NotifyVolume
	env := []string{"MENO_NOTIFY_EVENT=" + event}
	if a.st.Session != nil {
		env = append(env, "MENO_NOTIFY_SESSION="+a.st.Session.ID, "MENO_NOTIFY_TITLE="+a.st.Session.DisplayTitle())
	}
	if a.notifier != nil {
		a.notifier(cmd, env)
		return
	}
	a.notifyNow(cmd, env, volume)
}

// notifyNow rings the bell or starts the command in the background.
func (a *App) notifyNow(cmd string, env []string, volume int) {
	if cmd == "" || cmd == "bell" {
		_ = a.scr.Beep()
		return
	}
	go func() {
		defer safe.Recover(a.reportPanic)
		ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
		defer cancel()
		if name, ok := strings.CutPrefix(cmd, "sound:"); ok {
			if err := notify.Play(ctx, name, volume); err != nil {
				_ = a.scr.Beep()
			}
			return
		}
		c := exec.CommandContext(ctx, "sh", "-c", cmd)
		c.Env = append(os.Environ(), env...)
		_ = c.Run()
	}()
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
