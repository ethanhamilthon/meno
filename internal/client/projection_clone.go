package client

import "meno/internal/proto"

func cloneTurns(in []proto.Turn) []proto.Turn {
	out := append([]proto.Turn(nil), in...)
	for i := range out {
		t := &out[i]
		t.Parts = append([]proto.TurnPart(nil), t.Parts...)
		for j := range t.Parts {
			p := &t.Parts[j]
			p.Data = append([]byte(nil), p.Data...)
			if p.Call != nil {
				c := cloneLiveState(proto.LiveState{Calls: []proto.CallState{*p.Call}}).Calls[0]
				p.Call = &c
			}
		}
		t.Attachments = append([]proto.TurnAttachment(nil), t.Attachments...)
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
func cloneAsk(in *proto.AskForm) *proto.AskForm {
	if in == nil {
		return nil
	}
	a := *in
	a.Questions = append([]proto.AskQuestion(nil), a.Questions...)
	for i := range a.Questions {
		q := &a.Questions[i]
		q.Options = append([]string(nil), q.Options...)
		q.Choices = append([]proto.AskChoice(nil), q.Choices...)
	}
	return &a
}
func cloneRetry(in *proto.RetryState) *proto.RetryState {
	if in == nil {
		return nil
	}
	r := *in
	return &r
}
func cloneTasks(in []proto.Task) []proto.Task {
	out := append([]proto.Task(nil), in...)
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
