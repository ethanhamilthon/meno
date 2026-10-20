package daemon

import (
	"context"
	"errors"
	"testing"
)

func TestSecondServeReportsAlreadyRunning(t *testing.T) {
	home := t.TempDir()
	s, e := New(Options{Home: home})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveErr := make(chan error, 1)
	go func() { serveErr <- s.Serve(ctx) }()
	waitForServer(t, s)
	other, _ := New(Options{Home: home})
	e = other.Serve(context.Background())
	if !errors.Is(e, ErrAlreadyRunning) {
		t.Fatalf("error %v", e)
	}
	s.Stop("test")
	cancel()
	if e = <-serveErr; e != nil {
		t.Fatal(e)
	}
}
