package daemon

import "context"

func (f *FakeBackend) ApplyClientID(_ context.Context, key string, fn func() (any, error)) (any, error) {
	return f.idempotency.Do(key, fn)
}
