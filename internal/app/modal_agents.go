package app

import (
	"time"

	"github.com/gdamore/tcell/v2"

	"fmt"
	"meno/internal/agent"
	"meno/internal/safe"
	"meno/internal/ui"
)

// agentsView is the :agents dialog: the working children of this session.
type agentsView struct {
	OpenID string // the child whose conversation is open ("" = the list)
}

func (a *App) openAgents() {
	if a.kids == nil {
		a.notifyErr("agents are not available")
		return
	}
	a.st.AgentsUI = agentsView{}
	a.onChildChanged(a.st.Session.ID)
	a.openOverlay(OverAgents)
}

// workingKids are the children of the current session that are doing something,
// with live state for the ones running in this process. Finished children are
// internal: nothing lists them here.
func (a *App) workingKids() []agent.ChildRecord {
	var out []agent.ChildRecord
	for _, r := range a.st.Kids {
		live := r
		a.kids.mu.Lock()
		c := a.kids.runs[r.ID]
		a.kids.mu.Unlock()
		if c != nil {
			c.mu.Lock()
			live = c.rec
			c.mu.Unlock()
		}
		if live.Working() {
			out = append(out, live)
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

// sendToChild forwards a user message through the steering queue with the
// user as its source. It does not start another request.
func (a *App) sendToChild(id, text string) {
	if text == "" {
		return
	}
	a.running.Add(1)
	owner, parent := a.st.RequestID, a.st.Session.ID
	go func() {
		defer safe.Recover(a.reportPanic)
		defer a.running.Done()
		recs, err := a.kids.store.Get(parent, id)
		if err != nil {
			a.postEvent(errNotice{"agent not found"})
			return
		}
		c, err := a.kids.revive(recs)
		if err != nil {
			a.postEvent(errNotice{err.Error()})
			return
		}
		if _, err := a.kids.deliver(c, owner, agent.SourceUser, text); err != nil {
			a.postEvent(errNotice{err.Error()})
		}
	}()
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
		if k.Status == agent.ChildBlocked {
			glyph, fg = "?", ui.Orange
		}
		it := item{glyph: glyph, glyphFG: fg, label: k.ID[len(k.ID)-6:] + "  " + oneLine(k.Task), right: k.Status + " " + clock(time.Since(k.Created))}
		it.detail = costText(k)
		items = append(items, it)
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

func costText(k agent.ChildRecord) string {
	s := "in " + itoa(k.Usage.In) + " · out " + itoa(k.Usage.Out)
	if k.Unpriced && k.Cost == 0 {
		return s + " · cost unknown"
	}
	s += " · $" + fmt.Sprintf("%.4f", k.Cost)
	if k.Unpriced {
		s += "+ (some requests unpriced)"
	}
	return s
}

func (a *App) drawAgentChat(c ui.Canvas, w, h int) {
	id := a.st.AgentsUI.OpenID
	b := centered(w, h, 0, max(8, h-4))
	b.frame(c)
	titleRow("agent "+id[len(id)-6:], "")(c, b)
	a.kids.mu.Lock()
	run := a.kids.runs[id]
	a.kids.mu.Unlock()
	room := max(0, b.h-5)
	if run != nil {
		lines := ui.BuildLines(viewTurns(run.sess.Snapshot()), b.w, a.jsOpts())
		if len(lines) > room {
			lines = lines[len(lines)-room:]
		}
		for i, l := range lines {
			ui.DrawLine(c, b.x, b.y+2+i, b.w, l)
		}
	}
	b.fill(c, b.h-3, ui.BgPopupIn)
	b.put(c, 2, b.h-3, "›", ui.FgWhite, ui.BgPopupIn)
	a.st.Query.Placeholder = ""
	a.st.Query.Draw(c, b.x+4, b.y+b.h-3, max(1, b.w-8), 1, ui.Style{FG: ui.FgWhite, BG: ui.BgPopupIn}.TCell(), true)
	b.hint(c, "enter send to this agent · esc back")
}

// errNotice carries an error from a goroutine to the UI.
type errNotice struct{ text string }
