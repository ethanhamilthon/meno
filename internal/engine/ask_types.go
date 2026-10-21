package engine

import (
	"sync/atomic"
	"time"

	"meno/internal/form"
)

type tokenState struct {
	value  string
	groups map[string]bool
}
type askReply struct {
	answers []form.Answer
	err     error
}
type askState struct {
	id       string
	form     form.Form
	by       string
	answer   chan askReply
	deadline time.Time
	claimed  atomic.Bool // set by the first answer; later answers lose
}

// claim reports whether the caller is the first to answer this question.
func (a *askState) claim() bool { return a.claimed.CompareAndSwap(false, true) }
