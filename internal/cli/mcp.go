package cli

import (
	"strings"
)

func runMCP(args []string, streams IO, api API, jsonOut bool) (int, error) {
	if len(args) == 0 {
		return 0, usageError("usage: meno mcp <servers|tools|call|results|add|remove|restart>")
	}
	cmd := args[0]
	if !contains([]string{"servers", "tools", "call", "results", "add", "remove", "restart"}, cmd) {
		return 0, unknown(cmd, []string{"servers", "tools", "call", "results", "add", "remove", "restart"})
	}
	if err := apiRequired(api); err != nil {
		return 0, err
	}
	args = args[1:]
	switch cmd {
	case "servers":
		if len(args) > 0 {
			return 0, usageError("usage: meno mcp servers")
		}
		items, err := api.MCPServers()
		if err != nil {
			return 0, err
		}
		return 0, emit(streams, jsonOut, items, formatList(items, func(x Server) string { return x.Name + statusSuffix(x.Status) }))
	case "tools":
		return mcpTools(args, streams, api, jsonOut)
	case "call":
		return mcpCall(args, streams, api, jsonOut)
	case "results":
		values, _, pos, err := parseFlags(args, map[string]bool{"offset": true, "limit": true}, nil)
		if err != nil {
			return 0, err
		}
		if len(pos) != 1 {
			return 0, usageError("usage: meno mcp results ID [--offset N --limit N]")
		}
		offset, err := intFlag("offset", values["offset"], 0, 0)
		if err != nil {
			return 0, err
		}
		limit, err := intFlag("limit", values["limit"], 0, 0)
		if err != nil {
			return 0, err
		}
		res, err := api.MCPResults(pos[0], offset, limit)
		if err != nil {
			return 0, err
		}
		return 0, emit(streams, jsonOut, res, res.Text)
	case "add", "remove", "restart":
		if len(args) != 1 {
			return 0, usageError("usage: meno mcp " + cmd + " SERVER (only when the user asks)")
		}
		if cmd != "add" {
			if err := checkServer(api, args[0]); err != nil {
				return 0, err
			}
		}
		res, err := api.MCPAdmin(cmd, args[0])
		if err != nil {
			return 0, err
		}
		return 0, emit(streams, jsonOut, res, res.Name+statusSuffix(res.Status)+"\n")
	default:
		return 0, unknown(cmd, []string{"servers", "tools", "call", "results", "add", "remove", "restart"})
	}
}

func mcpCall(args []string, streams IO, api API, jsonOut bool) (int, error) {
	if len(args) < 1 {
		return 0, usageError("usage: meno mcp call SERVER TOOL [ARGS]")
	}
	server, tool := args[0], ""
	shortForm := strings.Contains(args[0], ".") && (len(args) == 1 || args[1] == "-" || strings.HasPrefix(strings.TrimSpace(args[1]), "{") || strings.Contains(args[1], "="))
	if !shortForm && len(args) == 1 {
		if !strings.Contains(args[0], ".") {
			return 0, usageError("usage: meno mcp call SERVER TOOL [ARGS]")
		}
	}
	if !shortForm && len(args) > 1 {
		tool = args[1]
	}
	if shortForm {
		parts := strings.SplitN(server, ".", 2)
		if len(parts) != 2 {
			return 0, usageError("usage: meno mcp call SERVER TOOL [ARGS]")
		}
		server, tool = parts[0], parts[1]
		args = args[1:]
	} else {
		args = args[2:]
	}
	if err := checkServer(api, server); err != nil {
		return 0, err
	}
	tools, err := api.MCPTools(server)
	if err != nil {
		return 0, err
	}
	if len(tools) > 0 {
		found := false
		names := make([]string, 0, len(tools))
		for _, t := range tools {
			names = append(names, t.Name)
			if t.Name == tool {
				found = true
			}
		}
		if !found {
			return 0, unknown(tool, names)
		}
	}
	params, err := parseObject(args)
	if err != nil {
		return 0, err
	}
	res, err := api.MCPCall(server, tool, params)
	if err != nil {
		return 0, err
	}
	return 0, emit(streams, true, res, "")
}

func checkServer(api API, name string) error {
	servers, err := api.MCPServers()
	if err != nil {
		return err
	}
	names := make([]string, 0, len(servers))
	for _, s := range servers {
		names = append(names, s.Name)
		if s.Name == name {
			return nil
		}
	}
	return unknown(name, names)
}
func statusSuffix(status string) string {
	if status == "" {
		return ""
	}
	return " (" + status + ")"
}
func descriptionSuffix(s string) string {
	if s == "" {
		return ""
	}
	return " — " + s
}
