// Package safe keeps a panic in one goroutine from terminating the whole
// process. The panic is handed to the caller, which decides how to recover.
package safe

import (
	"fmt"
	"runtime/debug"
)

// Call runs fn and returns a panic as an error that carries the stack.
func Call(fn func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("internal error: %v\n%s", r, debug.Stack())
		}
	}()
	fn()
	return nil
}

// Recover is for `defer safe.Recover(onPanic)` as the first line of a goroutine
// body: it stops a panic from crashing the process and reports it. onPanic may be nil.
func Recover(onPanic func(err error)) {
	if r := recover(); r != nil {
		err := fmt.Errorf("internal error: %v\n%s", r, debug.Stack())
		if onPanic != nil {
			onPanic(err)
		}
	}
}

// Go runs fn in a new goroutine. A panic is passed to onPanic (which may be nil).
func Go(fn func(), onPanic func(err error)) {
	go func() {
		if err := Call(fn); err != nil && onPanic != nil {
			onPanic(err)
		}
	}()
}
