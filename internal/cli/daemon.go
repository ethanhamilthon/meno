package cli

import (
	"encoding/json"
	"fmt"
	"strings"
)

func runDaemon(args []string, s IO, a API, j bool) (int, error) {
	if len(args) == 0 {
		return 0, usageError("usage: meno daemon run|start|stop|restart|status|logs")
	}
	if !contains([]string{"run", "start", "stop", "restart", "status", "logs"}, args[0]) {
		return 0, unknown(args[0], []string{"run", "start", "stop", "restart", "status", "logs"})
	}
	values, switches, pos, err := parseFlags(args[1:], nil, map[string]bool{"force": true})
	if err != nil {
		return 0, err
	}
	if len(pos) > 0 {
		return 0, usageError("unexpected daemon arguments")
	}
	action := args[0]
	if !contains([]string{"run", "start", "stop", "restart", "status", "logs"}, action) {
		return 0, unknown(action, []string{"run", "start", "stop", "restart", "status", "logs"})
	}
	if err = apiRequired(a); err != nil {
		return 0, err
	}
	_ = values
	res, err := a.Daemon(action, switches["force"])
	if err != nil {
		return 0, err
	}
	return 0, emit(s, j, res, daemonText(action, res))
}
func daemonText(action string, d DaemonStatus) string {
	if action == "status" {
		if d.Running {
			return fmt.Sprintf("running (pid %d)\n", d.PID)
		}
		return "stopped\n"
	}
	return action + " requested\n"
}
func runSessions(args []string, s IO, a API, j bool) (int, error) {
	values, _, pos, err := parseFlags(args, map[string]bool{"cwd": true}, nil)
	if err != nil {
		return 0, err
	}
	if len(pos) > 0 {
		return 0, usageError("usage: meno sessions [--cwd DIR]")
	}
	if err = apiRequired(a); err != nil {
		return 0, err
	}
	items, err := a.Sessions(values["cwd"])
	if err != nil {
		return 0, err
	}
	return 0, emit(s, j, items, formatList(items, func(x Session) string { return x.ID + "\t" + x.Status + "\t" + x.Name + "\t" + x.CWD }))
}
func runSend(args []string, s IO, a API, j bool) (int, error) {
	values, switches, pos, err := parseFlags(args, map[string]bool{"s": true}, map[string]bool{"new": true, "detach": true})
	if err != nil {
		return 0, err
	}
	if len(pos) == 0 {
		return 0, usageError("usage: meno send [-s ID|--new] [--detach] TEXT")
	}
	if switches["new"] && values["s"] != "" {
		return 0, usageError("--new cannot be combined with -s")
	}
	if err = apiRequired(a); err != nil {
		return 0, err
	}
	res, err := a.Send(SendRequest{Text: strings.Join(pos, " "), SessionID: values["s"], New: switches["new"], Detach: switches["detach"]})
	if err != nil {
		return 0, err
	}
	return 0, emit(s, j, res, res.Text)
}
func runTail(args []string, s IO, a API, j bool) (int, error) {
	if len(args) != 1 {
		return 0, usageError("usage: meno tail ID")
	}
	if err := apiRequired(a); err != nil {
		return 0, err
	}
	events, err := a.Tail(args[0])
	if err != nil {
		return 0, err
	}
	if j {
		return 0, emit(s, true, events, "")
	}
	for _, e := range events {
		b, _ := json.Marshal(e)
		fmt.Fprintln(s.Out, string(b))
	}
	return 0, nil
}
