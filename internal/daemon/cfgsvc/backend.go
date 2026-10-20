package cfgsvc

import (
	"context"
	"meno/internal/daemon"
	"meno/internal/proto"
)

// Backend delegates session operations to another daemon backend and routes
// configuration resources through Service.
type Backend struct {
	daemon.Backend
	Service *Service
}

func (b Backend) Resource(ctx context.Context, method, path, query string, req proto.ResourceRequest) (proto.ResourceResponse, error) {
	return b.Service.Resource(ctx, method, path, query, req)
}
