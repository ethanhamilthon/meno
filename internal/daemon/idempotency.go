package daemon

import "sync"

// Idempotency caches results by client-generated request id to make retries safe.
type Idempotency struct {
	mu      sync.Mutex
	results map[string]any
	pending map[string]*pendingResult
}
type pendingResult struct {
	done  chan struct{}
	value any
	err   error
}

// NewIdempotency creates a bounded-lifetime in-memory result cache.
func NewIdempotency() *Idempotency {
	return &Idempotency{results: map[string]any{}, pending: map[string]*pendingResult{}}
}

// Do returns the first operation result for key and runs fn at most once concurrently.
func (i *Idempotency) Do(key string, fn func() (any, error)) (any, error) {
	if key == "" {
		return fn()
	}
	i.mu.Lock()
	if v, ok := i.results[key]; ok {
		i.mu.Unlock()
		return v, nil
	}
	if p := i.pending[key]; p != nil {
		i.mu.Unlock()
		<-p.done
		return p.value, p.err
	}
	p := &pendingResult{done: make(chan struct{})}
	i.pending[key] = p
	i.mu.Unlock()
	p.value, p.err = fn()
	i.mu.Lock()
	delete(i.pending, key)
	if p.err == nil {
		i.results[key] = p.value
	}
	close(p.done)
	i.mu.Unlock()
	return p.value, p.err
}
