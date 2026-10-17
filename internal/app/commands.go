package app

import (
	"context"
	"strings"

	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/ui"
)

// interruptsAgent lists commands that would pull the session, VM or process out
// from under a running agent. Everything else (dialogs, export...) is safe.
var interruptsAgent = map[string]bool{
	"new": true, "resume": true, "reset": true, "clear": true, "compact": true, "q": true, "quit": true,
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
		a.quit = true
	case "new":
		a.saveSession()
		a.st.Session = agent.NewSession(a.paths.Cwd, a.st.Settings.Model)
		a.st.Scroll = 0
		a.st.LastUsage = llm.Usage{}
		a.st.Usage = llm.Usage{}
		a.st.Spent = 0
		a.st.Context = 0
		a.st.Unpriced = false
		a.vm.Reset()
	case "reset":
		a.vm.Reset()
		a.notify("js vm restarted")
	case "compact":
		a.compact()
	case "clear":
		a.st.Session.Replace(nil)
		a.st.Session.Summary = ""
		a.st.Scroll = 0
		a.st.Context = 0
		// Usage and cost are cumulative for this session, not its current context.
		a.saveSession()
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
	case "resume":
		a.openResume()
	default:
		a.notifyErr("unknown command :" + name)
	}
}

func (a *App) saveSession() {
	if a.st.Session == nil {
		return
	}
	a.st.Session.Metrics = agent.Metrics{LastUsage: a.st.LastUsage, Usage: a.st.Usage, Spent: a.st.Spent, Context: a.st.Context, Unpriced: a.st.Unpriced}
	// Empty sessions must also be saved: :clear may have removed persisted turns.
	if err := agent.SaveSession(a.paths.SessionsDir(), a.st.Session); err != nil {
		a.notifyErr("save failed: " + err.Error())
	}
}

func (a *App) compact() {
	p, ok := a.currentProvider()
	if !ok {
		a.notifyErr("provider not configured")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.cancel = cancel
	a.mu.Unlock()
	a.st.Running = true
	a.st.Phase = "compacting"
	sess, model, client := a.st.Session, a.st.Settings.Model, a.client(p)
	a.running.Add(1)
	go func() {
		defer a.running.Done()
		defer cancel()
		before, after, err := agent.Compact(ctx, client, p, model, sess, func(u llm.Usage) {
			a.postEvent(agentEv{ev: agent.Ev{Kind: agent.EvUsage, Usage: u, Provider: p, Model: model}})
		})
		a.postEvent(compactDone{before: before, after: after, err: err})
	}()
}

func (a *App) exportSession(args []string) {
	name := "meno-session.md"
	if len(args) > 0 {
		name = args[0]
	}
	var b strings.Builder
	for _, t := range a.st.Session.Snapshot() {
		switch {
		case t.Role == llm.RoleUser:
			b.WriteString("## User\n\n" + t.Text + "\n\n")
		case t.Code != "":
			b.WriteString("```js\n" + t.Code + "\n```\n\n")
			if t.Output != "" {
				b.WriteString("```\n" + t.Output + "\n```\n\n")
			}
		case t.Text != "":
			b.WriteString("## Assistant\n\n" + t.Text + "\n\n")
		}
	}
	if err := writeFile(name, b.String()); err != nil {
		a.notifyErr("export failed: " + err.Error())
		return
	}
	a.notify("session saved to ./" + name)
}

func (a *App) openResume() {
	list, err := agent.ListSessions(a.paths.SessionsDir())
	if err != nil {
		a.notifyErr("resume failed: " + err.Error())
		return
	}
	a.st.Sessions = list
	a.st.Sel = 0
	a.openOverlay(OverResume)
}

var _ = ui.BgTerm
