package agent

import "context"

type callIDKey struct{}

// WithCallID returns a context that carries the id of the tool call being run,
// so a ToolRunner can name its log file and its process environment after it.
func WithCallID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, callIDKey{}, id)
}

// CallID returns the id WithCallID stored, or "".
func CallID(ctx context.Context) string {
	id, _ := ctx.Value(callIDKey{}).(string)
	return id
}
