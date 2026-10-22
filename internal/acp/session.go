package acp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"meno/internal/api"
	"meno/internal/client"
)

// outcome ends one ACP prompt.
type outcome struct {
	stop string // end_turn or cancelled
	err  error
	// asked is set when the prompt ends at a question; the daemon run stays paused.
	asked *api.AskForm
}

// session is one ACP session: a daemon session followed for as long as the
// adapter runs. Updates reach the client even outside a foreground prompt.
type session struct {
	id   string
	srv  *Server
	stop context.CancelFunc

	mu          sync.Mutex
	loading     bool
	ready       chan struct{}
	readyOnce   sync.Once
	sentText    string
	sentThink   string
	seenCalls   map[string]bool
	seenTurns   map[string]bool
	waiter      chan outcome
	requestID   string
	cancelled   bool
	lastErr     string
	pendingAsk  *api.AskForm
	announced   string
	title       string
	model, prov string
	effort      string
	running     bool
	ctxSize     int
}

func newSession(id string, srv *Server, loading bool) *session {
	return &session{id: id, srv: srv, loading: loading, ready: make(chan struct{}),
		seenCalls: map[string]bool{}, seenTurns: map[string]bool{}}
}

// follow starts streaming daemon updates into ACP updates and waits for the
// first state, so replay finishes before session/load answers.
func (s *session) follow(ctx context.Context) error {
	ctx, s.stop = context.WithCancel(ctx)
	errc := make(chan error, 1)
	go func() {
		err := s.srv.backend.Follow(ctx, s.id, s.apply)
		s.readyOnce.Do(func() { close(s.ready) })
		errc <- err
	}()
	select {
	case <-s.ready:
		select {
		case err := <-errc:
			if err != nil && ctx.Err() == nil {
				return err
			}
		default:
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *session) send(update map[string]any) {
	_ = s.srv.conn.Notify("session/update", map[string]any{"sessionId": s.id, "update": update})
}

// apply turns one accepted projection update into ACP updates.
func (s *session) apply(u client.ProjectionUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	first := u.Reset && !s.isReady()
	v := u.Values
	if u.Changed&client.MetadataChanged != 0 {
		s.applyMetadata(v, first)
	}
	if first && s.loading {
		for _, up := range historyUpdates(v.Turns) {
			s.send(up)
		}
	}
	if first {
		for _, t := range v.Turns {
			s.seenTurns[t.ID] = true
		}
	}
	if u.Changed&client.ActivityChanged != 0 {
		s.running = v.Running
		if v.Running && v.RequestID != "" {
			s.requestID = v.RequestID
		}
	}
	if u.Changed&client.LiveChanged != 0 {
		s.applyLive(v.Live.Text, v.Live.Thinking)
	}
	if u.Effects.CallStarted != nil {
		c := u.Effects.CallStarted
		if !s.seenCalls[c.CallID] {
			s.seenCalls[c.CallID] = true
			s.send(toolStart(c.CallID, c.Command, c.Title))
		}
	}
	if e := u.Effects.CallEnded; e != nil {
		if !s.seenCalls[e.CallID] {
			s.seenCalls[e.CallID] = true
			s.send(toolStart(e.CallID, "", ""))
		}
		s.send(toolEnd(e.CallID, e.Output, e.Exit, e.TaskID))
	}
	if u.Changed&client.TurnsChanged != 0 && !u.Reset {
		s.applyTurns(v.Turns)
	}
	if u.Changed&client.UsageChanged != 0 {
		s.applyUsage(v)
	}
	if u.Effects.Error != nil {
		s.lastErr = u.Effects.Error.Message
	}
	if u.Changed&client.AskChanged != 0 {
		s.applyAsk(v.Ask)
	}
	if u.Effects.Finished {
		s.finish(u.Effects.Aborted)
	}
	s.readyOnce.Do(func() { close(s.ready) })
	return nil
}

func (s *session) isReady() bool {
	select {
	case <-s.ready:
		return true
	default:
		return false
	}
}

func (s *session) applyLive(text, think string) {
	if rest, _ := remainder(text, s.sentText); rest != "" {
		s.send(chunk("agent_message_chunk", rest))
	}
	if rest, _ := remainder(think, s.sentThink); rest != "" {
		s.send(chunk("agent_thought_chunk", rest))
	}
	s.sentText, s.sentThink = text, think
}

func (s *session) applyTurns(turns []api.Turn) {
	for _, t := range turns {
		if t.Role != "assistant" {
			if t.ID != "" {
				s.seenTurns[t.ID] = true
			}
			continue
		}
		if t.ID != "" && s.seenTurns[t.ID] {
			continue
		}
		for _, up := range assistantUpdates(t, s.sentText, s.sentThink, s.seenCalls) {
			s.send(up)
		}
		for _, p := range t.Parts {
			if p.Call != nil {
				s.seenCalls[p.Call.CallID] = true
			}
		}
		if t.ID != "" {
			s.seenTurns[t.ID] = true
		}
		s.sentText, s.sentThink = "", ""
	}
}

func (s *session) applyMetadata(v client.MirrorState, first bool) {
	if name := v.Session.Name; name != s.title {
		s.title = name
		if !first && name != "" {
			s.send(map[string]any{"sessionUpdate": "session_info_update", "title": name, "updatedAt": time.Now().UTC().Format(time.RFC3339)})
		}
	}
	changed := v.Model != s.model || v.Provider != s.prov || v.Effort != s.effort
	s.model, s.prov, s.effort = v.Model, v.Provider, v.Effort
	if changed && !first {
		go s.srv.sendConfig(s)
	}
}

func (s *session) applyUsage(v client.MirrorState) {
	if s.ctxSize <= 0 {
		return
	}
	u := map[string]any{"sessionUpdate": "usage_update", "used": v.Context, "size": s.ctxSize}
	if !v.Unpriced && v.Spent > 0 {
		u["cost"] = map[string]any{"amount": v.Spent, "currency": "USD"}
	}
	s.send(u)
}

// applyAsk announces an open form once, as a chat question.
func (s *session) applyAsk(f *api.AskForm) {
	s.pendingAsk = f
	if f == nil || f.AskID == s.announced {
		return
	}
	s.announced = f.AskID
	s.send(chunk("agent_message_chunk", formatQuestion(*f)))
	if s.waiter != nil {
		s.resolve(outcome{stop: "end_turn", asked: f})
	}
}

// finish ends the foreground prompt when its run is done.
func (s *session) finish(aborted bool) {
	if s.waiter == nil {
		return
	}
	o := outcome{stop: "end_turn"}
	switch {
	case aborted || s.cancelled:
		o.stop = "cancelled"
	case s.lastErr != "":
		o.err = fmt.Errorf("%s", s.lastErr)
	}
	s.resolve(o)
}

func (s *session) resolve(o outcome) {
	w := s.waiter
	s.waiter, s.cancelled, s.lastErr = nil, false, ""
	w <- o
}

// begin registers the foreground prompt; it fails while another is active.
func (s *session) begin() (chan outcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.waiter != nil {
		return nil, &rpcError{Code: codeInvalidRequest, Message: "a prompt is already running in this session"}
	}
	s.waiter = make(chan outcome, 1)
	s.cancelled, s.lastErr = false, ""
	return s.waiter, nil
}

// abandon drops the foreground prompt after a failed start.
func (s *session) abandon() {
	s.mu.Lock()
	s.waiter = nil
	s.mu.Unlock()
}

// wait blocks for the prompt outcome. The client's cancel is handled by cancel.
func (s *session) wait(ctx context.Context, w chan outcome) outcome {
	select {
	case o := <-w:
		return o
	case <-ctx.Done():
		return outcome{stop: "cancelled"}
	}
}

// cancel stops the run of the current prompt. It never touches a form.
func (s *session) cancel(ctx context.Context) {
	s.mu.Lock()
	rid, active := s.requestID, s.waiter != nil
	if active {
		s.cancelled = true
	}
	running := s.running
	s.mu.Unlock()
	if !active {
		return
	}
	if !running {
		s.mu.Lock()
		if s.waiter != nil {
			s.resolve(outcome{stop: "cancelled"})
		}
		s.mu.Unlock()
		return
	}
	_ = s.srv.backend.Stop(ctx, s.id, api.StopRequest{RequestID: rid})
}

// detach ends the adapter's side. A run executing for a prompt is stopped; a
// run paused on a question stays paused for a later connection.
func (s *session) detach() {
	s.mu.Lock()
	rid, active, asked := s.requestID, s.running && s.waiter != nil, s.pendingAsk != nil
	s.mu.Unlock()
	if active && !asked {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = s.srv.backend.Stop(ctx, s.id, api.StopRequest{RequestID: rid})
		cancel()
	}
	if s.stop != nil {
		s.stop()
	}
}

// formatQuestion renders a form as a normal chat message with its answer syntax.
func formatQuestion(f api.AskForm) string {
	var b strings.Builder
	title := f.Title
	if title == "" {
		title = "Meno needs an answer"
	}
	fmt.Fprintf(&b, "**%s** (question id: `%s`)\n\n", title, f.AskID)
	example := map[string]string{}
	for i, q := range f.Questions {
		fmt.Fprintf(&b, "%d. `%s` — %s", i+1, q.ID, q.Prompt)
		switch q.Type {
		case "single":
			b.WriteString(" (choose one)")
			example[q.ID] = "<option id>"
		case "multi":
			b.WriteString(" (choose any)")
			example[q.ID] = "<option ids>"
		default:
			example[q.ID] = "<your answer>"
		}
		b.WriteString("\n")
		choices := q.Choices
		if len(choices) == 0 {
			for _, o := range q.Options {
				choices = append(choices, api.AskChoice{ID: o, Label: o})
			}
		}
		for _, c := range choices {
			fmt.Fprintf(&b, "   - `%s`: %s\n", c.ID, c.Label)
		}
		if q.Recommendation != "" {
			fmt.Fprintf(&b, "   (suggested: %s)\n", q.Recommendation)
		}
	}
	if f.Deadline != "" {
		fmt.Fprintf(&b, "\nAnswer before %s.\n", f.Deadline)
	}
	fmt.Fprintf(&b, "\nReply with `/answer %s %s` (a single choice is a string, multiple choices an array, extra text as {\"selected\":[...],\"text\":\"...\"}) or `/answer %s --cancel`.", f.AskID, string(mustJSON(example)), f.AskID)
	return b.String()
}
