// Package proc supervises background processes started by the main agent:
// one process group per task, bounded logs on disk, durable final results.
package proc

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	"meno/internal/safe"
	"meno/internal/tools"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Task statuses.
const (
	Starting    = "starting"
	Running     = "running"
	Succeeded   = "succeeded"
	Failed      = "failed"
	Cancelled   = "cancelled"
	TimedOut    = "timed_out"
	Interrupted = "interrupted"
)

var graceTime atomic.Int64 // nanoseconds

func init() { graceTime.Store(int64(5 * time.Second)) }

// Task is a process and its outcome.
type Task struct {
	ID       string    `json:"id"`
	Session  string    `json:"session"`
	Owner    string    `json:"owner"` // root request that started it
	Name     string    `json:"name"`
	Command  string    `json:"command"`
	Cwd      string    `json:"cwd"`
	Status   string    `json:"status"`
	ExitCode *int      `json:"exitCode"`
	Reason   string    `json:"reason"`
	Notify   bool      `json:"notify"`
	Started  time.Time `json:"started"`
	Ended    time.Time `json:"ended"`
}

// Terminal reports whether the task is over.
func (t Task) Terminal() bool { return t.Status != Starting && t.Status != Running }

// StartOptions describe a task. Env entries are KEY=VALUE and replace the
// inherited environment; Cwd and Env are captured at the call.
type StartOptions struct {
	Session, Owner, Name, Command, Cwd string
	Env                                []string
	Notify                             bool
}

// Hooks connect the supervisor to the rest of the program.
type Hooks struct {
	// Finished is called once when a task reaches a terminal state, after it
	// is stored. It must not block.
	Finished func(Task)
}

// Supervisor owns the running tasks of one meno process.
type Supervisor struct {
	d     *sql.DB
	dir   string // logs: dir/<session>/<id>.log
	hooks Hooks
	limit int64

	mu      sync.Mutex
	live    map[string]*running
	waiters map[string]int // tasks someone is waiting for right now
	closing bool
	wg      sync.WaitGroup
}

type running struct {
	task      Task
	cmd       *exec.Cmd
	done      chan struct{}
	cancelled bool
	call      *tools.Call // set for a launched (foreground) process
	promoted  bool
	finished  bool
	log       *logWriter
}

// New returns a supervisor on the database d. Tasks that a previous run left
// unfinished are marked interrupted; nothing is restarted.
func New(d *sql.DB, logDir string, h Hooks) (*Supervisor, error) {
	s := &Supervisor{d: d, dir: logDir, hooks: h, limit: DefaultLogLimit, live: map[string]*running{}, waiters: map[string]int{}}
	return s, s.markInterrupted()
}

// SetLogLimit changes the disk budget of logs of tasks started afterwards.
func (s *Supervisor) SetLogLimit(n int64) { s.limit = n }

func (s *Supervisor) logPath(session, id string) string {
	return filepath.Join(s.dir, session, id+".log")
}

// admit reserves a supervisor slot unless shutdown has begun. The check and the
// wg.Add share one lock with Shutdown's closing flag, so a process is either
// refused or counted before Shutdown can finish waiting.
func (s *Supervisor) admit() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return false
	}
	s.wg.Add(1)
	return true
}

// register makes r visible to Shutdown. It reports true when shutdown started
// before r was registered: Shutdown's snapshot missed it, so the caller must
// terminate it.
func (s *Supervisor) register(r *running) (late bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.live[r.task.ID] = r
	if s.closing {
		r.cancelled = true
		return true
	}
	return false
}

// Start launches the command with /bin/sh -c in its own process group, stdin
// closed, and returns once the process exists. Everything is validated first.
func (s *Supervisor) Start(o StartOptions) (Task, error) {
	if o.Command == "" {
		return Task{}, errors.New("tasks.start: command is empty")
	}
	if o.Session == "" {
		return Task{}, errors.New("tasks.start: no session")
	}
	if st, err := os.Stat(o.Cwd); err != nil || !st.IsDir() {
		return Task{}, fmt.Errorf("tasks.start: cwd %q is not a directory", o.Cwd)
	}
	if !s.admit() {
		return Task{}, errors.New("tasks.start: meno is shutting down")
	}
	admitted := true
	defer func() {
		if admitted {
			s.wg.Done() // the process never reached supervise
		}
	}()
	t := Task{ID: "t" + newID(), Session: o.Session, Owner: o.Owner, Name: o.Name, Command: o.Command, Cwd: o.Cwd,
		Status: Starting, Notify: o.Notify, Started: time.Now().UTC()}
	lw, err := newLogWriter(s.logPath(t.Session, t.ID), s.limit)
	if err != nil {
		return Task{}, err
	}
	cmd := exec.Command("/bin/sh", "-c", o.Command)
	cmd.Dir = o.Cwd
	if o.Env != nil {
		cmd.Env = o.Env
	} else {
		cmd.Env = os.Environ()
	}
	// Setsid: no controlling terminal, so background tasks cannot stop meno via tty signals.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	stdout, err1 := cmd.StdoutPipe()
	stderr, err2 := cmd.StderrPipe()
	if err := errors.Join(err1, err2); err != nil {
		lw.close()
		return Task{}, err
	}
	if err := s.save(t); err != nil {
		lw.close()
		return Task{}, err
	}
	if err := cmd.Start(); err != nil {
		lw.close()
		t.Status, t.Reason, t.Ended = Failed, "could not start: "+err.Error(), time.Now().UTC()
		_ = s.save(t)
		return Task{}, fmt.Errorf("tasks.start: %w", err)
	}
	t.Status = Running
	_ = s.save(t)
	r := &running{task: t, cmd: cmd, done: make(chan struct{}), log: lw}
	late := s.register(r)
	var pumps sync.WaitGroup
	pumps.Add(2)
	go lw.pump(Stdout, bufio.NewReader(stdout), pumps.Done)
	go lw.pump(Stderr, bufio.NewReader(stderr), pumps.Done)
	admitted = false // supervise now owns the wg slot
	go s.supervise(r, &pumps)
	if late {
		s.terminate(r) // Shutdown began while this process was starting
	}
	return t, nil
}

func (s *Supervisor) supervise(r *running, pumps *sync.WaitGroup) {
	defer s.wg.Done()
	pumps.Wait() // output is drained before Wait closes the pipes
	err := r.cmd.Wait()
	r.log.close()
	s.mu.Lock()
	t := r.task
	t.Ended = time.Now().UTC()
	code := r.cmd.ProcessState.ExitCode()
	switch {
	case r.cancelled:
		t.Status, t.Reason = Cancelled, "cancelled"
	case err == nil:
		t.Status, t.ExitCode = Succeeded, &code
	default:
		t.Status, t.ExitCode = Failed, &code
		if code < 0 {
			t.ExitCode, t.Reason = nil, "terminated by a signal: "+err.Error()
		}
	}
	if r.cancelled {
		if code >= 0 {
			t.ExitCode = &code
		}
	}
	r.task = t
	// Saved before the task leaves live: Wait treats "not live" as "finished",
	// so the stored status must already be final by then.
	_ = s.save(t)
	delete(s.live, t.ID)
	s.mu.Unlock()
	if s.hooks.Finished != nil {
		s.hooks.Finished(t) // before done closes: a waiter that acks sees the event
	}
	close(r.done)
}

// terminate sends SIGTERM to the process group and SIGKILL after GraceTime.
func (s *Supervisor) terminate(r *running) {
	if r.call != nil {
		r.call.Cancel()
		return
	}
	pgid := r.cmd.Process.Pid
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	go func() {
		defer safe.Recover(nil)
		select {
		case <-r.done:
		case <-time.After(time.Duration(graceTime.Load())):
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		}
	}()
}

func (s *Supervisor) liveTask(id string) *running {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.live[id]
}
