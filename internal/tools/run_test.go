package tools

import "context"

// Run starts a bash call and waits for its result; production code uses
// Start and Wait directly.
func Run(ctx context.Context, options RunOptions) (Result, error) {
	call, err := Start(ctx, options)
	if err != nil {
		return Result{}, err
	}
	return call.Wait()
}
