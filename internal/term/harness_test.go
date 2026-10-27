//go:build termtest

package term

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/hinshun/vt10x"

	"meno/internal/config"
	"meno/internal/db"
	"meno/internal/llm"
)

var binPath string
var ownedHomes sync.Map

// TestMain builds the real binary once.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "meno-term")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binPath = filepath.Join(dir, "meno")
	out, err := exec.Command("go", "build", "-o", binPath, "meno/cmd/meno").CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n%s", err, out)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// session is one meno process in a pseudo-terminal, with its screen emulated.
type session struct {
	t      *testing.T
	cmd    *exec.Cmd
	pty    *os.File
	vt     vt10x.Terminal
	home   string
	work   string
	exited chan struct{}
	mu     sync.Mutex
	err    error  // exit status
	emuErr error  // first error of the terminal emulator
	raw    []byte // every byte meno wrote to the terminal
}

type options struct {
	cols, rows int
	fake       *fakeLLM // preconfigure provider "fake" and model "fake-model" (protocol below)
	proto      string
	home       string   // reuse a home directory (for /resume across runs)
	args       []string // command line arguments of meno
	work       string   // working directory; a new one by default
	firstRun   bool     // no provider yet: meno opens the login list that cannot be closed
	// jsonDefaults keeps the default provider and model only in settings.json and
	// leaves the SQLite settings table empty, like a home the daemon has written.
	jsonDefaults bool
}

// seedHome writes the provider and settings of the fake model into o.home.
func seedHome(t *testing.T, o options, work string) {
	t.Helper()
	if o.fake != nil {
		st := config.NewStore(config.Paths{Home: o.home, Cwd: work})
		// Scenarios say "chat" or "responses"; provider.json is validated and
		// needs the full protocol names.
		proto := llm.ProtoChat
		if o.proto == "responses" {
			proto = llm.ProtoResponses
		}
		if err := st.SaveProviders([]llm.Provider{{Name: "fake", BaseURL: o.fake.URL(), APIKey: "k", Protocol: proto}}); err != nil {
			t.Fatal(err)
		}
		s := config.DefaultSettings()
		s.Provider, s.Model, s.View = "fake", "fake-model", "full" // the scenarios read the whole timeline
		if o.jsonDefaults {
			if err := config.SaveSettingsFile(st.P.SettingsFile(), s); err != nil {
				t.Fatal(err)
			}
		} else if err := st.SaveSettings(s); err != nil {
			t.Fatal(err)
		}
		// Release meno.db in this process before meno opens it.
		db.CloseAll()
	}
}

// start launches meno. With opts.fake set, the provider and model are already
// chosen, otherwise it is a first run.
func start(t *testing.T, o options) *session {
	t.Helper()
	if o.cols == 0 {
		o.cols, o.rows = 100, 32
	}
	if o.home == "" {
		o.home = t.TempDir()
	}
	work := o.work
	if work == "" {
		work = t.TempDir()
	}
	seedHome(t, o, work)
	cmd := exec.Command(binPath, o.args...)
	cmd.Dir = work
	cmd.Env = append(os.Environ(), "MENO_HOME="+o.home, "HOME="+o.home, "TERM=xterm-256color", "COLORTERM=truecolor", "NO_COLOR=", "MENO_NO_METADATA=1", "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(o.rows), Cols: uint16(o.cols)})
	if err != nil {
		t.Fatal(err)
	}
	s := &session{t: t, cmd: cmd, pty: ptmx, home: o.home, work: work, exited: make(chan struct{})}
	s.vt = vt10x.New(vt10x.WithSize(o.cols, o.rows), vt10x.WithWriter(ptmx))
	go func() {
		// Keep draining the pty whatever the emulator makes of the bytes: meno
		// blocks (even on exit) once the terminal buffer is full.
		buf := make([]byte, 32*1024)
		var pending []byte
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				s.mu.Lock()
				s.raw = append(s.raw, buf[:n]...)
				s.mu.Unlock()
				data := append(pending, buf[:n]...)
				// vt10x stops at a UTF-8 sequence cut by the read boundary and
				// reports the bytes it consumed; hand it the rest next time.
				w, werr := s.vt.Write(data)
				if werr != nil {
					s.noteEmuErr(werr)
					w = len(data)
				}
				pending = append([]byte(nil), data[w:]...)
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		close(s.exited)
	}()
	t.Cleanup(func() {
		if !s.Exited() {
			_ = cmd.Process.Signal(syscall.SIGKILL)
			<-s.exited
		}
		ptmx.Close()
	})
	// Keys sent before the program has put the terminal in raw mode would be
	// echoed by the tty and lost, so wait for the first frame.
	if o.firstRun {
		s.WaitFor("add a provider to start")
	} else if len(o.args) > 0 {
		s.WaitFor("Type your next task here") // a stored session has no welcome screen
	} else {
		s.WaitFor("Type to start building anything")
	}
	return s
}

func (s *session) noteEmuErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.emuErr == nil {
		s.emuErr = err
	}
}

// Screen is the current screen, one line per row, trailing blanks trimmed.
func (s *session) Screen() string {
	raw := s.vt.String()
	lines := strings.Split(raw, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func (s *session) Exited() bool {
	select {
	case <-s.exited:
		return true
	default:
		return false
	}
}

// Send types raw bytes.
func (s *session) Send(text string) {
	s.t.Helper()
	if _, err := s.pty.WriteString(text); err != nil && !s.Exited() {
		s.t.Fatal(err)
	}
}

// Keys for Send.
const (
	Enter  = "\r"
	Esc    = "\x1b"
	CtrlC  = "\x03"
	CtrlS  = "\x13"
	Up     = "\x1b[A"
	Down   = "\x1b[B"
	Escape = Esc
)

// Type sends text slowly enough for the editor to keep up with each key.
func (s *session) Type(text string) {
	s.t.Helper()
	for _, r := range text {
		s.Send(string(r))
		time.Sleep(4 * time.Millisecond)
	}
}

// Cmd runs a "/command" typed inline in the input.
func (s *session) Cmd(c string) {
	s.t.Helper()
	s.Type("/" + c)
	time.Sleep(60 * time.Millisecond)
	s.Send(Enter)
}

// Normal is kept for legacy helpers: INPUT is the only typing mode now.
func (s *session) Normal() {}

// WaitFor waits until the screen contains want.
func (s *session) WaitFor(want string) string {
	s.t.Helper()
	return s.WaitForWithin(want, 10*time.Second)
}

func (s *session) WaitForWithin(want string, d time.Duration) string {
	s.t.Helper()
	deadline := time.Now().Add(d)
	for {
		scr := s.Screen()
		if strings.Contains(scr, want) {
			return scr
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("timed out waiting for %q (exited=%v emuErr=%v); screen:\n%s", want, s.Exited(), s.emuErr, scr)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// WaitGone waits until the screen no longer contains s.
func (s *session) WaitGone(gone string) {
	s.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for strings.Contains(s.Screen(), gone) {
		if time.Now().After(deadline) {
			s.t.Fatalf("%q is still on screen:\n%s", gone, s.Screen())
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// Never checks that want does not show up within d.
func (s *session) Never(unwanted string, d time.Duration) {
	s.t.Helper()
	end := time.Now().Add(d)
	for time.Now().Before(end) {
		if strings.Contains(s.Screen(), unwanted) {
			s.t.Fatalf("%q appeared:\n%s", unwanted, s.Screen())
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// Resize changes the terminal size; meno gets SIGWINCH from the kernel.
func (s *session) Resize(cols, rows int) {
	s.t.Helper()
	s.vt.Resize(cols, rows)
	if err := pty.Setsize(s.pty, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}); err != nil {
		s.t.Fatal(err)
	}
}

// WaitExit waits for the process to end and returns its error (nil = status 0).
func (s *session) WaitExit() error {
	s.t.Helper()
	select {
	case <-s.exited:
	case <-time.After(10 * time.Second):
		s.t.Fatalf("meno did not exit; screen:\n%s", s.Screen())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// RawLen is how many bytes meno has written to the terminal so far.
func (s *session) RawLen() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.raw)
}

// RawSince returns the bytes written after offset.
func (s *session) RawSince(offset int) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.raw[offset:]...)
}
