// Package callfd lets a bash call reach its owner without a socket file.
//
// The owner passes two descriptors to the call: a control socket (fd 3) and a
// lock file (fd 4). To connect, a client takes the lock, writes 'R' to the
// control socket, receives, as a passed descriptor, one end of a fresh
// socketpair, and answers 'A'; the owner serves the other end. Every client gets
// a private connection, so several meno commands of one call run side by side.
// The lock only keeps their short hand-shakes apart (it is a POSIX record lock,
// held per process). The owner closes its copy of the passed end only after the
// 'A': closing it earlier can lose the connection while it is in flight.
package callfd

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

// FD is the descriptor number of the control socket in the call. The lock file is FD+1.
const FD = 3

// Channel is the owner side of one call's descriptors.
type Channel struct {
	ctl     *net.UnixConn
	child   []*os.File
	handler func(net.Conn)
}

// Open creates the descriptors for one call. handler serves each connection.
func Open(handler func(net.Conn)) (*Channel, error) {
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		return nil, err
	}
	unix.CloseOnExec(fds[0])
	unix.CloseOnExec(fds[1])
	parent, child := os.NewFile(uintptr(fds[0]), "ctl-owner"), os.NewFile(uintptr(fds[1]), "ctl-call")
	defer parent.Close()
	conn, err := net.FileConn(parent)
	if err != nil {
		child.Close()
		return nil, err
	}
	lock, err := os.CreateTemp("", "meno-call-")
	if err != nil {
		conn.Close()
		child.Close()
		return nil, err
	}
	_ = os.Remove(lock.Name())
	c := &Channel{ctl: conn.(*net.UnixConn), child: []*os.File{child, lock}, handler: handler}
	go c.serve()
	return c, nil
}

// Files are the ExtraFiles of the call (fd 3 and 4).
func (c *Channel) Files() []*os.File { return c.child }

// ReleaseChild closes the owner's copies of the call's descriptors; call it
// once the process has started.
func (c *Channel) ReleaseChild() {
	for _, f := range c.child {
		f.Close()
	}
}

// Close stops serving. Connections already handed out stay until they close.
func (c *Channel) Close() { c.ctl.Close() }

// serve answers requests: 'R' asks for a connection, 'A' says it arrived. The
// owner keeps its copies of the passed end until then. Closing them right after
// sendmsg can lose the connection while it is still in flight.
func (c *Channel) serve() {
	buf := make([]byte, 1)
	var pendingFile *os.File
	var pendingConn net.Conn
	release := func() {
		if pendingFile != nil {
			pendingFile.Close()
			pendingConn.Close()
			pendingFile, pendingConn = nil, nil
		}
	}
	defer release()
	for {
		if _, err := c.ctl.Read(buf); err != nil {
			return
		}
		switch buf[0] {
		case ackByte:
			release()
		case requestByte:
			release() // a client that died before it acknowledged
			served, remote, err := Pair()
			if err != nil {
				return
			}
			f, err := remote.(*net.UnixConn).File()
			if err != nil {
				served.Close()
				remote.Close()
				return
			}
			if _, _, err = c.ctl.WriteMsgUnix([]byte{requestByte}, unix.UnixRights(int(f.Fd())), nil); err != nil {
				served.Close()
				f.Close()
				remote.Close()
				return
			}
			pendingFile, pendingConn = f, remote
			go c.handler(served)
		}
	}
}

const (
	requestByte = 'R'
	ackByte     = 'A'
)

// Pair returns two connected stream sockets. Unlike net.Pipe they are buffered
// by the kernel, so a request and a streamed response cannot deadlock.
func Pair() (a, b net.Conn, err error) {
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		return nil, nil, err
	}
	unix.CloseOnExec(fds[0])
	unix.CloseOnExec(fds[1])
	fa, fb := os.NewFile(uintptr(fds[0]), "a"), os.NewFile(uintptr(fds[1]), "b")
	defer fa.Close()
	defer fb.Close()
	if a, err = net.FileConn(fa); err != nil {
		return nil, nil, err
	}
	if b, err = net.FileConn(fb); err != nil {
		a.Close()
		return nil, nil, err
	}
	return a, b, nil
}

// Dial returns a dial function for the descriptors the call inherited.
func Dial() func(context.Context) (net.Conn, error) {
	return func(context.Context) (net.Conn, error) { return connect(FD, FD+1) }
}

// inherited wraps the descriptors once: a finalizer of os.NewFile would close
// them, so the files live as long as the process.
var dialMu sync.Mutex // record locks are per process: this keeps goroutines apart

var inherited struct {
	once      sync.Once
	ctl, lock *os.File
}

func connect(ctlFD, lockFD int) (net.Conn, error) {
	inherited.once.Do(func() {
		inherited.ctl = os.NewFile(uintptr(ctlFD), "ctl")
		inherited.lock = os.NewFile(uintptr(lockFD), "lock")
	})
	ctlFile, lock := inherited.ctl, inherited.lock
	dialMu.Lock()
	defer dialMu.Unlock()
	if ctlFile == nil || lock == nil {
		return nil, errors.New("not inside an agent call (no inherited descriptors)")
	}
	conn, err := net.FileConn(ctlFile)
	if err != nil {
		return nil, errors.New("not inside an agent call: " + err.Error())
	}
	defer conn.Close()
	fl := unix.Flock_t{Type: unix.F_WRLCK, Whence: 0}
	if err = unix.FcntlFlock(lock.Fd(), unix.F_SETLKW, &fl); err != nil {
		return nil, err
	}
	defer func() { fl.Type = unix.F_UNLCK; _ = unix.FcntlFlock(lock.Fd(), unix.F_SETLK, &fl) }()
	uc := conn.(*net.UnixConn)
	if _, err = uc.Write([]byte{requestByte}); err != nil {
		return nil, err
	}
	buf, oob := make([]byte, 1), make([]byte, unix.CmsgSpace(4))
	_, oobn, _, _, err := uc.ReadMsgUnix(buf, oob)
	if err != nil {
		return nil, err
	}
	msgs, err := unix.ParseSocketControlMessage(oob[:oobn])
	if err != nil || len(msgs) == 0 {
		return nil, errors.New("no connection from the owner")
	}
	fds, err := unix.ParseUnixRights(&msgs[0])
	if err != nil || len(fds) == 0 {
		return nil, errors.New("no connection from the owner")
	}
	f := os.NewFile(uintptr(fds[0]), "conn")
	defer f.Close()
	conn2, err := net.FileConn(f)
	if err != nil {
		return nil, err
	}
	_, _ = uc.Write([]byte{ackByte}) // the owner may now close its copies
	return conn2, nil
}
