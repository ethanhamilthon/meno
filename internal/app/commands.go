package app

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"meno/internal/client"
	"meno/internal/proto"
	"meno/internal/ui"
)

// interruptsAgent lists commands that would pull the session, VM or directory
// out from under a running agent of the current session. Everything else
// (dialogs, export...) is safe, and so is working with other sessions.
var interruptsAgent = map[string]bool{
	"fork": true, "compact": true, "cd": true,
}

func (a *App) runCommand(raw string) {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return
	}
	name := fields[0]
	if a.st.Running && interruptsAgent[name] {
		a.notifyErr("agent is running · /stop it first (or ctrl+c)")
		return
	}
	args := fields[1:]
	switch name {
	case "q", "quit", "q!", "quit!":
		// Leaving only detaches this terminal: runs and tasks stay in the daemon.
		// "q!" used to cancel everything; it is kept as an alias of the safe exit.
		a.quit = true
	case "drafts":
		a.restoreDrafts()
	case "stop-all":
		a.stopAll()
	case "edit":
		if len(args) != 0 {
			a.notifyErr("edit takes no arguments")
			return
		}
		a.endInsert()
		a.startEdit()
	case "new":
		a.newSession()
		a.st.Mode = ui.ModeInput
	case "cd":
		a.changeDir(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), name)))
	case "fork":
		a.openFork()
	case "rename":
		a.renameSession(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), name)))
	case "tasks":
		a.openTasks()
	case "compact":
		a.compact()
	case "export":
		a.exportSession(args)
	case "stop":
		a.abortTurn()
	case "context":
		if len(args) != 0 {
			a.notifyErr("context takes no arguments")
			return
		}
		a.openOverlay(OverContext)
	case "help":
		a.openOverlay(OverHelp)
	case "model":
		a.fetchModels()
		a.openOverlay(OverModel)
	case "settings":
		a.fetchModels()
		a.openOverlay(OverSettings)
	case "login":
		a.openLogin()
	case "skills":
		a.reloadSkills()
		a.openOverlay(OverSkills)
	case "mcp":
		a.openOverlay(OverMCP)
	case "sessions":
		a.openSessions()
	default:
		a.notifyErr("unknown command :" + name)
	}
}

// changeDir implements /cd for the current session. arg may be a path
// (relative to the session's directory, or starting with ~) or "-" for the
// previous directory.
func (a *App) changeDir(arg string) {
	if arg == "" {
		a.notify("working directory: " + a.st.Cwd)
		return
	}
	var target string
	switch {
	case arg == "-":
		if a.st.PrevCwd == "" {
			a.notifyErr("cd: no previous directory")
			return
		}
		target = a.st.PrevCwd
	default:
		target = expandHome(arg)
		if !filepath.IsAbs(target) {
			target = filepath.Join(a.st.Cwd, target)
		}
		target = filepath.Clean(target)
	}
	if !dirExists(target) {
		if _, err := os.Stat(target); err == nil {
			a.notifyErr("cd: not a directory: " + target)
		} else {
			a.notifyErr("cd: no such directory: " + target)
		}
		return
	}
	if target == a.st.Cwd {
		a.notify("already in " + target)
		return
	}
	a.setCwd(a.st.Runtime, target)
	a.reloadSkills()
	a.notify("working directory: " + target)
}

// cancelAll stops the running turn of every followed session.
// stopAll cancels every running turn, but only after the same command is
// repeated within ten seconds: bulk cancellation is never implicit.
func (a *App) stopAll() {
	var running []string
	for _, rt := range a.rtList {
		if rt.Running {
			running = append(running, "“"+rt.sessionTitle()+"”")
		}
	}
	if len(running) == 0 {
		a.notify("no turn is running")
		return
	}
	if now := nowFn(); a.stopAllAsk.IsZero() || now.Sub(a.stopAllAsk) > 10*time.Second {
		a.stopAllAsk = now
		a.notifyErr("this stops " + strings.Join(running, ", ") + " · repeat /stop-all within 10s to confirm")
		return
	}
	a.stopAllAsk = time.Time{}
	a.cancelAll()
	a.notify("stopped " + strconv.Itoa(len(running)) + " turn(s)")
}

func (a *App) cancelAll() {
	for _, rt := range a.rtList {
		if rt.Running {
			sid, rid := rt.ID(), rt.RequestID
			a.rpc(func(ctx context.Context, c *client.Client) error {
				return c.Stop(ctx, sid, proto.StopRequest{RequestID: rid})
			})
		}
	}
}

// markUnseen flags a finished turn of a session that is not on screen.
func (a *App) markUnseen(sid string) {
	if rt := a.rts[sid]; rt != nil {
		rt.Unseen = true
	}
}

// animating reports whether something on screen changes without input: a
// running turn in any session, or the /sessions list with live statuses.
func (a *App) animating() bool {
	if a.st.Over == OverSessions || a.bgAny() {
		return true
	}
	for _, rt := range a.rtList {
		if rt.Running {
			return true
		}
	}
	return false
}
