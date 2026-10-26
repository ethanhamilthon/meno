package daemon

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type completeIdleBackend struct {
	NotImplementedBackend
	busy   atomic.Bool
	broken atomic.Bool
}

func (b *completeIdleBackend) HasUpdateWork(context.Context) (bool, error) {
	if b.broken.Load() {
		return false, errors.New("work unavailable")
	}
	return b.busy.Load(), nil
}

func TestIdleDefaultAndReset(t *testing.T) {
	s, e := New(Options{Home: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	if s.idle != 10*time.Second {
		t.Fatal(s.idle)
	}
	base := time.Now()
	s.lastActive = base
	if s.checkIdle(context.Background(), base.Add(9*time.Second)) {
		t.Fatal("premature exit")
	}
	s.touchIdle()
	s.idleMu.Lock()
	last := s.lastActive
	s.idleMu.Unlock()
	if s.checkIdle(context.Background(), last.Add(9*time.Second)) {
		t.Fatal("did not reset")
	}
	if !s.checkIdle(context.Background(), last.Add(10*time.Second)) {
		t.Fatal("no idle shutdown")
	}
}

func TestIdleCountsEitherStreamAndCompleteWork(t *testing.T) {
	for _, kind := range []string{"global", "session", "work", "error"} {
		t.Run(kind, func(t *testing.T) {
			b := &completeIdleBackend{}
			s, e := New(Options{Home: t.TempDir(), Backend: b})
			if e != nil {
				t.Fatal(e)
			}
			base := time.Now()
			s.lastActive = base
			var cancel func()
			switch kind {
			case "global":
				_, cancel, _ = s.global.Subscribe(0, "")
			case "session":
				_, cancel, _ = s.hub.Subscribe("s", 0, "")
			case "work":
				b.busy.Store(true)
			case "error":
				b.broken.Store(true)
			}
			now := base.Add(20 * time.Second)
			if s.checkIdle(context.Background(), now) {
				t.Fatal("active state stopped daemon")
			}
			if cancel != nil {
				cancel()
			}
			b.busy.Store(false)
			b.broken.Store(false)
			if s.checkIdle(context.Background(), now.Add(9*time.Second)) {
				t.Fatal("grace was not restarted")
			}
			if !s.checkIdle(context.Background(), now.Add(10*time.Second)) {
				t.Fatal("finished state kept daemon alive")
			}
		})
	}
}

func TestIdleWaitsForFiniteRequestsAndResetsGrace(t *testing.T) {
	for _, method := range []string{"GET", "POST"} {
		t.Run(method, func(t *testing.T) {
			s, e := New(Options{Home: t.TempDir(), IdleExit: 40 * time.Millisecond})
			if e != nil {
				t.Fatal(e)
			}
			entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			h := s.mutationGate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { close(entered); <-release }))
			go func() { h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, "/v1/test", nil)); close(done) }()
			<-entered
			s.idleMu.Lock()
			s.lastActive = time.Now().Add(-time.Second)
			s.idleMu.Unlock()
			if s.checkIdle(context.Background(), time.Now()) {
				t.Fatal("idle crossed admitted request")
			}
			close(release)
			<-done
			if s.checkIdle(context.Background(), time.Now()) {
				t.Fatal("completed request did not reset grace")
			}

		})
	}
}

func TestIdleRejectsLateReadsAndStreams(t *testing.T) {
	s, e := New(Options{Home: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	s.lastActive = time.Now().Add(-time.Minute)
	if !s.checkIdle(context.Background(), time.Now()) {
		t.Fatal("shutdown not committed")
	}
	for _, path := range []string{"/v1/hello", "/v1/events", "/v1/sessions/s/events"} {
		w := httptest.NewRecorder()
		s.routes().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 503 {
			t.Fatalf("late %s: %d", path, w.Code)
		}
	}
}

func TestLongRequestStillAdmitsItsAnswer(t *testing.T) {
	s, e := New(Options{Home: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	waiting, answer, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	h := s.mutationGate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wait" {
			close(waiting)
			<-answer
		} else {
			close(answer)
		}
	}))
	go func() { h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/wait", nil)); close(done) }()
	<-waiting
	if s.checkIdle(context.Background(), time.Now().Add(time.Minute)) {
		t.Fatal("stopped with long request")
	}
	answered := make(chan struct{})
	go func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/answer", nil))
		close(answered)
	}()
	select {
	case <-answered:
	case <-time.After(time.Second):
		t.Fatal("long request blocked its own answer")
	}
	<-done
}
