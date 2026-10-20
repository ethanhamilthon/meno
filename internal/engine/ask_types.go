package engine

import (
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
}
