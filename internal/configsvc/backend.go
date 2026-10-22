package configsvc

import (
	"meno/internal/daemon"
)

// Backend delegates session operations to another daemon backend and routes
// configuration resources through Service.
type Backend struct {
	daemon.Backend
	Service *Service
}
