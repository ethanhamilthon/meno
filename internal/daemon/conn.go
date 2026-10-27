package daemon

import (
	"context"
	"errors"
	"net"
	"sync"

	"meno/internal/callfd"
)

// MemListener is an in-process net.Listener: Dial hands the server end to Accept.
type MemListener struct {
	ch   chan net.Conn
	done chan struct{}
	once sync.Once
}

// NewMemListener creates a listener that needs no file or port.
func NewMemListener() *MemListener {
	return &MemListener{ch: make(chan net.Conn), done: make(chan struct{})}
}

func (l *MemListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.ch:
		return c, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *MemListener) Close() error { l.once.Do(func() { close(l.done) }); return nil }

func (l *MemListener) Addr() net.Addr { return memAddr{} }

// Dial connects a client to the listener.
func (l *MemListener) Dial(ctx context.Context) (net.Conn, error) {
	client, server, err := callfd.Pair()
	if err != nil {
		return nil, err
	}
	select {
	case l.ch <- server:
		return client, nil
	case <-l.done:
	case <-ctx.Done():
	}
	client.Close()
	server.Close()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return nil, errors.New("listener closed")
}

type memAddr struct{}

func (memAddr) Network() string { return "mem" }
func (memAddr) String() string  { return "mem" }

// oneConn is a listener that yields one connection, then blocks until that
// connection closes and reports the listener closed.
type oneConn struct {
	c    net.Conn
	once sync.Once
	done chan struct{}
}

func (l *oneConn) Accept() (net.Conn, error) {
	var c net.Conn
	l.once.Do(func() { c = &notifyConn{Conn: l.c, closed: l.done} })
	if c != nil {
		return c, nil
	}
	<-l.done
	return nil, net.ErrClosed
}
func (l *oneConn) Close() error   { return nil }
func (l *oneConn) Addr() net.Addr { return memAddr{} }

type notifyConn struct {
	net.Conn
	closed chan struct{}
	once   sync.Once
}

func (c *notifyConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return c.Conn.Close()
}
