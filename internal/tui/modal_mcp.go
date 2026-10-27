package tui

import (
	"reflect"
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/mcp"
	"meno/internal/render"
)

var mcpTransports = []string{"stdio", "sse", "streamable-http"}

func mcpTarget(s mcp.Server) string {
	if s.URL != "" {
		return s.URL
	}
	return s.Command
}

func (a *App) mcpItems() []item {
	var out []item
	for _, s := range a.st.Servers {
		g, gfg := "○", render.FgDim
		right := "stopped"
		detail := mcpTarget(s)
		switch {
		case !s.Enabled:
			right = "disabled"
		case a.st.ServerInfo[s.Name].Starting:
			g, gfg = "◌", render.Yellow
			right = "starting…"
		case a.st.ServerInfo[s.Name].Running:
			g, gfg = "●", render.Green
			right = s.Transport + " · running"
		case a.st.ServerInfo[s.Name].Err != "":
			g, gfg = "✕", render.Red
			right = "error"
			detail = a.st.ServerInfo[s.Name].Err
		}
		out = append(out, item{glyph: g, glyphFG: gfg, label: s.Name, right: right, detail: detail})
	}
	return append(out, item{glyph: "+", glyphFG: render.Blue, label: "new mcp server", accent: true})
}

func (a *App) mcpKey(k *tcell.EventKey) {
	n := len(a.st.Servers)
	switch {
	case a.navKey(k, n+1, true):
	case k.Key() == tcell.KeyEnter && a.st.Sel >= n:
		a.startMCPWizard()
	case k.Key() == tcell.KeyEnter:
		a.openMCPEdit(a.st.Sel)
	case isKey(k, 'n'):
		a.startMCPWizard()
	case k.Rune() == ' ' && a.st.Sel < n:
		s := &a.st.Servers[a.st.Sel]
		s.Enabled = !s.Enabled
		a.saveMCPServer(*s, false)
	case isKey(k, 'r') && a.st.Sel < n:
		a.startMCP(a.st.Sel)
	}
}

// startMCP (re)starts server i in the background, or stops it when disabled.
func (a *App) startMCP(i int) {
	if i < 0 || i >= len(a.st.Servers) {
		return
	}
	s := a.st.Servers[i]
	if !s.Enabled {
		return
	}
	a.restartMCP(s.Name)
}

func (a *App) openMCPEdit(i int) {
	if i < 0 || i >= len(a.st.Servers) {
		a.st.EditMCP = mcp.Server{Transport: "stdio", Enabled: true}
		a.st.EditIdx = -1
	} else {
		a.st.EditMCP = a.st.Servers[i]
		a.st.EditIdx = i
	}
	a.openOverlay(OverMCPEdit)
}

func (a *App) drawMCP(c render.Canvas, w, h int) {
	running := 0
	for _, n := range a.st.ServerInfo {
		if n.Running {
			running++
		}
	}
	header := titleRow("mcp servers", itoa(running)+" running")
	a.listBox(c, w, h, 73, header, a.mcpItems(), a.st.Sel,
		"j/k select · space toggle · enter edit · n new · r restart · esc close")
}

func (a *App) mcpFields() []field {
	s := &a.st.EditMCP
	return []field{
		{label: "name", get: func() string { return s.Name }, set: func(v string) error { s.Name = v; return nil }},
		{label: "transport", opts: mcpTransports, get: func() string { return s.Transport }, set: func(v string) error { s.Transport = v; return nil }},
		{label: "command", get: func() string { return s.Command }, set: func(v string) error { s.Command = v; return nil }},
		{label: "args", get: func() string { return strings.Join(s.Args, " ") }, set: func(v string) error {
			s.Args = splitArgs(v)
			return nil
		}},
		{label: "cwd", get: func() string { return s.Cwd }, set: func(v string) error { s.Cwd = v; return nil }},
		{label: "url", get: func() string { return s.URL }, set: func(v string) error { s.URL = v; return nil }},
		{label: "headers", get: func() string { return mapJSON(s.Headers) }, set: func(v string) error {
			m, err := parseMapJSON(v)
			if err != nil {
				return err
			}
			if len(m) == 0 {
				m = nil
			}
			s.Headers = m
			return nil
		}},
		{label: "env", get: func() string { return mapJSON(s.Env) }, set: func(v string) error {
			m, err := parseMapJSON(v)
			if err != nil {
				return err
			}
			s.Env = m
			return nil
		}},
	}
}

func (a *App) mcpEditKey(k *tcell.EventKey) {
	a.formKey(k, a.mcpFields(), a.saveMCPEdit)
	switch {
	case isKey(k, 'r') && !a.st.Editing && a.st.EditIdx >= 0:
		a.startMCP(a.st.EditIdx)
	case isKey(k, 'd') && !a.st.Editing && a.st.EditIdx >= 0:
		a.st.Servers = append(a.st.Servers[:a.st.EditIdx], a.st.Servers[a.st.EditIdx+1:]...)
		a.deleteMCPServer(a.st.EditMCP.Name)
		a.openOverlay(OverMCP)
	}
}

func (a *App) saveMCPEdit() {
	isNew := a.st.EditIdx < 0 || a.st.EditIdx >= len(a.st.Servers)
	s := a.st.EditMCP
	if err := mcp.ValidateServer(s); err != nil {
		a.st.FormNote = err.Error()
		return
	}
	for j, o := range a.st.Servers {
		if j != a.st.EditIdx && o.Name == s.Name {
			a.st.FormNote = "mcp server " + s.Name + " already exists"
			return
		}
	}
	a.st.FormNote = ""
	if a.st.EditIdx >= 0 && a.st.EditIdx < len(a.st.Servers) {
		if reflect.DeepEqual(a.st.Servers[a.st.EditIdx], s) {
			return // nothing changed: do not restart the server
		}
		if a.st.Servers[a.st.EditIdx].Name != s.Name {
			a.st.FormNote = "Server names cannot change; create a new server instead"
			return
		}
		a.st.Servers[a.st.EditIdx] = s
	} else {
		a.st.Servers = append(a.st.Servers, s)
		a.st.EditIdx = len(a.st.Servers) - 1
	}
	a.saveMCPServer(s, isNew)
}

func (a *App) drawMCPEdit(c render.Canvas, w, h int) {
	fs := a.mcpFields()
	b := a.panelBox(w, h, 73, len(fs)+6)
	b.frame(c)
	b.put(c, 2, 0, "mcp /", render.FgDim, render.BgPopup)
	name := a.st.EditMCP.Name
	if name == "" {
		name = "new server"
	}
	b.put(c, 8, 0, name, render.Blue, render.BgPopup)
	if a.st.EditIdx >= 0 && a.st.ServerInfo[name].Running {
		b.putRight(c, 0, "● running", render.Green, render.BgPopup)
	} else {
		b.putRight(c, 0, "○ stopped", render.FgDim, render.BgPopup)
	}
	a.drawForm(c, b, 2, fs, a.st.Field, a.st.Editing, &a.st.Edit)
	drawFormNote(c, b, a.st.FormNote)
	b.hint(c, "j/k move · enter edit · r restart · d delete · esc back")
}
