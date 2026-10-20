package engine

import "context"

// callOpts is what one agent command carries into a resource handler: the
// request context, the session's directory and environment, and the options.
type callOpts struct {
	Ctx  context.Context
	Cwd  string
	Env  []string
	Opts map[string]any
}
