package web

import (
	"context"
	"errors"
	"testing"
)

type fakeBackend struct {
	res SearchResult
	err error
}

func (f fakeBackend) Search(context.Context, string) (SearchResult, error) { return f.res, f.err }

func TestSearchNotConfiguredIsExplicit(t *testing.T) {
	_, err := (&Service{}).Search(context.Background(), "q", SearchOptions{})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("got %v", err)
	}
}

func TestSearchReturnsSourcesAndMarksThemUntrusted(t *testing.T) {
	s := &Service{Backend: fakeBackend{res: SearchResult{Answer: "a", Sources: []Source{{URL: "https://x.test"}}}}}
	r, err := s.Search(context.Background(), " golang ", SearchOptions{})
	if err != nil || r.Query != "golang" || len(r.Sources) != 1 || r.Notice == "" {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestSearchBackendErrorIsPropagated(t *testing.T) {
	boom := errors.New("401 unauthorized")
	_, err := (&Service{Backend: fakeBackend{err: boom}}).Search(context.Background(), "q", SearchOptions{})
	if !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	if _, err = (&Service{Backend: fakeBackend{}}).Search(context.Background(), "  ", SearchOptions{}); err == nil {
		t.Fatal("empty query must fail")
	}
}
