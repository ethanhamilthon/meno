package cli

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

	"meno/internal/api"
	"meno/internal/config"
	"meno/internal/version"

	_ "modernc.org/sqlite" // read-only integrity check of meno.db
)

// Doctor runs the read-only checks. Local checks (socket, files, database) work
// without a daemon; daemon checks are skipped with a clear finding when it is
// unreachable. Nothing here writes, logs in or restarts anything.
func (a *DaemonAPI) Doctor(r DoctorRequest) (DoctorReport, error) {
	var out []Check
	add := func(name, status, detail, fix string) {
		out = append(out, Check{Name: name, Status: status, Detail: detail, Fix: fix})
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
	out = append(out, skillChecks(a.home)...)
	out = append(out, databaseCheck(a.home))
	if up {
		out = append(out, a.daemonChecks(r.Probe)...)
	}
	return DoctorReport{Checks: out}, nil
}

func (a *DaemonAPI) daemonChecks(probe bool) []Check {
	var out []Check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h, err := a.c.Hello(ctx)
	if err != nil {
		return append(out, Check{Name: "daemon hello", Status: "error", Detail: err.Error(), Fix: "restart it with: meno daemon restart"})
	}
	switch {
	case h.Protocol != api.Protocol:
		out = append(out, Check{Name: "protocol", Status: "error", Detail: fmt.Sprintf("daemon speaks %d, this client %d", h.Protocol, api.Protocol), Fix: "restart the daemon after updating: meno daemon restart"})
	case strings.TrimPrefix(h.Version, "v") != version.Number():
		out = append(out, Check{Name: "version", Status: "warn", Detail: "daemon " + h.Version + ", client " + version.Number(), Fix: "meno daemon restart"})
	default:
		out = append(out, Check{Name: "version", Status: "ok", Detail: h.Version + ", protocol " + fmt.Sprint(h.Protocol)})
	}
	if ps, err := a.c.Providers(ctx); err != nil {
		out = append(out, Check{Name: "providers", Status: "error", Detail: err.Error()})
	} else {
		usable := 0
		for _, p := range ps.Providers {
			if !p.Disabled && (p.HasAPIKey || p.Run != "") {
				usable++
			}
		}
		if usable == 0 {
			out = append(out, Check{Name: "providers", Status: "warn", Detail: "no enabled provider has credentials", Fix: "add one with /login in the TUI"})
		} else {
			out = append(out, Check{Name: "providers", Status: "ok", Detail: fmt.Sprintf("%d usable of %d", usable, len(ps.Providers))})
		}
	}
	if ms, err := a.c.McpServers(ctx); err != nil {
		out = append(out, Check{Name: "mcp", Status: "error", Detail: err.Error()})
	} else {
		for _, s := range ms.Servers {
			switch {
			case s.Enabled && s.Error != "":
				out = append(out, Check{Name: "mcp " + s.Name, Status: "warn", Detail: oneLine(s.Error), Fix: "meno mcp restart " + s.Name})
			case s.Enabled && s.Starting:
				out = append(out, Check{Name: "mcp " + s.Name, Status: "warn", Detail: "still starting"})
			case s.Enabled && !s.Running:
				out = append(out, Check{Name: "mcp " + s.Name, Status: "warn", Detail: "enabled but not running", Fix: "meno mcp restart " + s.Name})
			default:
				out = append(out, Check{Name: "mcp " + s.Name, Status: "ok"})
			}
		}
		for _, w := range ms.Warnings {
			out = append(out, Check{Name: "mcp config", Status: "warn", Detail: oneLine(w)})
		}
	}
	if probe {
		out = append(out, a.probeModels(ctx)...)
	}
	return out
}

// probeModels lists the models of each enabled provider: one bounded request
// each, only when the user asked with --probe.
func (a *DaemonAPI) probeModels(ctx context.Context) []Check {
	var out []Check
	ps, err := a.c.Providers(ctx)
	if err != nil {
		return nil
	}
	for _, p := range ps.Providers {
		if p.Disabled {
			continue
		}
		pc, cancel := context.WithTimeout(ctx, 10*time.Second)
		var items ModelList
		err := a.resource(pc, "GET", "models", "", "provider="+p.Name, nil, &items)
		cancel()
		if err != nil {
			out = append(out, Check{Name: "probe " + p.Name, Status: "warn", Detail: oneLine(err.Error())})
		} else if msg := items.Errors[p.Name]; msg != "" {
			out = append(out, Check{Name: "probe " + p.Name, Status: "warn", Detail: oneLine(msg)})
		} else {
			out = append(out, Check{Name: "probe " + p.Name, Status: "ok", Detail: fmt.Sprintf("%d models", len(items.Models))})
		}
	}
	return out
}

// configChecks parses the JSON files without printing their contents.
func configChecks(home string) []Check {
	var out []Check
	for _, name := range []string{"settings.json", "provider.json", "mcp.json"} {
		b, err := os.ReadFile(filepath.Join(home, name))
		switch {
		case os.IsNotExist(err):
			continue
		case err != nil:
			out = append(out, Check{Name: name, Status: "error", Detail: "unreadable", Fix: "check the permissions of " + filepath.Join(home, name)})
		case len(b) > 0 && !json.Valid(b):
			out = append(out, Check{Name: name, Status: "error", Detail: "not valid JSON", Fix: "fix or remove " + filepath.Join(home, name)})
		default:
			out = append(out, Check{Name: name, Status: "ok"})
		}
	}
	return append(out, systemPromptCheck(home)...)
}

// systemPromptCheck reports a system_prompt setting that names a file that
// cannot be used: new sessions then fall back to the built-in prompt.
func systemPromptCheck(home string) []Check {
	b, err := os.ReadFile(filepath.Join(home, "settings.json"))
	if err != nil {
		return nil
	}
	var st struct {
		SystemPrompt string `json:"system_prompt"`
	}
	if json.Unmarshal(b, &st) != nil || st.SystemPrompt == "" || st.SystemPrompt == config.DefaultSystemPrompt {
		return nil
	}
	paths := config.Paths{Home: home}
	if _, err := paths.LoadSystemPrompt(st.SystemPrompt); err != nil {
		return []Check{{Name: "system prompt", Status: "warn", Detail: oneLine(err.Error()), Fix: "create " + filepath.Join(paths.SystemPromptsDir(), st.SystemPrompt+".md") + " or choose another prompt in the settings"}}
	}
	return []Check{{Name: "system prompt", Status: "ok", Detail: st.SystemPrompt}}
}

// databaseCheck opens meno.db read-only and runs SQLite's quick integrity check.
func databaseCheck(home string) Check {
	path := filepath.Join(home, "meno.db")
	if _, err := os.Stat(path); err != nil {
		return Check{Name: "database", Status: "ok", Detail: "not created yet"}
	}
	d, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(2000)")
	if err != nil {
		return Check{Name: "database", Status: "error", Detail: oneLine(err.Error())}
	}
	defer d.Close()
	var res string
	if err = d.QueryRow("PRAGMA quick_check").Scan(&res); err != nil {
		return Check{Name: "database", Status: "error", Detail: oneLine(err.Error()), Fix: "restore meno.db from a backup"}
	}
	if res != "ok" {
		return Check{Name: "database", Status: "error", Detail: oneLine(res), Fix: "restore meno.db from a backup"}
	}
	return Check{Name: "database", Status: "ok", Detail: "integrity ok"}
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
