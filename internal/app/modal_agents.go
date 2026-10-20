package app

import (
	"context"
	"encoding/json"

	"github.com/gdamore/tcell/v2"

	"meno/internal/client"
	"meno/internal/proto"
	"meno/internal/ui"
)

// agentsView is the :agents dialog: the working children of this session.
type agentsView struct {
	OpenID string // the child whose conversation is open ("" = the list)
	Turns  []ui.Turn
}

func (a *App) openAgents() {
	a.st.AgentsUI = agentsView{}
	a.refreshSession(a.st.Session.ID())
	a.openOverlay(OverAgents)
}

// workingKids are the children of the current session that are doing something.
func (a *App) workingKids() []kidRow {
	var out []kidRow
	for _, k := range a.st.Kids {
		if kidWorking(k.Status) {
			out = append(out, k)
		}
	}
	return out
}

func (a *App) agentsKey(k *tcell.EventKey) {
	v := &a.st.AgentsUI
	if v.OpenID == "" {
		kids := a.workingKids()
		switch {
		case a.navKey(k, len(kids), true):
		case k.Key() == tcell.KeyEnter && len(kids) > 0:
			v.OpenID = kids[clampSel(a.st.Sel, len(kids))].ID
			a.st.Query.Reset()
			a.loadChildTurns(v.OpenID)
		}
		return
	}
	switch k.Key() {
	case tcell.KeyEnter:
		a.sendToChild(v.OpenID, a.st.Query.String())
		a.st.Query.Reset()
	default:
		a.st.Query.Handle(k)
	}
}

type childTurns struct {
	id    string
	turns []ui.Turn
}

// loadChildTurns reads the conversation of a subagent from the daemon.
func (a *App) loadChildTurns(id string) {
	a.spawn(func() {
		var sn proto.Snapshot
		if err := a.rpc(func(ctx context.Context, c *client.Client) error {
			var e error
			sn, e = c.Snapshot(ctx, id)
			return e
		}); err != nil {
			return
		}
		var ts []ui.Turn
		for _, t := range sn.Turns {
			ts = append(ts, turnFromWire(t))
		}
		a.postEvent(childTurns{id, ts})
	})
}

// sendToChild tells the daemon to forward a message to a subagent.
func (a *App) sendToChild(id, text string) {
	if text == "" {
		return
	}
	sid := a.st.Session.ID()
	a.spawn(func() {
		data, _ := json.Marshal(map[string]any{"id": id, "text": text})
		err := a.rpc(func(ctx context.Context, c *client.Client) error {
			_, e := c.Resource(ctx, "POST", "/v1/sessions/"+sid+"/agents/send", "", proto.ResourceRequest{Data: data})
			return e
		})
		if err != nil {
			a.postEvent(errNotice{text: err.Error()})
		}
	})
}

func (a *App) drawAgents(c ui.Canvas, w, h int) {
	v := a.st.AgentsUI
	kids := a.workingKids()
	if v.OpenID != "" {
		a.drawAgentChat(c, w, h)
		return
	}
	items := make([]item, 0, len(kids))
	for _, k := range kids {
		glyph, fg := "●", ui.Green
		if k.Status == "blocked" {
			glyph, fg = "?", ui.Orange
		}
		items = append(items, item{glyph: glyph, glyphFG: fg, label: tail6(k.ID) + "  " + oneLine(k.Task), right: k.Status})
	}
	sel := clampSel(a.st.Sel, len(items))
	bh := 4 + itemsHeight(items, sel)
	if len(items) == 0 {
		bh++
	}
	b := centered(w, h, 0, bh)
	b.frame(c)
	titleRow("agents · working", itoa(len(items)))(c, b)
	if len(items) == 0 {
		b.put(c, 4, 2, "no agents are working in this session", ui.FgDim, ui.BgPopup)
	}
	drawItems(c, b, 2, items, sel)
	b.hint(c, "↑/↓ select · enter open and talk · esc close")
}

func tail6(s string) string {
	if len(s) > 6 {
		return s[len(s)-6:]
	}
	return s
}

func (a *App) drawAgentChat(c ui.Canvas, w, h int) {
	id := a.st.AgentsUI.OpenID
	b := centered(w, h, 0, max(8, h-4))
	b.frame(c)
	titleRow("agent "+tail6(id), "")(c, b)
	room := max(0, b.h-5)
	lines := ui.BuildLines(a.st.AgentsUI.Turns, b.w, a.jsOpts())
	if len(lines) > room {
		lines = lines[len(lines)-room:]
	}
	for i, l := range lines {
		ui.DrawLine(c, b.x, b.y+2+i, b.w, l)
	}
	b.fill(c, b.h-3, ui.BgPopupIn)
	b.put(c, 2, b.h-3, "›", ui.FgWhite, ui.BgPopupIn)
	a.st.Query.Placeholder = ""
	a.st.Query.Draw(c, b.x+4, b.y+b.h-3, max(1, b.w-8), 1, ui.Style{FG: ui.FgWhite, BG: ui.BgPopupIn}.TCell(), true)
	b.hint(c, "enter send to this agent · esc back")
}

// errNotice carries an error from a goroutine to the UI.
type errNotice struct{ text string }
