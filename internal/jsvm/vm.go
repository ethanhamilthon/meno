package jsvm

import (
	"context"
	"fmt"
	"sort"
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

// fresh gives the VM a clean runtime. Every Exec starts with one: nothing a
// call declares or changes survives it. Durable data lives in Go services
// (tasks, agents), never in the interpreter.
func (v *VM) fresh() {
	v.rt = goja.New()
	v.out.Reset()
	v.bind()
}

// SetRoot changes the working directory of fs and sh. Variables survive; the
// next call sees the new directory. It does not wait for a running Exec.
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
	_ = rt.Set("__parseURL", func(href, base string) any {
		if m := ParseURL(href, base); m != nil {
			return m
		}
		return nil
	})
	_ = rt.Set("__fetch", func(rawURL, method string, headers map[string]string, body string, hasBody bool, redirect string) (map[string]any, error) {
		r, err := Fetch(v.ctx, v.active, rawURL, method, headers, body, hasBody, redirect)
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": r.Status, "statusText": r.StatusText, "url": r.URL, "redirected": r.Redirected, "headers": r.Headers, "body": r.Body}, nil
	})
	_ = rt.Set("env", environ())
	v.bindFS()
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
	v.bindHelp()
	if _, err := rt.RunProgram(preludeProg); err != nil {
		panic("jsvm prelude: " + err.Error())
	}
	if _, err := rt.RunProgram(askProg); err != nil {
		panic("jsvm ask: " + err.Error())
	}
	v.stringify = captureStringify(rt)
}

// shellEnv is the environment for sh(): the JS object env, so that
// env.FOO = "x" is visible to the commands the code starts.
func (v *VM) shellEnv() []string {
	m, ok := v.rt.Get("env").Export().(map[string]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(m))
	for k, val := range m {
		if val == nil {
			continue
		}
		out = append(out, k+"="+fmt.Sprint(val))
	}
	sort.Strings(out)
	return out
}

func (v *VM) write(s string) {
	remaining := max(0, v.maxOut-v.out.buf.Len())
	if len(s) > remaining {
		v.out.cut = true
		s = s[:remaining]
	}
	// Slice before converting so a huge single write needs no huge byte copy.
	_, _ = v.out.Write([]byte(s))
}
