package jsvm

import (
	"context"
	"fmt"
	"strings"
	"sync"
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
	mu      sync.Mutex
	rt      *goja.Runtime
	gate    *Gate
	timeout time.Duration
	out     strings.Builder
	maxOut  int
}

type Options struct {
	Gate    *Gate
	Timeout time.Duration
	MaxOut  int
}

func New(o Options) *VM {
	if o.MaxOut == 0 {
		o.MaxOut = 64 * 1024
	}
	v := &VM{gate: o.Gate, timeout: o.Timeout, maxOut: o.MaxOut}
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
	v.mu.Lock()
	defer v.mu.Unlock()
	v.timeout = d
}

func (v *VM) bind() {
	rt := v.rt
	_ = rt.Set("print", func(args ...any) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		v.write(strings.Join(parts, " ") + "\n")
	})
	fsh := FS{Gate: v.gate}
	obj := rt.NewObject()
	_ = obj.Set("read", func(p string) (string, error) { return fsh.Read(p) })
	_ = obj.Set("write", func(p, c string) error { return fsh.Write(p, c) })
	_ = obj.Set("edit", func(p, o, n string) (int, error) { return fsh.Edit(p, o, n) })
	_ = obj.Set("glob", func(p string) ([]string, error) { return fsh.Glob(p) })
	_ = obj.Set("exists", func(p string) bool { return fsh.Exists(p) })
	_ = rt.Set("fs", obj)
	_ = rt.Set("sh", func(cmd string) (map[string]any, error) {
		res, err := (Shell{Gate: v.gate, Timeout: v.timeout, MaxOut: v.maxOut}).Run(context.Background(), cmd)
		if err != nil {
			return nil, err
		}
		return map[string]any{"stdout": res.Stdout, "stderr": res.Stderr, "code": res.ExitCode, "timedOut": res.TimedOut}, nil
	})
}

func (v *VM) write(s string) {
	if v.out.Len() >= v.maxOut {
		return
	}
	v.out.WriteString(s)
}

func (v *VM) Exec(ctx context.Context, code string) Result {
	start := time.Now()
	v.mu.Lock()
	defer v.mu.Unlock()
	v.out.Reset()
	timedOut := false
	var timer *time.Timer
	if v.timeout > 0 {
		timer = time.AfterFunc(v.timeout, func() {
			timedOut = true
			v.rt.Interrupt("timeout")
		})
	}
	stop := context.AfterFunc(ctx, func() { v.rt.Interrupt("aborted") })
	defer func() {
		stop()
		if timer != nil {
			timer.Stop()
		}
		v.rt.ClearInterrupt()
	}()
	val, err := v.run(code)
	r := Result{Output: v.out.String(), Elapsed: time.Since(start), TimedOut: timedOut}
	if err != nil {
		r.Err = err.Error()
		if timedOut {
			r.Err = fmt.Sprintf("js timeout after %s", v.timeout)
		}
		return r
	}
	if val != nil && !goja.IsUndefined(val) {
		r.Value = val.String()
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
