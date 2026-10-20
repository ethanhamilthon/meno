package engine

import (
	"database/sql"
	"errors"
	"fmt"
	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/proc"
	"meno/internal/proto"
	"os"
	"time"
)

func notFound(err error) error {
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, sql.ErrNoRows) {
		return api(proto.CodeNotFound, "session not found")
	}
	return err
}
func cloneMap(m map[string]string) map[string]string {
	o := map[string]string{}
	for k, v := range m {
		o[k] = v
	}
	return o
}
func (e *Engine) provider(name string) (llm.Provider, bool) {
	for _, p := range e.providerList() {
		if p.Name == name && !p.Disabled {
			return p, true
		}
	}
	return llm.Provider{}, false
}
func turnWire(t agent.Turn) proto.Turn {
	role := t.Role
	content := t.Text
	parts := []proto.TurnPart{}
	if t.Thinking != "" {
		parts = append(parts, proto.TurnPart{Type: "thinking", Text: t.Thinking})
	}
	if t.Code != "" || t.CallID != "" {
		c := &proto.CallState{CallID: t.CallID, Command: t.Code, Title: t.Title, Output: t.Output, ElapsedMS: t.ElapsedMs}
		parts = append(parts, proto.TurnPart{Type: "call", Call: c})
	}
	if t.Err != "" {
		parts = append(parts, proto.TurnPart{Type: "error", Text: t.Err})
	}
	if len(parts) == 0 && content != "" {
		parts = append(parts, proto.TurnPart{Type: "text", Text: content})
	}
	created := ""
	if !t.Time.IsZero() {
		created = t.Time.UTC().Format(time.RFC3339Nano)
	}
	out := proto.Turn{ID: fmt.Sprint(t.HID), Role: role, Content: content, Parts: parts, CreatedAt: created,
		Kind: t.Kind, Mode: t.Mode, Thinking: thinkingOf(t), Err: t.Err, ElapsedMS: t.ElapsedMs, LimitMS: t.LimitMs, TimedOut: t.TimedOut}
	for _, a := range t.Attachments {
		out.Attachments = append(out.Attachments, proto.TurnAttachment{N: a.N, Mime: a.Mime, SHA: a.SHA, W: a.W, H: a.H})
	}
	if t.Run != nil {
		r := t.Run
		out.Run = &proto.RunStats{Status: r.Status, ElapsedMS: r.ElapsedMs, Turns: r.Turns, ToolCalls: r.ToolCalls, ToolMS: r.ToolMs, In: r.Usage.In, Out: r.Usage.Out, Cached: r.Usage.Cached, Cost: r.Cost, Err: r.Err}
	}
	return out
}
func taskWire(t proc.Task) proto.Task {
	return proto.Task{ID: t.ID, Name: t.Name, Status: t.Status, Command: t.Command, StartedAt: t.Started.UTC().Format(time.RFC3339Nano), Exit: t.ExitCode}
}
func askWire(a *askState) *proto.AskForm {
	if a == nil {
		return nil
	}
	f := proto.AskForm{AskID: a.id, By: a.by, Questions: []proto.AskQuestion{}}
	for _, q := range a.form.Questions {
		x := proto.AskQuestion{ID: q.ID, Prompt: q.Text, Type: q.Type, Required: true}
		x.Recommendation = q.Recommendation
		for _, o := range q.Options {
			x.Options = append(x.Options, o.ID)
			x.Choices = append(x.Choices, proto.AskChoice{ID: o.ID, Label: o.Label})
		}
		f.Questions = append(f.Questions, x)
	}
	if !a.deadline.IsZero() {
		f.Deadline = a.deadline.UTC().Format(time.RFC3339Nano)
	}
	return &f
}
