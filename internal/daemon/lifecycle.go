package daemon

import (
	"context"
	"io"
	"net"
	"net/http"
	"time"
)

// Start runs the start hook (the configuration service).
func (s *Server) Start() error {
	if s.onStart != nil {
		return s.onStart()
	}
	return nil
}

// Serve accepts connections on l until Stop or l is closed.
func (s *Server) Serve(l net.Listener) error {
	s.track(l)
	return s.serve(l, s.handler)
}

// ServeCall serves one connection of a bash call until it closes. The channel
// itself says who is asking: every request gets the token and call ID of the
// call, whatever the client sent, so a process of the call cannot ask as a human
// or as another session. The connection is not tracked: it ends with the call.
func (s *Server) ServeCall(c net.Conn, token, call string) {
	bound := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del("Authorization")
		r.Header.Set("X-Meno-Token", token)
		r.Header.Set("X-Meno-Call", call)
		s.handler.ServeHTTP(w, r)
	})
	_ = s.serve(&oneConn{c: c, done: make(chan struct{})}, bound)
}

func (s *Server) serve(l net.Listener, h http.Handler) error {
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second}
	s.mu.Lock()
	s.servers[srv] = struct{}{}
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.servers, srv); s.mu.Unlock() }()
	err := srv.Serve(l)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// ServeMem starts the server and serves it on an in-memory listener. It returns
// the dial function of a client.
func (s *Server) ServeMem() (func(context.Context) (net.Conn, error), error) {
	if err := s.Start(); err != nil {
		return nil, err
	}
	ln := NewMemListener()
	s.track(ln) // before the goroutine runs, so an early Stop still closes it
	go func() { _ = s.Serve(ln) }()
	return ln.Dial, nil
}

// Stop publishes bye to every stream, closes the listeners and runs the stop hook once.
func (s *Server) Stop(reason string) {
	s.stopOnce.Do(func() {
		s.mu.Lock()
		for _, c := range s.closers {
			_ = c.Close()
		}
		s.mu.Unlock()
		s.hub.Close(reason)
		s.global.Close(reason)
		if s.onStop != nil {
			s.onStop()
		}
		s.mu.Lock()
		open := make([]*http.Server, 0, len(s.servers))
		for srv := range s.servers {
			open = append(open, srv)
		}
		s.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, srv := range open {
			_ = srv.Shutdown(ctx) // lets running requests finish
		}
	})
}

func (s *Server) track(c io.Closer) {
	s.mu.Lock()
	s.closers = append(s.closers, c)
	s.mu.Unlock()
}
