package app

import (
	"context"
	"errors"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/cliproxy"
	"meno/internal/safe"
	"meno/internal/ui"
)

// loginFlow is one OAuth login in progress.
type loginFlow struct {
	ID     int
	Svc    cliproxy.Service
	Steps  []string
	Login  *cliproxy.Login // nil until the helper has given the page
	Err    string
	cancel context.CancelFunc
}

// pollEvery is how often a login asks the helper whether the browser is done.
var pollEvery = 1500 * time.Millisecond

// startLogin begins a login to svc: it prepares the helper, opens the page in
// the browser and waits for the account to appear.
func (a *App) startLogin(svc cliproxy.Service) {
	a.cancelFlow()
	a.flowSeq++
	ctx, cancel := context.WithCancel(context.Background())
	id := a.flowSeq
	a.openOverlay(OverLoginFlow)
	a.st.Flow = loginFlow{ID: id, Svc: svc, cancel: cancel}
	every := pollEvery // read here, on the UI goroutine
	safe.Go(func() { a.runLogin(ctx, id, svc, every) }, a.reportPanic)
}

func (a *App) cancelFlow() {
	if a.st.Flow.cancel != nil {
		a.st.Flow.cancel()
		a.st.Flow.cancel = nil
	}
}

func (a *App) runLogin(ctx context.Context, id int, svc cliproxy.Service, every time.Duration) {
	step := func(s string) { a.postEvent(flowStep{id: id, text: s}) }
	if err := a.ensureOAuthServer(ctx, step); err != nil {
		a.postEvent(flowDone{id: id, err: err})
		return
	}
	step("asking " + svc.Label + " for the login page")
	lg, err := a.oauth.StartLogin(ctx, svc.ID)
	a.postEvent(flowLogin{id: id, login: lg, err: err})
	if err != nil {
		return
	}
	limit := lg.ExpiresIn
	if limit <= 0 {
		limit = 10 * time.Minute
	}
	deadline := time.Now().Add(limit)
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
		if time.Now().After(deadline) {
			a.postEvent(flowDone{id: id, err: errors.New("the login expired: start it again")})
			return
		}
		status, msg, err := a.oauth.LoginStatus(ctx, lg.State)
		switch {
		case err != nil:
			if failures++; failures >= 5 {
				a.postEvent(flowDone{id: id, err: err})
				return
			}
		case status == "ok":
			a.postEvent(flowDone{id: id})
			return
		case status == "error":
			if msg == "" {
				msg = "the login failed"
			}
			a.postEvent(flowDone{id: id, err: errors.New(msg)})
			return
		default:
			failures = 0
		}
	}
}

func (a *App) onFlowStep(m flowStep) {
	if m.id == a.st.Flow.ID {
		a.st.Flow.Steps = append(a.st.Flow.Steps, m.text)
	}
}

func (a *App) onFlowLogin(m flowLogin) {
	f := &a.st.Flow
	if m.id != f.ID {
		return
	}
	if m.err != nil {
		f.Err = m.err.Error()
		return
	}
	f.Login = &m.login
	a.scr.SetClipboard([]byte(m.login.URL))
	if err := a.openURL(m.login.URL); err != nil {
		f.Steps = append(f.Steps, "could not open a browser: "+err.Error())
		return
	}
	f.Steps = append(f.Steps, "opened the page in your browser; the address is also copied")
}

func (a *App) onFlowDone(m flowDone) {
	f := &a.st.Flow
	if m.id != f.ID {
		return
	}
	if m.err != nil {
		f.Err = m.err.Error()
		return
	}
	a.cancelFlow()
	a.notify("signed in to " + f.Svc.Label)
	a.openOverlay(OverLogin)
	a.refreshAccounts()
}

func (a *App) loginFlowKey(k *tcell.EventKey) {
	f := &a.st.Flow
	if a.st.Editing { // pasting the address the browser ended on
		switch {
		case k.Key() == tcell.KeyEsc:
			a.endEdit()
		case k.Key() == tcell.KeyEnter && f.Login != nil:
			a.submitCallback(a.st.Edit.String())
			a.endEdit()
		default:
			a.st.Edit.Handle(k)
		}
		return
	}
	switch {
	case f.Err != "" && (k.Key() == tcell.KeyEnter || isKey(k, 'r')):
		a.startLogin(f.Svc)
	case f.Login != nil && isKey(k, 'o'):
		if err := a.openURL(f.Login.URL); err != nil {
			a.notifyErr(err.Error())
		}
	case f.Login != nil && isKey(k, 'y'):
		a.scr.SetClipboard([]byte(f.Login.URL))
		a.notify("address copied")
	case f.Login != nil && !f.Login.Device && isKey(k, 'p'):
		a.beginEdit("", false, false)
		a.st.Edit.Placeholder = "paste the address of the page the browser ended on"
	}
}

// submitCallback hands the helper the address the browser was sent to.
func (a *App) submitCallback(addr string) {
	if addr == "" {
		return
	}
	safe.Go(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := a.oauth.SubmitCallback(ctx, addr); err != nil {
			a.postEvent(errNotice{text: "login: " + err.Error()})
		}
	}, a.reportPanic)
}

func (a *App) drawLoginFlow(c ui.Canvas, w, h int) {
	f := &a.st.Flow
	url := ""
	if f.Login != nil {
		url = f.Login.URL
	}
	urlRows := wrapRunes(url, max(8, modalWidth(w)-8), 4)
	bh := 8 + len(urlRows) + min(len(f.Steps), 3)
	b := centered(w, h, 73, bh)
	b.frame(c)
	titleRow("login · "+f.Svc.Label, "")(c, b)
	y := 2
	for _, s := range f.Steps[max(0, len(f.Steps)-3):] {
		b.put(c, 2, y, ui.Truncate(s, max(1, b.w-6)), ui.FgDim, ui.BgPopup)
		y++
	}
	hint := "esc cancel"
	switch {
	case f.Err != "":
		b.put(c, 2, y+1, ui.Truncate(f.Err, max(1, b.w-6)), ui.Red, ui.BgPopup)
		hint = "enter try again · esc back"
	case f.Login != nil && f.Login.Device:
		b.put(c, 2, y+1, "Open the page and confirm this code:", ui.FgWhite, ui.BgPopup)
		b.put(c, 2, y+2, f.Login.UserCode, ui.Green, ui.BgPopup)
		y += 3
		for i, r := range urlRows {
			b.put(c, 2, y+i, r, ui.FgDim, ui.BgPopup)
		}
		hint = "o open again · y copy address · esc cancel"
	case f.Login != nil:
		b.put(c, 2, y+1, "Sign in in your browser. Waiting for it to finish.", ui.FgWhite, ui.BgPopup)
		y += 2
		for i, r := range urlRows {
			b.put(c, 2, y+i, r, ui.FgDim, ui.BgPopup)
		}
		hint = "o open again · y copy address · p paste the final address · esc cancel"
	}
	if a.st.Editing {
		b.fill(c, b.h-3, ui.BgPopupIn)
		a.st.Edit.Draw(c, b.x+4, b.y+b.h-3, max(1, b.w-8), 1, ui.Style{FG: ui.FgWhite, BG: ui.BgPopupIn}.TCell(), true)
		hint = "enter send · esc back"
	}
	b.hint(c, hint)
}

// wrapRunes cuts s into rows of at most width cells, at most maxRows of them;
// the last row ends in … when text is left.
func wrapRunes(s string, width, maxRows int) []string {
	var rows []string
	rs := []rune(s)
	for len(rs) > 0 && len(rows) < maxRows {
		n := min(width, len(rs))
		rows = append(rows, string(rs[:n]))
		rs = rs[n:]
	}
	if len(rs) > 0 && len(rows) > 0 {
		last := []rune(rows[len(rows)-1])
		rows[len(rows)-1] = string(last[:len(last)-1]) + "…"
	}
	return rows
}
