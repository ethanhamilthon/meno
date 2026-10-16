package app

import (
	"strings"

	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/ui"
)

func (a *App) runCommand(raw string) {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return
	}
	name := fields[0]
	args := fields[1:]
	switch name {
	case "q", "quit":
		a.quit = true
	case "new":
		a.saveSession()
		a.st.Session = agent.NewSession(a.paths.Cwd, a.st.Settings.Model)
		a.st.Scroll = 0
	case "reset":
		a.vm.Reset()
		a.st.Toast = "js vm restarted"
	case "compact":
		a.compact()
	case "clear":
		a.st.Session.Replace(nil)
	case "export":
		a.exportSession(args)
	case "stop":
		a.abortTurn()
	case "help":
		a.openOverlay(OverHelp)
	case "model":
		a.openOverlay(OverModel)
	case "settings":
		a.openOverlay(OverSettings)
	case "providers":
		a.openOverlay(OverProviders)
	case "skills":
		a.openOverlay(OverSkills)
	case "mcp":
		a.openOverlay(OverMCP)
	case "prompts":
		a.openOverlay(OverPrompts)
	case "resume":
		a.openResume()
	default:
		a.st.Toast = "unknown command :" + name
	}
}

func (a *App) saveSession() {
	if a.st.Session.Snapshot() != nil {
		_ = agent.SaveSession(a.paths.SessionsDir(), a.st.Session)
	}
}

func (a *App) compact() {
	p, ok := a.currentProvider()
	if !ok {
		a.st.Toast = "provider not configured"
		return
	}
	before, after, err := agent.Compact(contextBG(), a.client(p), p, a.st.Settings.Model, a.st.Session)
	if err != nil {
		a.st.Toast = "compact failed: " + err.Error()
		return
	}
	a.st.Toast = "контекст сжат · " + itoa(before) + " → " + itoa(after) + " токенов"
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
		a.st.Toast = "export failed: " + err.Error()
		return
	}
	a.st.Toast = "сессия сохранена в ./" + name
}

func (a *App) openResume() {
	list, _ := agent.ListSessions(a.paths.SessionsDir())
	a.st.Sessions = list
	a.st.Sel = 0
	a.openOverlay(OverResume)
}

var _ = ui.BgTerm
