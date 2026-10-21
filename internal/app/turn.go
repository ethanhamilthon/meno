package app

import (
	"context"
	"strings"

	"meno/internal/client"
	"meno/internal/llm"
	"meno/internal/proto"
)

func (a *App) submit() {
	if a.st.Ask != nil {
		return
	}
	text := strings.TrimSpace(a.messageText())
	if text == "" {
		return
	}
	if a.offline() {
		// The draft stays in the composer; nothing is sent over a broken stream.
		a.notifyFailure("offline · not sent · your draft is kept · esc to dismiss")
		return
	}
	msg := a.draftMessage(text)
	if len(msg.Attachments) > 0 {
		if !a.imagesAllowed() {
			return
		}
	}
	if _, ok := a.currentProvider(); !ok && !a.st.Running {
		a.notifyErr("provider not configured · select provider and model with /model")
		return
	}
	a.clearDraft()
	a.saveDraft()
	a.send(msg)
}

// clearDraft empties the composer after its content was sent.
func (a *App) clearDraft() {
	a.setInput(nil)
	a.st.Images = nil
	a.st.Undo, a.st.Redo, a.st.InsBase = nil, nil, a.snapshot()
}

// send hands a message to the daemon. An idle session starts a run; a running
// one queues the message as steering for its next step.
func (a *App) send(msg msgDraft) {
	rt := a.st.Runtime
	sid := rt.ID()
	req := proto.MessageRequest{Text: msg.Text, ClientID: newClientID(), Origin: a.instance}
	for _, at := range msg.Attachments {
		req.Attachments = append(req.Attachments, proto.Attachment{Name: "image", MIME: at.Mime, Data: base64Of(msg.data[at.SHA])})
	}
	// Queue entries come from ordered injected events, not optimistic sends or
	// HTTP acknowledgments, which can arrive after a message was consumed.
	rt.sending++
	if rt.Session.Title == "" && rt.Session.Len() == 0 {
		rt.Session.Title = "" // The daemon derives the authoritative readable title.
	}
	if !rt.Running {
		rt.Running, rt.Phase, rt.LastErr = true, "preparing", false
		rt.RequestID = ""
		rt.RunStart, rt.RunTurns, rt.RunJS, rt.RunUsage0 = nowFn(), 0, 0, rt.Usage
		rt.Title, rt.Scroll = "", 0
		a.clearLive()
	}
	a.pushSave(func() { // after any model or mode change the user made just before
		var resp proto.MessageResponse
		err := a.rpc(func(ctx context.Context, c *client.Client) error {
			var e error
			resp, e = c.Message(ctx, sid, req)
			return e
		})
		a.postEvent(sendResult{sid: sid, resp: resp, err: err, msg: msg})
	})
}

// sendResult is the daemon's answer to a message.
type sendResult struct {
	sid  string
	resp proto.MessageResponse
	err  error
	msg  msgDraft
}

func (a *App) onSendResult(m sendResult) {
	rt := a.rts[m.sid]
	if rt == nil {
		return
	}
	rt.sending = max(0, rt.sending-1)
	if sessionGone(m.err) {
		a.replaceGone(rt, m.msg)
		return
	}
	if m.err != nil {
		rt.Running, rt.Phase = false, ""
		rt.Queued = nil
		a.noticeFor(rt, "not sent: "+m.err.Error()+" · esc to dismiss", true, true)
		a.restoreDraftFor(rt, []msgDraft{m.msg})
		a.saveDraftFor(rt)
		return
	}
	if m.resp.RequestID != "" {
		rt.RequestID = m.resp.RequestID
	}
}

// abortTurn asks the daemon to stop the run of the current session.
func (a *App) abortTurn() {
	rt := a.st.Runtime
	sid, rid := rt.ID(), rt.RequestID
	a.spawn(func() {
		if err := a.rpc(func(ctx context.Context, c *client.Client) error {
			return c.Stop(ctx, sid, proto.StopRequest{RequestID: rid})
		}); err != nil {
			a.postEvent(errNotice{text: "stop failed: " + err.Error()})
		}
	})
}

func (a *App) currentProvider() (llm.Provider, bool) {
	prov, model, _ := a.selection()
	if model == "" || prov == "" {
		return llm.Provider{}, false
	}
	for _, p := range a.st.Providers {
		if p.Name == prov && p.Active() {
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
