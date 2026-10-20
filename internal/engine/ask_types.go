package engine

import (
	"time"

	"meno/internal/jsvm"
)

type tokenState struct {
	value  string
	groups map[string]bool
}
type askReply struct {
	answers []jsvm.FormAnswer
	err     error
}
type askState struct {
	id       string
	form     jsvm.Form
	by       string
	answer   chan askReply
	deadline time.Time
}
