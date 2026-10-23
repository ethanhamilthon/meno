package cli

import (
	"context"
	"time"
)

// Daemon reports the state of the daemon. Start, stop and restart are done by
// "meno daemon" itself, which runs before this DaemonAPI is used.
func (a *DaemonAPI) Daemon(action string, force bool) (DaemonStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h, err := a.c.Hello(ctx)
	if err != nil {
		return DaemonStatus{}, nil
	}
	return DaemonStatus{Running: true, PID: h.PID, Version: h.Version}, nil
}
