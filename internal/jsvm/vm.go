package jsvm

import (
	"context"
	"fmt"
	"sort"
	"strings"
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
	root         string
	timeout      atomic.Int64  // time.Duration; atomic so settings can change during a run
	active       time.Duration // timeout of the running Exec
	mcpHost      atomic.Pointer[MCPHost]
	mcpAdmin     atomic.Pointer[MCPAdmin]
	asker        atomic.Pointer[Asker]
	pauseTimeout func() func() // set by the running Exec: stops the js timeout, the returned func restarts it
	out          boundedBuffer
	maxOut       int
	ctx          context.Context
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
	v.Reset()
	return v
}

func (v *VM) Reset() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.rt = goja.New()
	v.out.Reset()
	v.bind()
}

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
	fsh := FS{Root: v.root}
	obj := rt.NewObject()
	_ = obj.Set("read", func(p string) (string, error) { return fsh.Read(p) })
	_ = obj.Set("write", func(p, c string) error { return fsh.Write(p, c) })
	_ = obj.Set("edit", func(p, o, n string) (int, error) { return fsh.Edit(p, o, n) })
	_ = obj.Set("glob", func(p string) ([]string, error) { return fsh.Glob(p) })
	_ = obj.Set("exists", func(p string) bool { return fsh.Exists(p) })
	_ = rt.Set("fs", obj)
	_ = rt.Set("sh", func(cmd string) (map[string]any, error) {
		res, err := (Shell{Root: v.root, Timeout: v.active, MaxOut: v.maxOut, Env: v.shellEnv()}).Run(v.ctx, cmd)
		if err != nil {
			return nil, err
		}
		return map[string]any{"stdout": res.Stdout, "stderr": res.Stderr, "code": res.ExitCode, "timedOut": res.TimedOut}, nil
	})
	v.bindMCPAdmin()
	v.bindAsk()
	if _, err := rt.RunString(preludeJS); err != nil {
		panic("jsvm prelude: " + err.Error())
	}
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

// Exec runs code with the VM's configured timeout.
func (v *VM) Exec(ctx context.Context, code string) Result {
	return v.exec(ctx, code, nil)
}

// ExecTimeout runs code with a timeout of its own (0 = no limit) instead of the
// configured one. The timeout covers sh() and fetch() calls made by the code.
func (v *VM) ExecTimeout(ctx context.Context, code string, timeout time.Duration) Result {
	return v.exec(ctx, code, &timeout)
}

func (v *VM) exec(ctx context.Context, code string, override *time.Duration) Result {
	start := time.Now()
	v.mu.Lock()
	defer v.mu.Unlock()
	v.out.Reset()
	timeout := time.Duration(v.timeout.Load())
	if override != nil {
		timeout = *override
	}
	v.active = timeout
	v.syncMCP()
	if err := ctx.Err(); err != nil {
		return Result{Err: err.Error()}
	}
	v.ctx = ctx
	var timedOut atomic.Bool
	timerDone := make(chan struct{})
	var timer *time.Timer
	deadline := time.Now().Add(timeout)
	v.pauseTimeout = func() func() {
		if timer == nil || !timer.Stop() {
			return func() {}
		}
		left := max(time.Until(deadline), time.Millisecond)
		return func() {
			deadline = time.Now().Add(left)
			timer.Reset(left)
		}
	}
	if timeout > 0 {
		timer = time.AfterFunc(timeout, func() {
			timedOut.Store(true)
			v.rt.Interrupt("timeout")
			close(timerDone)
		})
	}
	abortDone := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { v.rt.Interrupt("aborted"); close(abortDone) })
	defer func() {
		if !stop() {
			<-abortDone
		}
		if timer != nil && !timer.Stop() {
			<-timerDone
		}
		v.rt.ClearInterrupt()
	}()
	val, err := v.run(code)
	r := Result{Output: v.out.String(), Elapsed: time.Since(start), TimedOut: timedOut.Load()}
	if err != nil {
		r.Err = err.Error()
		if timedOut.Load() {
			r.Err = fmt.Sprintf("js timeout after %s", timeout)
		}
		return r
	}
	if val != nil && !goja.IsUndefined(val) && !v.out.cut {
		var value string
		if ex := v.rt.Try(func() { value = val.String() }); ex != nil {
			r.Output = v.out.String()
			r.Err = ex.Error()
			return r
		}
		// Printed output and the returned value share one byte budget.
		outputLen := len(v.out.String())
		v.write(value)
		combined := v.out.String()
		if v.out.cut && outputLen > len(combined)-len(truncationMarker(v.maxOut)) {
			r.Output = combined
		} else {
			r.Output, r.Value = combined[:outputLen], combined[outputLen:]
		}
	}
	return r
}

func (v *VM) run(code string) (goja.Value, error) {
	if !strings.Contains(code, "await") {
		return v.rt.RunString(code)
	}
	wrapped := "(async () => {\n" + code + "\n})()"
	val, err := v.rt.RunString(wrapped)
	if err != nil {
		return nil, err
	}
	if p, ok := val.Export().(*goja.Promise); ok {
		switch p.State() {
		case goja.PromiseStateRejected:
			return nil, fmt.Errorf("%v", p.Result().Export())
		case goja.PromiseStateFulfilled:
			return p.Result(), nil
		}
		return nil, fmt.Errorf("await: promise still pending (no event loop for host I/O)")
	}
	return val, nil
}
