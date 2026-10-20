package cli

import (
	"fmt"
	"strings"
	"time"
)

func runTasks(args []string, streams IO, api API, jsonOut bool) (int, error) {
	if len(args) == 0 {
		return 0, usageError("usage: meno tasks <start|list|status|logs|wait|cancel>")
	}
	cmd := args[0]
	if !contains([]string{"start", "list", "status", "logs", "wait", "cancel"}, cmd) {
		return 0, unknown(cmd, []string{"start", "list", "status", "logs", "wait", "cancel"})
	}
	if err := apiRequired(api); err != nil {
		return 0, err
	}
	args = args[1:]
	switch cmd {
	case "start":
		values, switches, pos, err := parseFlags(args, map[string]bool{"cwd": true, "env": true, "name": true, "timeout": true}, map[string]bool{"no-notify": true})
		if err != nil {
			return 0, err
		}
		if len(pos) == 0 {
			return 0, usageError("usage: meno tasks start CMD [--cwd DIR --env K=V --name NAME --no-notify]")
		}
		if _, ok := values["timeout"]; ok {
			return 0, usageError("tasks start has no --timeout: a task runs until it ends or you cancel it (meno tasks cancel ID)")
		}
		env := map[string]string{}
		for _, pair := range strings.Split(values["env"], "\x00") {
			if pair == "" {
				continue
			}
			k, v, ok := strings.Cut(pair, "=")
			if !ok || k == "" {
				return 0, usageError("--env must be K=V")
			}
			env[k] = v
		}
		task, err := api.TasksStart(TaskStartRequest{Command: strings.Join(pos, " "), CWD: values["cwd"], Name: values["name"], Env: env, Notify: !switches["no-notify"]})
		if err != nil {
			return 0, err
		}
		return 0, emit(streams, jsonOut, task, task.ID+"\n")
	case "list":
		include := false
		for _, a := range args {
			if a == "--all" {
				include = true
			} else if a == "--finished" {
				include = true
			} else {
				return 0, unknown(a, []string{"--all", "--finished"})
			}
		}
		items, err := api.TasksList(include)
		if err != nil {
			return 0, err
		}
		return 0, emit(streams, jsonOut, items, formatList(items, func(x Task) string { return x.ID + "\t" + x.Status + "\t" + x.Name }))
	case "status":
		if len(args) != 1 {
			return 0, usageError("usage: meno tasks status ID")
		}
		task, err := api.TaskStatus(args[0])
		if err != nil {
			return 0, err
		}
		return 0, emit(streams, jsonOut, task, task.ID+" "+task.Status+"\n")
	case "logs":
		values, _, pos, err := parseFlags(args, map[string]bool{"from": true, "limit": true}, nil)
		if err != nil {
			return 0, err
		}
		if len(pos) != 1 {
			return 0, usageError("usage: meno tasks logs ID [--from N --limit N]")
		}
		from, err := intFlag("from", values["from"], 0, 0)
		if err != nil {
			return 0, err
		}
		limit, err := intFlag("limit", values["limit"], 200, 1)
		if err != nil {
			return 0, err
		}
		logs, err := api.TaskLogs(pos[0], from, limit)
		if err != nil {
			return 0, err
		}
		var b strings.Builder
		for _, r := range logs.Records {
			b.WriteString(r.Text)
		}
		return 0, emit(streams, jsonOut, logs, b.String())
	case "wait":
		values, _, pos, err := parseFlags(args, map[string]bool{"timeout": true}, nil)
		if err != nil {
			return 0, err
		}
		if len(pos) != 1 {
			return 0, usageError("usage: meno tasks wait ID [--timeout SECONDS]")
		}
		seconds, err := intFlag("timeout", values["timeout"], 0, 1)
		if err != nil {
			return 0, err
		}
		res, err := api.TaskWait(pos[0], time.Duration(seconds)*time.Second)
		if err != nil {
			return 0, err
		}
		if res.TimedOut {
			return ExitWaitTimeout, emit(streams, jsonOut, res, fmt.Sprintf("task %s is still running\n", res.Task.ID))
		}
		return 0, emit(streams, jsonOut, res, res.Task.ID+" "+res.Task.Status+"\n")
	case "cancel":
		if len(args) != 1 {
			return 0, usageError("usage: meno tasks cancel ID")
		}
		task, err := api.TaskCancel(args[0])
		if err != nil {
			return 0, err
		}
		return 0, emit(streams, jsonOut, task, task.ID+" "+task.Status+"\n")
	default:
		return 0, unknown(cmd, []string{"start", "list", "status", "logs", "wait", "cancel"})
	}
}
