package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/safe"
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
		a.notifyErr("agent is running · :stop it first (or ctrl+c)")
		return
	}
	args := fields[1:]
	switch name {
	case "q", "quit":
		if busy := a.busySessions(); len(busy) > 0 {
			a.notifyErr("not quitting: " + strings.Join(busy, ", ") + " · :q! quits anyway")
			return
		}
		a.quit = true
	case "q!", "quit!":
		a.cancelAll()
		a.quit = true
	case "new":
		a.newSession()
	case "cd":
		a.changeDir(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), name)))
	case "fork":
		a.openFork()
	case "rename":
		a.renameSession(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), name)))
	case "cache":
		a.cacheReport()
	case "agents":
		a.openAgents()
	case "tasks":
		a.openTasks()
	case "compact":
		a.compact()
	case "notify":
		a.notifyCommand(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), name)))
	case "attach":
		a.attachCommand(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), name)))
	case "export":
		a.exportSession(args)
	case "stop":
		a.abortTurn()
	case "help":
		a.openOverlay(OverHelp)
	case "model":
		a.fetchModels()
		a.openOverlay(OverModel)
	case "settings":
		a.openOverlay(OverSettings)
	case "providers":
		a.openOverlay(OverProviders)
	case "skills":
		a.reloadSkills()
		a.openOverlay(OverSkills)
	case "theme":
		a.openTheme()
	case "mcp":
		a.openOverlay(OverMCP)
	case "sessions":
		a.openSessions()
	default:
		a.notifyErr("unknown command :" + name)
	}
}

// changeDir implements :cd for the current session. arg may be a path
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

// setCwd moves a session to dir: its VM root, its saved directory and what the
// next system prompt or event says about it.
func (a *App) setCwd(rt *Runtime, dir string) {
	if rt.Prompt.Base == nil && !rt.Empty() && rt.Prompt.CwdFrom == "" {
		// The model has not been told a directory in this segment yet.
		rt.Prompt.CwdFrom = rt.Cwd
	}
	rt.PrevCwd, rt.Cwd = rt.Cwd, dir
	rt.VM.SetRoot(dir)
	rt.Session.Cwd = dir
	if rt.Empty() {
		rt.Prompt = promptState{} // built for the new directory at the first message
	}
	a.saveRT(rt)
}

// cancelAll stops the running turn of every session.
func (a *App) cancelAll() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, rt := range a.rtList {
		if rt.cancel != nil {
			rt.cancel()
		}
	}
}

// markUnseen flags a finished turn of a session that is not on screen.
func (a *App) markUnseen(sid string) {
	if rt := a.rts[sid]; rt != nil && rt != a.st.Runtime {
		rt.Unseen = true
	}
}

// animating reports whether something on screen changes without input: a
// running turn in any session, or the :sessions list with live statuses.
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

func (a *App) saveSession() { a.saveRT(a.st.Runtime) }

// saveRT stores a session. Sessions nobody ever wrote to are not stored, and
// neither is one whose turn is running (that goroutine is still changing it);
// its turn saves it when it ends.
func (a *App) saveRT(rt *Runtime) {
	if rt.Running && !a.closing {
		return
	}
	if rt.Empty() && !rt.Saved && len(rt.Session.Requests) == 0 {
		return
	}
	rt.Session.Metrics = agent.Metrics{LastUsage: rt.LastUsage, Usage: rt.Usage, Spent: rt.Spent, Context: rt.Context, Unpriced: rt.Unpriced}
	if err := agent.SaveSession(a.paths.Home, rt.Session); err != nil {
		a.inRT(rt.ID(), func() { a.notifyErr("save failed: " + err.Error()) })
		return
	}
	rt.Saved = true
}

func (a *App) compact() {
	p, ok := a.currentProvider()
	if !ok {
		a.notifyErr("provider not configured")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.st.cancel = cancel
	a.mu.Unlock()
	a.st.Running = true
	a.st.LastErr = false
	a.st.Phase = "compacting"
	_, model, effort := a.selection()
	sess, sid, client := a.st.Session, a.st.Session.ID, a.client(p)
	// The same system prompt and tools as the session's requests: the summary is
	// served from their cache. Before the first request there is no prompt.
	opts := agent.CompactOptions{System: a.st.Prompt.Frozen, Effort: effort, ModelTimeout: a.st.Settings.ModelTimeout()}
	a.running.Add(1)
	go func() {
		defer safe.Recover(a.reportPanic)
		defer a.running.Done()
		defer cancel()
		before, after, err := agent.CompactWith(ctx, client, p, model, sess, opts, func(u llm.Usage) {
			a.postEvent(agentEv{sid: sid, ev: agent.Ev{Kind: agent.EvUsage, Usage: u, Provider: p, Model: model}})
		})
		a.postEvent(compactDone{sid: sid, before: before, after: after, err: err})
	}()
}

func (a *App) exportSession(args []string) {
	name := "meno-session.md"
	if len(args) > 0 {
		name = args[0]
	}
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(a.st.Cwd, path)
	}
	var b strings.Builder
	show := func(p string) string {
		if rel, err := filepath.Rel(filepath.Dir(path), p); err == nil {
			p = rel
		}
		return "![image](" + p + ")"
	}
	for _, t := range a.st.Session.Snapshot() {
		switch {
		case t.Role == llm.RoleUser && t.Kind == agent.KindView:
			b.WriteString("*" + a.st.Session.TextWithImagePaths(t, show) + "*\n\n")
		case t.Role == llm.RoleUser:
			text := a.st.Session.TextWithImagePaths(t, show)
			b.WriteString("## User\n\n" + text + "\n\n")
		case t.Code != "":
			b.WriteString("```js\n" + t.Code + "\n```\n\n")
			if t.Output != "" {
				b.WriteString("```\n" + t.Output + "\n```\n\n")
			}
		case t.Text != "":
			b.WriteString("## Assistant\n\n" + t.Text + "\n\n")
		}
	}
	if err := writeFile(path, b.String()); err != nil {
		a.notifyErr("export failed: " + err.Error())
		return
	}
	shown := name
	if !filepath.IsAbs(name) {
		shown = "./" + name
	}
	a.notify("session saved to " + shown)
}
