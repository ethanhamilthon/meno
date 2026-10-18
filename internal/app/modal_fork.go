package app

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/ui"
)

// forkRow is a message of the current session that a fork can end at.
type forkRow struct {
	hid  int64
	role string
	text string
}

// forkRows lists the completed user and assistant messages, newest first, so
// the default selection is the latest completed message.
func (a *App) forkRows() []forkRow {
	var rows []forkRow
	for _, t := range a.st.Session.Snapshot() {
		text := strings.TrimSpace(t.Text)
		if text == "" || t.Kind != "" {
			continue
		}
		if t.Role == llm.RoleUser || t.Role == llm.RoleAssistant {
			rows = append(rows, forkRow{hid: t.HID, role: t.Role, text: text})
		}
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows
}

func (a *App) forkFiltered() ([]forkRow, []int) {
	rows := a.forkRows()
	q := string(a.st.Query)
	return rows, filterIdx(len(rows), func(i int) bool { return containsFold(rows[i].text, q) })
}

// openFork shows the messages a fork may end at.
func (a *App) openFork() {
	a.saveSession()
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
			a.forkAt(rows[idx[clampSel(a.st.Sel, len(idx))]].hid)
		}
	default:
		a.queryKey(k)
	}
}

// forkAt creates a fork of the current session ending at the message hid and
// opens it. No model is called. A working directory that no longer exists is
// not replaced silently: the user picks one with :cd and forks again.
func (a *App) forkAt(hid int64) {
	a.closeOverlay()
	src := a.st.Session
	fork, warn, err := agent.ForkSession(a.paths.Home, src.ID, hid, "")
	if err != nil {
		if strings.Contains(err.Error(), "choose a directory") {
			a.notifyErr(err.Error() + " · use :cd <dir>, then :fork again")
		} else {
			a.notifyErr("fork failed: " + err.Error())
		}
		return
	}
	a.openSession(fork.ID)
	if warn != "" {
		a.notifyErr(warn)
		return
	}
	a.notify("forked · independent session, no model call yet")
}

func (a *App) drawFork(c ui.Canvas, w, h int) {
	rows, idx := a.forkFiltered()
	items := make([]item, 0, len(idx))
	for _, i := range idx {
		r := rows[i]
		glyph, fg := "›", ui.FgDim
		if r.role == llm.RoleAssistant {
			glyph, fg = "◆", ui.FgDim
		}
		first, _, _ := strings.Cut(r.text, "\n")
		items = append(items, item{glyph: glyph, glyphFG: fg, label: first, right: r.role})
	}
	a.searchList(c, w, h, 72, len(rows), "fork at message", "search messages", items, "↑/↓ choose · enter fork · esc cancel")
}
