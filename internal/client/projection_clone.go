package client

import "meno/internal/api"

func cloneTurns(in []api.Turn) []api.Turn {
	out := append([]api.Turn(nil), in...)
	for i := range out {
		t := &out[i]
		t.Parts = append([]api.TurnPart(nil), t.Parts...)
		for j := range t.Parts {
			p := &t.Parts[j]
			p.Data = append([]byte(nil), p.Data...)
			if p.Call != nil {
				c := cloneLiveState(api.LiveState{Calls: []api.CallState{*p.Call}}).Calls[0]
				p.Call = &c
			}
		}
		t.Attachments = append([]api.TurnAttachment(nil), t.Attachments...)
		if t.Run != nil {
			r := *t.Run
			if r.Cost != nil {
				n := *r.Cost
				r.Cost = &n
			}
			t.Run = &r
		}
	}
	return out
}
func cloneAsk(in *api.AskForm) *api.AskForm {
	if in == nil {
		return nil
	}
	a := *in
	a.Questions = append([]api.AskQuestion(nil), a.Questions...)
	for i := range a.Questions {
		q := &a.Questions[i]
		q.Options = append([]string(nil), q.Options...)
		q.Choices = append([]api.AskChoice(nil), q.Choices...)
	}
	return &a
}
func cloneRetry(in *api.RetryState) *api.RetryState {
	if in == nil {
		return nil
	}
	r := *in
	return &r
}
func cloneTasks(in []api.Task) []api.Task {
	out := append([]api.Task(nil), in...)
	for i := range out {
		if out[i].Exit != nil {
			n := *out[i].Exit
			out[i].Exit = &n
		}
	}
	return out
}
func cloneStarted(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
