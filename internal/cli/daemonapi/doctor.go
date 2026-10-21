package daemonapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"meno/internal/cli"
	"meno/internal/proto"
	"meno/internal/version"

	_ "modernc.org/sqlite" // read-only integrity check of meno.db
)

// Doctor runs the read-only checks. Local checks (socket, files, database) work
// without a daemon; daemon checks are skipped with a clear finding when it is
// unreachable. Nothing here writes, logs in or restarts anything.
func (a *API) Doctor(r cli.DoctorRequest) (cli.DoctorReport, error) {
	var out []cli.Check
	add := func(name, status, detail, fix string) {
		out = append(out, cli.Check{Name: name, Status: status, Detail: detail, Fix: fix})
	}
	sock := filepath.Join(a.home, "run", "meno.sock")
	up := false
	if c, err := net.DialTimeout("unix", sock, 2*time.Second); err != nil {
		add("daemon socket", "error", "cannot connect to "+sock, "start it with: meno daemon start")
	} else {
		c.Close()
		up = true
		add("daemon socket", "ok", sock, "")
	}
	out = append(out, configChecks(a.home)...)
	out = append(out, databaseCheck(a.home))
	if up {
		out = append(out, a.daemonChecks(r.Probe)...)
	}
	return cli.DoctorReport{Checks: out}, nil
}

func (a *API) daemonChecks(probe bool) []cli.Check {
	var out []cli.Check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h, err := a.c.Hello(ctx)
	if err != nil {
		return append(out, cli.Check{Name: "daemon hello", Status: "error", Detail: err.Error(), Fix: "restart it with: meno daemon restart"})
	}
	switch {
	case h.Protocol != proto.Protocol:
		out = append(out, cli.Check{Name: "protocol", Status: "error", Detail: fmt.Sprintf("daemon speaks %d, this client %d", h.Protocol, proto.Protocol), Fix: "restart the daemon after updating: meno daemon restart"})
	case strings.TrimPrefix(h.Version, "v") != version.Number():
		out = append(out, cli.Check{Name: "version", Status: "warn", Detail: "daemon " + h.Version + ", client " + version.Number(), Fix: "meno daemon restart"})
	default:
		out = append(out, cli.Check{Name: "version", Status: "ok", Detail: h.Version + ", protocol " + fmt.Sprint(h.Protocol)})
	}
	if ps, err := a.c.Providers(ctx); err != nil {
		out = append(out, cli.Check{Name: "providers", Status: "error", Detail: err.Error()})
	} else {
		usable := 0
		for _, p := range ps.Providers {
			if !p.Disabled && (p.HasAPIKey || p.Run != "" || p.Source == "oauth") {
				usable++
			}
		}
		if usable == 0 {
			out = append(out, cli.Check{Name: "providers", Status: "warn", Detail: "no enabled provider has credentials", Fix: "add one with /login in the TUI"})
		} else {
			out = append(out, cli.Check{Name: "providers", Status: "ok", Detail: fmt.Sprintf("%d usable of %d", usable, len(ps.Providers))})
		}
	}
	if ms, err := a.c.McpServers(ctx); err != nil {
		out = append(out, cli.Check{Name: "mcp", Status: "error", Detail: err.Error()})
	} else {
		for _, s := range ms.Servers {
			switch {
			case s.Enabled && s.Error != "":
				out = append(out, cli.Check{Name: "mcp " + s.Name, Status: "warn", Detail: oneLine(s.Error), Fix: "meno mcp restart " + s.Name})
			case s.Enabled && s.Starting:
				out = append(out, cli.Check{Name: "mcp " + s.Name, Status: "warn", Detail: "still starting"})
			case s.Enabled && !s.Running:
				out = append(out, cli.Check{Name: "mcp " + s.Name, Status: "warn", Detail: "enabled but not running", Fix: "meno mcp restart " + s.Name})
			default:
				out = append(out, cli.Check{Name: "mcp " + s.Name, Status: "ok"})
			}
		}
		for _, w := range ms.Warnings {
			out = append(out, cli.Check{Name: "mcp config", Status: "warn", Detail: oneLine(w)})
		}
	}
	if probe {
		out = append(out, a.probeModels(ctx)...)
	}
	return out
}

// probeModels lists the models of each enabled provider: one bounded request
// each, only when the user asked with --probe.
func (a *API) probeModels(ctx context.Context) []cli.Check {
	var out []cli.Check
	ps, err := a.c.Providers(ctx)
	if err != nil {
		return nil
	}
	for _, p := range ps.Providers {
		if p.Disabled {
			continue
		}
		pc, cancel := context.WithTimeout(ctx, 10*time.Second)
		var items []cli.Model
		err := a.resource(pc, "GET", "models", "", "provider="+p.Name, nil, &items)
		cancel()
		if err != nil {
			out = append(out, cli.Check{Name: "probe " + p.Name, Status: "warn", Detail: oneLine(err.Error())})
		} else {
			out = append(out, cli.Check{Name: "probe " + p.Name, Status: "ok", Detail: fmt.Sprintf("%d models", len(items))})
		}
	}
	return out
}

// configChecks parses the JSON files without printing their contents.
func configChecks(home string) []cli.Check {
	var out []cli.Check
	for _, name := range []string{"settings.json", "provider.json", "mcp.json"} {
		b, err := os.ReadFile(filepath.Join(home, name))
		switch {
		case os.IsNotExist(err):
			continue
		case err != nil:
			out = append(out, cli.Check{Name: name, Status: "error", Detail: "unreadable", Fix: "check the permissions of " + filepath.Join(home, name)})
		case len(b) > 0 && !json.Valid(b):
			out = append(out, cli.Check{Name: name, Status: "error", Detail: "not valid JSON", Fix: "fix or remove " + filepath.Join(home, name)})
		default:
			out = append(out, cli.Check{Name: name, Status: "ok"})
		}
	}
	return out
}

// databaseCheck opens meno.db read-only and runs SQLite's quick integrity check.
func databaseCheck(home string) cli.Check {
	path := filepath.Join(home, "meno.db")
	if _, err := os.Stat(path); err != nil {
		return cli.Check{Name: "database", Status: "ok", Detail: "not created yet"}
	}
	d, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(2000)")
	if err != nil {
		return cli.Check{Name: "database", Status: "error", Detail: oneLine(err.Error())}
	}
	defer d.Close()
	var res string
	if err = d.QueryRow("PRAGMA quick_check").Scan(&res); err != nil {
		return cli.Check{Name: "database", Status: "error", Detail: oneLine(err.Error()), Fix: "restore meno.db from a backup"}
	}
	if res != "ok" {
		return cli.Check{Name: "database", Status: "error", Detail: oneLine(res), Fix: "restore meno.db from a backup"}
	}
	return cli.Check{Name: "database", Status: "ok", Detail: "integrity ok"}
}

func oneLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
