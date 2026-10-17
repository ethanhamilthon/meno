package agent

import (
	"context"
	"testing"

	"meno/internal/llm"
)

// pushing is a scripted client that queues a user message during its first request.
type pushing struct {
	scripted
	q    *Queue
	seen [][]llm.Msg
}

func (p *pushing) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	p.seen = append(p.seen, req.Msgs)
	if len(p.seen) == 1 {
		p.q.Push(Message{Text: "also check b"})
		p.q.Push(Message{Text: "and c"})
	}
	return p.scripted.Stream(ctx, req, emit)
}

func TestQueuedMessageJoinsAfterToolResult(t *testing.T) {
	q := &Queue{}
	cl := &pushing{q: q, scripted: scripted{steps: [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: "1+1"}, {Kind: llm.EvDone}},
		{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}},
	}}}
	var evs []Ev
	r := newRunner(cl, &evs)
	r.D.Queue = q
	s := NewSession(".", "m")
	if err := r.Turn(context.Background(), s, "start"); err != nil {
		t.Fatal(err)
	}
	if len(cl.seen) != 2 {
		t.Fatalf("requests: %d", len(cl.seen))
	}
	second := cl.seen[1]
	// user, assistant call, tool result, queued user message
	if len(second) != 4 || second[2].Role != llm.RoleTool || second[3].Role != llm.RoleUser || second[3].Text != "also check b\n\nand c" {
		t.Fatalf("second request: %+v", second)
	}
	if q.Len() != 0 {
		t.Fatal("queue not drained")
	}
}

func TestQueuedMessageDuringFinalAnswerContinuesTurn(t *testing.T) {
	q := &Queue{}
	cl := &pushing{q: q, scripted: scripted{steps: [][]llm.Event{
		{{Kind: llm.EvText, Text: "done"}, {Kind: llm.EvDone}},
		{{Kind: llm.EvText, Text: "and b"}, {Kind: llm.EvDone}},
	}}}
	var evs []Ev
	r := newRunner(cl, &evs)
	r.D.Queue = q
	s := NewSession(".", "m")
	if err := r.Turn(context.Background(), s, "start"); err != nil {
		t.Fatal(err)
	}
	turns := s.Snapshot()
	if len(turns) != 4 || turns[2].Role != llm.RoleUser || turns[3].Text != "and b" {
		t.Fatalf("turns: %+v", turns)
	}
	done := 0
	for _, e := range evs {
		if e.Kind == EvDone {
			done++
		}
	}
	if done != 1 {
		t.Fatalf("EvDone emitted %d times", done)
	}
}

func TestCombineRenumbersImages(t *testing.T) {
	m := Combine([]Message{
		{Text: "look [Image #1] and [Image #2]", Attachments: []Attachment{{N: 1, SHA: "a"}, {N: 2, SHA: "b"}}},
		{Text: "also [Image #1]", Attachments: []Attachment{{N: 1, SHA: "c"}}},
	})
	if m.Text != "look [Image #1] and [Image #2]\n\nalso [Image #3]" {
		t.Fatalf("text: %q", m.Text)
	}
	if len(m.Attachments) != 3 || m.Attachments[2].N != 3 || m.Attachments[2].SHA != "c" {
		t.Fatalf("attachments: %+v", m.Attachments)
	}
}

func TestOnlyNewestImagesAreSent(t *testing.T) {
	s := NewSession(".", "m")
	for i := 0; i < 4; i++ {
		s.Append(Turn{Role: llm.RoleUser, Text: "a [Image #1] b [Image #2]", Attachments: []Attachment{{N: 1, Mime: "image/png", SHA: "x"}, {N: 2, Mime: "image/png", SHA: "y"}}})
	}
	msgs := s.ToMsgs()
	total := 0
	for _, m := range msgs {
		total += len(m.Images)
	}
	if total != MaxRequestImages {
		t.Fatalf("images sent: %d", total)
	}
	if msgs[0].Text != "a [image omitted] b [image omitted]" || msgs[1].Text != "a [image omitted] b [Image #2]" || len(msgs[1].Images) != 1 || msgs[1].Images[0].N != 2 {
		t.Fatalf("omitted: %+v %+v", msgs[0], msgs[1])
	}
}

func TestTextWithImagePaths(t *testing.T) {
	s := NewSession(".", "m")
	s.AttachDir = "/a"
	tr := Turn{Text: "x [Image #1] y [Image #2]", Attachments: []Attachment{{N: 1, Mime: "image/jpeg", SHA: "h1"}}}
	got := s.TextWithImagePaths(tr, func(p string) string { return "<" + p + ">" })
	if got != "x </a/h1.jpg> y [Image #2]" {
		t.Fatal(got)
	}
}

func TestViewedImagesJoinAsSeparateTurnAndStayOutOfUserCounts(t *testing.T) {
	q := &Queue{}
	q.Push(Message{Text: "v1 [Image #1]", Attachments: []Attachment{{N: 1, SHA: "a"}}, Viewed: true})
	q.Push(Message{Text: "hello"})
	if q.Len() != 1 || q.Viewed() != 1 || len(q.Items()) != 1 {
		t.Fatalf("len=%d viewed=%d", q.Len(), q.Viewed())
	}
	cl := &scripted{steps: [][]llm.Event{{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}}}}
	var evs []Ev
	r := newRunner(cl, &evs)
	r.D.Queue = q
	s := NewSession(".", "m")
	if err := r.Turn(context.Background(), s, "start"); err != nil {
		t.Fatal(err)
	}
	turns := s.Snapshot()
	if len(turns) != 4 || turns[1].Kind != KindView || len(turns[1].Attachments) != 1 || turns[2].Kind != "" || turns[2].Text != "hello" {
		t.Fatalf("turns: %+v", turns)
	}
	if got := q.TakeWritten(); len(got) != 0 {
		t.Fatal("queue not drained")
	}
}
