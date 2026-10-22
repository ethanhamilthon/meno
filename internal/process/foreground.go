package process

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// LaunchOptions describe a foreground process: a bash call. Until it is
// promoted it is private to the run that made it: no task is listed or stored.
// Path and Args, when set, run that program without a shell (restricted modes).
type LaunchOptions struct {
	Session, Owner, Name, Command, Cwd string
	Env                                []string
	CallID, OutputDir                  string
	Path                               string
	Args                               []string
}

// Foreground is a launched process. The supervisor owns it from the start;
// promotion only makes it visible as a task, it never starts anything again.
type Foreground struct {
	s *Supervisor
	r *running
}

// Launch starts the process and returns at once.
func (s *Supervisor) Launch(o LaunchOptions) (*Foreground, error) {
	if st, err := os.Stat(o.Cwd); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("working directory %q is not a directory", o.Cwd)
	}
	if !s.admit() {
		return nil, errors.New("meno is shutting down")
	}
	admitted := true
	defer func() {
		if admitted {
			s.release() // the process never reached superviseForeground
		}
	}()
	t := Task{ID: "t" + newID(), Session: o.Session, Owner: o.Owner, Name: o.Name, Command: o.Command, Cwd: o.Cwd,
		Status: Running, Notify: true, Started: time.Now().UTC()}
	lw, err := newLogWriter(s.logPath(t.Session, t.ID), s.limit)
	if err != nil {
		return nil, err
	}
	var split lineSplitter
	ro := RunOptions{Command: o.Command, Dir: o.Cwd, Env: o.Env, OutputDir: o.OutputDir, CallID: o.CallID,
		Stream: func(d string) { split.feed(lw, d) }}
	var call *Call
	if o.Path != "" {
		call, err = StartProgram(ro, o.Path, o.Args)
	} else {
		call, err = Start(ro)
	}
	if err != nil {
		lw.close()
		return nil, err
	}
	r := &running{task: t, done: make(chan struct{}), log: lw, call: call}
	late := s.register(r)
	admitted = false // superviseForeground now owns the wg slot
	go s.superviseForeground(r, &split)
	if late {
		s.terminate(r)
	}
	return &Foreground{s: s, r: r}, nil
}

// TaskID is the ID the process has, or will have once it is promoted.
func (f *Foreground) TaskID() string { return f.r.task.ID }

// Done is closed when the process and its output are finished.
func (f *Foreground) Done() <-chan struct{} { return f.r.done }

// Result is the outcome of the call; valid after Done.
func (f *Foreground) Result() (Result, error) { return f.r.call.Wait() }

// Cancel stops the process unless it has been promoted: a task outlives the
// request that began it.
func (f *Foreground) Cancel() {
	f.s.mu.Lock()
	keep := f.r.promoted
	f.s.mu.Unlock()
	if !keep {
		f.r.call.Cancel()
	}
}

// Promote makes the running process a public task. It reports false when the
// process has already finished (its normal result then stands). Promoting twice
// returns the same task.
func (f *Foreground) Promote() (Task, bool) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	if f.r.finished {
		return Task{}, false
	}
	if !f.r.promoted {
		f.r.promoted = true
		_ = f.s.save(f.r.task)
	}
	return f.r.task, true
}

func (s *Supervisor) superviseForeground(r *running, split *lineSplitter) {
	defer s.release()
	<-r.call.Done()
	res, _ := r.call.Wait()
	split.flush(r.log)
	r.log.close()
	s.mu.Lock()
	r.finished = true
	t := r.task
	t.Ended = time.Now().UTC()
	code := res.ExitCode
	switch {
	case res.Cancelled:
		t.Status, t.Reason = Cancelled, "cancelled"
	case code == 0:
		t.Status, t.ExitCode = Succeeded, &code
	default:
		t.Status, t.ExitCode = Failed, &code
	}
	r.task = t
	promoted := r.promoted
	if promoted {
		_ = s.save(t)
	}
	delete(s.live, t.ID)
	s.mu.Unlock()
	if promoted && s.hooks.Finished != nil {
		s.hooks.Finished(t)
	}
	close(r.done)
}

// lineSplitter turns raw chunks into log records, one per line.
type lineSplitter struct{ buf strings.Builder }

func (l *lineSplitter) feed(w *logWriter, d string) {
	for {
		i := strings.IndexByte(d, '\n')
		if i < 0 {
			break
		}
		l.buf.WriteString(d[:i])
		l.flush(w)
		d = d[i+1:]
	}
	l.buf.WriteString(d)
	for l.buf.Len() >= MaxRecord {
		s := l.buf.String()
		l.buf.Reset()
		w.write(Stdout, s[:MaxRecord])
		l.buf.WriteString(s[MaxRecord:])
	}
}

func (l *lineSplitter) flush(w *logWriter) {
	if l.buf.Len() > 0 {
		w.write(Stdout, l.buf.String())
		l.buf.Reset()
	}
}
