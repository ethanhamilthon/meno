package main

import (
	"context"
	"fmt"
	"io"
	"meno/internal/client"
	"meno/internal/config"
	"meno/internal/daemon"
	"meno/internal/daemon/cfgsvc"
	"meno/internal/db"
	"meno/internal/engine"
	ver "meno/internal/version"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func daemonCLI(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "usage: meno daemon run|start|stop|restart|status|logs")
		return 1
	}
	home := os.Getenv("MENO_HOME")
	if home == "" {
		p, e := config.DefaultPaths()
		if e != nil {
			fmt.Fprintln(errOut, e)
			return 1
		}
		home = p.Home
	}
	switch args[0] {
	case "run":
		if len(args) > 1 {
			fmt.Fprintln(errOut, "meno: daemon run takes no arguments")
			return 1
		}
		return runDaemon(home, errOut)
	case "start":
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		if _, e := client.Ensure(ctx, client.Options{Home: home}); e != nil {
			fmt.Fprintln(errOut, "meno: daemon start:", e)
			return 1
		}
		fmt.Fprintln(out, "daemon running")
		return 0
	case "stop":
		return stopDaemon(home, out, errOut)
	case "restart":
		return restartDaemon(home, args, out, errOut)
	case "status":
		return daemonStatus(home, out)
	case "logs":
		return daemonLogs(home, args[1:], out, errOut)
	default:
		fmt.Fprintf(errOut, "meno: unknown daemon command %q\n", args[0])
		return 1
	}
}
func runDaemon(home string, errOut io.Writer) int {
	startupLock, err := daemon.StartupOwnership(home)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	if startupLock != nil {
		defer startupLock.Close()
	}
	backup, err := db.PrepareEpoch(home)
	if err != nil {
		fmt.Fprintln(errOut, "database reset:", err)
		return 1
	}
	if backup != "" {
		fmt.Fprintln(errOut, "meno 0.5.0 starts with a fresh database; previous data backed up to", backup)
	}
	service, e := cfgsvc.New(cfgsvc.Options{Paths: config.Paths{Home: home}})
	if e != nil {
		fmt.Fprintln(errOut, e)
		return 1
	}
	hub := daemon.NewHub("", 20000)
	global := daemon.NewGlobalHub("", 2000)
	eng, e := engine.New(engine.Options{Home: home, EventHub: hub, GlobalHub: global, Resources: service, Socket: daemon.SocketPath(home)})
	if e != nil {
		fmt.Fprintln(errOut, e)
		return 1
	}
	s, e := daemon.New(daemon.Options{StartupLock: startupLock, Home: home, Version: ver.Label(), Backend: eng, Hub: hub, Global: global, OnStart: service.Start, OnStop: func() { eng.Close(); service.Close() }})
	if e != nil {
		eng.Close()
		fmt.Fprintln(errOut, e)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	service.SetNotify(func(event string) { s.GlobalHub().Publish("", event, map[string]any{}) })
	if e = s.Serve(ctx); e != nil {
		fmt.Fprintln(errOut, "meno: daemon:", e)
		return 1
	}
	return 0
}
