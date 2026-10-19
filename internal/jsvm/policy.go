package jsvm

import (
	"errors"
	"fmt"
)

// Policy limits what the code of a run may do. The zero value allows all. The
// mode of a request sets it before the first call.
type Policy struct {
	// ReadOnly refuses fs.write, fs.patch and fs.edit, runs sh in a read-only
	// sandbox, and turns off tasks.start and agents.spawn.
	ReadOnly bool
	// Why names the mode in the error the model reads, for example "plan mode".
	Why string
}

// SetPolicy sets the policy for the following calls.
func (v *VM) SetPolicy(p Policy) { v.policy.Store(&p) }

func (v *VM) currentPolicy() Policy {
	if p := v.policy.Load(); p != nil {
		return *p
	}
	return Policy{}
}

// denied is the error for something the policy forbids.
func (v *VM) denied(what string) error {
	p := v.currentPolicy()
	why := p.Why
	if why == "" {
		why = "this mode"
	}
	return fmt.Errorf("%s is not available in %s: it is read-only. Describe the change in your answer instead", what, why)
}

// writable returns an error when the policy makes the run read-only.
func (v *VM) writable(what string) error {
	if v.currentPolicy().ReadOnly {
		return v.denied(what)
	}
	return nil
}

// errNoSandbox means a read-only run needs a sandbox for sh and there is none.
var errNoSandbox = errors.New("sh is not available in a read-only mode on this system: it has no sandbox tool (sandbox-exec or bwrap). Use fs.read, fs.search and fs.glob")
