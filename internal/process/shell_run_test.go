package process

// Run starts a bash call and waits for its result; production code uses
// Start and Wait directly.
func Run(options RunOptions) (Result, error) {
	call, err := Start(options)
	if err != nil {
		return Result{}, err
	}
	return call.Wait()
}
