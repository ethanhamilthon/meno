package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"

	"meno/internal/api"
	"meno/internal/daemon"
)

// StopConfirmed requests shutdown without ever starting a daemon, then takes
// lifetime ownership to prove service cleanup finished. The caller must hold
// the transition lock; retain the returned handle through executable commit.
func StopConfirmed(ctx context.Context, home string, idleOnly bool) (*os.File, error) {
	c := New(home)
	h, err := c.Hello(ctx)
	absent := errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED)
	if err != nil && !absent {
		return nil, fmt.Errorf("cannot contact daemon: %w", err)
	}
	if err == nil {
		path := "/v1/shutdown"
		if idleOnly {
			if h.Protocol != api.Protocol {
				return nil, fmt.Errorf("daemon protocol %d cannot safely stop for update; finish work, close all meno windows, run 'meno daemon stop', then retry", h.Protocol)
			}
			path += "/idle"
		}
		if err = c.post(ctx, path, api.ShutdownRequest{Reason: "client request"}, nil); err != nil {
			return nil, fmt.Errorf("daemon refused shutdown: %w", err)
		}
	}
	f, err := daemon.AcquireOwnership(ctx, home, "daemon.lock")
	if err != nil {
		return nil, fmt.Errorf("cannot confirm daemon cleanup completed: %w", err)
	}
	// Owning daemon.lock is authoritative even if a stale socket remains after a crash.
	return f, nil
}
