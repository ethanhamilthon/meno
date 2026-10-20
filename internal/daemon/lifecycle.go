package daemon

import (
	"context"
	"net/http"
	"os"
	"time"
)

// Serve listens on the configured unix socket until context cancellation.
func (s *Server) Serve(ctx context.Context) error {
	lock, err := lockFile(s.runDir)
	if err != nil {
		return err
	}
	inherited := os.Getenv("MENO_DAEMON_LOCK_FD") != ""
	_ = os.Unsetenv("MENO_DAEMON_LOCK_FD")
	if inherited {
		if err = waitForInheritedLock(lock, 5*time.Second); err != nil {
			lock.Close()
			return err
		}
	} else if err = acquireLock(lock); err != nil {
		lock.Close()
		return ErrAlreadyRunning
	}
	s.lock = lock
	defer s.cleanup()
	listener, err := listenSocket(s.socket)
	if err != nil {
		return err
	}
	s.listener = listener
	writePID(s.runDir, s.pid)
	_ = RotateLog(s.logPath)
	s.http = &http.Server{Handler: s.routes(), ReadHeaderTimeout: 5 * time.Second}
	go s.stopOnContext(ctx)
	go s.idleLoop(ctx)
	err = s.http.Serve(listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
func (s *Server) stopOnContext(ctx context.Context) { <-ctx.Done(); s.Stop("signal") }

// Stop publishes bye and stops HTTP listeners.
func (s *Server) Stop(reason string) {
	s.stopOnce.Do(func() {
		s.hub.Close(reason)
		s.global.Close(reason)
		if s.http != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.http.Shutdown(ctx)
		}
		close(s.done)
	})
}
func (s *Server) cleanup() {
	if s.listener != nil {
		_ = s.listener.Close()
	}
	if s.http != nil {
		_ = s.http.Close()
	}
	closeSocket(s.socket)
	removePID(s.runDir)
	if s.lock != nil {
		_ = unlockFile(s.lock)
		_ = s.lock.Close()
	}
}
func (s *Server) idleLoop(ctx context.Context) {
	if s.idle <= 0 {
		return
	}
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	last := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.done:
			return
		case <-tick.C:
			if s.hub.Subscribers() > 0 || s.global.Subscribers() > 0 || s.backend.HasWork(ctx) {
				last = time.Now()
				continue
			}
			if time.Since(last) >= s.idle {
				s.Stop("idle")
				return
			}
		}
	}
}
