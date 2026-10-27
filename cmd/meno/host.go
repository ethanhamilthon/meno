package main

import (
	"context"
	"fmt"

	"meno/internal/client"
	"meno/internal/config"
	"meno/internal/configsvc"
	"meno/internal/daemon"
	"meno/internal/engine"
	"meno/internal/mcp"
	ver "meno/internal/version"
)

// host is the engine of this process: the configuration service, the engine
// and the protocol server, reached through memory. Every client of the engine
// (the TUI, ACP, a headless run, one human command) starts its own host.
type host struct {
	srv    *daemon.Server
	cancel context.CancelFunc
	client *client.Client
}

// hostOption changes how the engine of a host is built (tests inject a model).
type hostOption func(*engine.Options)

// startHost opens the database, starts the configuration service and the
// engine, and returns a client connected to them.
func startHost(home string, opts ...hostOption) (*host, error) {
	if err := checkOldDaemon(home); err != nil {
		return nil, err
	}
	manager := mcp.NewManager()
	service, err := configsvc.New(configsvc.Options{MCP: manager, Paths: config.Paths{Home: home}})
	if err != nil {
		return nil, err
	}
	hub := daemon.NewHub("", 20000)
	global := daemon.NewGlobalHub("", 2000)
	eo := engine.Options{MCP: manager, Home: home, EventHub: hub, GlobalHub: global, Resources: service}
	for _, o := range opts {
		o(&eo)
	}
	eng, err := engine.New(eo)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	srv := daemon.New(daemon.Options{
		Version: ver.Label(), Backend: eng, Hub: hub, Global: global,
		OnStart: func() error { return service.Start(ctx) },
		OnStop:  func() { eng.Close(); service.Close() },
	})
	eng.SetCallServer(srv.ServeCall)
	service.SetNotify(func(event string) { global.Publish("", event, map[string]any{}) })
	dial, err := srv.ServeMem()
	if err != nil {
		cancel()
		srv.Stop("start failed")
		return nil, fmt.Errorf("start: %w", err)
	}
	return &host{srv: srv, cancel: cancel, client: client.New(dial)}, nil
}

// Close stops every run and task of this process and releases the engine.
func (h *host) Close() {
	h.srv.Stop("exit")
	h.cancel()
}
