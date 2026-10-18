package jsvm

import (
	_ "embed"

	"github.com/dop251/goja"
)

//go:embed ask.js
var askJS string

//go:embed prelude.js
var preludeJS string

var (
	preludeProg = goja.MustCompile("prelude.js", preludeJS, false)
	askProg     = goja.MustCompile("ask.js", askJS, false)
)
