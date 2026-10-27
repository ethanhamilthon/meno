package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"meno/internal/config"

	_ "modernc.org/sqlite" // read-only integrity check of meno.db
)

// Doctor runs the read-only checks of the configuration files, the skills, the
// database and, when there is an engine, of the providers and MCP servers.
// Nothing here writes, logs in or restarts anything.
func (a *OwnerAPI) Doctor(r DoctorRequest) (DoctorReport, error) {
	var out []Check
	out = append(out, configChecks(a.home)...)
	out = append(out, skillChecks(a.home)...)
	out = append(out, databaseCheck(a.home))
	if a.c != nil {
		out = append(out, a.engineChecks(r.Probe)...)
	}
	return DoctorReport{Checks: out}, nil
}

func (a *OwnerAPI) engineChecks(probe bool) []Check {
	var out []Check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
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
func (a *OwnerAPI) probeModels(ctx context.Context) []Check {
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
