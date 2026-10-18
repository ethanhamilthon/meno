package app

import (
	"context"
	"strings"
	"time"

	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/safe"
)

func (a *App) submit() {
	if a.st.Ask != nil {
		return
	}
	text := strings.TrimSpace(a.messageText())
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
	a.st.Title = ""
	a.st.RunStart, a.st.RunTurns, a.st.RunJS, a.st.RunUsage0 = time.Now(), 0, 0, a.st.Usage
	a.st.Scroll = 0
	a.clearLive()
	cont := msg.Text == "" && len(msg.Attachments) == 0 // a continuation: events woke an idle session
	if !cont {
		a.st.RequestID = agent.NewID()
	}
	sess, sid := a.st.Session, a.st.Session.ID
	prov, model, effort := a.selection()
	effort = llm.ClampEffort(a.effortsFor(prov, model), effort) // the model may not take the stored level
	// Stamp the session: from now on it keeps this model, whatever the default becomes.
	sess.Provider, sess.Model, sess.Effort = prov, model, effort
	idx := sess.Len()
	hasHistory := idx > 0 || sess.Summary != ""
	if !cont {
		sess.Append(agent.Turn{Role: llm.RoleUser, Text: msg.Text, Attachments: msg.Attachments})
	}
	// Everything the goroutine needs is captured here; it must not touch a.st.
	src, ps := a.worldSrc(), a.st.Prompt
	d := agent.Deps{
		Client: a.client(p), Provider: p, Model: model,
		Effort: effort, Queue: &a.st.Queue, Cost: a.pricer(p, model),
		Events: a.eventSource(sid), AutoCompact: a.autoCompact(p, model), DebugPayload: a.debugPayload(),
		VM: a.st.VM, Thinking: a.st.Settings.Thinking != "off", ModelTimeout: a.st.Settings.ModelTimeout(),
		Emit:  func(e agent.Ev) { a.postEvent(agentEv{sid: sid, ev: e}) },
		Retry: a.providerRetry(p),
	}
	hook := a.prepareHook
	a.running.Add(1)
	go func() {
		defer safe.Recover(a.reportPanic)
		defer a.running.Done()
		defer cancel()
		if hook != nil {
			hook()
		}
		prep := prepare(ctx, ps, src, hasHistory)
		switch {
		case !cont:
			sess.SetSystem(idx, prep.Event)
		case prep.Event != "":
			// A wake-up has no user message to carry the changes of the world, and
			// the baseline moves on now: they go in as an event of their own.
			sess.Append(agent.Turn{Role: llm.RoleUser, Kind: agent.KindEvent, Text: prep.Event, Time: time.Now().UTC()})
		}
		a.postEvent(preparedEv{sid, prep})
		d.System = prep.State.Frozen
		// A panic in the agent loop ends this turn with an error; the app keeps running.
		var err error
		if perr := safe.Call(func() { err = (&agent.Runner{D: d}).Continue(ctx, sess) }); perr != nil {
			err = perr
		}
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
	a.cancelRequest()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.st.cancel != nil {
		a.st.cancel()
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
	if err == nil {
		a.wakeIfPending() // events that arrived after the last step
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
	if n := len(calls); n > 0 && calls[n-1].ID == "" && !calls[n-1].Running {
		calls[n-1].ID = id
		return &calls[n-1]
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
