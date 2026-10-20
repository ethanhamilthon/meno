package jsvm

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
)

type Result struct {
	Output   string
	Value    string
	Err      string
	TimedOut bool
	Elapsed  time.Duration
}

type VM struct {
	mu           sync.Mutex
	rt           *goja.Runtime
	rootMu       sync.RWMutex
	root         string        // working directory of fs and sh; see SetRoot
	timeout      atomic.Int64  // time.Duration; atomic so settings can change during a run
	active       time.Duration // timeout of the running Exec
	mcpHost      atomic.Pointer[MCPHost]
	mcpAdmin     atomic.Pointer[MCPAdmin]
	asker        atomic.Pointer[Asker]
	viewer       atomic.Pointer[Viewer]
	pauseTimeout func() func() // set by the running Exec: stops the js timeout, the returned func restarts it
	out          boundedBuffer
	maxOut       int
	ctx          context.Context
	stringify    goja.Callable // the original JSON.stringify, for results
	sessionHost  atomic.Pointer[SessionHost]
	tasksHost    atomic.Pointer[TasksHost]
	agentsHost   atomic.Pointer[AgentsHost]
	agentsChild  atomic.Bool
	webHost      atomic.Pointer[WebSearcher]
	policy       atomic.Pointer[Policy]
	goalHost     atomic.Pointer[GoalHost]
}

type Options struct {
	Root    string
	Timeout time.Duration
	MaxOut  int
}

func New(o Options) *VM {
	if o.MaxOut <= 0 {
		o.MaxOut = 64 * 1024
	}
	v := &VM{root: o.Root, maxOut: o.MaxOut, out: boundedBuffer{max: o.MaxOut}}
	v.timeout.Store(int64(o.Timeout))
	return v
}

// fresh gives the VM a clean runtime. The runtime then lives until Reset: what
// one call declares, the next call sees. The caller resets it when a new user
// request starts, so the state lasts for one answer of the agent and no longer.
// Durable data still belongs in files and Go services (tasks, agents).
func (v *VM) fresh() {
	v.rt = goja.New()
	v.bind()
}

// Reset drops the runtime and with it every variable and function the calls
// declared. The next Exec starts with a clean one. It waits for a running Exec.
func (v *VM) Reset() {
	if v == nil {
		return
	}
	v.mu.Lock()
	v.rt = nil
	v.mu.Unlock()
}

// SetRoot changes the working directory of fs and sh. The next call sees the
// new directory. It does not wait for a running Exec.
func (v *VM) SetRoot(dir string) {
	v.rootMu.Lock()
	v.root = dir
	v.rootMu.Unlock()
}

// Root is the current working directory of fs and sh.
func (v *VM) Root() string {
	v.rootMu.RLock()
	defer v.rootMu.RUnlock()
	return v.root
}

// Timeout is the configured js timeout (0 = no limit).
func (v *VM) Timeout() time.Duration { return time.Duration(v.timeout.Load()) }

func (v *VM) SetTimeout(d time.Duration) {
	v.timeout.Store(int64(d))
}

func (v *VM) bind() {
	rt := v.rt
	_ = rt.Set("__write", func(s string) { v.write(s) })
	v.bindFS()
	v.bindStdlib()
	_ = rt.Set("sh", func(cmd string) (map[string]any, error) {
		res, err := (Shell{Root: v.Root(), Timeout: v.active, MaxOut: v.maxOut, Env: v.shellEnv()}).Run(v.ctx, cmd)
		if err != nil {
			return nil, err
		}
		return map[string]any{"stdout": res.Stdout, "stderr": res.Stderr, "code": res.ExitCode, "timedOut": res.TimedOut}, nil
	})
	_ = rt.Set("view", func(p string) (string, error) { return v.view(p) })
	v.bindMCPAdmin()
	v.bindAsk()
	v.bindSession()
	v.bindMCPResults()
	v.bindTasks()
	v.bindAgents()
	v.bindWebSearch()
	v.bindGoal()
	v.bindHelp()
	if _, err := rt.RunProgram(preludeProg); err != nil {
		panic("jsvm prelude: " + err.Error())
	}
	if _, err := rt.RunProgram(askProg); err != nil {
		panic("jsvm ask: " + err.Error())
	}
	v.restrict(rt)
	v.stringify = captureStringify(rt)
}

// shellEnv is the environment for sh() and background tasks: the process
// environment as it is now. There is no JS env object any more.
func (v *VM) shellEnv() []string { return os.Environ() }

func (v *VM) write(s string) {
	remaining := max(0, v.maxOut-v.out.buf.Len())
	if len(s) > remaining {
		v.out.cut = true
		s = s[:remaining]
	}
	// Slice before converting so a huge single write needs no huge byte copy.
	_, _ = v.out.Write([]byte(s))
}

// Run implements the agent tool-runner interface.
func (v *VM) Run(ctx context.Context, code string, timeout time.Duration) (string, string, string, bool, time.Duration) {
	r := v.ExecTimeout(ctx, code, timeout)
	return r.Output, r.Value, r.Err, r.TimedOut, r.Elapsed
}
