package jsvm

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
)

// Exec runs code with the VM's configured timeout.
func (v *VM) Exec(ctx context.Context, code string) Result {
	return v.exec(ctx, code, nil)
}

// ExecTimeout runs code with a timeout of its own (0 = no limit) instead of the
// configured one. The timeout covers sh() and websearch() calls made by the code.
func (v *VM) ExecTimeout(ctx context.Context, code string, timeout time.Duration) Result {
	return v.exec(ctx, code, &timeout)
}

func (v *VM) exec(ctx context.Context, code string, override *time.Duration) Result {
	start := time.Now()
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.rt == nil {
		v.fresh()
	}
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
	callCtx, cancelCall := context.WithCancel(ctx)
	defer cancelCall()
	v.ctx = callCtx
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
			cancelCall()
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
			r.Err = fmt.Sprintf("js timeout after %s: the call took too long and was stopped (pass a higher timeout to the js tool, or run long commands in the background with tasks.start)", timeout)
		}
		return r
	}
	if val != nil && !goja.IsUndefined(val) && !v.out.cut {
		value, rerr := v.render(val)
		if rerr != nil {
			r.Output = v.out.String()
			r.Err = rerr.Error()
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
	c, err := compile(code)
	if err != nil {
		return nil, err
	}
	val, err := v.rt.RunProgram(c.prog)
	if err != nil || c.kind != kindAsync {
		return val, err
	}
	// An async body returns a promise. Host tools are synchronous, so a
	// promise that is already settled is read now; one still pending can never
	// settle, because the runtime ends with this call.
	if p, ok := val.Export().(*goja.Promise); ok {
		switch p.State() {
		case goja.PromiseStateRejected:
			return nil, fmt.Errorf("%s", p.Result().String())
		case goja.PromiseStateFulfilled:
			return p.Result(), nil
		}
		return nil, fmt.Errorf("await: promise still pending (host calls are synchronous; there is no event loop)")
	}
	return val, nil
}
