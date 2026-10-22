package engine

import (
	"database/sql"
	"errors"
	"fmt"
	"meno/internal/agent"
	"meno/internal/api"
	"meno/internal/llm"
	"meno/internal/process"
	"os"
	"time"
)

func notFound(err error) error {
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, sql.ErrNoRows) {
		return dapi(api.CodeNotFound, "session not found")
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
func turnWire(t agent.Turn) api.Turn {
	role := t.Role
	content := t.Text
	parts := []api.TurnPart{}
	if t.Thinking != "" {
		parts = append(parts, api.TurnPart{Type: "thinking", Text: t.Thinking})
	}
	if t.Code != "" || t.CallID != "" {
		c := &api.CallState{CallID: t.CallID, Command: t.Code, Title: t.Title, Output: t.Output, ElapsedMS: t.ElapsedMs}
		parts = append(parts, api.TurnPart{Type: "call", Call: c})
	}
	if t.Err != "" {
		parts = append(parts, api.TurnPart{Type: "error", Text: t.Err})
	}
	if len(parts) == 0 && content != "" {
		parts = append(parts, api.TurnPart{Type: "text", Text: content})
	}
	created := ""
	if !t.Time.IsZero() {
		created = t.Time.UTC().Format(time.RFC3339Nano)
	}
	out := api.Turn{ID: fmt.Sprint(t.HID), Role: role, Content: content, Parts: parts, CreatedAt: created,
		Kind: t.Kind, Thinking: thinkingOf(t), Err: t.Err, ElapsedMS: t.ElapsedMs, LimitMS: t.LimitMs, TimedOut: t.TimedOut}
	for _, a := range t.Attachments {
		out.Attachments = append(out.Attachments, api.TurnAttachment{N: a.N, Mime: a.Mime, SHA: a.SHA, W: a.W, H: a.H})
	}
	if t.Run != nil {
		r := t.Run
		out.Run = &api.RunStats{Status: r.Status, ElapsedMS: r.ElapsedMs, Turns: r.Turns, ToolCalls: r.ToolCalls, ToolMS: r.ToolMs, In: r.Usage.In, Out: r.Usage.Out, Cached: r.Usage.Cached, CacheWrite: r.Usage.CacheWrite, CacheKnown: r.Usage.CacheReported, Cost: r.Cost, Partial: r.Partial, Err: r.Err}
	}
	return out
}
func taskWire(t process.Task) api.Task {
	return api.Task{ID: t.ID, Name: t.Name, Status: t.Status, Command: t.Command, StartedAt: t.Started.UTC().Format(time.RFC3339Nano), Exit: t.ExitCode}
}
func askWire(a *askState) *api.AskForm {
	if a == nil {
		return nil
	}
	f := api.AskForm{AskID: a.id, By: a.by, Questions: []api.AskQuestion{}}
	for _, q := range a.form.Questions {
		x := api.AskQuestion{ID: q.ID, Prompt: q.Text, Type: q.Type, Required: true}
		x.Recommendation = q.Recommendation
		for _, o := range q.Options {
			x.Options = append(x.Options, o.ID)
			x.Choices = append(x.Choices, api.AskChoice{ID: o.ID, Label: o.Label})
		}
		f.Questions = append(f.Questions, x)
	}
	if !a.deadline.IsZero() {
		f.Deadline = a.deadline.UTC().Format(time.RFC3339Nano)
	}
	return &f
}
