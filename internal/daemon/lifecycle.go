package daemon

import (
	"context"
	"net/http"
	"os"
	"time"
)

// Serve listens on the configured unix socket until context cancellation.
func (s *Server) Serve(ctx context.Context) error {
	lock := s.lock
	var err error
	if lock == nil {
		lock, err = lockFile(s.runDir)
	}
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
	} else if s.lock == nil {
		if err = acquireLock(lock); err != nil {
			lock.Close()
			return ErrAlreadyRunning
		}
	}
	s.lock = lock
	defer s.cleanup()
	listener, err := listenSocket(s.socket)
	if err != nil {
		return err
	}
	s.listener = listener
	if err = s.startHTTP(ctx); err != nil {
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
	go s.stopOnContext(ctx)
	go s.idleLoop(ctx)
	err = s.http.Serve(listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
func (s *Server) stopOnContext(ctx context.Context) {
	select {
	case <-ctx.Done():
		s.Stop("signal")
	case <-s.done:
	}
}

// Stop publishes bye and stops HTTP listeners.
func (s *Server) Stop(reason string) {
	s.stopOnce.Do(func() {
		s.startMu.Lock()
		defer s.startMu.Unlock()
		s.admission.Lock()
		s.stopping = true
		s.admission.Unlock()
		s.hub.Close(reason)
		s.global.Close(reason)
		if s.onStop != nil {
			s.onStop()
		}
		if s.http != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.http.Shutdown(ctx)
		}
		close(s.done)
	})
}
func (s *Server) cleanup() {
	s.Stop("serve ended")
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

// startHTTP and Stop serialize startup hooks and HTTP construction. Shutdown
// before Serve must not leave a server listening after stopOnce has completed.
func (s *Server) startHTTP(ctx context.Context) error {
	s.startMu.Lock()
	defer s.startMu.Unlock()
	s.admission.Lock()
	stopped := s.stopping
	s.admission.Unlock()
	if stopped {
		return http.ErrServerClosed
	}
	s.http = &http.Server{Handler: s.routes(), ReadHeaderTimeout: 5 * time.Second}
	writePID(s.runDir, s.pid)
	if s.onStart != nil {
		if err := s.onStart(ctx); err != nil {
			return err
		}
	}
	_ = RotateLog(s.logPath)
	return nil
}
