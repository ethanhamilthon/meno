package mcp

import (
	"context"
	"testing"
	"time"
)

func TestWaitReady(t *testing.T) {
	m := NewManager()
	if !m.WaitReady(context.Background(), time.Second) {
		t.Fatal("nothing starting: ready at once")
	}
	m.BeginStart("a")
	if !m.Starting("a") {
		t.Fatal("BeginStart must mark the server")
	}
	start := time.Now()
	if m.WaitReady(context.Background(), 80*time.Millisecond) || time.Since(start) < 70*time.Millisecond {
		t.Fatal("must time out while a server is starting")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if m.WaitReady(ctx, time.Minute) {
		t.Fatal("a canceled context ends the wait")
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		m.EndStart("a")
	}()
	if !m.WaitReady(context.Background(), 3*time.Second) || m.Starting("a") {
		t.Fatal("must wake up when the server settles")
	}
}
