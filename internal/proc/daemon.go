package proc

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"meno/internal/safe"
)

// Daemons runs long-lived helper commands (a local proxy for a provider) that
// meno starts at launch and stops at exit. Each one runs in its own session and
// process group, so stopping it also stops what it started. Output goes to a
// log file. A daemon is not waited for: nothing here checks that it is ready.
type Daemons struct {
	logDir string
	// OnExit is called when a daemon ends by itself (not when it is stopped).
	OnExit func(name string, err error, logPath string)

	mu    sync.Mutex
	procs map[string]*daemon
}

type daemon struct {
	command string
	cmd     *exec.Cmd
	done    chan struct{}
	stopped bool
}

func NewDaemons(logDir string) *Daemons {
	return &Daemons{logDir: logDir, procs: map[string]*daemon{}}
}

// LogPath is where the output of the daemon name is written.
func (d *Daemons) LogPath(name string) string {
	safeName := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '_'
	}, name)
	return filepath.Join(d.logDir, safeName+".log")
}

// Start runs command with /bin/sh -c in dir. If the daemon already runs the
// same command, nothing happens; a different command replaces the old one.
func (d *Daemons) Start(name, command, dir string, env []string) error {
	if strings.TrimSpace(command) == "" {
		d.Stop(name)
		return nil
	}
	d.mu.Lock()
	if old := d.procs[name]; old != nil && old.command == command {
		select {
		case <-old.done:
		default:
			d.mu.Unlock()
			return nil
		}
	}
	d.mu.Unlock()
	d.Stop(name)
	if err := os.MkdirAll(d.logDir, 0o700); err != nil {
		return err
	}
	logPath := d.LogPath(name)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = f, f
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		f.Close()
		return fmt.Errorf("could not start %q: %w", command, err)
	}
	p := &daemon{command: command, cmd: cmd, done: make(chan struct{})}
	d.mu.Lock()
	d.procs[name] = p
	d.mu.Unlock()
	go func() {
		defer safe.Recover(nil)
		err := cmd.Wait()
		f.Close()
		d.mu.Lock()
		stopped := p.stopped
		d.mu.Unlock()
		close(p.done)
		if !stopped && d.OnExit != nil {
			if err == nil {
				err = errors.New("exited")
			}
			d.OnExit(name, err, logPath)
		}
	}()
	return nil
}

// Running reports whether the daemon name is alive.
func (d *Daemons) Running(name string) bool {
	d.mu.Lock()
	p := d.procs[name]
	d.mu.Unlock()
	if p == nil {
		return false
	}
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

// Stop ends the daemon name: SIGTERM to its group, SIGKILL after a short wait.
func (d *Daemons) Stop(name string) {
	d.mu.Lock()
	p := d.procs[name]
	delete(d.procs, name)
	if p != nil {
		p.stopped = true
	}
	d.mu.Unlock()
	if p == nil {
		return
	}
	pgid := p.cmd.Process.Pid
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	select {
	case <-p.done:
	case <-time.After(2 * time.Second):
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		<-p.done
	}
}

// StopAll stops every daemon.
func (d *Daemons) StopAll() {
	d.mu.Lock()
	names := make([]string, 0, len(d.procs))
	for n := range d.procs {
		names = append(names, n)
	}
	d.mu.Unlock()
	var wg sync.WaitGroup
	for _, n := range names {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.Stop(n)
		}()
	}
	wg.Wait()
}
