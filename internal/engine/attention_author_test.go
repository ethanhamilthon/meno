package engine

import (
	"context"
	"errors"
	"sync"
	"testing"

	"meno/internal/agent"
	"meno/internal/api"
	"meno/internal/llm"
)

func assertCompleted(t *testing.T, e *Engine, id string, want int64) {
	t.Helper()
	got, _ := e.attention(id)
	if got != want {
		t.Fatalf("completed = %d, want %d", got, want)
	}
}

func TestCompletionMessageClients(t *testing.T) {
	for _, client := range []string{"tui", "acp", "cli", "agent", "", "unknown"} {
		t.Run(client, func(t *testing.T) {
			e, _ := fixture(t, &fakeProvider{steps: [][]llm.Event{{{Kind: llm.EvText, Text: "answer"}, {Kind: llm.EvDone}}}})
			id := newSession(t, e)
			if _, err := e.Message(context.Background(), id, api.MessageRequest{Text: "hello", Client: client}); err != nil {
				t.Fatal(err)
			}
			waitFor(t, e, id, func(s *runState) bool { return !s.running })
			want := int64(0)
			if client == "tui" {
				want = 1
			}
			assertCompleted(t, e, id, want)
			s, err := agent.LoadSession(e.home, id)
			if err != nil || s.Len() < 2 {
				t.Fatalf("history not saved: %v", err)
			}
			st, _ := e.load(context.Background(), id)
			st.mu.Lock()
			defer st.mu.Unlock()
			if len(st.authors) != 0 {
				t.Fatal("authors leaked after finish")
			}
		})
	}
}

// Keep the provider blocked while manipulating the exact runner drain boundary.
func authorRun(t *testing.T, clients ...string) (*Engine, *runState) {
	t.Helper()
	f := &fakeProvider{wait: true, seen: make(chan struct{}, 1)}
	e, _ := fixture(t, f)
	id := newSession(t, e)
	for i, c := range clients {
		if _, err := e.Message(context.Background(), id, api.MessageRequest{Text: c + " message", Client: c}); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			<-f.seen
		}
	}
	s, _ := e.load(context.Background(), id)
	return e, s
}

func TestLastAcceptedAuthorAndRecall(t *testing.T) {
	for _, tc := range []struct {
		clients []string
		want    bool
	}{
		{[]string{"acp", "tui"}, true}, {[]string{"tui", "cli"}, false},
		{[]string{"agent", "tui", "agent"}, false}, {[]string{"tui", "agent", "tui"}, true},
	} {
		t.Run(tc.clients[0]+tc.clients[1]+tc.clients[len(tc.clients)-1], func(t *testing.T) {
			e, s := authorRun(t, tc.clients...)
			s.mu.Lock()
			got := lastAuthorCounts(s.authors)
			s.mu.Unlock()
			if got != tc.want {
				t.Fatal("last author did not win")
			}
			// Simulate the loop consuming one batch, then recall only newer messages.
			s.queue.Take()
			if _, err := e.Message(context.Background(), s.session.ID, api.MessageRequest{Text: "new agent message", Client: "agent"}); err != nil {
				t.Fatal(err)
			}
			e.queueResource(s, "DELETE")
			s.mu.Lock()
			got = lastAuthorCounts(s.authors)
			s.mu.Unlock()
			if got != tc.want {
				t.Fatal("recall removed consumed authors")
			}
		})
	}
	t.Run("recall reverts", func(t *testing.T) {
		e, s := authorRun(t, "tui", "agent")
		s.queue.Push(agent.Message{Viewed: true})
		e.queueResource(s, "DELETE")
		s.mu.Lock()
		defer s.mu.Unlock()
		if !lastAuthorCounts(s.authors) || s.queue.Viewed() != 1 {
			t.Fatal("recall did not preserve previous author and viewed image")
		}
	})
}

func TestConsumedMessagesDecideCanceledAndErrorCompletion(t *testing.T) {
	for _, runErr := range []error{context.Canceled, errors.New("provider failed")} {
		for _, client := range []string{"tui", "agent"} {
			t.Run(runErr.Error()+client, func(t *testing.T) {
				e, _ := fixture(t, &fakeProvider{})
				id := newSession(t, e)
				s, _ := e.load(context.Background(), id)
				s.session.Append(agent.Turn{Role: llm.RoleUser, Text: "start"})
				s.authors = []bool{client == "tui", client != "tui"}
				s.queue.Push(agent.Message{Text: "dropped leftover"})
				e.finish(s, runErr)
				want := int64(0)
				if client == "tui" {
					want = 1
				}
				assertCompleted(t, e, id, want)
				if len(s.authors) != 0 || s.queue.Len() != 0 {
					t.Fatal("dropped authors leaked")
				}
			})
		}
	}
}

func TestLeftoverAuthorsBelongToFollowup(t *testing.T) {
	for _, tc := range []struct {
		authors       []bool
		first, second int64
	}{
		{[]bool{true, false}, 1, 1}, {[]bool{false, false, true}, 0, 1}, {[]bool{true, true, false}, 1, 1},
	} {
		t.Run("leftovers", func(t *testing.T) {
			f := &fakeProvider{wait: true, seen: make(chan struct{}, 1)}
			e, _ := fixture(t, f)
			id := newSession(t, e)
			s, _ := e.load(context.Background(), id)
			s.authors = tc.authors
			s.running = true
			s.session.Append(agent.Turn{Role: llm.RoleUser, Text: "first"})
			for range tc.authors[1:] {
				s.queue.Push(agent.Message{Text: "tail"})
			}
			s.queue.Push(agent.Message{Viewed: true, Text: "view"})
			e.finish(s, nil)
			<-f.seen
			assertCompleted(t, e, id, tc.first)
			s.mu.Lock()
			got := lastAuthorCounts(s.authors)
			s.mu.Unlock()
			if got != tc.authors[len(tc.authors)-1] {
				t.Fatal("follow-up lost last leftover author")
			}
			if err := e.Stop(context.Background(), id, api.StopRequest{}); err != nil {
				t.Fatal(err)
			}
			waitFor(t, e, id, func(s *runState) bool { return !s.running })
			assertCompleted(t, e, id, tc.second)
		})
	}
}

func TestConcurrentAcceptanceAndIdempotentReplay(t *testing.T) {
	e, s := authorRun(t, "tui")
	var wg sync.WaitGroup
	for _, c := range []string{"tui", "agent"} {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			_, err := e.Message(context.Background(), s.session.ID, api.MessageRequest{Text: c, Client: c})
			if err != nil {
				t.Error(err)
			}
		}(c)
	}
	wg.Wait()
	s.mu.Lock()
	items := s.queue.Take()
	got := lastAuthorCounts(s.authors)
	s.mu.Unlock()
	if got != (items[len(items)-1].Text == "tui") {
		t.Fatal("authors differ from acceptance order")
	}
	send := func(key, c string) {
		t.Helper()
		_, err := e.ApplyClientID(context.Background(), key, func() (any, error) {
			return e.Message(context.Background(), s.session.ID, api.MessageRequest{Text: c, Client: c})
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	send("old", "tui")
	send("new", "agent")
	send("old", "tui")
	s.mu.Lock()
	defer s.mu.Unlock()
	if lastAuthorCounts(s.authors) || s.queue.Len() != 2 {
		t.Fatal("old retry changed authorship")
	}
}

func TestRejectedMessageDoesNotChangeAuthor(t *testing.T) {
	e, s := authorRun(t, "tui")
	for _, q := range []api.MessageRequest{{Client: "agent"}, {Client: "agent", Text: "bad image", Attachments: []api.Attachment{{}}}} {
		if _, err := e.Message(context.Background(), s.session.ID, q); err == nil {
			t.Fatal("invalid message accepted")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.authors) != 1 || !lastAuthorCounts(s.authors) {
		t.Fatal("rejected message changed author")
	}
}

func TestSystemWakeAndActiveSteeringAuthorship(t *testing.T) {
	for _, human := range []bool{false, true} {
		t.Run("system wake", func(t *testing.T) {
			f := &fakeProvider{wait: true, seen: make(chan struct{}, 1)}
			e, _ := fixture(t, f)
			id := usedSession(t, e)
			e.enqueueEvent(agent.Event{ID: "wake", Session: id, Source: agent.SourceTask, Kind: "task_done", Text: "finished"}, false)
			<-f.seen
			s, _ := e.load(context.Background(), id)
			s.mu.Lock()
			counts := lastAuthorCounts(s.authors)
			s.mu.Unlock()
			if counts {
				t.Fatal("system wake inherited TUI author")
			}
			if human {
				if _, err := e.Message(context.Background(), id, api.MessageRequest{Text: "human", Client: "tui"}); err != nil {
					t.Fatal(err)
				}
				s.queue.Take() // accepted by the running loop
			}
			e.enqueueEvent(agent.Event{ID: "active", Session: id, Source: agent.SourceTask, Kind: "task_done", Text: "another event"}, false)
			s.mu.Lock()
			counts = lastAuthorCounts(s.authors)
			s.mu.Unlock()
			if counts != human {
				t.Fatal("active steering changed authorship")
			}
			if err := e.Stop(context.Background(), id, api.StopRequest{}); err != nil {
				t.Fatal(err)
			}
			waitFor(t, e, id, func(s *runState) bool { return !s.running })
			want := int64(0)
			if human {
				want = 1
			}
			assertCompleted(t, e, id, want)
		})
	}
}
