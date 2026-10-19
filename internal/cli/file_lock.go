package cli

import "sync"

var fileLocks sync.Map

func lockFile(path string) func() {
	lock, _ := fileLocks.LoadOrStore(path, &sync.Mutex{})
	mu := lock.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}
