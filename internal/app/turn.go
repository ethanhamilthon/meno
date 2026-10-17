package app

import (
	"context"
	"strings"
	"time"

	"meno/internal/agent"
	"meno/internal/llm"
)

func (a *App) submit() {
	if a.st.Ask != nil {
		a.answerTyped()
		return
	}
	text := strings.TrimSpace(string(a.st.Input))
	if text == "" {
		return
	}
	msg := a.draftMessage(text)
	if len(msg.Attachments) > 0 {
		if !a.imagesAllowed() {
			return
		}
		if err := a.storeImages(msg); err != nil {
			a.notifyErr("can't save image: " + err.Error())
			return
		}
	}
	if a.st.Running {
		// Steering: the runner picks it up before its next request to the model.
		a.st.Queue.Push(msg)
		a.clearDraft()
		return
	}
	if _, ok := a.currentProvider(); !ok {
		a.notifyErr("provider not configured · select provider and model with :model")
		return
	}
	a.clearDraft()
	a.startTurn(msg)
}

// clearDraft empties the composer after its content was sent.
func (a *App) clearDraft() {
	a.setInput(nil)
	a.st.Images = nil
	a.st.Undo, a.st.Redo, a.st.InsBase = nil, nil, a.snapshot()
}

func (a *App) startTurn(msg agent.Message) {
	p, ok := a.currentProvider()
	if !ok {
		_, model, _ := a.selection()
		a.notifyErr("provider not configured for model " + model)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.st.cancel = cancel
	a.mu.Unlock()
	a.st.Running = true
	a.st.LastErr = false
	a.st.Phase = "preparing"
	a.st.RunStart, a.st.RunTurns, a.st.RunJS, a.st.RunUsage0 = time.Now(), 0, 0, a.st.Usage
	a.st.Scroll = 0
	a.clearLive()
	sess, sid := a.st.Session, a.st.Session.ID
	prov, model, effort := a.selection()
	// Stamp the session: from now on it keeps this model, whatever the default becomes.
	sess.Provider, sess.Model, sess.Effort = prov, model, effort
	idx := sess.Len()
	hasHistory := idx > 0 || sess.Summary != ""
	sess.Append(agent.Turn{Role: llm.RoleUser, Text: msg.Text, Attachments: msg.Attachments})
	// Everything the goroutine needs is captured here; it must not touch a.st.
	src, ps := a.worldSrc(), a.st.Prompt
	d := agent.Deps{
		Client: a.client(p), Provider: p, Model: model,
		Effort: effort, Queue: &a.st.Queue, Cost: a.pricer(p, model),
		VM: a.st.VM, Thinking: a.st.Settings.Thinking != "off", ModelTimeout: a.st.Settings.ModelTimeout(),
		Emit:  func(e agent.Ev) { a.postEvent(agentEv{sid: sid, ev: e}) },
		Retry: a.retry,
	}
	hook := a.prepareHook
	a.running.Add(1)
	go func() {
		defer a.running.Done()
		defer cancel()
		if hook != nil {
			hook()
		}
		prep := prepare(ctx, ps, src, hasHistory)
		sess.SetSystem(idx, prep.Event)
		a.postEvent(preparedEv{sid, prep})
		d.System = prep.State.Frozen
		err := (&agent.Runner{D: d}).Continue(ctx, sess)
		a.postEvent(turnDone{sid: sid, err: err})
	}()
}

// onPrepared applies the result of preparing a message on the UI goroutine.
func (a *App) onPrepared(p prepared) {
	a.st.Prompt = p.State
	if p.SkillsOK {
		a.st.Skills = p.Skills
	}
	for _, w := range p.Warnings {
		a.notifyErr(w)
	}
	if a.st.Running && a.st.Phase == "preparing" {
		a.st.Phase = "thinking"
	}
}

func (a *App) abortTurn() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.st.cancel != nil {
		a.st.cancel()
	}
}

func (a *App) onAgent(e agent.Ev) {
	if e.Kind != agent.EvRetry && e.Kind != agent.EvUsage {
		a.st.Retry = nil
	}
	switch e.Kind {
	case agent.EvRetry:
		// The failed request's partial output is void: the request starts over.
		a.clearLive()
		a.st.Phase = "retrying"
		a.st.Retry = &retryState{Attempt: e.Attempt, Max: e.Max, Until: time.Now().Add(e.Delay), Reason: briefReason(e.Err)}
		a.notify("retry " + itoa(e.Attempt) + "/" + itoa(e.Max) + " in " + fmtWait(e.Delay) + " — " + a.st.Retry.Reason)
		return
	}
	switch e.Kind {
	case agent.EvThink, agent.EvText, agent.EvCode, agent.EvJSStart, agent.EvJSEnd:
		a.st.LiveBase = e.Turns
	}
	switch e.Kind {
	case agent.EvThink:
		a.st.LiveThink += e.Thinking
		a.st.Phase = "thinking"
	case agent.EvText:
		a.st.LiveText += e.Text
		a.st.Phase = "writing"
	case agent.EvCode:
		a.st.Phase = "writing"
		a.liveCall(e.CallID).Code = e.Code
	case agent.EvJSStart:
		lc := a.liveCall(e.CallID)
		lc.Code, lc.Running = e.Code, true
		lc.Started, lc.Limit = time.Now(), e.Limit
		a.st.Phase = "running"
	case agent.EvJSEnd:
		// The finished call (and the text before it) is in the session now.
		a.dropLiveCall(e.CallID)
		a.st.RunJS++
		a.st.LiveText, a.st.LiveThink = "", ""
		a.st.Phase = "thinking"
	case agent.EvInjected:
		a.clearLive()
		a.st.Phase = "thinking"
	case agent.EvDone, agent.EvAborted:
		a.clearLive()
		a.st.Phase = ""
	case agent.EvUsage:
		a.st.RunTurns++
		u := e.Usage
		a.st.Usage.In += u.In
		a.st.Usage.Out += u.Out
		a.st.Usage.Cached += u.Cached
		a.st.Usage.CacheWrite += u.CacheWrite
		a.st.LastUsage = u
		a.st.Context = u.In + u.Out
		p, model := e.Provider, e.Model
		if model == "" {
			p, _ = a.currentProvider()
			_, model, _ = a.selection()
		}
		record := agent.RequestUsage{Provider: p.Name, Model: model, Usage: u, Time: time.Now().UTC()}
		if info, ok := llm.LookupMetadata(a.st.Pricing, p, model); ok && info.CanPrice(u) {
			cost := info.Cost(u)
			record.Cost = &cost
			a.st.Spent += cost
		} else if u.In > 0 || u.Out > 0 || u.Cached > 0 || u.CacheWrite > 0 {
			a.st.Unpriced = true
		}
		if a.st.Session != nil {
			a.st.Session.Requests = append(a.st.Session.Requests, record)
		}
	case agent.EvError:
		a.notifyErr(e.Err)
	}
}

func (a *App) onTurnDone(err error) {
	a.st.Running = false
	a.st.RunStart = time.Time{}
	a.st.Retry = nil
	a.st.Phase = ""
	a.mu.Lock()
	a.st.cancel = nil
	a.mu.Unlock()
	a.clearLive()
	if err != nil && !isAbort(err) {
		a.st.LastErr = true
		a.notifyErr(err.Error())
	}
	a.saveSession()
	// Messages that arrived after the last step: send them as the next turn, or
	// hand them back when the turn did not end well.
	if rest := a.st.Queue.Take(); len(rest) > 0 {
		if err == nil {
			a.startTurn(agent.Combine(rest)) // not done yet: the user spoke again
			return
		}
		a.restoreDraft(rest)
	}
	switch {
	case err == nil:
		a.signal("done")
	case !isAbort(err):
		a.signal("error")
	}
}

func isAbort(err error) bool {
	return err == context.Canceled
}

func (a *App) currentProvider() (llm.Provider, bool) {
	prov, model, _ := a.selection()
	if model == "" || prov == "" {
		return llm.Provider{}, false
	}
	for _, p := range a.st.Providers {
		if p.Name == prov {
			return p, true
		}
	}
	return llm.Provider{}, false
}

func (a *App) scrollBy(d int) {
	w, h := a.scr.Size()
	maxScroll := max(0, len(a.transcriptWithSelection(w))-max(0, h-a.bottomHeight()))
	a.st.Scroll = min(max(0, a.st.Scroll+d), maxScroll)
}

func (a *App) clearLive() {
	a.st.LiveText, a.st.LiveThink, a.st.LiveCalls = "", "", nil
}

// liveCall finds the live call with this id. Chat streams may deliver code
// before the id is known: such a call is adopted by the first event that has
// one.
func (a *App) liveCall(id string) *liveCall {
	calls := a.st.LiveCalls
	for i := range calls {
		if id != "" && calls[i].ID == id {
			return &calls[i]
		}
	}
	for i := len(calls) - 1; i >= 0; i-- {
		if calls[i].ID == "" && !calls[i].Running {
			calls[i].ID = id
			return &calls[i]
		}
		break
	}
	a.st.LiveCalls = append(a.st.LiveCalls, liveCall{ID: id})
	return &a.st.LiveCalls[len(a.st.LiveCalls)-1]
}

func (a *App) dropLiveCall(id string) {
	out := a.st.LiveCalls[:0]
	for _, c := range a.st.LiveCalls {
		if c.ID != id {
			out = append(out, c)
		}
	}
	a.st.LiveCalls = out
}

// briefReason squeezes a provider error into one short line.
func briefReason(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 70 {
		s = string(r[:69]) + "…"
	}
	return s
}

// fmtWait renders a wait as whole seconds, rounded up.
func fmtWait(d time.Duration) string {
	return itoa(int((d+time.Second-1)/time.Second)) + "s"
}

// retryText is the status-bar text while a retry is pending, with a countdown.
func (a *App) retryText() string {
	r := a.st.Retry
	if r == nil {
		return ""
	}
	head := "retry " + itoa(r.Attempt) + "/" + itoa(r.Max)
	if left := time.Until(r.Until); left > 0 {
		head += " in " + fmtWait(left)
	} else {
		head += " · sending"
	}
	return head + " — " + r.Reason
}

// pricer prices one request of the model, for the cost in the run summary.
func (a *App) pricer(p llm.Provider, model string) func(llm.Usage) (float64, bool) {
	info, ok := llm.LookupMetadata(a.st.Pricing, p, model)
	if !ok {
		return nil
	}
	return func(u llm.Usage) (float64, bool) {
		if !info.CanPrice(u) {
			return 0, false
		}
		return info.Cost(u), true
	}
}

// clock writes how long something has gone on: 0:42, 12:05, 1:02:03.
func clock(d time.Duration) string {
	s := int(max(0, d) / time.Second)
	if s >= 3600 {
		return itoa(s/3600) + ":" + pad2(s%3600/60) + ":" + pad2(s%60)
	}
	return itoa(s/60) + ":" + pad2(s%60)
}

func pad2(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

// runningText is the status-bar text while the agent works: which request of
// the run it is on, for how long, how many js calls and the tokens spent.
func (a *App) runningText() string {
	if !a.st.Running || a.st.RunStart.IsZero() {
		return ""
	}
	turn := a.st.RunTurns + 1 // the request being answered
	if a.st.Phase == "running" {
		turn = max(1, a.st.RunTurns) // the call belongs to the request just answered
	}
	parts := []string{"turn " + itoa(turn), clock(time.Since(a.st.RunStart))}
	if n := a.st.RunJS; n > 0 {
		if n == 1 {
			parts = append(parts, "1 js call")
		} else {
			parts = append(parts, itoa(n)+" js calls")
		}
	}
	in, out := a.st.Usage.In-a.st.RunUsage0.In, a.st.Usage.Out-a.st.RunUsage0.Out
	if in > 0 || out > 0 {
		parts = append(parts, "↓ "+compactTokens(max(0, in))+" ↑ "+compactTokens(max(0, out)))
	}
	return strings.Join(parts, " · ")
}
