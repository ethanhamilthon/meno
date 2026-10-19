package cli

import (
	"fmt"
	"io"
	"strings"
	"time"
)

func runAgents(args []string, streams IO, api API, jsonOut bool) (int, error) {
	if len(args) == 0 {
		return 0, usageError("usage: meno agents <spawn|list|status|send|wait|result|cancel|report|block>")
	}
	cmd := args[0]
	if !contains([]string{"spawn", "list", "status", "send", "wait", "result", "cancel", "report", "block"}, cmd) {
		return 0, unknown(cmd, []string{"spawn", "list", "status", "send", "wait", "result", "cancel", "report", "block"})
	}
	args = args[1:]
	switch cmd {
	case "report", "block":
		if err := apiRequired(api); err != nil {
			return 0, err
		}
		text := strings.Join(args, " ")
		if strings.TrimSpace(text) == "" {
			return 0, usageError("usage: meno agents " + cmd + " TEXT")
		}
		var err error
		if cmd == "report" {
			err = api.AgentReport(text)
		} else {
			err = api.AgentBlock(text)
		}
		if err != nil {
			return 0, err
		}
		return 0, emit(streams, jsonOut, map[string]string{"status": cmd, "text": text}, cmd+" sent\n")
	default:
		if err := apiRequired(api); err != nil {
			return 0, err
		}
	}
	switch cmd {
	case "spawn":
		values, _, pos, err := parseFlags(args, map[string]bool{"cwd": true, "provider": true, "model": true, "effort": true}, nil)
		if err != nil {
			return 0, err
		}
		task := strings.Join(pos, " ")
		if len(pos) == 1 && pos[0] == "-" || len(pos) == 0 {
			b, e := io.ReadAll(streams.In)
			if e != nil {
				return 0, e
			}
			task = string(b)
		}
		if strings.TrimSpace(task) == "" {
			return 0, usageError("usage: meno agents spawn [--cwd --provider --model --effort] TASK")
		}
		agent, err := api.AgentsSpawn(AgentSpawnRequest{Task: task, CWD: values["cwd"], Provider: values["provider"], Model: values["model"], Effort: values["effort"]})
		if err != nil {
			return 0, err
		}
		return 0, emit(streams, jsonOut, agent, agent.ID+"\n")
	case "list":
		if len(args) > 0 {
			return 0, usageError("usage: meno agents list")
		}
		items, err := api.AgentsList()
		if err != nil {
			return 0, err
		}
		return 0, emit(streams, jsonOut, items, formatList(items, func(x Agent) string { return x.ID + "\t" + x.Status + "\t" + x.Task }))
	case "status":
		if len(args) > 1 {
			return 0, usageError("usage: meno agents status [ID]")
		}
		if len(args) == 0 {
			items, err := api.AgentsList()
			if err != nil {
				return 0, err
			}
			return 0, emit(streams, jsonOut, items, formatList(items, func(x Agent) string { return x.ID + " " + x.Status }))
		}
		a, err := api.AgentStatus(args[0])
		if err != nil {
			return 0, err
		}
		return 0, emit(streams, jsonOut, a, a.ID+" "+a.Status+"\n")
	case "send":
		if len(args) < 2 {
			return 0, usageError("usage: meno agents send ID TEXT")
		}
		a, err := api.AgentSend(args[0], strings.Join(args[1:], " "))
		if err != nil {
			return 0, err
		}
		return 0, emit(streams, jsonOut, a, a.ID+" "+a.Status+"\n")
	case "wait":
		values, _, pos, err := parseFlags(args, map[string]bool{"timeout": true}, nil)
		if err != nil {
			return 0, err
		}
		if len(pos) > 1 {
			return 0, usageError("usage: meno agents wait [ID] [--timeout SECONDS]")
		}
		seconds, err := intFlag("timeout", values["timeout"], 0, 1)
		if err != nil {
			return 0, err
		}
		id := ""
		if len(pos) == 1 {
			id = pos[0]
		}
		res, err := api.AgentWait(id, time.Duration(seconds)*time.Second)
		if err != nil {
			return 0, err
		}
		if res.TimedOut {
			return ExitWaitTimeout, emit(streams, jsonOut, res, fmt.Sprintf("agent %s is still running\n", res.Agent.ID))
		}
		return 0, emit(streams, jsonOut, res, res.Agent.ID+" "+res.Agent.Status+"\n")
	case "result":
		if len(args) > 1 {
			return 0, usageError("usage: meno agents result [ID]")
		}
		id := ""
		if len(args) == 1 {
			id = args[0]
		}
		a, err := api.AgentResult(id)
		if err != nil {
			return 0, err
		}
		return 0, emit(streams, jsonOut, a, a.Result)
	case "cancel":
		if len(args) != 1 {
			return 0, usageError("usage: meno agents cancel ID")
		}
		a, err := api.AgentCancel(args[0])
		if err != nil {
			return 0, err
		}
		return 0, emit(streams, jsonOut, a, a.ID+" "+a.Status+"\n")
	default:
		return 0, unknown(cmd, []string{"spawn", "list", "status", "send", "wait", "result", "cancel", "report", "block"})
	}
}
