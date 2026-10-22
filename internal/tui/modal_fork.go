package tui

import (
	"context"
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/api"
	"meno/internal/client"
	"meno/internal/render"
)

// forkRow is a message of the current session that a fork can end at.
type forkRow struct {
	turn int
	role string
	text string
}

// forkRows lists the completed user and assistant messages, newest first, so
// the default selection is the latest completed message.
func (a *App) forkRows() []forkRow {
	var rows []forkRow
	for i, t := range a.st.Session.Turns() {
		text := strings.TrimSpace(t.Text)
		if text == "" || t.Kind != "" {
			continue
		}
		if t.Role == render.RoleUser || t.Role == render.RoleAssistant {
			rows = append(rows, forkRow{turn: i + 1, role: t.Role, text: text})
		}
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows
}

func (a *App) forkFiltered() ([]forkRow, []int) {
	rows := a.forkRows()
	q := a.st.Query.String()
	return rows, filterIdx(len(rows), func(i int) bool { return containsFold(rows[i].text, q) })
}

// openFork shows the messages a fork may end at.
func (a *App) openFork() {
	if len(a.forkRows()) == 0 {
		a.notifyErr("fork: nothing to fork yet")
		return
	}
	a.openOverlay(OverFork)
}

func (a *App) forkKey(k *tcell.EventKey) {
	rows, idx := a.forkFiltered()
	switch {
	case a.navKey(k, len(idx), false):
	case k.Key() == tcell.KeyEnter:
		if len(idx) > 0 {
			a.forkAt(rows[idx[clampSel(a.st.Sel, len(idx))]].turn)
		}
	default:
		a.queryKey(k)
	}
}

// forkAt creates a fork of the current session ending at the message hid and
// opens it. No model is called. A working directory that no longer exists is
// not replaced silently: the user picks one with /cd and forks again.
func (a *App) forkAt(turn int) {
	a.closeOverlay()
	sid := a.st.Session.ID()
	a.spawn(func() {
		var r api.ForkResponse
		err := a.rpc(func(ctx context.Context, c *client.Client) error {
			var e error
			r, e = c.Fork(ctx, sid, api.ForkRequest{Turn: turn, ClientID: newClientID()})
			return e
		})
		a.postEvent(forked{id: r.Session.ID, err: err})
	})
}

type forked struct {
	id  string
	err error
}

func (a *App) onForked(m forked) {
	if m.err != nil {
		a.notifyErr("fork failed: " + m.err.Error())
		return
	}
	a.openSession(m.id)
	a.notify("forked · independent session, no model call yet")
}

func (a *App) drawFork(c render.Canvas, w, h int) {
	rows, idx := a.forkFiltered()
	items := make([]item, 0, len(idx))
	for _, i := range idx {
		r := rows[i]
		glyph, fg := "›", render.FgDim
		if r.role == render.RoleAssistant {
			glyph, fg = "◆", render.FgDim
		}
		first, _, _ := strings.Cut(r.text, "\n")
		items = append(items, item{glyph: glyph, glyphFG: fg, label: first, right: r.role})
	}
	a.searchList(c, w, h, 72, len(rows), "fork at message", "search messages", items, "↑/↓ choose · enter fork · esc cancel")
}
